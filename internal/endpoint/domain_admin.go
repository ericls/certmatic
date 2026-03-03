package endpoint

import (
	"net/http"

	"github.com/ericls/certmatic/internal/dns"
	"github.com/ericls/certmatic/pkg/domain"
	"github.com/go-chi/chi/v5"
)

type DomainAdminEndpoint struct {
	domainRepo       domain.DomainRepo
	DNSRecordManager *dns.DNSRecordManager
}

func newDomainAdminEndpoint(dbRepo domain.DomainRepo, dnsRecordManager *dns.DNSRecordManager) *DomainAdminEndpoint {
	return &DomainAdminEndpoint{domainRepo: dbRepo, DNSRecordManager: dnsRecordManager}
}

func (e *DomainAdminEndpoint) BuildDomainAdminRouter() chi.Router {
	r := chi.NewRouter()
	r.Route("/{hostname}", func(r chi.Router) {
		r.Get("/", e.handleGetDomain())
		r.Put("/", e.handleUpsertDomain())
		r.Post("/", e.handleUpsertDomain())
		r.Delete("/", e.handleDeleteDomain())
	})
	return r
}

type BaseDomainResponse struct {
	Hostname          string `json:"hostname" yaml:"hostname"`
	TenantID          string `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	OwnershipVerified bool   `json:"ownership_verified" yaml:"ownership_verified"`
}

type UpsertDomainRequest struct {
	TenantID          *string `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	OwnershipVerified *bool   `json:"ownership_verified,omitempty" yaml:"ownership_verified,omitempty"`
}

type DomainResponse struct {
	BaseDomainResponse
	RequiredDNSRecords []domain.DNSRecord `json:"required_dns_records,omitempty" yaml:"required_dns_records,omitempty"`
}

func (e *DomainAdminEndpoint) handleUpsertDomain() http.HandlerFunc {
	return JSONHandler(http.StatusOK, func(r *http.Request, body UpsertDomainRequest) (DomainResponse, error) {
		hostname := chi.URLParam(r, "hostname")
		var d *domain.Domain
		maybeDomain, err := e.domainRepo.Get(r.Context(), hostname)
		if err == domain.ErrNotFound {
			d = &domain.Domain{
				Hostname: hostname,
			}
		} else if err != nil {
			return DomainResponse{}, err
		} else {
			d = maybeDomain.Domain
		}
		if body.TenantID != nil {
			d.TenantID = *body.TenantID
		}
		if body.OwnershipVerified != nil {
			d.OwnershipVerified = *body.OwnershipVerified
		}
		err = e.domainRepo.Set(r.Context(), d)
		if err != nil {
			return DomainResponse{}, err
		}
		return e.buildDomainResponse(d), nil
	})
}

func (e *DomainAdminEndpoint) handleGetDomain() http.HandlerFunc {
	return JSONHandler(http.StatusOK, func(r *http.Request, _ struct{}) (DomainResponse, error) {
		hostname := chi.URLParam(r, "hostname")
		sd, err := e.domainRepo.Get(r.Context(), hostname)
		if err == domain.ErrNotFound {
			return DomainResponse{}, HTTPError{
				Status:  http.StatusNotFound,
				Message: "domain not found",
			}
		} else if err != nil {
			return DomainResponse{}, err
		}
		return e.buildDomainResponse(sd.Domain), nil
	})
}

func (e *DomainAdminEndpoint) handleDeleteDomain() http.HandlerFunc {
	return JSONHandler(http.StatusNoContent, func(r *http.Request, _ struct{}) (struct{}, error) {
		hostname := chi.URLParam(r, "hostname")
		err := e.domainRepo.Delete(r.Context(), hostname)
		if err == domain.ErrNotFound {
			return struct{}{}, HTTPError{
				Status:  http.StatusNotFound,
				Message: "domain not found",
			}
		}
		return struct{}{}, err
	})
}

func (e *DomainAdminEndpoint) buildDomainResponse(d *domain.Domain) DomainResponse {
	return DomainResponse{
		BaseDomainResponse: BaseDomainResponse{
			Hostname:          d.Hostname,
			TenantID:          d.TenantID,
			OwnershipVerified: d.OwnershipVerified,
		},
		RequiredDNSRecords: e.DNSRecordManager.GetRequiredDNSRecords(d.Hostname),
	}
}
