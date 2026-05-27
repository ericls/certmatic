package caddy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyevents"
	"github.com/ericls/certmatic/internal/config"
	"github.com/ericls/certmatic/internal/dns"
	internal_domain "github.com/ericls/certmatic/internal/repo/domain"
	reporqlite "github.com/ericls/certmatic/internal/repo/rqlite"
	reposession "github.com/ericls/certmatic/internal/repo/session"
	"github.com/ericls/certmatic/internal/repo/sqlite"
	internalwebhook "github.com/ericls/certmatic/internal/webhook"
	"github.com/ericls/certmatic/pkg/domain"
	pkgsession "github.com/ericls/certmatic/pkg/session"
	"github.com/ericls/certmatic/pkg/webhook"
	"go.uber.org/zap"
)

var usagePool = caddy.NewUsagePool()

func init() {
	caddy.RegisterModule(App{})
	httpcaddyfile.RegisterGlobalOption("certmatic", parseGlobalCertmatic)
}

// Certmatic Caddy app. It provides a managed experience for SaaS
// platforms to provide custom domain support. It also includes a
// end-user portal that guides them through the process of
// configuring their DNS and obtaining TLS certificates for their domains.
//
// It is configured under the top-level "certmatic" app key in JSON,
// or via the `certmatic { ... }` global option in the Caddyfile. The
// HTTP handlers in this module ([AdminHandler], [AskHandler],
// [PortalHandler]) all read shared state from this app.
type App struct {
	// DomainStore configures persistence for tenant domain records.
	// Supported types are "memory" (default), "sqlite", and "rqlite".
	// The "config" sub-object holds type-specific settings (e.g.
	// "file_path" for sqlite, "http_addr" for rqlite).
	DomainStore config.Store `json:"domain_store"`

	// SessionStore configures persistence for portal authenticated sessions.
	// Accepts the same store types as DomainStore.
	SessionStore config.Store `json:"session_store"`

	// ChallengeType selects the ACME challenge used when issuing
	// certificates. One of "http-01" or "dns-01". When set to
	// "dns-01", DNSDelegationDomain and CNameTarget control the
	// CNAME-delegation flow so tenants don't need to grant DNS
	// credentials.
	ChallengeType dns.ChallengeType `json:"challenge_type,omitempty"`

	// DNSDelegationDomain is the parent zone Certmatic controls and
	// writes _acme-challenge records into when using DNS-01
	// delegation (e.g. "acme.example-saas.com"). Tenants CNAME their own
	// _acme-challenge.<domain> records to a subdomain of this zone.
	DNSDelegationDomain string `json:"dns_delegation_domain,omitempty"`

	// CNameTarget is the hostname tenants are instructed to CNAME
	// their domains to so traffic reaches this Caddy instance
	// (e.g. "edge.example-saas.com"). Surfaced in the portal UI.
	CNameTarget string `json:"cname_target,omitempty"`

	// PortalSigningKey is the secret used to sign portal session
	// tokens. Must be a stable, sufficiently random string; rotating
	// it invalidates all existing portal sessions. Supports Caddy
	// replacer placeholders (e.g. "{env.CERTMATIC_PORTAL_KEY}").
	PortalSigningKey string `json:"portal_signing_key,omitempty"`

	// PortalBaseURL is the externally reachable base URL of the
	// portal (e.g. "https://certmatic.example-saas.com"). Used to build
	// absolute links in emails and redirects.
	PortalBaseURL string `json:"portal_base_url,omitempty"`

	// PortalAssetsDir, if set, serves the portal UI from this
	// filesystem directory instead of the binary's embedded assets.
	PortalAssetsDir string `json:"portal_assets_dir,omitempty"`

	// WebhookDispatcher configures outbound webhooks fired on
	// lifecycle events (domain added, certificate issued/renewed,
	// etc.). See [webhook.DispatcherConfig] for endpoint settings.
	WebhookDispatcher webhook.DispatcherConfig `json:"webhook_dispatcher"`

	// DNSNameserver overrides the resolver used for DNS lookups
	// performed during domain validation (e.g. "1.1.1.1:53"). When
	// empty, the system resolver is used.
	DNSNameserver string `json:"dns_nameserver,omitempty"`

	logger            zap.Logger              `json:"-"`
	config            config.Config           `json:"-"`
	domainRepo        domain.DomainRepo       `json:"-"`
	dnsRecordManager  *dns.DNSRecordManager   `json:"-"`
	lookup            dns.Lookup              `json:"-"`
	sessionStore      pkgsession.SessionStore `json:"-"`
	signingKeyBytes   []byte                  `json:"-"`
	webhookDispatcher webhook.Dispatcher      `json:"-"`
	cancelCleanup     context.CancelFunc      `json:"-"`
}

