package caddy

import (
	"strings"

	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/ericls/certmatic/internal/config"
	"github.com/ericls/certmatic/internal/dns"
)

func (a *App) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "domain_store":
				if !d.NextArg() {
					return d.ArgErr()
				}
				val := d.Val()
				if val == "memory" {
					a.DomainStore = config.Store{
						Type:   string(config.StorageTypeMemory),
						Config: map[string]interface{}{},
					}
				} else if strings.HasPrefix(val, "postgres://") {
					a.DomainStore = config.Store{
						Type:   string(config.StorageTypePostgres),
						Config: map[string]interface{}{"ConnectionString": val},
					}
				} else if strings.HasPrefix(val, "sqlite://") {
					filePath := strings.TrimPrefix(val, "sqlite://")
					a.DomainStore = config.Store{
						Type:   string(config.StorageTypeSqlite),
						Config: map[string]interface{}{"FilePath": filePath},
					}
				} else {
					return d.Errf("invalid domain store config: %s. Expected 'memory', 'postgres://...' or 'sqlite://...'", val)
				}
			case "challenge_type":
				if !d.NextArg() {
					return d.ArgErr()
				}
				val := d.Val()
				switch val {
				case "http-01":
					a.ChallengeType = dns.ChallengeTypeHTTP01
				case "":
					a.ChallengeType = dns.ChallengeTypeHTTP01
				case "dns-01":
					a.ChallengeType = dns.ChallengeTypeDNS01
				default:
					return d.Errf("invalid challenge type: %s. Expected 'http-01' or 'dns-01'", val)
				}
			case "dns_delegation_domain":
				if !d.NextArg() {
					return d.ArgErr()
				}
				a.DNSDelegationDomain = d.Val()
			case "cname_target":
				if !d.NextArg() {
					return d.ArgErr()
				}
				val := d.Val()
				if val == "" {
					return d.Errf("cname_target cannot be empty")
				}
				a.CNameTarget = val
			default:
				return d.Errf("unrecognized certmatic option: %s", d.Val())
			}
		}
	}
	return nil
}

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
