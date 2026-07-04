package endpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domainrepo "github.com/ericls/certmatic/internal/repo/domain"
	reposession "github.com/ericls/certmatic/internal/repo/session"
	"github.com/ericls/certmatic/pkg/domain"
	pkgsession "github.com/ericls/certmatic/pkg/session"
)

func setupPortalSessionAdmin() (*portalSessionAdminEndpoint, *domainrepo.InMemoryDomainRepo) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	store := reposession.NewMemorySessionStore()
	e := newPortalSessionAdminEndpoint(repo, store, []byte("test-signing-key"), "https://portal.example.com/portal")
	return e, repo
}

func TestCreatePortalSession_HostnameRequired(t *testing.T) {
	e, _ := setupPortalSessionAdmin()

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.handleCreateSession().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected %d, got %d", http.StatusUnprocessableEntity, rec.Code)
	}
}

func TestCreatePortalSession_DomainNotFound(t *testing.T) {
	e, _ := setupPortalSessionAdmin()

	body, _ := json.Marshal(createPortalSessionRequest{Hostname: "notfound.example.com"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.handleCreateSession().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestCreatePortalSession_Success(t *testing.T) {
	e, repo := setupPortalSessionAdmin()
	repo.Set(context.Background(), &domain.Domain{Hostname: "sub.tenant.com"})

	body, _ := json.Marshal(createPortalSessionRequest{
		Hostname: "sub.tenant.com",
		BackURL:  "https://app.example.com/settings",
		BackText: "Back to settings",
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.handleCreateSession().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	resp := decodeData[createPortalSessionResponse](t, rec)
	if resp.URL == "" {
		t.Error("expected non-empty URL")
	}
	if resp.ExpiresAt.IsZero() {
		t.Error("expected non-zero expires_at")
	}
	if !strings.Contains(resp.URL, "https://portal.example.com/portal") {
		t.Errorf("expected URL to contain portal base, got %q", resp.URL)
	}
	if !strings.Contains(resp.URL, "token=") {
		t.Errorf("expected URL to contain token param, got %q", resp.URL)
	}
}

func TestCreatePortalSession_CertIssuanceModeSkip_Persisted(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	store := reposession.NewMemorySessionStore()
	key := []byte("test-signing-key")
	e := newPortalSessionAdminEndpoint(repo, store, key, "https://portal.example.com/portal")
	repo.Set(context.Background(), &domain.Domain{Hostname: "sub.tenant.com"})

	body, _ := json.Marshal(createPortalSessionRequest{
		Hostname:         "sub.tenant.com",
		CertIssuanceMode: pkgsession.CertIssuanceModeSkip,
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.handleCreateSession().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	resp := decodeData[createPortalSessionResponse](t, rec)

	// Extract the token from the URL and redeem it from the store.
	tokenIdx := strings.Index(resp.URL, "token=")
	if tokenIdx < 0 {
		t.Fatalf("URL missing token: %q", resp.URL)
	}
	token := resp.URL[tokenIdx+len("token="):]
	sess, err := store.RedeemToken(key, token)
	if err != nil {
		t.Fatalf("RedeemToken: %v", err)
	}
	if sess.CertIssuanceMode != pkgsession.CertIssuanceModeSkip {
		t.Errorf("expected persisted mode=skip, got %q", sess.CertIssuanceMode)
	}
}

func TestCreatePortalSession_CertIssuanceMode_Invalid(t *testing.T) {
	e, repo := setupPortalSessionAdmin()
	repo.Set(context.Background(), &domain.Domain{Hostname: "sub.tenant.com"})

	body, _ := json.Marshal(map[string]string{
		"hostname":           "sub.tenant.com",
		"cert_issuance_mode": "bogus",
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.handleCreateSession().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected %d for invalid mode, got %d", http.StatusUnprocessableEntity, rec.Code)
	}
}

func TestCreatePortalSession_InvalidBackURL(t *testing.T) {
	e, repo := setupPortalSessionAdmin()
	repo.Set(context.Background(), &domain.Domain{Hostname: "sub.tenant.com"})

	body, _ := json.Marshal(createPortalSessionRequest{
		Hostname: "sub.tenant.com",
		BackURL:  "not-a-valid-url",
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.handleCreateSession().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected %d for invalid back_url, got %d", http.StatusUnprocessableEntity, rec.Code)
	}
}
