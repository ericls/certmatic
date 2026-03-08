package endpoint

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ericls/certmatic/internal/certman"
	"github.com/go-chi/chi/v5"
)

type CertAdminEndpoint struct {
	certMan          certman.CertMan
	certWaitTimeout  time.Duration
	certPollInterval time.Duration
}

func newCertAdminEndpoint(certMan certman.CertMan, certWaitTimeout, certPollInterval time.Duration) *CertAdminEndpoint {
	return &CertAdminEndpoint{
		certMan:          certMan,
		certWaitTimeout:  certWaitTimeout,
		certPollInterval: certPollInterval,
	}
}

type CertResponse struct {
	Hostname  string    `json:"hostname" yaml:"hostname"`
	NotBefore time.Time `json:"not_before" yaml:"not_before"`
	NotAfter  time.Time `json:"not_after" yaml:"not_after"`
	Issuer    string    `json:"issuer" yaml:"issuer"`
}

type PokeCertResponse struct {
	Hostname string `json:"hostname" yaml:"hostname"`
}

func (e *CertAdminEndpoint) BuildCertAdminRouter() chi.Router {
	r := chi.NewRouter()
	r.Head("/{hostname}", e.handleCertExists())
	r.Get("/{hostname}", e.handleCertGet())
	r.Post("/{hostname}/poke", e.handlePokeCert())
	r.Post("/{hostname}/ensure", e.handlePokeAndWaitCert())
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

func (e *CertAdminEndpoint) handleCertGet() http.HandlerFunc {
	return JSONHandler(http.StatusOK, func(r *http.Request, _ struct{}) (CertResponse, error) {
		hostname := chi.URLParam(r, "hostname")
		certInfo, err := e.certMan.GetCertInfo(r.Context(), hostname)
		if err != nil {
			return CertResponse{},
				HTTPError{Status: http.StatusInternalServerError, Message: "error getting certificate info"}
		}
		if certInfo == nil {
			return CertResponse{},
				HTTPError{Status: http.StatusNotFound, Message: fmt.Sprintf("certificate not found for hostname: %s", hostname)}
		}
		return CertResponse{
			Hostname:  certInfo.Hostname,
			NotBefore: certInfo.NotBefore,
			NotAfter:  certInfo.NotAfter,
			Issuer:    certInfo.Issuer,
		}, nil
	})
}

func (e *CertAdminEndpoint) handlePokeCert() http.HandlerFunc {
	return JSONHandler(http.StatusOK, func(r *http.Request, _ struct{}) (PokeCertResponse, error) {
		hostname := chi.URLParam(r, "hostname")
		cert, _ := e.certMan.GetCertInfo(r.Context(), hostname)
		if cert != nil && cert.NotAfter.After(time.Now()) && cert.NotBefore.Before(time.Now()) {
			return PokeCertResponse{},
				HTTPError{Status: http.StatusBadRequest, Message: fmt.Sprintf("certificate already exists and is valid for hostname: %s", hostname)}
		}
		err := e.certMan.PokeCert(r.Context(), hostname)
		if err != nil {
			return PokeCertResponse{},
				HTTPError{Status: http.StatusInternalServerError, Message: fmt.Sprintf("error poking certificate: %v", err)}
		}
		return PokeCertResponse{Hostname: hostname}, nil
	})
}

func (e *CertAdminEndpoint) handlePokeAndWaitCert() http.HandlerFunc {
	return JSONHandler(http.StatusOK, func(r *http.Request, _ struct{}) (CertResponse, error) {
		hostname := chi.URLParam(r, "hostname")
		err := e.certMan.PokeCert(r.Context(), hostname)
		if err != nil {
			return CertResponse{},
				HTTPError{Status: http.StatusInternalServerError, Message: fmt.Sprintf("error poking certificate: %v", err)}
		}
		var certInfo *certman.CertInfo
		timeout := time.After(e.certWaitTimeout)
		// Wait for the certificate to be issued.
		// TODO: investigate ways to utilize the event system to avoid polling.
		waitDuration := e.certPollInterval
		for {
			certInfo, err := e.certMan.GetCertInfo(r.Context(), hostname)
			if err != nil {
				return CertResponse{},
					HTTPError{Status: http.StatusInternalServerError, Message: fmt.Sprintf("error getting certificate info: %v", err)}
			}
			if certInfo != nil && certInfo.NotAfter.After(time.Now()) && certInfo.NotBefore.Before(time.Now()) {
				break
			}
			select {
			case <-timeout:
				return CertResponse{},
					HTTPError{Status: http.StatusGatewayTimeout, Message: fmt.Sprintf("timeout waiting for certificate: %s", hostname)}
			default:
				time.Sleep(waitDuration)
				if waitDuration < 10*time.Second {
					waitDuration += e.certPollInterval
				}
			}
		}
		certInfo, err = e.certMan.GetCertInfo(r.Context(), hostname)
		if err != nil {
			return CertResponse{},
				HTTPError{Status: http.StatusInternalServerError, Message: fmt.Sprintf("error getting certificate info: %v", err)}
		}
		return CertResponse{
			Hostname:  certInfo.Hostname,
			NotBefore: certInfo.NotBefore,
			NotAfter:  certInfo.NotAfter,
			Issuer:    certInfo.Issuer,
		}, nil
	})
}