func (App) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "certmatic",
		New: func() caddy.Module { return new(App) },
	}
}

func (a *App) Start() error {
	a.logger.Debug(
		"certmatic app started with",
		zap.String("store_type", a.DomainStore.Type),
		zap.String("challenge_type", string(a.ChallengeType)),
		zap.String("dns_delegation_domain", a.DNSDelegationDomain),
		zap.String("cname_target", a.CNameTarget),
	)

	ctx, cancel := context.WithCancel(context.Background())
	a.cancelCleanup = cancel
	go a.sessionCleanupLoop(ctx)

	return nil
}

func (a *App) sessionCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.sessionStore.ClearExpired(); err != nil {
				a.logger.Error("failed to clear expired sessions", zap.Error(err))
			} else {
				a.logger.Debug("cleared expired sessions")
			}
		}
	}
}

func (a *App) Stop() error {
	if a.cancelCleanup != nil {
		a.cancelCleanup()
	}
	if d, ok := a.webhookDispatcher.(interface{ Destruct() error }); ok {
		d.Destruct()
	}
	return nil
}

func replaceStoreConfig(repl *caddy.Replacer, s *config.Store) {
	for k, v := range s.Config {
		if str, ok := v.(string); ok {
			s.Config[k] = repl.ReplaceAll(str, "")
		}
	}
}

func loadFromPool(
	pool *caddy.UsagePool,
	conf config.Store,
	keyPrefix string,
	sqliteCtor func(string) (caddy.Destructor, error),
	rqliteCtor func(string) (caddy.Destructor, error),
	memoryCtor func() (caddy.Destructor, error),
) (caddy.Destructor, error) {
	switch conf.GetStoreType() {
	case config.StorageTypeSqlite:
		sqliteCfg, err := config.AsSqliteStorageConfig(conf.Config)
		if err != nil {
			return nil, fmt.Errorf("parse %s sqlite config: %w", keyPrefix, err)
		}
		val, _, err := pool.LoadOrNew(keyPrefix+":sqlite:"+sqliteCfg.FilePath, func() (caddy.Destructor, error) {
			return sqliteCtor(sqliteCfg.FilePath)
		})
		if err != nil {
			return nil, err
		}
		return val.(caddy.Destructor), nil
	case config.StorageTypeRqlite:
		rqliteCfg, err := config.AsRqliteStorageConfig(conf.Config)
		if err != nil {
			return nil, fmt.Errorf("parse %s rqlite config: %w", keyPrefix, err)
		}
		val, _, err := pool.LoadOrNew(keyPrefix+":rqlite:"+rqliteCfg.HTTPAddr, func() (caddy.Destructor, error) {
			return rqliteCtor(rqliteCfg.HTTPAddr)
		})
		if err != nil {
			return nil, err
		}
		return val.(caddy.Destructor), nil
	default:
		val, _, err := pool.LoadOrNew(keyPrefix+":memory", memoryCtor)
		if err != nil {
			return nil, err
		}
		return val.(caddy.Destructor), nil
	}
}

