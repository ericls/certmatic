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
}
