package caddy

import (
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

type FooHandler struct {
	Bar string `json:"bar,omitempty"`
}

func (FooHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.certmatic_foo",
		New: func() caddy.Module { return new(FooHandler) },
	}
}

func (h *FooHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("X-Foo", "Bar")
	w.Write([]byte("FooHandler: " + h.Bar + "\n"))
	return nil
}

func parseCaddyfileAsk(d httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	h := &FooHandler{
		Bar: "default",
	}
	d.NextArg()
	if d.NextArg() {
		h.Bar = d.Val()
	}
	return h, nil
}

func init() {
	caddy.RegisterModule(App{})
	httpcaddyfile.RegisterGlobalOption("certmatic", parseGlobalCertmatic)
	caddy.RegisterModule(FooHandler{})
	httpcaddyfile.RegisterHandlerDirective("certmatic_foo", parseCaddyfileAsk)
}

// App is the certmatic Caddy app that manages domain resolution.
type App struct {
	ConfigPath string `json:"config_path,omitempty"`

	Foo string `json:"foo,omitempty"`
}

func (App) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "certmatic",
		New: func() caddy.Module { return new(App) },
	}
}

func (a *App) Start() error {
	return nil
}

func (a *App) Stop() error {
	return nil
}

// Provision implements caddy.Provisioner.
func (a *App) Provision(ctx caddy.Context) error {
	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (a *App) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "config":
				if !d.NextArg() {
					return d.ArgErr()
				}
				a.ConfigPath = d.Val()
			default:
				return d.Errf("unrecognized certmatic option: %s", d.Val())
			}
		}
	}
	return nil
}

var (
	_ caddy.App             = (*App)(nil)
	_ caddy.Provisioner     = (*App)(nil)
	_ caddyfile.Unmarshaler = (*App)(nil)
)
