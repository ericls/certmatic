# Certmatic

Certmatic was born out of frustration after repeatedly implementing custom domain support for different SaaS applications. It aims to provide a managed experience for the common tasks involved: on-demand SSL certificate issuance and renewal, domain ownership verification, guiding users through DNS setup, and SSL termination.

In a typical setup Certmatic runs as a [Caddy](https://caddyserver.com) plugin for the custom domain ingress.

A typical user flow looks like this:
1. A SaaS user goes to their app's settings page and clicks "Add custom domain"
2. The SaaS persist this setting and calls Certmatic's Admin API to add the domain and create a portal session
3. The user is redirected to Certmatic's customer portal, which guides them through DNS configuration step by step, verifies ownership, and issues the certificate.
4. The user is redirected back to the app, and the custom domain is active and secured with SSL.

> **Certmatic is in active development.** Core functionality works (domain management, ownership verification, certificate issuance, portal UI) but APIs and configuration may change. See [Roadmap](#roadmap) below.

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
# Assume the main SaaS app is running on upstream:8080, and Caddy is the ingress for both the app and certmatic portal.
{
    certmatic {
        # See below for more configuration options
        domain_store   sqlite://./certmatic.db
        session_store  sqlite://./certmatic.db
        challenge_type http-01
        cname_target   custom-domain.example-saas.com
        portal_signing_key 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
        portal_base_url https://certmatic-portal.example-saas.com/
        webhook_dispatcher memory {
            url http://upstream:8080/webhooks/certmatic
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
    handle_path /web_client/portal/* {
        certmatic_portal_assets
    }
    handle_path /* {
        certmatic_portal
    }
}

# This host name is not meant to be publicly accessible.
# Protect it with authentication and/or bind to a private listener.
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
        to http://upstream:8080
    }
}
```

The Admin API has no built-in authentication — **you must secure it yourself.** Since certmatic here runs as a Caddy plugin, you can use any Caddy middleware to protect the admin routes: `basic_auth`, `forward_auth` (delegate to your app's auth), `remote_ip` (IP allowlisting), mutual TLS, or simply bind the admin handler to an internal-only listener.

### Run

```bash
./caddy run --config Caddyfile --adapter caddyfile
```

### Add a domain and create a portal session

```bash
# Add a domain
curl -X PUT https://certmatic-internal.example-saas.com/admin/domain/custom.example.com \
  -d '{"tenant_id": "tenant-123"}'

# Create a portal session for the customer
curl -X POST https://certmatic-internal.example-saas.com/admin/portal/sessions \
  -d '{
    "hostname": "custom.example.com",
    "ownership_verification_mode": "dns_challenge",
    "back_url": "https://example-saas.com/settings/custom-domain",
    "back_text": "Back to settings"
  }'
# Returns: { "data": { "url": "https://certmatic-portal.example-saas.com/?token=...", "expires_at": "..." } }
```

Redirect your customer to the returned URL. The portal guides them through DNS setup, verifies ownership, and issues the certificate.

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
| `portal_dev_mode`       | No        | When true, proxies portal UI to Vite dev server                                                              |
| `webhook_dispatcher`    | No        | Webhook event dispatcher. Syntax: `webhook_dispatcher <queue_backend_type> { url <endpoint> }`. Currently only supports `memory` backend type. Multiple `url` lines can be specified. Events (e.g. `domain_verified`) are delivered asynchronously with retries. |

Four Caddy handler directives are provided:

- `certmatic_admin` — mounts the Admin API. **You must protect this with authentication** (see Caddyfile example above).
- `certmatic_portal` — mounts the Portal (token exchange + session-scoped API)
- `certmatic_portal_assets` — serves the built portal UI static assets
- `certmatic_ask` — mounts the on-demand TLS ask endpoint. Point Caddy's `on_demand_tls { ask <url> }` at this handler. Returns 200 for domains that exist in the system and are ownership-verified, 403 otherwise. Keep this on a localhost-only or private listener — do not expose it publicly.

## Roadmap

- **PostgreSQL storage backend** — for deployments where PostgreSQL is already available
- **Provider-specific DNS guides in the portal** — step-by-step instructions for popular DNS providers (Cloudflare, Route 53, GoDaddy, etc.)
- **Ingress-friendly deployment** — support for running behind load balancers and Kubernetes ingress controllers
- **Admin dashboard UI** — web interface for managing domains and certificates

## Development

### Prerequisites

- Go 1.25+
- Node.js 22, for portal UI
- pnpm, for portal UI

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

With `portal_dev_mode` enabled in the Caddyfile, the portal `index.html` file will load portal frontend assets from `/web_client`, you can then delegate that path to the Vite dev server for hot-reloading the UI. This is already set up in `Caddyfile.dev` used by the `run_dev_server.sh` script, assuming the Vite server runs on the default port `5173`.

## Documentation

- [API Reference](docs/api-reference.md) — full Admin and Portal API documentation
