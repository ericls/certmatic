package endpoint

import (
	"net/http"

	"github.com/ericls/certmatic/pkg/domain"
)

type AskEndpoint struct {
	domainRepo domain.DomainRepo
}

func NewAskEndpoint(domainRepo domain.DomainRepo) *AskEndpoint {
	return &AskEndpoint{domainRepo: domainRepo}
}

func (e *AskEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("domain")
	if hostname == "" {
		http.Error(w, "missing domain", http.StatusBadRequest)
		return
	}
	sd, err := e.domainRepo.Get(r.Context(), hostname)
	if err != nil || !sd.Domain.OwnershipVerified {
		http.Error(w, "not allowed", http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
}
