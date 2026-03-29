package endpoint

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	reposession "github.com/ericls/certmatic/internal/repo/session"
	pkgsession "github.com/ericls/certmatic/pkg/session"
	"github.com/go-chi/chi/v5"
)

var portalTestKey = []byte("portal-test-key-32bytes-long!!!!!")

func signToken(key []byte, sessionID string) string {
	idB64 := base64.RawURLEncoding.EncodeToString([]byte(sessionID))
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(idB64))
	sig := mac.Sum(nil)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return idB64 + "." + sigB64
}

func storeTestSession(t *testing.T, store pkgsession.SessionStore, sessionID string, expiresAt time.Time) {
	t.Helper()
	sess := &pkgsession.Session{
		SessionID: sessionID,
		Hostname:  "test.com",
		ExpiresAt: expiresAt,
	}
	if err := store.StoreSession(sess); err != nil {
		t.Fatalf("StoreSession: %v", err)
	}
}

// ---------- Token exchange ----------

func TestTokenExchange_Valid(t *testing.T) {
	store := reposession.NewMemorySessionStore()
	defer store.Destruct()

	sessionID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	storeTestSession(t, store, sessionID, time.Now().Add(1*time.Hour))
	token := signToken(portalTestKey, sessionID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?token="+token, nil)
	handleTokenExchange(rec, req, store, portalTestKey, "https://portal.example.com/portal")

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	loc := rec.Header().Get("Location")
	want := "https://portal.example.com/portal/" + sessionID + "/"
	if loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
}

func TestTokenExchange_Expired(t *testing.T) {
	store := reposession.NewMemorySessionStore()
	defer store.Destruct()

	sessionID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	storeTestSession(t, store, sessionID, time.Now().Add(-1*time.Hour))
	token := signToken(portalTestKey, sessionID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?token="+token, nil)
	handleTokenExchange(rec, req, store, portalTestKey, "https://portal.example.com/portal")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	var resp apiResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Errors) == 0 || resp.Errors[0].Message != "token expired" {
		t.Errorf("expected 'token expired' error, got %+v", resp.Errors)
	}
}

func TestTokenExchange_Replayed(t *testing.T) {
	store := reposession.NewMemorySessionStore()
	defer store.Destruct()

	sessionID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	storeTestSession(t, store, sessionID, time.Now().Add(1*time.Hour))
	token := signToken(portalTestKey, sessionID)

	// First redemption.
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/?token="+token, nil)
	handleTokenExchange(rec1, req1, store, portalTestKey, "https://portal.example.com/portal")
	if rec1.Code != http.StatusFound {
		t.Fatalf("first exchange: status = %d, want %d", rec1.Code, http.StatusFound)
	}

	// Second redemption should fail.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/?token="+token, nil)
	handleTokenExchange(rec2, req2, store, portalTestKey, "https://portal.example.com/portal")
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("replay: status = %d, want %d", rec2.Code, http.StatusUnauthorized)
	}
}

func TestTokenExchange_Invalid(t *testing.T) {
	store := reposession.NewMemorySessionStore()
	defer store.Destruct()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?token=garbage", nil)
	handleTokenExchange(rec, req, store, portalTestKey, "https://portal.example.com/portal")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// ---------- Session middleware ----------

func TestSessionMiddleware_Valid(t *testing.T) {
	store := reposession.NewMemorySessionStore()
	defer store.Destruct()

	sessionID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	storeTestSession(t, store, sessionID, time.Now().Add(1*time.Hour))

	var gotSession *pkgsession.Session
	r := chi.NewRouter()
	r.Route("/{sessionID}", func(r chi.Router) {
		r.Use(sessionMiddlewareByPath(store))
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			gotSession = sessionFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/"+sessionID+"/", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotSession == nil {
		t.Fatal("session not injected into context")
	}
	if gotSession.SessionID != sessionID {
		t.Errorf("SessionID = %q, want %q", gotSession.SessionID, sessionID)
	}
}

func TestSessionMiddleware_Invalid(t *testing.T) {
	store := reposession.NewMemorySessionStore()
	defer store.Destruct()

	r := chi.NewRouter()
	r.Route("/{sessionID}", func(r chi.Router) {
		r.Use(sessionMiddlewareByPath(store))
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nonexistent-session/", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSessionMiddleware_Expired(t *testing.T) {
	store := reposession.NewMemorySessionStore()
	defer store.Destruct()

	sessionID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	storeTestSession(t, store, sessionID, time.Now().Add(-1*time.Hour))

	r := chi.NewRouter()
	r.Route("/{sessionID}", func(r chi.Router) {
		r.Use(sessionMiddlewareByPath(store))
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/"+sessionID+"/", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
