package endpoint

import (
	"net/http"

	"github.com/ericls/certmatic/internal/certman"
	"github.com/ericls/certmatic/internal/dns"
	"github.com/ericls/certmatic/pkg/domain"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func MakeAdminRouter(
	dbRepo domain.DomainRepo,
	dnsRecordManager *dns.DNSRecordManager,
	certMan certman.CertMan,
	logger *zap.Logger,
) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestLogger(&ZapFormatter{Logger: logger}))
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	domainRouter := newDomainAdminEndpoint(dbRepo, dnsRecordManager).BuildDomainAdminRouter()
	certRouter := newCertAdminEndpoint(certMan).BuildCertAdminRouter()
	r.Mount("/cert", certRouter)
	r.Mount("/domain", domainRouter)
	return r
}
