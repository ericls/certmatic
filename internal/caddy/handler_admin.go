package caddy

import (
	"fmt"
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddytls"
	"github.com/ericls/certmatic/internal/certman"
	"github.com/ericls/certmatic/internal/endpoint"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type AdminHandler struct {
	app    *App
	logger *zap.Logger

	router chi.Router
	// tlsApp *caddytls.TLS
	cerMan certman.CertMan
}

func init() {
	caddy.RegisterModule(AdminHandler{})
	httpcaddyfile.RegisterHandlerDirective("certmatic_admin", parseCaddyfileAdmin)
	httpcaddyfile.RegisterDirectiveOrder("certmatic_admin", httpcaddyfile.After, "header")
}

func (AdminHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.certmatic_handler_admin",
		New: func() caddy.Module { return new(AdminHandler) },
	}
}

// Provision implements caddy.Provisioner.
func (h *AdminHandler) Provision(ctx caddy.Context) error {
	// APP
	app, err := ctx.App("certmatic")
	if err != nil {
		return err
	}
	h.app = app.(*App)
	if h.app.domainRepo == nil {
		return fmt.Errorf("domainRepo is not initialized in app")
	}
	h.logger = h.app.logger.With(zap.String("module", "admin_handler"))
	// TLS
	tlsAppInterface, err := ctx.App("tls")
	if err != nil {
		return fmt.Errorf("error getting tls app: %w", err)
	}
	if tlsAppInterface == nil {
		return fmt.Errorf("tls app is not initialized")
	}
	tlsApp, ok := tlsAppInterface.(*caddytls.TLS)
	if !ok {
		return fmt.Errorf("tls app is not of type *caddytls.TLS")
	}
	// Storage
	storage := ctx.Storage()
	if storage == nil {
		return fmt.Errorf("no storage found in context")
	}
	certMan := certman.NewCaddyCertMan(storage, tlsApp)
	// HTTP handler
	h.logger.Info("provisioning admin handler")
	adminRouter := endpoint.MakeAdminRouter(h.app.domainRepo, h.app.dnsRecordManager, certMan,
		h.logger, h.app.sessionStore, h.app.signingKeyBytes, h.app.PortalBaseURL)
	h.router = chi.NewRouter()
	h.router.Mount("/", adminRouter)
	return nil
}

func parseCaddyfileAdmin(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	return &AdminHandler{}, nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	h.router.ServeHTTP(w, r)
	return nil
}

var (
	_ caddyhttp.MiddlewareHandler = (*AdminHandler)(nil)
	_ caddy.Provisioner           = (*AdminHandler)(nil)
)
