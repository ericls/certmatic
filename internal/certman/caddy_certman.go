package certman

import (
	"context"

	"github.com/caddyserver/caddy/v2/modules/caddytls"
	"github.com/caddyserver/certmagic"
)

type CaddyCertMan struct {
	storage certmagic.Storage
	tlsApp  *caddytls.TLS
}

func NewCaddyCertMan(storage certmagic.Storage, tlsApp *caddytls.TLS) *CaddyCertMan {
	return &CaddyCertMan{
		storage: storage,
		tlsApp:  tlsApp,
	}
}

func (c *CaddyCertMan) HasCert(ctx context.Context, hostname string) (bool, error) {
	// This depends on implementation details of certmagic's storage structure.
	// Certmagic currently stores certificates under the path: "certificates/{issuer}/{hostname}/{hostname}.crt".
	// TODO: Maybe ask certmagic to provide a more direct API for this in the future
	prefix := "certificates/"
	keys, err := c.storage.List(ctx, prefix, false)
	if err != nil {
		return false, err
	}

	for _, issuerDir := range keys {
		certKey := issuerDir + "/" + hostname + "/" + hostname + ".crt"
		if c.storage.Exists(ctx, certKey) {
			return true, nil
		}
	}
	return false, nil
}

func (c *CaddyCertMan) PokeCert(ctx context.Context, hostname string) error {
	return c.tlsApp.Manage(map[string]struct{}{hostname: {}})
}
