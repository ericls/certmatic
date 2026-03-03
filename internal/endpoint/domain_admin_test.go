package endpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ericls/certmatic/internal/dns"
	domainrepo "github.com/ericls/certmatic/internal/repo/domain"
	"github.com/ericls/certmatic/pkg/domain"
	"github.com/go-chi/chi/v5"
)

func setupTestRouter(repo *domainrepo.InMemoryDomainRepo) chi.Router {
	dnsManager := dns.NewDNSRecordManager(dns.ChallengeTypeDNS01, "", "")
	endpoint := newDomainAdminEndpoint(repo, dnsManager)
	return endpoint.BuildDomainAdminRouter()
}

func TestGetDomain_NotFound(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	router := setupTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/example.com/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestGetDomain_Found(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{
		Hostname:          "example.com",
		TenantID:          "tenant-1",
		OwnershipVerified: true,
	})
	router := setupTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/example.com/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp DomainResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Hostname != "example.com" {
		t.Errorf("expected hostname %q, got %q", "example.com", resp.Hostname)
	}
	if resp.TenantID != "tenant-1" {
		t.Errorf("expected tenant_id %q, got %q", "tenant-1", resp.TenantID)
	}
	if !resp.OwnershipVerified {
		t.Error("expected ownership_verified to be true")
	}
}

func TestUpsertDomain_Create(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	router := setupTestRouter(repo)

	body := UpsertDomainRequest{
		TenantID:          ptr("tenant-1"),
		OwnershipVerified: ptr(true),
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/example.com/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp DomainResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Hostname != "example.com" {
		t.Errorf("expected hostname %q, got %q", "example.com", resp.Hostname)
	}
	if resp.TenantID != "tenant-1" {
		t.Errorf("expected tenant_id %q, got %q", "tenant-1", resp.TenantID)
	}
	if !resp.OwnershipVerified {
		t.Error("expected ownership_verified to be true")
	}

	// Verify domain was stored
	if _, err := repo.Get(context.Background(), "example.com"); err != nil {
		t.Error("expected domain to be stored in repo")
	}
}

func TestUpsertDomain_Update(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{
		Hostname:          "example.com",
		TenantID:          "tenant-1",
		OwnershipVerified: false,
	})
	router := setupTestRouter(repo)

	body := UpsertDomainRequest{
		TenantID:          ptr("tenant-2"),
		OwnershipVerified: ptr(true),
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/example.com/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp DomainResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TenantID != "tenant-2" {
		t.Errorf("expected tenant_id %q, got %q", "tenant-2", resp.TenantID)
	}
	if !resp.OwnershipVerified {
		t.Error("expected ownership_verified to be true")
	}
}

func TestUpsertDomain_PartialUpdate(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{
		Hostname:          "example.com",
		TenantID:          "tenant-1",
		OwnershipVerified: false,
	})
	router := setupTestRouter(repo)

	// Only update ownership_verified, leave tenant_id unchanged
	body := UpsertDomainRequest{
		OwnershipVerified: ptr(true),
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/example.com/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp DomainResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// tenant_id should remain unchanged
	if resp.TenantID != "tenant-1" {
		t.Errorf("expected tenant_id %q, got %q", "tenant-1", resp.TenantID)
	}
	if !resp.OwnershipVerified {
		t.Error("expected ownership_verified to be true")
	}
}

func TestDeleteDomain_NotFound(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	router := setupTestRouter(repo)

	req := httptest.NewRequest(http.MethodDelete, "/example.com/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestDeleteDomain_Success(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{
		Hostname: "example.com",
	})
	router := setupTestRouter(repo)

	req := httptest.NewRequest(http.MethodDelete, "/example.com/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}

	// Verify domain was deleted
	if _, err := repo.Get(context.Background(), "example.com"); err != domain.ErrNotFound {
		t.Error("expected domain to be deleted from repo")
	}
}

func ptr[T any](v T) *T {
	return &v
}
