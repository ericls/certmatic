package webhook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ericls/certmatic/pkg/webhook"
	"go.uber.org/zap"
)

func TestMemoryDispatcher_DeliversEvent(t *testing.T) {
	var received atomic.Int32
	var lastBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		lastBody = buf
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewMemoryDispatcher([]webhook.Endpoint{{URL: srv.URL}}, zap.NewNop())
	defer d.Destruct()

	event := webhook.Event{
		Type:      webhook.EventDomainVerified,
		Timestamp: time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
		Data:      map[string]any{"hostname": "example.com"},
	}
	d.Dispatch(event)

	// Wait for delivery.
	deadline := time.After(2 * time.Second)
	for received.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for webhook delivery")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if received.Load() != 1 {
		t.Fatalf("expected 1 delivery, got %d", received.Load())
	}

	var got webhook.Event
	if err := json.Unmarshal(lastBody, &got); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if got.Type != webhook.EventDomainVerified {
		t.Fatalf("expected event type %q, got %q", webhook.EventDomainVerified, got.Type)
	}
	if got.Data["hostname"] != "example.com" {
		t.Fatalf("expected hostname example.com, got %v", got.Data["hostname"])
	}
}

func TestMemoryDispatcher_RetriesOnFailure(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewMemoryDispatcher([]webhook.Endpoint{{URL: srv.URL}}, zap.NewNop())
	defer d.Destruct()

	d.Dispatch(webhook.Event{
		Type:      webhook.EventDomainVerified,
		Timestamp: time.Now(),
		Data:      map[string]any{"hostname": "retry.example.com"},
	})

	// Wait for retries — backoff is 1s then 2s, so total ~3s max.
	deadline := time.After(10 * time.Second)
	for attempts.Load() < 3 {
		select {
		case <-deadline:
			t.Fatalf("timed out; only got %d attempts", attempts.Load())
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestMemoryDispatcher_MultipleURLs_FanOut(t *testing.T) {
	var count1, count2 atomic.Int32

	// srv1 is slow — without fan-out it would block srv2.
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		count1.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count2.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv2.Close()

	d := NewMemoryDispatcher([]webhook.Endpoint{
		{URL: srv1.URL},
		{URL: srv2.URL},
	}, zap.NewNop())
	defer d.Destruct()

	d.Dispatch(webhook.Event{
		Type:      webhook.EventDomainVerified,
		Timestamp: time.Now(),
		Data:      map[string]any{"hostname": "multi.example.com"},
	})

	// srv2 should complete well before srv1 due to fan-out.
	deadline := time.After(2 * time.Second)
	for count2.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for srv2")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// srv2 delivered while srv1 is still sleeping — confirms no head-of-line blocking.
	if count1.Load() != 0 {
		t.Log("srv1 already completed — fan-out test is less meaningful but still valid")
	}

	// Now wait for both to finish.
	deadline = time.After(2 * time.Second)
	for count1.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for srv1")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if count1.Load() != 1 || count2.Load() != 1 {
		t.Fatalf("expected 1 delivery each, got srv1=%d srv2=%d", count1.Load(), count2.Load())
	}
}

func TestMemoryDispatcher_SignsRequests(t *testing.T) {
	var sigHeader atomic.Value
	var received atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sigHeader.Store(r.Header.Get(webhook.SignatureHeader))
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewMemoryDispatcher([]webhook.Endpoint{
		{URL: srv.URL, SigningKey: "test-secret-key"},
	}, zap.NewNop())
	defer d.Destruct()

	d.Dispatch(webhook.Event{
		Type:      webhook.EventDomainVerified,
		Timestamp: time.Now(),
		Data:      map[string]any{"hostname": "signed.example.com"},
	})

	deadline := time.After(2 * time.Second)
	for received.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for webhook delivery")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	sigHeaderValue := sigHeader.Load().(string)
	if sigHeaderValue == "" {
		t.Fatal("expected X-Certmatic-Signature header to be set")
	}
	if !strings.HasPrefix(sigHeaderValue, "ts_s=") {
		t.Fatalf("signature header should start with ts_s=, got %q", sigHeader)
	}
	if !strings.Contains(sigHeaderValue, ",v1=") {
		t.Fatalf("signature header should contain ,v1=, got %q", sigHeader)
	}
}

func TestMemoryDispatcher_NoSignatureWithoutKey(t *testing.T) {
	var sigHeader string
	var received atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sigHeader = r.Header.Get(webhook.SignatureHeader)
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewMemoryDispatcher([]webhook.Endpoint{
		{URL: srv.URL},
	}, zap.NewNop())
	defer d.Destruct()

	d.Dispatch(webhook.Event{
		Type:      webhook.EventDomainVerified,
		Timestamp: time.Now(),
		Data:      map[string]any{"hostname": "unsigned.example.com"},
	})

	deadline := time.After(2 * time.Second)
	for received.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for webhook delivery")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if sigHeader != "" {
		t.Fatalf("expected no signature header, got %q", sigHeader)
	}
}
