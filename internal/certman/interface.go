package certman

import "context"

type CertMan interface {
	// HasCert checks if a certificate for the given hostname exists in the certificate storage.
	HasCert(ctx context.Context, hostname string) (bool, error)

	// PokeCert triggers the certificate issuance process for the given hostname.
	PokeCert(ctx context.Context, hostname string) error
}
