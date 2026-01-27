package caddy

import "github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"

func parseGlobalCertmatic(d *caddyfile.Dispenser, existingVal any) (any, error) {
	return d.ArgErr(), nil
}
