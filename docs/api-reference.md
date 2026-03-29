# Certmatic API Reference

## Response Format

All JSON endpoints return responses in this envelope:

```json
{
  "data": { ... },
  "errors": []
}
```

On error:

```json
{
  "data": null,
  "errors": [{"message": "error description", "field": "optional_field_name"}]
}
```

Validation errors return HTTP 422 with per-field error details.

---

## Admin API

The Admin API is mounted via the `certmatic_admin` Caddy handler directive. It provides domain management, certificate management, and portal session creation.

**Authentication:** The Admin API has no built-in authentication. **You must secure it yourself** using Caddy's built-in middleware — `basic_auth`, `forward_auth`, `remote_ip`, mutual TLS, or localhost binding.

### Domain Endpoints

#### GET /domain/{hostname}

Get domain details.

**Response (200):**
```json
{
  "data": {
    "hostname": "custom.example.com",
    "tenant_id": "tenant-123",
    "ownership_verified": false,
    "verification_token": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "required_dns_records": [
      {"type": "CNAME", "name": "custom.example.com", "value": "ingress.example.com"}
    ]
  }
}
```

**Errors:**
- `404` — domain not found

#### PUT /domain/{hostname}

Create or update a domain. Also accepts POST.

**Request body:**
```json
{
  "tenant_id": "tenant-123",
  "ownership_verified": true
}
```

All fields are optional. On first creation, a `verification_token` (UUID) is auto-generated and persists across updates.

**Response (200):** Same format as GET.

#### DELETE /domain/{hostname}

Delete a domain.

**Response (204):**
```json
{
  "data": {
    "ok": true,
    "hostname": "custom.example.com"
  }
}
```

**Errors:**
- `404` — domain not found

---

### Certificate Endpoints

#### HEAD /cert/{hostname}

Check if a certificate exists.

**Response:**
- `200` — certificate exists
- `404` — no certificate

No response body.

#### GET /cert/{hostname}

Get certificate details.

**Response (200):**
```json
{
  "data": {
    "hostname": "custom.example.com",
    "not_before": "2025-01-15T00:00:00Z",
    "not_after": "2025-04-15T00:00:00Z",
    "issuer": "CN=Let's Encrypt Authority X3,O=Let's Encrypt,C=US"
  }
}
```

**Errors:**
- `404` — certificate not found

#### POST /cert/{hostname}/poke

Trigger certificate issuance (fire-and-forget). Returns immediately without waiting for the certificate to be issued.

**Response (200):**
```json
{
  "data": {
    "hostname": "custom.example.com"
  }
}
```

**Errors:**
- `400` — certificate already exists and is valid

#### POST /cert/{hostname}/ensure

Trigger certificate issuance and wait for it to complete. Polls internally (up to 1 minute).

**Response (200):** Same format as GET /cert/{hostname}.

**Errors:**
- `504` — timed out waiting for certificate

#### DELETE /cert/{hostname}

Delete a certificate.

**Response:** `204 No Content`

---

### Portal Session Endpoints

#### POST /portal/sessions

Create a portal session token. The returned URL can be given to your customer to access the portal.

**Request body:**
```json
{
  "hostname": "custom.example.com",
  "ownership_verification_mode": "dns_challenge",
  "back_url": "https://your-app.com/settings",
  "back_text": "Back to settings",
  "verify_ownership_url": "https://your-app.com/api/verify",
  "verify_ownership_text": "Verify ownership"
}
```

| Field | Required | Description |
|---|---|---|
| `hostname` | Yes | Domain to create session for (must already exist) |
| `ownership_verification_mode` | No | `dns_challenge` (default) or `provider_managed` |
| `back_url` | No | URL for the "back" button in the portal (max 2048 chars) |
| `back_text` | No | Label for the back button (max 256 chars) |
| `verify_ownership_url` | No | URL for provider-managed verification button (max 2048 chars) |
| `verify_ownership_text` | No | Label for the verification button (max 256 chars) |

