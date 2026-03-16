package endpoint

import (
	"net/http"
	"time"

	"github.com/ericls/certmatic/internal/portal"
	"github.com/ericls/certmatic/pkg/domain"
)

type portalSessionAdminEndpoint struct {
	domainRepo    domain.DomainRepo
	sessionStore  portal.SessionStore
	signingKey    []byte
	portalBaseURL string
}

func newPortalSessionAdminEndpoint(
	domainRepo domain.DomainRepo,
	sessionStore portal.SessionStore,
	signingKey []byte,
	portalBaseURL string,
) *portalSessionAdminEndpoint {
	return &portalSessionAdminEndpoint{
		domainRepo:    domainRepo,
		sessionStore:  sessionStore,
		signingKey:    signingKey,
		portalBaseURL: portalBaseURL,
	}
}

type createPortalSessionRequest struct {
	Hostname string `json:"hostname" validate:"required"`
	BackURL  string `json:"back_url"`
	BackText string `json:"back_text"`
}

type createPortalSessionResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (e *portalSessionAdminEndpoint) handleCreateSession() http.HandlerFunc {
	return JSONHandler(http.StatusOK, func(r *http.Request, body createPortalSessionRequest) (createPortalSessionResponse, error) {
		_, err := e.domainRepo.Get(r.Context(), body.Hostname)
		if err == domain.ErrNotFound {
			return createPortalSessionResponse{}, HTTPError{
				Status:  http.StatusNotFound,
				Message: "hostname not found",
			}
		}
		if err != nil {
			return createPortalSessionResponse{}, err
		}

		token, expiresAt, err := portal.CreateToken(e.signingKey, body.Hostname, 60*time.Minute, body.BackURL, body.BackText)
		if err != nil {
			return createPortalSessionResponse{}, err
		}

		// Token URL: portalBaseURL + "/?token=" + token
		// The trailing slash ensures handle_path /portal/* matches.
		url := e.portalBaseURL + "/?token=" + token
		return createPortalSessionResponse{
			URL:       url,
			ExpiresAt: expiresAt,
		}, nil
	})
}