// Provision implements caddy.Provisioner.
func (a *App) Provision(ctx caddy.Context) error {
	a.logger = *ctx.Logger(a)
	a.logger.Debug("provisioning certmatic app")

	repl := caddy.NewReplacer()
	a.DNSDelegationDomain = repl.ReplaceAll(a.DNSDelegationDomain, "")
	a.CNameTarget = repl.ReplaceAll(a.CNameTarget, "")
	a.PortalSigningKey = repl.ReplaceAll(a.PortalSigningKey, "")
	a.PortalBaseURL = repl.ReplaceAll(a.PortalBaseURL, "")
	a.PortalAssetsDir = repl.ReplaceAll(a.PortalAssetsDir, "")
	a.DNSNameserver = repl.ReplaceAll(a.DNSNameserver, "")
	replaceStoreConfig(repl, &a.DomainStore)
	replaceStoreConfig(repl, &a.SessionStore)
	for i := range a.WebhookDispatcher.Endpoints {
		ep := &a.WebhookDispatcher.Endpoints[i]
		ep.URL = repl.ReplaceAll(ep.URL, "")
		ep.SigningKey = repl.ReplaceAll(ep.SigningKey, "")
	}

	// --- Domain repo ---
	val, err := loadFromPool(usagePool, a.DomainStore, "domainRepo",
		func(fp string) (caddy.Destructor, error) { return sqlite.NewDomainStore(fp) },
		func(addr string) (caddy.Destructor, error) {
			return reporqlite.NewDomainStore(addr)
		},
		func() (caddy.Destructor, error) {
			return internal_domain.NewInMemoryDomainRepo("inmemory"), nil
		},
	)
	if err != nil {
		return fmt.Errorf("domain store: %w", err)
	}
	a.domainRepo = val.(domain.DomainRepo)

	// --- Session store ---
	val, err = loadFromPool(usagePool, a.SessionStore, "sessionStore",
		func(fp string) (caddy.Destructor, error) { return sqlite.NewSessionStore(fp) },
		func(addr string) (caddy.Destructor, error) {
			return reporqlite.NewSessionStore(addr)
		},
		func() (caddy.Destructor, error) { return reposession.NewMemorySessionStore(), nil },
	)
	if err != nil {
		return fmt.Errorf("session store: %w", err)
	}
	a.sessionStore = val.(pkgsession.SessionStore)

	var lookup dns.Lookup
	if a.DNSNameserver != "" {
		lookup = dns.DirectUDPLookup(a.DNSNameserver)
	} else {
		lookup = dns.NetLookup()
	}
	a.dnsRecordManager = dns.NewDNSRecordManager(a.ChallengeType, a.DNSDelegationDomain, a.CNameTarget, lookup)
	a.lookup = lookup

	// Portal signing key
	if a.PortalSigningKey == "" {
		a.PortalSigningKey = os.Getenv("CERTMATIC_PORTAL_SIGNING_KEY")
	}
	if a.PortalSigningKey != "" {
		key, err := hex.DecodeString(a.PortalSigningKey)
		if err != nil {
			return fmt.Errorf("portal_signing_key must be a hex-encoded byte string (got decode error: %w)", err)
		}
		if len(key) < 16 {
			return fmt.Errorf("portal_signing_key must be at least 16 bytes (got %d)", len(key))
		}
		a.signingKeyBytes = key
	} else {
		a.logger.Warn("portal_signing_key not set; using ephemeral random key — portal tokens will not survive restarts")
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return fmt.Errorf("generate ephemeral portal signing key: %w", err)
		}
		a.signingKeyBytes = key
	}

	// --- Webhook dispatcher ---
	switch a.WebhookDispatcher.Type {
	case "memory":
		a.webhookDispatcher = internalwebhook.NewMemoryDispatcher(
			a.WebhookDispatcher.Endpoints,
			ctx.Logger(a).Named("webhook"),
		)
	default:
		a.webhookDispatcher = webhook.NoopDispatcher{}
	}

	return nil
}

func (a *App) WebhookDispatcherInstance() webhook.Dispatcher {
	return a.webhookDispatcher
}

func (a *App) Handle(ctx context.Context, event caddy.Event) error {
	a.logger.Info("received event", zap.String("event", event.Name()),
		zap.String("origin", string(event.Origin().CaddyModule().ID)))
	return nil
}

var (
	_ caddy.App             = (*App)(nil)
	_ caddy.Provisioner     = (*App)(nil)
	_ caddyfile.Unmarshaler = (*App)(nil)
	_ caddyevents.Handler   = (*App)(nil)
)
