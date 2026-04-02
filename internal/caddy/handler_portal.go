package caddy

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddytls"
	"github.com/ericls/certmatic/internal/certman"
	"github.com/ericls/certmatic/internal/endpoint"
	portalstatic "github.com/ericls/certmatic/portal"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// PortalHandler is the Caddy module for the customer-facing portal.
//
// Caddyfile usage:
//
//	certmatic_portal
//
// Dev mode is enabled by setting portal_dev_dir in the global certmatic block.
type PortalHandler struct {
	app    *App
	logger *zap.Logger
	router chi.Router
}

func init() {
	caddy.RegisterModule(PortalHandler{})
	httpcaddyfile.RegisterHandlerDirective("certmatic_portal", parseCaddyfilePortal)
	httpcaddyfile.RegisterDirectiveOrder("certmatic_portal", httpcaddyfile.After, "header")
}

func (PortalHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.certmatic_handler_portal",
		New: func() caddy.Module { return new(PortalHandler) },
	}
}

// Provision implements caddy.Provisioner.
func (h *PortalHandler) Provision(ctx caddy.Context) error {
	app, err := ctx.App("certmatic")
	if err != nil {
		return err
	}
	h.app = app.(*App)
	if h.app.domainRepo == nil {
		return fmt.Errorf("domainRepo is not initialized in certmatic app")
	}
	h.logger = h.app.logger.With(zap.String("module", "portal_handler"))

	tlsAppInterface, err := ctx.App("tls")
	if err != nil {
		return fmt.Errorf("error getting tls app: %w", err)
	}
	tlsApp, ok := tlsAppInterface.(*caddytls.TLS)
	if !ok {
		return fmt.Errorf("tls app is not of type *caddytls.TLS")
	}
	storage := ctx.Storage()
	if storage == nil {
		return fmt.Errorf("no storage found in context")
	}
	certMan := certman.NewCaddyCertMan(storage, tlsApp)

	var assetsFS fs.FS
	var version string

	if h.app.PortalAssetsDir != "" {
		assetsFS = os.DirFS(h.app.PortalAssetsDir)
		version = portalstatic.ComputeVersion(assetsFS) + "_dynamic"
		h.logger.Info("portal dev mode", zap.String("dir", h.app.PortalAssetsDir))
	} else {
		sub, err := fs.Sub(portalstatic.EmbeddedFS, "ui/dist")
		if err != nil {
			return fmt.Errorf("portal embedded FS: %w", err)
		}
		assetsFS = sub
		version = portalstatic.ComputeVersion(sub)
		h.logger.Info("portal prod mode", zap.String("version", version))
	}

	portalRouter := endpoint.MakePortalRouter(
		h.app.domainRepo,
		h.app.dnsRecordManager,
		certMan,
		h.app.sessionStore,
		h.app.signingKeyBytes,
		h.app.PortalBaseURL,
		assetsFS,
		version,
		h.logger,
		h.app.webhookDispatcher,
		h.app.lookup,
	)
	h.router = chi.NewRouter()
	h.router.Mount("/", portalRouter)
	return nil
}

func parseCaddyfilePortal(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	return &PortalHandler{}, nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (h *PortalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	h.router.ServeHTTP(w, r)
	return nil
}

var (
	_ caddyhttp.MiddlewareHandler = (*PortalHandler)(nil)
	_ caddy.Provisioner           = (*PortalHandler)(nil)
)
