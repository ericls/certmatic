package certman

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"

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

func (c *CaddyCertMan) GetCertInfo(ctx context.Context, hostname string) (*CertInfo, error) {
	// This is a bit more complex since we need to read the certificate and parse it to get the info.
	// Again, this depends on certmagic's storage structure.
	prefix := "certificates/"
	keys, err := c.storage.List(ctx, prefix, false)
	if err != nil {
		return nil, err
	}

	for _, issuerDir := range keys {
		certKey := issuerDir + "/" + hostname + "/" + hostname + ".crt"
		if c.storage.Exists(ctx, certKey) {
			certData, err := c.storage.Load(ctx, certKey)
			if err != nil {
				return nil, err
			}
			certInfo, err := parseCertInfo(hostname, certData)
			if err != nil {
				return nil, err
			}
			return certInfo, nil
		}
	}
	return nil, nil // Not found
}

func (c *CaddyCertMan) PokeCert(ctx context.Context, hostname string) error {
	// NOTE: Known limitation — this is a no-op if the cert is already in certmagic's
	// in-memory cache (managed=true), even if the .crt has been deleted from storage.
	// certmagic.Config.manageOne() short-circuits on a cache hit before checking storage.
	// See the DeleteCert comment below for the full picture and the fix path.
	return c.tlsApp.Manage(map[string]struct{}{hostname: {}})
}

func (c *CaddyCertMan) DeleteCert(ctx context.Context, hostname string) error {
	// NOTE: DeleteCert only removes the .crt file from
	// certmagic's persistent storage. It does NOT evict the certificate from
	// certmagic's in-memory Cache (a *certmagic.Cache held as the package-level
	// variable `certCache` in caddytls, unexported).
	//
	// Consequence: after DeleteCert, a call to PokeCert → tlsApp.Manage →
	// certmagic.manageOne() finds the cert in the in-memory cache with managed=true
	// and returns immediately without triggering ACME. Any subsequent poll loop that
	// reads the .crt back from storage will time out because storage has no .crt.
	//
	// TODO: Consider upstream a change to certmagic to add a method to evict a cert from the in-memory cache
	prefix := "certificates/"
	keys, err := c.storage.List(ctx, prefix, false)
	if err != nil {
		return err
	}

	for _, issuerDir := range keys {
		certKey := issuerDir + "/" + hostname + "/" + hostname + ".crt"
		if c.storage.Exists(ctx, certKey) {
			return c.storage.Delete(ctx, certKey)
		}
	}
	return nil
}

func parseCertInfo(hostname string, certData []byte) (*CertInfo, error) {
	block, _ := pem.Decode(certData)
	if block == nil {
		return nil, fmt.Errorf("failed to parse certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	fmt.Println(cert.Subject)
	return &CertInfo{
		Hostname:  hostname,
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
		Issuer:    cert.Issuer.CommonName,
	}, nil
}
