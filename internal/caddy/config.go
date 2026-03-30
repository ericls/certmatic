package caddy

import (
	"strings"

	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/ericls/certmatic/internal/config"
	"github.com/ericls/certmatic/internal/dns"
	"github.com/ericls/certmatic/pkg/webhook"
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
						Config: map[string]any{},
					}
				} else if strings.HasPrefix(val, "postgres://") {
					a.DomainStore = config.Store{
						Type:   string(config.StorageTypePostgres),
						Config: map[string]any{"connection_string": val},
					}
				} else if filePath, ok := strings.CutPrefix(val, "sqlite://"); ok {
					a.DomainStore = config.Store{
						Type:   string(config.StorageTypeSqlite),
						Config: map[string]any{"file_path": filePath},
					}
				} else if after, ok := strings.CutPrefix(val, "rqlite://"); ok {
					httpAddr := "http://" + after
					a.DomainStore = config.Store{
						Type:   string(config.StorageTypeRqlite),
						Config: map[string]any{"http_addr": httpAddr},
					}
				} else {
					return d.Errf("invalid domain store config: %s. Expected 'memory', 'sqlite://...', 'rqlite://...' or 'postgres://...'", val)
				}
			case "session_store":
				if !d.NextArg() {
					return d.ArgErr()
				}
				val := d.Val()
				if val == "memory" {
					a.SessionStore = config.Store{
						Type:   string(config.StorageTypeMemory),
						Config: map[string]any{},
					}
				} else if strings.HasPrefix(val, "postgres://") {
					a.SessionStore = config.Store{
						Type:   string(config.StorageTypePostgres),
						Config: map[string]any{"connection_string": val},
					}
				} else if filePath, ok := strings.CutPrefix(val, "sqlite://"); ok {
					a.SessionStore = config.Store{
						Type:   string(config.StorageTypeSqlite),
						Config: map[string]any{"file_path": filePath},
					}
				} else if after, ok := strings.CutPrefix(val, "rqlite://"); ok {
					httpAddr := "http://" + after
					a.SessionStore = config.Store{
						Type:   string(config.StorageTypeRqlite),
						Config: map[string]any{"http_addr": httpAddr},
					}
				} else {
					return d.Errf("invalid session store config: %s. Expected 'memory', 'sqlite://...', 'rqlite://...' or 'postgres://...'", val)
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
			case "portal_signing_key":
				if !d.NextArg() {
					return d.ArgErr()
				}
				a.PortalSigningKey = d.Val()
			case "portal_base_url":
				if !d.NextArg() {
					return d.ArgErr()
				}
				a.PortalBaseURL = d.Val()
			case "portal_dev_mode":
				a.PortalDevMode = true
			case "webhook_dispatcher":
				if !d.NextArg() {
					return d.ArgErr()
				}
				a.WebhookDispatcher = webhook.DispatcherConfig{Type: d.Val()}
				for d.NextBlock(1) {
					switch d.Val() {
					case "url":
						if !d.NextArg() {
							return d.ArgErr()
						}
						a.WebhookDispatcher.URLs = append(a.WebhookDispatcher.URLs, d.Val())
					default:
						return d.Errf("unrecognized webhook_dispatcher option: %s", d.Val())
					}
				}
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
