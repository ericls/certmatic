# Webhook Signatures

When a `signing_key` is configured for a webhook URL, certmatic signs each outgoing request using HMAC-SHA256 (following the same scheme as Stripe). The signature is delivered in a single HTTP header:

```
X-Certmatic-Signature: ts_s=1700000000,v1=5257a869e7ecebeda32affa62cdca3fa51cad7e77a0e56ff536d0ce8e108d8bd
```

- `ts_s` — Unix timestamp in seconds of when the request was signed
- `v1` — hex-encoded HMAC-SHA256 of the signed payload

The signed payload is the timestamp and the raw request body joined by a period: `<timestamp>.<body>`.

To verify a webhook request:

1. Extract the `ts_s` and `v1` values from the `X-Certmatic-Signature` header
2. Construct the signed payload: `{ts_s}.{raw_request_body}`
3. Compute HMAC-SHA256 of the signed payload using your signing key
4. Compare the result to `v1` (use a constant-time comparison)
5. Optionally, check that `ts_s` is within an acceptable time window to prevent replay attacks

Example verification in Python:

```python
import hashlib, hmac, time

def verify_webhook(payload: bytes, header: str, signing_key: str, tolerance_seconds: int = 300) -> bool:
    parts = dict(kv.split("=", 1) for kv in header.split(","))
    timestamp = parts["ts_s"]
    signature = parts["v1"]

    if abs(time.time() - int(timestamp)) > tolerance_seconds:
        return False

    expected = hmac.new(
        signing_key.encode(), f"{timestamp}.".encode() + payload, hashlib.sha256
    ).hexdigest()

    return hmac.compare_digest(expected, signature)
```
