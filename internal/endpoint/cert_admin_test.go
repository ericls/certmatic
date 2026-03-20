package endpoint

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ericls/certmatic/internal/certman"
)

// mockCertMan is a test double for certman.CertMan.
type mockCertMan struct {
	hasCertFn     func(ctx context.Context, hostname string) (bool, error)
	getCertInfoFn func(ctx context.Context, hostname string) (*certman.CertInfo, error)
	pokeCertFn    func(ctx context.Context, hostname string) error
	deleteCertFn  func(ctx context.Context, hostname string) error
}

func (m *mockCertMan) HasCert(ctx context.Context, hostname string) (bool, error) {
	return m.hasCertFn(ctx, hostname)
}

func (m *mockCertMan) GetCertInfo(ctx context.Context, hostname string) (*certman.CertInfo, error) {
	return m.getCertInfoFn(ctx, hostname)
}

func (m *mockCertMan) PokeCert(ctx context.Context, hostname string) error {
	return m.pokeCertFn(ctx, hostname)
}

func (m *mockCertMan) DeleteCert(ctx context.Context, hostname string) error {
	return m.deleteCertFn(ctx, hostname)
}

func validCert(hostname string) *certman.CertInfo {
	return &certman.CertInfo{
		Hostname:  hostname,
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
		Issuer:    "Test CA",
	}
}

func setupCertAdminRouter(cm certman.CertMan) http.Handler {
	e := newCertAdminEndpoint(cm, 1*time.Minute, 2*time.Second)
	return e.BuildCertAdminRouter()
}

// --- handleCertExists (HEAD /{hostname}) ---

