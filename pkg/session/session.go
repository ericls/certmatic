package session

import (
	"errors"
	"time"
)

var (
	ErrInvalidToken  = errors.New("invalid token")
	ErrExpiredToken  = errors.New("token expired")
	ErrTokenReplayed = errors.New("token already used")
)

// OwnershipVerificationMode controls what the portal shows the user for ownership verification.
type OwnershipVerificationMode string

const (
	// OwnershipVerificationModeDNSChallenge instructs the portal to show a DNS TXT record that
	// the user must add to prove ownership. When the user runs Setup Check, the portal performs
	// a live DNS lookup against _certmatic-verify.{hostname} and automatically sets
	// ownership_verified=true on the domain if the record matches.
	OwnershipVerificationModeDNSChallenge OwnershipVerificationMode = "dns_challenge"

	// OwnershipVerificationModeProviderManaged indicates that an external SaaS/provider controls
	// verification. The portal shows a configurable "Verify Ownership" button linking to the
	// provider dashboard. The provider (or admin) calls ownership_verified=true on the admin API.
	OwnershipVerificationModeProviderManaged OwnershipVerificationMode = "provider_managed"
)

// CertIssuanceMode controls how the portal treats certificate issuance for a session.
type CertIssuanceMode string

const (
	// CertIssuanceModeInPortal is the default. The portal offers an "Issue certificate" step
	// that pokes the internal cert manager and polls until a certificate is issued before the
	// user is sent back. This is also the effective mode when the field is empty, so existing
	// integrations keep the current behavior without opting in.
	CertIssuanceModeInPortal CertIssuanceMode = "in_portal"

	// CertIssuanceModeSkip tells the portal to declare success once ownership is verified and
	// the required DNS records validate. The certificate step is hidden and the user is sent
	// back immediately. Use this when the certificate is issued out-of-band (later, elsewhere,
	// or by an external system) and the portal only needs to confirm DNS setup.
	CertIssuanceModeSkip CertIssuanceMode = "skip"
)

// EffectiveCertIssuanceMode returns the mode to apply, treating the zero value as in_portal.
func (s *Session) EffectiveCertIssuanceMode() CertIssuanceMode {
	if s.CertIssuanceMode == "" {
		return CertIssuanceModeInPortal
	}
	return s.CertIssuanceMode
}

// Session represents an authenticated portal session scoped to a single hostname.
type Session struct {
	SessionID                 string
	Hostname                  string
	ExpiresAt                 time.Time
	BackURL                   string
	BackText                  string
	OwnershipVerificationMode OwnershipVerificationMode
	VerifyOwnershipURL        string
	VerifyOwnershipText       string
	CertIssuanceMode          CertIssuanceMode
}
