package endpoint

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/ericls/certmatic/internal/certman"
	"github.com/ericls/certmatic/internal/dns"
	"github.com/ericls/certmatic/pkg/domain"
	pkgsession "github.com/ericls/certmatic/pkg/session"
	"github.com/ericls/certmatic/pkg/webhook"
	portalstatic "github.com/ericls/certmatic/portal"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type contextKey string

const sessionContextKey contextKey = "portal_session"

// MakePortalRouter creates the chi router for the portal.
// It handles /portal/* routes: static assets, session management, and API.
func MakePortalRouter(
	domainRepo domain.DomainRepo,
	dnsRecordManager *dns.DNSRecordManager,
	certMan certman.CertMan,
	sessionStore pkgsession.SessionStore,
	signingKey []byte,
	portalBaseURL string,
	assetsFS fs.FS,
	version string,
	logger *zap.Logger,
	webhookDispatcher webhook.Dispatcher,
) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestLogger(&ZapFormatter{Logger: logger}))

	de := &portalDomainEndpoint{
		domainRepo:        domainRepo,
		dnsRecordManager:  dnsRecordManager,
		certMan:           certMan,
		certWaitTimeout:   2 * time.Minute,
		certPollInterval:  2 * time.Second,
		lookup:            dns.NetLookup(),
		webhookDispatcher: webhookDispatcher,
	}

	// Static assets — served from embedded FS (prod) or local disk (dev).
	r.Handle("/assets/*", http.StripPrefix("/assets", http.FileServerFS(assetsFS)))

	// Root: token exchange only.
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("token"); token != "" {
			handleTokenExchange(w, r, sessionStore, signingKey, portalBaseURL)
			return
		}
		http.NotFound(w, r)
	})

	// Session-scoped routes. UUID pattern prevents any non-session paths from matching.
	const uuidPattern = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
	r.Route("/{sessionID:"+uuidPattern+"}", func(r chi.Router) {
		r.Use(sessionMiddlewareByPath(sessionStore))
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			serveIndexHTML(w, version)
		})
		r.Get("/api/domain", de.handleGetDomain())
		r.Post("/api/domain/check", de.handleDomainCheck())
		r.Post("/api/domain/cert/ensure", de.handleEnsureCert())
	})

	return r
}

func serveIndexHTML(w http.ResponseWriter, version string) {
	html, err := portalstatic.GenerateHTML(portalstatic.HTMLData{
		AssetsBase: "/portal/assets/",
		Version:    version,
	})
	if err != nil {
		http.Error(w, "failed to render portal HTML: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html)) //nolint:errcheck
}

func handleTokenExchange(w http.ResponseWriter, r *http.Request, store pkgsession.SessionStore, signingKey []byte, portalBaseURL string) {
	token := r.URL.Query().Get("token")
	session, err := store.RedeemToken(signingKey, token)
	if err != nil {
		if errors.Is(err, pkgsession.ErrExpiredToken) {
			writeError(w, http.StatusUnauthorized, "token expired")
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid or already-used token")
		return
	}

	base := strings.TrimSuffix(portalBaseURL, "/")
	http.Redirect(w, r, base+"/"+session.SessionID+"/", http.StatusFound)
}

func sessionMiddlewareByPath(store pkgsession.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := chi.URLParam(r, "sessionID")
			if sessionID == "" {
				writeError(w, http.StatusUnauthorized, "session required")
				return
			}
			session, err := store.GetSession(sessionID)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or expired session")
				return
			}
			ctx := context.WithValue(r.Context(), sessionContextKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func sessionFromContext(ctx context.Context) *pkgsession.Session {
	s, _ := ctx.Value(sessionContextKey).(*pkgsession.Session)
	return s
}
