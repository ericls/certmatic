package certman

import (
	"context"
	"time"
)

type CertInfo struct {
	Hostname  string
	NotBefore time.Time
	NotAfter  time.Time
	Issuer    string
}

type CertMan interface {
	// HasCert checks if a certificate for the given hostname exists in the certificate storage.
	HasCert(ctx context.Context, hostname string) (bool, error)

	// GetCertInfo retrieves information about the certificate for the given hostname, if it exists.
	GetCertInfo(ctx context.Context, hostname string) (*CertInfo, error)

	// PokeCert triggers the certificate issuance process for the given hostname.
	PokeCert(ctx context.Context, hostname string) error

	// DeleteCert removes a matching certificate from storage, if it exists. This is useful for testing and cleanup.
	DeleteCert(ctx context.Context, hostname string) error
}
