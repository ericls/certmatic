package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const SignatureHeader = "X-Certmatic-Signature"

// Sign computes a webhook signature header value.
// Format: ts_s=<unix_timestamp>,v1=<hex_hmac_sha256>
// The signed payload is "<timestamp>.<body>".
func Sign(signingKey []byte, timestamp time.Time, body []byte) string {
	ts := fmt.Sprintf("%d", timestamp.Unix())
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("ts_s=%s,v1=%s", ts, sig)
}
