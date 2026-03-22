# Certmatic

Certmatic was born out of frustration after repeatedly implementing custom domain support for different SaaS applications. It aims to provide a managed experience for the common tasks involved: on-demand SSL certificate issuance and renewal, domain ownership verification, guiding users through DNS setup, and SSL termination.

Certmatic runs as a [Caddy](https://caddyserver.com) plugin. In a typical setup, a SaaS application uses Certmatic for SSL termination on custom domains and calls its Admin API to manage domain verification and certificate lifecycle. Ultimately Certmatic aims to be composable, for example, to act as a certificate manager alone if you handle SSL termination separately (e.g., with another ingress controller or a CDN).

> **Certmatic is in active development.** Core functionality works (domain management, ownership verification, certificate issuance, portal UI) but APIs and configuration may change. See [Roadmap](#roadmap) below.

## Features

- **Automatic SSL certificates** — on-demand issuance and renewal via ACME (HTTP-01 or DNS-01 challenges)
- **Domain ownership verification** — DNS TXT challenge or provider-managed verification
- **Customer-facing portal** — React UI that walks users through DNS configuration step by step
- **Admin API** — add domains, manage certificates, create portal sessions
- **Storage** — in-memory or SQLite (PostgreSQL support is WIP). Currently single-node only.
- **Caddy-native** — runs as a Caddy module with full Caddyfile configuration

## Quick Start

### Build

Certmatic is a Caddy plugin. Build it into Caddy using [xcaddy](https://github.com/caddyserver/xcaddy):

```bash
xcaddy build --with github.com/ericls/certmatic
```

> Building as a standalone binary (`go build -o certmatic ./cmd/certmatic`) is available for development and specific use cases but is not the recommended deployment method.

### Configure

Create a `Caddyfile`:

```caddyfile
{
    certmatic {
        domain_store   sqlite://./certmatic.db
        session_store  sqlite://./certmatic.db
        challenge_type http-01
        cname_target   your-ingress.example.com
        portal_signing_key 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
        portal_base_url https://certmatic.example.com/portal
    }
}

certmatic.example.com {
    # Admin API — protected with basic_auth.
    handle_path /admin/* {
        basic_auth {
            admin $2a$14$... # caddy hash-password
        }
        certmatic_admin
    }

    handle_path /portal/* {
        certmatic_portal
    }

    handle_path /web_client/portal/* {
        certmatic_portal_assets
    }
}
```

The Admin API has no built-in authentication — **you must secure it yourself.** Since certmatic is a Caddy plugin, you can use any Caddy middleware to protect the admin routes: `basic_auth`, `forward_auth` (delegate to your app's auth), `remote_ip` (IP allowlisting), mutual TLS, or simply bind the admin handler to a `localhost`-only listener.

### Run

```bash
./caddy run --config Caddyfile --adapter caddyfile
```

### Add a domain and create a portal session

```bash
# Add a domain
curl -X PUT https://certmatic.example.com/admin/domain/custom.example.com \
  -d '{"tenant_id": "tenant-123"}'

# Create a portal session for the customer
curl -X POST https://certmatic.example.com/admin/portal/sessions \
  -d '{
    "hostname": "custom.example.com",
    "ownership_verification_mode": "dns_challenge",
    "back_url": "https://your-app.com/settings",
    "back_text": "Back to settings"
  }'
# Returns: { "data": { "url": "https://certmatic.example.com/portal/?token=...", "expires_at": "..." } }
```

Redirect your customer to the returned URL. The portal guides them through DNS setup, verifies ownership, and issues the certificate.

## Configuration Reference

All options go inside a `certmatic { }` block in the Caddyfile global options:

| Directive               | Required  | Description                                                                                                           |
| ----------------------- | --------- | --------------------------------------------------------------------------------------------------------------------- |
| `domain_store`          | Yes       | Domain storage backend: `memory` or `sqlite://path` (PostgreSQL support is WIP)                                       |
| `session_store`         | Yes       | Session storage backend: `memory` or `sqlite://path` (PostgreSQL support is WIP)                                      |
| `challenge_type`        | No        | ACME challenge method: `http-01` (default) or `dns-01`                                                                |
| `cname_target`          | Yes       | Domain that customer domains should point to (your ingress)                                                           |
| `dns_delegation_domain` | If dns-01 | Domain for ACME DNS challenge delegation                                                                              |
| `portal_signing_key`    | No        | Hex-encoded HMAC key for session tokens (min 32 hex chars). Auto-generated if omitted (tokens won't survive restarts) |
| `portal_base_url`       | Yes       | Full URL where the portal is accessible                                                                               |
| `portal_dev_mode`       | No        | Flag (no argument). Proxies portal UI to Vite dev server                                                              |

Three Caddy handler directives are provided:

- `certmatic_admin` — mounts the Admin API. **You must protect this with authentication** (see Caddyfile example above).
- `certmatic_portal` — mounts the Portal (token exchange + session-scoped API)
- `certmatic_portal_assets` — serves the built portal UI static assets

## Architecture

```
┌─────────────────────────────────────────┐
│                 Caddy                   │
│                                         │
│  ┌──────────────┐  ┌────────────────┐   │
│  │  Admin API   │  │    Portal      │   │
│  │  (internal)  │  │  (public)      │   │
│  └──────┬───────┘  └───────┬────────┘   │
│         │                  │            │
│  ┌──────┴──────────────────┴────────┐   │
│  │         Certmatic Module         │   │
│  │  Domain Repo · Session Store     │   │
│  │  DNS Manager · Cert Manager      │   │
│  └──────────────────────────────────┘   │
└─────────────────────────────────────────┘
         │                    │
       SQLite            ACME (Let's Encrypt)
```

- **Admin API** — your backend calls this to add domains, manage certs, and create portal sessions
- **Portal** — customer-facing React app (TypeScript + Tailwind) served by Caddy. Guides users through DNS setup, verifies records, and triggers certificate issuance
- **Certmatic Module** — Caddy app that ties everything together. Manages domain state, DNS record generation, session tokens, and certificate lifecycle via CertMagic

## Roadmap

- **PostgreSQL storage backend** — enables multi-node deployments (currently limited to single-node with SQLite)
- **Provider-specific DNS guides in the portal** — step-by-step instructions for popular DNS providers (Cloudflare, Route 53, GoDaddy, etc.)
- **Ingress-friendly deployment** — support for running behind load balancers and Kubernetes ingress controllers
- **Webhook / event notifications** — callbacks for domain and certificate lifecycle events (verified, issued, renewed, expiring)
- **Admin dashboard UI** — web interface for managing domains and certificates

## Development

### Prerequisites

- Go 1.25+
- Node.js (for portal UI)

### Run

```bash
./run_dev_server.sh
```

This uses [air](https://github.com/air-verse/air) for hot-reloading the Go backend with `Caddyfile.dev`.

### Portal UI

```bash
cd portal/ui
npm install
npm run dev    # Vite dev server on :5173
```

With `portal_dev_mode` enabled in the Caddyfile, Caddy proxies `/web_client/*` to the Vite dev server for hot module replacement.

## Documentation

- [API Reference](docs/api-reference.md) — full Admin and Portal API documentation
- [Integration Guide](docs/integration-guide.md) — step-by-step guide for adding certmatic to your SaaS app
