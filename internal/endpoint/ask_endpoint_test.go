package endpoint

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	domainrepo "github.com/ericls/certmatic/internal/repo/domain"
	"github.com/ericls/certmatic/pkg/domain"
)

func TestAskEndpoint_MissingDomain(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	ep := NewAskEndpoint(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ep.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAskEndpoint_DomainNotFound(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	ep := NewAskEndpoint(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?domain=unknown.com", nil)
	ep.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAskEndpoint_DomainNotVerified(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{Hostname: "unverified.com", OwnershipVerified: false})
	ep := NewAskEndpoint(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?domain=unverified.com", nil)
	ep.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAskEndpoint_DomainVerified(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{Hostname: "verified.com", OwnershipVerified: true})
	ep := NewAskEndpoint(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?domain=verified.com", nil)
	ep.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
