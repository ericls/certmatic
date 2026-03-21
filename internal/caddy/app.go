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
	"github.com/ericls/certmatic/internal/portal"
	internal_domain "github.com/ericls/certmatic/internal/repo/domain"
	"github.com/ericls/certmatic/pkg/domain"
	"go.uber.org/zap"
)

var usagePool = caddy.NewUsagePool()

func init() {
	caddy.RegisterModule(App{})
	httpcaddyfile.RegisterGlobalOption("certmatic", parseGlobalCertmatic)
}

type App struct {
	DomainStore         config.Store      `json:"domain_store,omitempty"`
	ChallengeType       dns.ChallengeType `json:"challenge_type,omitempty"`
	DNSDelegationDomain string            `json:"dns_delegation_domain,omitempty"`
	CNameTarget         string            `json:"cname_target,omitempty"`
	PortalSigningKey    string            `json:"portal_signing_key,omitempty"`
	PortalBaseURL       string            `json:"portal_base_url,omitempty"`
	PortalDevMode       bool              `json:"portal_dev_mode,omitempty"`

	logger           zap.Logger            `json:"-"`
	config           config.Config         `json:"-"`
	domainRepo       domain.DomainRepo     `json:"-"`
	dnsRecordManager *dns.DNSRecordManager `json:"-"`
	sessionStore     portal.SessionStore   `json:"-"`
	signingKeyBytes  []byte                `json:"-"`
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
	return nil
}

// Provision implements caddy.Provisioner.
func (a *App) Provision(ctx caddy.Context) error {
	a.logger = *ctx.Logger(a)
	a.logger.Debug("provisioning certmatic app")

	domainRepo, _, err := usagePool.LoadOrNew("domainRepo", func() (caddy.Destructor, error) {
		return internal_domain.NewDomainStoreFromConfig(a.DomainStore)
	})
	if err != nil {
		a.logger.Error("failed to create or load domain store from config", zap.Error(err))
		return err
	}
	a.domainRepo = domainRepo.(domain.DomainRepo)

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

	// Portal session store (survives hot-reloads via usagePool)
	sessionStoreVal, _, err := usagePool.LoadOrNew("portalSessionStore", func() (caddy.Destructor, error) {
		return portal.NewMemorySessionStore(), nil
	})
	if err != nil {
		return fmt.Errorf("create portal session store: %w", err)
	}
	a.sessionStore = sessionStoreVal.(portal.SessionStore)

	return nil
}

func (a *App) Handle(ctx context.Context, event caddy.Event) error {
	a.logger.Info("received event", zap.String("event", event.Name()), zap.String("origin", string(event.Origin().CaddyModule().ID)))
	return nil
}

var (
	_ caddy.App             = (*App)(nil)
	_ caddy.Provisioner     = (*App)(nil)
	_ caddyfile.Unmarshaler = (*App)(nil)
	_ caddyevents.Handler   = (*App)(nil)
)
