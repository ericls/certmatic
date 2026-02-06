package caddy

import (
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
)

func parseGlobalCertmatic(d *caddyfile.Dispenser, existingVal any) (any, error) {
	app := &App{}
	if err := app.UnmarshalCaddyfile(d); err != nil {
		return nil, err
	}

	// Return an httpcaddyfile.App to tell Caddy to load this app
	return httpcaddyfile.App{
		Name:  "certmatic",
		Value: caddyconfig.JSON(app, nil),
	}, nil
}
