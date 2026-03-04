package endpoint

import (
	"net/http"

	"github.com/ericls/certmatic/internal/certman"
	"github.com/go-chi/chi/v5"
)

type CertAdminEndpoint struct {
	certMan certman.CertMan
}

func newCertAdminEndpoint(certMan certman.CertMan) *CertAdminEndpoint {
	return &CertAdminEndpoint{certMan: certMan}
}

func (e *CertAdminEndpoint) BuildCertAdminRouter() chi.Router {
	r := chi.NewRouter()
	r.Head("/{hostname}", e.handleCertExists())
	r.Get("/{hostname}", e.handleCertExists())
	r.Post("/{hostname}/poke", e.handlePokeCert())
	return r
}

func (e *CertAdminEndpoint) handleCertExists() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hostname := chi.URLParam(r, "hostname")
		exists, err := e.certMan.HasCert(r.Context(), hostname)
		if err != nil {
			http.Error(w, "Error checking certificate: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if exists {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func (e *CertAdminEndpoint) handlePokeCert() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hostname := chi.URLParam(r, "hostname")
		// For now we just check if the cert exists, but in the future we could add more logic here to trigger a renewal or something
		exists, err := e.certMan.HasCert(r.Context(), hostname)
		if err != nil {
			http.Error(w, "Error checking certificate: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if exists {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Certificate already exists for " + hostname))
		} else {
			err := e.certMan.PokeCert(r.Context(), hostname)
			if err != nil {
				http.Error(w, "Error poking certificate: "+err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Certificate poke triggered for " + hostname))
		}
	}
}
