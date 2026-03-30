package caddy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

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

type App struct {
	DomainStore         config.Store             `json:"domain_store,omitempty"`
	SessionStore        config.Store             `json:"session_store,omitempty"`
	ChallengeType       dns.ChallengeType        `json:"challenge_type,omitempty"`
	DNSDelegationDomain string                   `json:"dns_delegation_domain,omitempty"`
	CNameTarget         string                   `json:"cname_target,omitempty"`
	PortalSigningKey    string                   `json:"portal_signing_key,omitempty"`
	PortalBaseURL       string                   `json:"portal_base_url,omitempty"`
	PortalAssetsDir     string                   `json:"portal_assets_dir,omitempty"`
	WebhookDispatcher   webhook.DispatcherConfig `json:"webhook_dispatcher,omitempty"`

	logger            zap.Logger              `json:"-"`
	config            config.Config           `json:"-"`
	domainRepo        domain.DomainRepo       `json:"-"`
	dnsRecordManager  *dns.DNSRecordManager   `json:"-"`
	sessionStore      pkgsession.SessionStore `json:"-"`
	signingKeyBytes   []byte                  `json:"-"`
	webhookDispatcher webhook.Dispatcher      `json:"-"`
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
	return nil
}

func (a *App) Stop() error {
	if d, ok := a.webhookDispatcher.(interface{ Destruct() error }); ok {
		d.Destruct()
	}
	return nil
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

	dnsRecordManager := dns.NewDNSRecordManager(a.ChallengeType, a.DNSDelegationDomain, a.CNameTarget, dns.NetLookup())
	a.dnsRecordManager = dnsRecordManager

	// Portal signing key
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
			a.WebhookDispatcher.URLs,
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
