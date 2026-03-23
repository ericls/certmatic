package caddy

import (
	"fmt"
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/ericls/certmatic/internal/endpoint"
)

type AskHandler struct {
	app     *App
	handler *endpoint.AskEndpoint
}

func init() {
	caddy.RegisterModule(AskHandler{})
	httpcaddyfile.RegisterHandlerDirective("certmatic_ask", parseCaddyfileAsk)
	httpcaddyfile.RegisterDirectiveOrder("certmatic_ask", httpcaddyfile.After, "header")
}

func (AskHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.certmatic_handler_ask",
		New: func() caddy.Module { return new(AskHandler) },
	}
}

// Provision implements caddy.Provisioner.
func (h *AskHandler) Provision(ctx caddy.Context) error {
	app, err := ctx.App("certmatic")
	if err != nil {
		return err
	}
	h.app = app.(*App)
	if h.app.domainRepo == nil {
		return fmt.Errorf("domainRepo is not initialized in app")
	}
	h.handler = endpoint.NewAskEndpoint(h.app.domainRepo)
	return nil
}

func parseCaddyfileAsk(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	return &AskHandler{}, nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (h *AskHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	h.handler.ServeHTTP(w, r)
	return nil
}

var (
	_ caddyhttp.MiddlewareHandler = (*AskHandler)(nil)
	_ caddy.Provisioner           = (*AskHandler)(nil)
)
