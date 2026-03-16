package endpoint

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"strings"

	"github.com/ericls/certmatic/internal/certman"
	"github.com/ericls/certmatic/internal/dns"
	"github.com/ericls/certmatic/internal/portal"
	portalstatic "github.com/ericls/certmatic/portal"
	"github.com/ericls/certmatic/pkg/domain"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type contextKey string

const sessionContextKey contextKey = "portal_session"

// MakePortalRouter creates the chi router for the portal.
// It only handles /portal/* routes (session management and API).
// Static web client assets are served separately under /web_client/*.
func MakePortalRouter(
	domainRepo domain.DomainRepo,
	dnsRecordManager *dns.DNSRecordManager,
	certMan certman.CertMan,
	sessionStore portal.SessionStore,
	signingKey []byte,
	portalBaseURL string,
	devMode bool,
	logger *zap.Logger,
) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	de := &portalDomainEndpoint{
		domainRepo:       domainRepo,
		dnsRecordManager: dnsRecordManager,
		certMan:          certMan,
	}

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
			serveIndexHTML(w, devMode)
		})
		r.Get("/api/domain", de.handleGetDomain())
		r.Post("/api/domain/check", de.handleDomainCheck())
	})

	return r
}

// MakeWebClientRouter serves the static web client assets from the embedded dist FS.
// In dev mode this handler is not used — Caddy routes /web_client/* directly to Vite.
func MakeWebClientRouter() chi.Router {
	r := chi.NewRouter()
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		sub, err := fs.Sub(portalstatic.EmbeddedFS, "ui/dist")
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		http.ServeFileFS(w, r, sub, path)
	})
	return r
}

func serveIndexHTML(w http.ResponseWriter, devMode bool) {
	var html string
	if devMode {
		html = portalstatic.GenerateDevHTML()
	} else {
		sub, err := fs.Sub(portalstatic.EmbeddedFS, "ui/dist")
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		var genErr error
		html, genErr = portalstatic.GenerateProductionHTML(sub)
		if genErr != nil {
			http.Error(w, "portal not built — run npm run build: "+genErr.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html)) //nolint:errcheck
}

func handleTokenExchange(w http.ResponseWriter, r *http.Request, store portal.SessionStore, signingKey []byte, portalBaseURL string) {
	token := r.URL.Query().Get("token")
	session, err := store.RedeemToken(signingKey, token)
	if err != nil {
		if errors.Is(err, portal.ErrExpiredToken) {
			writeError(w, http.StatusUnauthorized, "token expired")
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid or already-used token")
		return
	}

	base := strings.TrimSuffix(portalBaseURL, "/")
	http.Redirect(w, r, base+"/"+session.SessionID+"/", http.StatusFound)
}

func sessionMiddlewareByPath(store portal.SessionStore) func(http.Handler) http.Handler {
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

func sessionFromContext(ctx context.Context) *portal.Session {
	s, _ := ctx.Value(sessionContextKey).(*portal.Session)
	return s
}