func TestCertExists_Found(t *testing.T) {
	cm := &mockCertMan{
		hasCertFn: func(_ context.Context, _ string) (bool, error) { return true, nil },
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodHead, "/example.com", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestCertExists_NotFound(t *testing.T) {
	cm := &mockCertMan{
		hasCertFn: func(_ context.Context, _ string) (bool, error) { return false, nil },
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodHead, "/example.com", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestCertExists_Error(t *testing.T) {
	cm := &mockCertMan{
		hasCertFn: func(_ context.Context, _ string) (bool, error) {
			return false, errors.New("storage error")
		},
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodHead, "/example.com", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

// --- handleCertGet (GET /{hostname}) ---

func TestCertGet_Found(t *testing.T) {
	cert := validCert("example.com")
	cm := &mockCertMan{
		getCertInfoFn: func(_ context.Context, _ string) (*certman.CertInfo, error) { return cert, nil },
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodGet, "/example.com", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	resp := decodeData[CertResponse](t, rec)
	if resp.Hostname != "example.com" {
		t.Errorf("expected hostname %q, got %q", "example.com", resp.Hostname)
	}
	if resp.Issuer != "Test CA" {
		t.Errorf("expected issuer %q, got %q", "Test CA", resp.Issuer)
	}
}

func TestCertGet_NotFound(t *testing.T) {
	cm := &mockCertMan{
		getCertInfoFn: func(_ context.Context, _ string) (*certman.CertInfo, error) { return nil, nil },
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodGet, "/example.com", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestCertGet_Error(t *testing.T) {
	cm := &mockCertMan{
		getCertInfoFn: func(_ context.Context, _ string) (*certman.CertInfo, error) {
			return nil, errors.New("storage error")
		},
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodGet, "/example.com", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

// --- handlePokeCert (POST /{hostname}/poke) ---

func TestPokeCert_Success_NoCert(t *testing.T) {
	cm := &mockCertMan{
		getCertInfoFn: func(_ context.Context, _ string) (*certman.CertInfo, error) { return nil, nil },
		pokeCertFn:    func(_ context.Context, _ string) error { return nil },
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodPost, "/example.com/poke", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	resp := decodeData[PokeCertResponse](t, rec)
	if resp.Hostname != "example.com" {
		t.Errorf("expected hostname %q, got %q", "example.com", resp.Hostname)
	}
}

func TestPokeCert_Success_ExpiredCert(t *testing.T) {
	expired := &certman.CertInfo{
		Hostname:  "example.com",
		NotBefore: time.Now().Add(-2 * time.Hour),
		NotAfter:  time.Now().Add(-time.Hour), // already expired
		Issuer:    "Test CA",
	}
	cm := &mockCertMan{
		getCertInfoFn: func(_ context.Context, _ string) (*certman.CertInfo, error) { return expired, nil },
		pokeCertFn:    func(_ context.Context, _ string) error { return nil },
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodPost, "/example.com/poke", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestPokeCert_AlreadyValid(t *testing.T) {
	cm := &mockCertMan{
		getCertInfoFn: func(_ context.Context, hostname string) (*certman.CertInfo, error) {
			return validCert(hostname), nil
		},
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodPost, "/example.com/poke", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestPokeCert_PokeError(t *testing.T) {
	cm := &mockCertMan{
		getCertInfoFn: func(_ context.Context, _ string) (*certman.CertInfo, error) { return nil, nil },
		pokeCertFn:    func(_ context.Context, _ string) error { return errors.New("poke failed") },
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodPost, "/example.com/poke", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

// --- handlePokeAndWaitCert (POST /{hostname}/poke_and_wait) ---

func TestPokeAndWaitCert_Success(t *testing.T) {
	cert := validCert("example.com")
	cm := &mockCertMan{
		pokeCertFn: func(_ context.Context, _ string) error { return nil },
		// Return valid cert immediately so the loop breaks on first iteration.
		getCertInfoFn: func(_ context.Context, _ string) (*certman.CertInfo, error) { return cert, nil },
		hasCertFn: func(ctx context.Context, hostname string) (bool, error) {
			return true, nil
		},
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodPost, "/example.com/ensure", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	resp := decodeData[CertResponse](t, rec)
	if resp.Hostname != "example.com" {
		t.Errorf("expected hostname %q, got %q", "example.com", resp.Hostname)
	}
}

func TestPokeAndWaitCert_PokeError(t *testing.T) {
	cm := &mockCertMan{
		pokeCertFn: func(_ context.Context, _ string) error { return errors.New("poke failed") },
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodPost, "/example.com/ensure", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestPokeAndWaitCert_GetCertInfoError(t *testing.T) {
	cm := &mockCertMan{
		pokeCertFn: func(_ context.Context, _ string) error { return nil },
		getCertInfoFn: func(_ context.Context, _ string) (*certman.CertInfo, error) {
			return nil, errors.New("storage error")
		},
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodPost, "/example.com/ensure", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestPokeAndWaitCert_Timeout(t *testing.T) {
	cm := &mockCertMan{
		pokeCertFn: func(_ context.Context, _ string) error { return nil },
		// Never return a valid cert.
		getCertInfoFn: func(_ context.Context, _ string) (*certman.CertInfo, error) { return nil, nil },
	}
	e := newCertAdminEndpoint(cm, 10*time.Millisecond, 0)
	router := e.BuildCertAdminRouter()

	req := httptest.NewRequest(http.MethodPost, "/example.com/ensure", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("expected %d, got %d", http.StatusGatewayTimeout, rec.Code)
	}
}

// --- handleDeleteCert (DELETE /{hostname}) ---

func TestDeleteCert_Success(t *testing.T) {
	cm := &mockCertMan{
		deleteCertFn: func(_ context.Context, _ string) error { return nil },
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodDelete, "/example.com", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestDeleteCert_Error(t *testing.T) {
	cm := &mockCertMan{
		deleteCertFn: func(_ context.Context, _ string) error { return errors.New("storage error") },
	}
	router := setupCertAdminRouter(cm)

	req := httptest.NewRequest(http.MethodDelete, "/example.com", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}
