# Certmatic

Certmatic was born out of frustration after repeatedly implementing custom domain support for different SaaS applications. It aims to provide a managed experience for the common tasks involved.

Certmatic aims to be composable. It aims to allow users to pick whatever they need from the stack, whether it's just the portal UI, or it does the certificate obtaining without terminating TLS itself. (Some of these usage patterns are not fully supported or tested yet.)

That being said, a typical setup would include Certmatic as a plugin for the [Caddy](https://caddyserver.com) instance that serves the custom domain traffic.

A typical user flow looks like this:
1. A SaaS user goes to the SaaS app's settings page and clicks "Add custom domain"
2. The SaaS persists this setting and calls Certmatic's Admin API to add the domain and create a portal session
3. The user is redirected to Certmatic's customer portal, which guides them through DNS configuration step by step, verifies ownership, and issues the certificate.
4. The user is redirected back to the app, and the custom domain is active and secured with SSL.

> **⚠️ Certmatic is in active development.**

## Quick Start

### Build

Certmatic can act as a Caddy plugin. Build it into Caddy using [xcaddy](https://github.com/caddyserver/xcaddy):

```bash
xcaddy build --with github.com/ericls/certmatic
```

> Building as a standalone binary (`go build -o certmatic ./cmd/certmatic`) is available for development and specific use cases but is not the recommended deployment method.

### Configure

Create a `Caddyfile`:

```caddyfile
# Assumptions:
# the main SaaS app is running on upstream:8080
# the entrypoint for custom domain traffic is upstream-for-custom-domains:8080
# and Caddy is the ingress for both the app and certmatic portal.
# 
# Caddy is a powerful and flexible server, feel free to adjust the configuration to fit your architecture
{
    certmatic {
        # (See below for more configuration options)
        domain_store   sqlite://./certmatic.db
        session_store  sqlite://./certmatic.db
        # For rqlite:
        # domain_store rqlite://rqlite-server:4001?options...
        # session_store rqlite://rqlite-server:4001?options...

        challenge_type http-01
        cname_target   custom-domain.example-saas.com

        portal_signing_key 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
        portal_base_url https://certmatic-portal.example-saas.com/
        webhook_dispatcher memory {
            url http://upstream:8080/webhooks/certmatic {
                signing_key your-webhook-secret
            }
        }
    }
    on_demand_tls {
        ask      http://certmatic-internal.example-saas.com/ask
    }
}

example-saas.com {
    reverse_proxy upstream:8080
    handle /webhooks/certmatic {
        # block external access to the webhook endpoint
        return 403
    }
}

certmatic-portal.example-saas.com {
    handle_path /* {
        certmatic_portal
    }
}

# This host name is not meant to be publicly accessible.
# Protect it with authentication and/or bind to a private listener.
# Caddy comes with many built-in middleware options for authentication and access control, don't miss out on these.
certmatic-internal.example-saas.com {
    # Admin API — protected with basic_auth.
    handle_path /admin/* {
        basic_auth {
            admin $2a$14$... # caddy hash-password
        }
        certmatic_admin
    }
    # Managed `ask` handler for on-demand certificate issuance.
    handle /ask {
        certmatic_ask
    }
}

# This is the main ingress for your users' custom domains. 
https:// {
    tls {
        on_demand
    }
    reverse_proxy {
        to http://upstream-for-custom-domains:8080
    }
}
# Should probably handle non-HTTPS traffic to the custom domains too, maybe just redirecting to HTTPS
http:// {
    # replace with your desired behavior for non-HTTPS traffic to custom domains...
}
```

The Admin API has no built-in authentication — **you must secure it yourself.** Since certmatic here runs as a Caddy plugin, you can use any Caddy middleware to protect the admin routes: `basic_auth`, `forward_auth` (delegate to your app's auth), `remote_ip` (IP allowlisting), mutual TLS, or simply bind the admin handler to an internal-only listener.

With this setup, here are how things communicate:
- The SaaS app backend calls certmatic's Admin API at `certmatic-internal.example-saas.com/admin/*` to manage domains and create portal sessions
- The portal UI is served at `certmatic-portal.example-saas.com/*`
- The on-demand TLS `ask` endpoint becomes an internal API call
- End users accessing through the custom domains (`something-fun.example.com`) will be served by `upstream-for-custom-domains:8080` once their domain is verified

### Run

```bash
# This is a simple example, usually you'd already have Caddy as a service somewhere.
# Please read xcaddy's documentation for more details on how to package/run Caddy with plugin in production:
# https://caddyserver.com/docs/build#package-support-files-for-custom-builds-for-debianubunturaspbian
./caddy run --config Caddyfile --adapter caddyfile
```

### Add a domain and create a portal session

```bash
# Add a domain
curl -X PUT https://certmatic-internal.example-saas.com/admin/domain/custom.example.com \
  -d '{"tenant_id": "tenant-123"}'

# Create a portal session for the customer, using the DNS challenge verification mode.
curl -X POST https://certmatic-internal.example-saas.com/admin/portal/sessions \
  -d '{
    "hostname": "custom.example.com",
    "ownership_verification_mode": "dns_challenge",
    "back_url": "https://example-saas.com/settings/custom-domain",
    "back_text": "Back to settings"
  }'

# Create a portal session for the customer, using the provider managed verification mode.
# if you choose this, you will verify domain ownership yourself. On the portal UI, the DNS challenge step will be replaced with a button that redirects the end user to a URL you set. The button text and redirect URL can be set with `verify_ownership_text` and `verify_ownership_url`.
curl -X POST https://certmatic-internal.example-saas.com/admin/portal/sessions \
  -d '{
    "hostname": "custom.example.com",
    "ownership_verification_mode": "provider_managed",
    "verify_ownership_url": "https://example-saas.com/settings/custom-domain/verify",
    "verify_ownership_text": "Verify ownership",
    "back_url": "https://example-saas.com/settings/custom-domain",
    "back_text": "Back to settings"
  }'
# Returns: { "data": { "url": "https://certmatic-portal.example-saas.com/?token=...", "expires_at": "..." } }
```

Redirect your customer to the returned URL. The portal guides them through ownership verification and DNS configuration.

## Configuration Reference

All options go inside a `certmatic { }` block in the Caddyfile global options:

| Directive               | Required  | Description                                                                                                           |
| ----------------------- | --------- | --------------------------------------------------------------------------------------------------------------------- |
| `domain_store`          | Yes       | Domain storage backend: `memory`, `sqlite://path`, or `rqlite://host:port?options`. |
| `session_store`         | Yes       | Session storage backend: `memory`, `sqlite://path`, or `rqlite://host:port?options`. |
| `challenge_type`        | No        | ACME challenge method: `http-01` (default) or `dns-01`. DNS-01 requires a [Caddy DNS provider plugin](https://caddyserver.com/docs/modules/) built into the binary and configured in the `tls` block. |
| `cname_target`          | Yes       | Domain that customer domains should point to (your ingress)                                                           |
| `dns_delegation_domain` | If dns-01 | Domain for ACME DNS challenge delegation                                                                              |
| `portal_signing_key`    | No        | Hex-encoded HMAC key for session tokens (min 32 hex chars). Auto-generated if omitted (auto-generated tokens won't survive restarts) |
| `portal_base_url`       | Yes       | Full URL where the portal is accessible                                                                               |
| `portal_assets_dir`     | No        | Serve portal UI assets from this local directory instead of the embedded build. Useful for development (point at `portal/ui/dev-build`) or to use a custom/forked portal UI. |
| `webhook_dispatcher`    | No        | Webhook event dispatcher. Syntax: `webhook_dispatcher <queue_backend_type> { url <endpoint> { signing_key <key> } }`. Currently only supports `memory` backend type. Multiple `url` blocks can be specified, each with an optional `signing_key` used to sign outgoing webhook requests so receivers can verify them. Events (e.g. `domain_verified`) are delivered asynchronously with retries. See [Webhook Signatures](/docs/webhook-signature.md) for details. |
| `dns_nameserver`        | No        | UDP DNS server to use for DNS lookups (e.g. `8.8.8.8:53`). When set, Certmatic queries this server directly using Go's pure-Go resolver. If omitted, the OS system resolver is used. |

Three Caddy handler directives are provided:

- `certmatic_admin` — mounts the Admin API. **You must protect this with authentication** (see Caddyfile example above).
- `certmatic_portal` — mounts the Portal (token exchange + session-scoped API + static assets)
- `certmatic_ask` — mounts the on-demand TLS ask endpoint. Point Caddy's `on_demand_tls { ask <url> }` at this handler. Returns 200 for domains that exist in the system and are ownership-verified, 403 otherwise. Keep this on a localhost-only or private listener — do not expose it publicly.


Each URL can have its own signing key, so different endpoints can be operated by different parties:

```caddyfile
webhook_dispatcher memory {
    url http://service-a.internal/hooks {
        signing_key secret-for-service-a
    }
    url http://service-b.internal/hooks {
        signing_key secret-for-service-b
    }
    url http://service-c.internal/hooks
}
```

URLs without a `signing_key` will not receive a signature header.

## Roadmap

- **domainconnect.org spec implementation** — allow end users to automatically configure their DNS
- **PostgreSQL storage backend** — for deployments where PostgreSQL is already available
- **Provider-specific DNS guides in the portal** — step-by-step instructions for popular DNS providers (Cloudflare, Route 53, GoDaddy, etc.)
- **Ingress-friendly deployment** — support for running behind load balancers and Kubernetes ingress controllers
- **Admin dashboard UI** — web interface for managing domains and certificates

## Development

### Prerequisites

- Go 1.25+
- Node.js 22, for portal UI
- pnpm, for portal UI
- [pre-commit](https://pre-commit.com/) (Recommended. If you are familiar with Python tooling, feel free to use whatever is comfortable for you, otherwise you can use `brew install pre-commit` on macOS, or `pipx install pre-commit` cross-platform)

If using pre-commit, remember to install the git hooks:

```bash
pre-commit install
```

### Run

```bash
./run_dev_server.sh
```

This uses [air](https://github.com/air-verse/air) for hot-reloading the Go backend with `Caddyfile.dev`.

### Portal UI

```bash
cd portal/ui
pnpm install
pnpm run dev
```

This starts the Parcel watcher, which rebuilds the portal UI into `portal/ui/dev-build/` on every change. The `Caddyfile.dev` used by `run_dev_server.sh` sets `portal_assets_dir ./portal/ui/dev-build`, so the Go server picks up changes on the next page reload. Parcel also injects HMR.

To produce the committed production assets (embedded into the binary on build):

```bash
cd portal/ui
pnpm run build
```

Commit the resulting `portal/ui/dist/` files alongside your Go changes.

## Documentation

- [API Reference](docs/api-reference.md) — full Admin and Portal API documentation

## License

The Certmatic server is licensed under the [GNU Affero General Public License v3.0](LICENSE).

The portal UI (`portal/ui/`) is licensed under the [MIT License](portal/ui/LICENSE).
