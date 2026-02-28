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

func NewDomainAdminEndpoint(dbRepo domain.DomainRepo, dnsRecordManager *dns.DNSRecordManager) *DomainAdminEndpoint {
	return &DomainAdminEndpoint{domainRepo: dbRepo, DNSRecordManager: dnsRecordManager}
}

func (e *DomainAdminEndpoint) BuildDomainAdminRouter() chi.Router {
	r := chi.NewRouter()
	r.Route("/{hostname}", func(r chi.Router) {
		r.Post("/set", e.makeSetDomainHandler())
		r.Get("/get", e.makeGetDomainHandler())
	})
	return r
}

type BaseDomainResponse struct {
	Hostname          string `json:"hostname" yaml:"hostname"`
	TenantID          string `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	OwnershipVerified bool   `json:"ownership_verified" yaml:"ownership_verified"`
}

type SetDomainRequest struct {
	TenantID          *string `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	OwnershipVerified *bool   `json:"ownership_verified" yaml:"ownership_verified"`
}

type SerializedDomain struct {
	BaseDomainResponse
	RequiredDNSRecords []domain.DNSRecord `json:"required_dns_records,omitempty" yaml:"required_dns_records,omitempty"`
}

type SetDomainResponse = SerializedDomain

func (e *DomainAdminEndpoint) makeSetDomainHandler() http.HandlerFunc {
	return JSONHandler(http.StatusOK, func(r *http.Request, body SetDomainRequest) (SetDomainResponse, error) {
		hostname := chi.URLParam(r, "hostname")
		var d *domain.Domain
		maybeDomain, err := e.domainRepo.Get(r.Context(), hostname)
		// if error is domain.ErrNotFound, then create a new domain. Otherwise return error
		if err == domain.ErrNotFound {
			d = &domain.Domain{
				Hostname: hostname,
			}
		} else if err != nil {
			return SetDomainResponse{}, err
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
			return SetDomainResponse{}, err
		}
		return SetDomainResponse{
			BaseDomainResponse: BaseDomainResponse{
				Hostname:          d.Hostname,
				TenantID:          d.TenantID,
				OwnershipVerified: d.OwnershipVerified,
			},
			RequiredDNSRecords: e.DNSRecordManager.GetRequiredDNSRecords(d.Hostname),
		}, nil
	})
}

type GetDomainResponse = SerializedDomain

func (e *DomainAdminEndpoint) makeGetDomainHandler() http.HandlerFunc {
	return JSONHandler(http.StatusOK, func(r *http.Request, _ struct{}) (GetDomainResponse, error) {
		hostname := chi.URLParam(r, "hostname")
		sd, err := e.domainRepo.Get(r.Context(), hostname)
		if err == domain.ErrNotFound {
			return GetDomainResponse{}, HTTPError{
				Status:  http.StatusNotFound,
				Message: "domain not found",
			}
		} else if err != nil {
			return GetDomainResponse{}, err
		}
		d := sd.Domain
		return GetDomainResponse{
			BaseDomainResponse: BaseDomainResponse{
				Hostname:          d.Hostname,
				TenantID:          d.TenantID,
				OwnershipVerified: d.OwnershipVerified,
			},
			RequiredDNSRecords: e.DNSRecordManager.GetRequiredDNSRecords(d.Hostname),
		}, nil
	})
}
