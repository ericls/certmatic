package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSign_Format(t *testing.T) {
	key := []byte("secret")
	ts := time.Unix(1700000000, 0)
	body := []byte(`{"type":"domain_verified"}`)

	sig := Sign(key, ts, body)

	if !strings.HasPrefix(sig, "ts_s=1700000000,v1=") {
		t.Fatalf("unexpected format: %s", sig)
	}

	// Verify the HMAC is correct.
	parts := strings.SplitN(sig, ",v1=", 2)
	if len(parts) != 2 {
		t.Fatalf("could not split signature: %s", sig)
	}
	gotHex := parts[1]

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(fmt.Sprintf("%d", ts.Unix())))
	mac.Write([]byte("."))
	mac.Write(body)
	wantHex := hex.EncodeToString(mac.Sum(nil))

	if gotHex != wantHex {
		t.Fatalf("HMAC mismatch: got %s, want %s", gotHex, wantHex)
	}
}
