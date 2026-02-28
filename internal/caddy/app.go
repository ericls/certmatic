package caddy

import (
	"context"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyevents"
	"github.com/ericls/certmatic/internal/config"
	"github.com/ericls/certmatic/internal/dns"
	internal_domain "github.com/ericls/certmatic/internal/repo/domain"
	"github.com/ericls/certmatic/pkg/domain"
	"go.uber.org/zap"
)

// type FooHandler struct {
// 	Bar string `json:"bar,omitempty"`
// }

// func (FooHandler) CaddyModule() caddy.ModuleInfo {
// 	return caddy.ModuleInfo{
// 		ID:  "http.handlers.certmatic_foo",
// 		New: func() caddy.Module { return new(FooHandler) },
// 	}
// }

// func (h *FooHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
// 	w.WriteHeader(http.StatusOK)
// 	w.Header().Set("Content-Type", "text/plain")
// 	w.Header().Set("X-Foo", "Bar")
// 	w.Write([]byte("FooHandler: " + h.Bar + "\n"))
// 	return nil
// }

// func parseCaddyfileAsk(d httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
// 	h := &FooHandler{
// 		Bar: "default",
// 	}
// 	d.NextArg()
// 	if d.NextArg() {
// 		h.Bar = d.Val()
// 	}
// 	return h, nil
// }

var usagePool = caddy.NewUsagePool()

func init() {
	caddy.RegisterModule(App{})
	httpcaddyfile.RegisterGlobalOption("certmatic", parseGlobalCertmatic)
	// caddy.RegisterModule(FooHandler{})
	// httpcaddyfile.RegisterHandlerDirective("certmatic_foo", parseCaddyfileAsk)
}

type App struct {
	DomainStore         config.Store      `json:"domain_store,omitempty"`
	ChallengeType       dns.ChallengeType `json:"challenge_type,omitempty"`
	DNSDelegationDomain string            `json:"dns_delegation_domain,omitempty"`
	CNameTarget         string            `json:"cname_target,omitempty"`

	// Foo string `json:"foo,omitempty"`

	logger           zap.Logger            `json:"-"`
	config           config.Config         `json:"-"`
	domainRepo       domain.DomainRepo     `json:"-"`
	dnsRecordManager *dns.DNSRecordManager `json:"-"`
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
	// evts, err := ctx.App("events")
	// if err != nil {
	// 	a.logger.Error("failed to get events app", zap.Error(err))
	// 	return err
	// }
	// events_app := evts.(*caddyevents.App)
	// events_app.Subscribe(&caddyevents.Subscription{
	// 	Events:   []string{"cert_obtaining", "cert_obtained", "cert_failed"},
	// 	Modules:  []caddy.ModuleID{"tls"}, // optional: filter by origin module
	// 	Handlers: []caddyevents.Handler{a},
	// })
	// storage := ctx.Storage()
	// keys, err := storage.List(ctx, "", true)
	// if err != nil {
	// 	return err
	// }
	// a.logger.Debug("storage keys", zap.Int("count", len(keys)), zap.Strings("keys", keys))
	domainRepo, _, err := usagePool.LoadOrNew("domainRepo", func() (caddy.Destructor, error) {
		return internal_domain.NewDomainStoreFromConfig(a.DomainStore)
	})
	// domainRepo, err := internal_domain.NewDomainStoreFromConfig(a.DomainStore)
	if err != nil {
		a.logger.Error("failed to create or load domain store from config", zap.Error(err))
		return err
	}
	a.domainRepo = domainRepo.(domain.DomainRepo)
	dnsRecordManager := dns.NewDNSRecordManager(a.ChallengeType, a.DNSDelegationDomain, a.CNameTarget)
	a.dnsRecordManager = dnsRecordManager
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
