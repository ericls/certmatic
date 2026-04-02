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

The Admin API is mounted via the `certmatic_admin` Caddy handler directive. It provides domain management and portal session creation.

**Authentication:** The Admin API has no built-in authentication. **You must secure it yourself** using Caddy's built-in middleware — `basic_auth`, `forward_auth`, `remote_ip`, mutual TLS, or localhost binding.

### Domain Endpoints

#### GET /domain/{hostname}

Get domain details.

**Response (200):**
```json
{
  "data": {
    "hostname": "custom.example.com",
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

Create or update a domain.

**Request body:**
```json
{
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
