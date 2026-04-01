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

func TestSign_Deterministic(t *testing.T) {
	key := []byte("secret")
	ts := time.Unix(1700000000, 0)
	body := []byte(`{"data":"test"}`)

	sig1 := Sign(key, ts, body)
	sig2 := Sign(key, ts, body)

	if sig1 != sig2 {
		t.Fatalf("same inputs produced different signatures: %s vs %s", sig1, sig2)
	}
}

func TestSign_DifferentKeys(t *testing.T) {
	ts := time.Unix(1700000000, 0)
	body := []byte(`{"data":"test"}`)

	sig1 := Sign([]byte("key1"), ts, body)
	sig2 := Sign([]byte("key2"), ts, body)

	if sig1 == sig2 {
		t.Fatal("different keys produced the same signature")
	}
}

func TestSign_DifferentBodies(t *testing.T) {
	key := []byte("secret")
	ts := time.Unix(1700000000, 0)

	sig1 := Sign(key, ts, []byte(`{"a":1}`))
	sig2 := Sign(key, ts, []byte(`{"a":2}`))

	if sig1 == sig2 {
		t.Fatal("different bodies produced the same signature")
	}
}

func TestSign_DifferentTimestamps(t *testing.T) {
	key := []byte("secret")
	body := []byte(`{"data":"test"}`)

	sig1 := Sign(key, time.Unix(1700000000, 0), body)
	sig2 := Sign(key, time.Unix(1700000001, 0), body)

	if sig1 == sig2 {
		t.Fatal("different timestamps produced the same signature")
	}
}