**Ownership verification modes:**

- **`dns_challenge`** — the portal shows a TXT record (`_certmatic-verify.{hostname}`) for the user to add. When the portal's check endpoint detects the record, it automatically marks the domain as verified.
- **`provider_managed`** — the portal shows a button linking to `verify_ownership_url`. Your backend is responsible for verifying ownership and calling `PUT /domain/{hostname}` with `{"ownership_verified": true}`.

**Response (200):**
```json
{
  "data": {
    "url": "https://certmatic.example.com/portal/?token=ZjVkM2Nj...xQp5K-_j",
    "expires_at": "2025-01-15T01:00:00Z"
  }
}
```

Session tokens are one-time use and expire after 60 minutes.

**Errors:**
- `404` — hostname not found in domain store

---

## Portal API

The Portal API is mounted via the `certmatic_portal` Caddy handler directive. It is scoped to a session and accessed by end users (your customers) through the portal UI.

**Authentication:** Sessions are established by exchanging a one-time token (from the Admin API) via `GET /portal/?token=...`, which redirects to a session-scoped URL: `/portal/{sessionID}/`.

### GET /portal/{sessionID}/api/domain

Get domain information for the current session.

**Response (200):**
```json
{
  "data": {
    "hostname": "custom.example.com",
    "ownership_verified": false,
    "required_dns_records": [
      {"type": "CNAME", "name": "custom.example.com", "value": "ingress.example.com"}
    ],
    "cert": null,
    "back_url": "https://your-app.com/settings",
    "back_text": "Back to settings",
    "ownership_verification_mode": "dns_challenge",
    "ownership_txt_record": {
      "type": "TXT",
      "name": "_certmatic-verify.custom.example.com",
      "value": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
    }
  }
}
```

When a certificate exists, the `cert` field is populated:
```json
{
  "cert": {
    "not_before": "2025-01-15T00:00:00Z",
    "not_after": "2025-04-15T00:00:00Z",
    "issuer": "CN=Let's Encrypt Authority X3,O=Let's Encrypt,C=US"
  }
}
```

For `provider_managed` mode, `ownership_txt_record` is omitted and `verify_ownership_url` / `verify_ownership_text` are included instead.

### POST /portal/{sessionID}/api/domain/check

Run live DNS verification checks. Returns the status of each required record plus ownership and certificate checks.

**Response (200):**
```json
{
  "data": {
    "hostname": "custom.example.com",
    "checks": [
      {
        "name": "cname_record",
        "status": "ok",
        "expected": "ingress.example.com",
        "actual": "ingress.example.com",
        "message": "CNAME record is correctly configured."
      },
      {
        "name": "ownership_txt_record",
        "status": "ok",
        "expected": "a1b2c3d4-...",
        "actual": "a1b2c3d4-...",
        "message": "Ownership TXT record is correctly configured."
      },
      {
        "name": "ownership_verified",
        "status": "ok",
        "message": "Domain ownership is verified."
      },
      {
        "name": "certificate",
        "status": "pending",
        "message": "Certificate issuance in progress."
      }
    ],
    "overall": "pending"
  }
}
```

**Check names:** `cname_record`, `a_record`, `txt_record`, `ownership_txt_record`, `ownership_verified`, `certificate`

**Check statuses:** `ok`, `fail`, `pending`

**Side effect:** If the ownership TXT record check passes and the domain is not yet verified, the domain is automatically marked as verified.

### POST /portal/{sessionID}/api/domain/cert/ensure

Trigger certificate issuance and wait for completion (up to 2 minutes).

**Response (200):**
```json
{
  "data": {
    "hostname": "custom.example.com",
    "not_before": "2025-01-15T00:00:00Z",
    "not_after": "2025-04-15T00:00:00Z",
    "issuer": "CN=Let's Encrypt Authority X3,O=Let's Encrypt,C=US"
  }
}
```

**Errors:**
- `503` — cert manager not available
- `504` — timed out waiting for certificate
