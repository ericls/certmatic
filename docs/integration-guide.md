# Certmatic Integration Guide

This guide walks through adding custom domain support to your SaaS application using Certmatic.

## Overview

Certmatic sits between your application and your customers' domains. Your backend adds domains and creates portal sessions via the Admin API. Your customers visit the portal to configure their DNS records and verify ownership. Certmatic handles SSL certificate issuance and renewal automatically.

```
Your SaaS Backend                Certmatic                    Customer
      │                              │                           │
      │── PUT /domain/{hostname} ───>│                           │
      │── POST /portal/sessions ────>│                           │
      │<── portal URL ──────────────│                           │
      │                              │                           │
      │── redirect customer ────────────────────────────────────>│
      │                              │<── customer visits portal │
      │                              │── guides DNS setup ──────>│
      │                              │<── DNS records added ─────│
      │                              │── verifies + issues cert  │
      │                              │                           │
```

## Step 1: Deploy Certmatic

### Build

Certmatic is a Caddy plugin. Build it into Caddy using [xcaddy](https://github.com/caddyserver/xcaddy):

```bash
xcaddy build --with github.com/ericls/certmatic
```

> Building as a standalone binary (`go build -o certmatic ./cmd/certmatic`) is available for development and specific use cases but is not the recommended deployment method.

### Caddyfile

```caddyfile
{
    certmatic {
        domain_store   sqlite://./certmatic.db
        session_store  sqlite://./certmatic.db
        challenge_type http-01
        cname_target   ingress.your-saas.com
        portal_signing_key <your-hex-encoded-key>
        portal_base_url https://certmatic.your-saas.com/portal
    }
}

certmatic.your-saas.com {
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

The Admin API has no built-in authentication — **you must secure it yourself.** Since certmatic is a Caddy plugin, you can use any Caddy middleware on the admin routes. See [Securing the Admin API](#securing-the-admin-api) below for all available options.

### Choosing a Storage Backend

| Backend | Use Case |
|---|---|
| `memory` | Development and testing only. Data is lost on restart. |
| `sqlite://path` | Production single-node deployments. Good performance, no external dependencies. |
| `postgres://connstr` | **WIP.** Not yet supported. Planned for multi-node deployments. |

Domain and session stores can use different backends. Since only in-memory and SQLite are currently supported, certmatic is limited to single-node deployments for now.

### Choosing a Challenge Type

| Challenge | Pros | Cons |
|---|---|---|
| `http-01` | Simple setup. No DNS delegation needed. | Requires port 80 accessible from the internet. |
| `dns-01` | Works behind firewalls. Supports wildcard certs. | Requires `dns_delegation_domain` configuration and an additional CNAME record from the customer. |

For DNS-01, you also need to set `dns_delegation_domain` and ensure your ACME DNS provider is configured to serve challenge responses at that domain.

### Generating a Signing Key

The portal signing key is a hex-encoded secret (minimum 32 hex characters / 16 bytes). Generate one with:

```bash
openssl rand -hex 32
```

If you omit `portal_signing_key`, an ephemeral key is generated on startup. Portal session tokens will not survive server restarts.

## Step 2: Add Domains

When a customer wants to use a custom domain, add it via the Admin API:

```bash
curl -X PUT https://certmatic.your-saas.com/admin/domain/blog.customer.com \
  -H "Content-Type: application/json" \
  -d '{"tenant_id": "cust-42"}'
```

Response:
```json
{
  "data": {
    "hostname": "blog.customer.com",
    "tenant_id": "cust-42",
    "ownership_verified": false,
    "verification_token": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "required_dns_records": [
      {"type": "CNAME", "name": "blog.customer.com", "value": "ingress.your-saas.com"}
    ]
  }
}
```

The `tenant_id` is opaque to certmatic — use whatever identifier your application uses for the customer.

A `verification_token` is auto-generated and stable across updates. It is used for DNS-based ownership verification.

## Step 3: Create a Portal Session

Create a portal session and redirect your customer to the returned URL:

```bash
curl -X POST https://certmatic.your-saas.com/admin/portal/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "hostname": "blog.customer.com",
    "ownership_verification_mode": "dns_challenge",
    "back_url": "https://your-saas.com/settings/domains",
    "back_text": "Back to domain settings"
  }'
```

Response:
```json
{
  "data": {
    "url": "https://certmatic.your-saas.com/portal/?token=ZjVkM2Nj...",
    "expires_at": "2025-01-15T01:00:00Z"
  }
}
```

Redirect your customer to `data.url`. The portal will guide them through the remaining steps.

### Ownership Verification Modes

**`dns_challenge` (recommended for most cases)**

The portal displays a TXT record for the customer to add:

```
_certmatic-verify.blog.customer.com  TXT  "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
```

When the customer runs the setup check in the portal, certmatic looks up this record and automatically marks the domain as verified.

**`provider_managed`**

For cases where your application manages verification (e.g., you control DNS on behalf of the customer):

```json
{
  "hostname": "blog.customer.com",
  "ownership_verification_mode": "provider_managed",
  "verify_ownership_url": "https://your-saas.com/api/domains/blog.customer.com/verify",
  "verify_ownership_text": "Verify domain"
}
```

The portal shows a button linking to your `verify_ownership_url`. Your backend performs whatever verification is appropriate, then calls:

```bash
curl -X PUT https://certmatic.your-saas.com/admin/domain/blog.customer.com \
  -d '{"ownership_verified": true}'
```

## Step 4: DNS Setup

The portal guides customers through creating the required DNS records. Depending on your configuration, customers will need:

### Always Required

**Pointing record** — directs traffic to your infrastructure:
- **CNAME** for subdomains: `blog.customer.com CNAME ingress.your-saas.com`
- **A record** for apex domains (when the DNS provider doesn't support CNAME flattening): `customer.com A <IP>`

Certmatic auto-detects whether to request a CNAME or A record based on the domain type and DNS provider capabilities.

### For DNS-01 Challenge

**Challenge delegation CNAME:**
```
_acme-challenge.blog.customer.com CNAME _acme-challenge.acme.your-saas.com
```

This delegates ACME DNS challenge responses to your infrastructure.

### For DNS-Based Ownership Verification

**Ownership TXT record:**
```
_certmatic-verify.blog.customer.com TXT "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
```

The portal provides all of these records with correct values and allows customers to export them as a zone file or JSON.

## Step 5: Certificate Issuance

Once the domain is verified and DNS records are in place, certificates can be issued.

### From the Portal

The portal UI has a button to trigger certificate issuance, which calls `POST /portal/{sessionID}/api/domain/cert/ensure` and waits up to 2 minutes.

### From Your Backend

**Fire-and-forget:**
```bash
curl -X POST https://certmatic.your-saas.com/admin/cert/blog.customer.com/poke
```

**Wait for completion:**
```bash
curl -X POST https://certmatic.your-saas.com/admin/cert/blog.customer.com/ensure
```

The `ensure` endpoint polls internally and returns the certificate info when ready (timeout: 1 minute).

### Checking Certificate Status

```bash
# Quick existence check
curl -I https://certmatic.your-saas.com/admin/cert/blog.customer.com

# Full details
curl https://certmatic.your-saas.com/admin/cert/blog.customer.com
```

Certificates are automatically renewed by Caddy/CertMagic before expiration.

## Security Considerations

### Securing the Admin API

The Admin API has no built-in authentication. Since certmatic is a Caddy plugin, you can use any of Caddy's authentication and access control mechanisms to protect the admin routes. Choose whichever fits your deployment:

**Localhost binding** — simplest option when your backend runs on the same machine:
```caddyfile
localhost:9443 {
    handle_path /admin/* {
        certmatic_admin
    }
}
```

**`basic_auth`** — static credentials, good for simple setups:
```caddyfile
handle_path /admin/* {
    basic_auth {
        admin $2a$14$... # caddy hash-password
    }
    certmatic_admin
}
```

**`forward_auth`** — delegate to your application's auth endpoint:
```caddyfile
handle_path /admin/* {
    forward_auth your-app:8080 {
        uri /auth/verify
    }
    certmatic_admin
}
```

**`remote_ip`** — restrict by source IP:
```caddyfile
handle_path /admin/* {
    @denied not remote_ip 10.0.0.0/8 172.16.0.0/12
    respond @denied 403
    certmatic_admin
}
```

You can also use **mutual TLS (mTLS)** for certificate-based client authentication. These approaches can be combined — for example, localhost binding with `basic_auth` as defense in depth.

### Other

**Portal signing key** — Use a strong, random key (at least 32 hex characters). Store it as a secret. If compromised, an attacker could forge portal session tokens.

**Session tokens** — Tokens are one-time use and expire after 60 minutes. After exchange, the session is scoped to the original hostname and cannot be used to access other domains.
