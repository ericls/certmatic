package caddy

import (
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/ericls/certmatic/internal/endpoint"
	"github.com/go-chi/chi/v5"
)

// PortalAssetsHandler serves the embedded Vite-built portal web client assets.
//
// Caddyfile usage (production):
//
//	handle_path /web_client/portal/* { certmatic_portal_assets }
//
// handle_path must strip /web_client/portal so that chi sees paths like
// /assets/main-xxx.js, matching the layout of the Vite dist output.
// In dev mode, Caddy's reverse_proxy to the Vite dev server is used instead.
type PortalAssetsHandler struct {
	router chi.Router
}

func init() {
	caddy.RegisterModule(PortalAssetsHandler{})
	httpcaddyfile.RegisterHandlerDirective("certmatic_portal_assets", parseCaddyfilePortalAssets)
	httpcaddyfile.RegisterDirectiveOrder("certmatic_portal_assets", httpcaddyfile.After, "header")
}

func (PortalAssetsHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.certmatic_handler_portal_assets",
		New: func() caddy.Module { return new(PortalAssetsHandler) },
	}
}

func (h *PortalAssetsHandler) Provision(_ caddy.Context) error {
	h.router = endpoint.MakeWebClientRouter()
	return nil
}

func parseCaddyfilePortalAssets(_ httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	return &PortalAssetsHandler{}, nil
}

func (h *PortalAssetsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, _ caddyhttp.Handler) error {
	h.router.ServeHTTP(w, r)
	return nil
}

var (
	_ caddyhttp.MiddlewareHandler = (*PortalAssetsHandler)(nil)
	_ caddy.Provisioner           = (*PortalAssetsHandler)(nil)
)
