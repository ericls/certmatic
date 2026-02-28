package domain

import (
	"errors"
)

var (
	ErrNotFound             = errors.New("domain not found")
	ErrOwnershipNotVerified = errors.New("domain ownership not verified")
)

type Domain struct {
	Hostname string `json:"hostname" yaml:"hostname"`
	// TenantID identifies the tenant/customer this domain belongs to.
	// This value is opaque to the system, users can use any string format they prefer.
	TenantID string `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`

	OwnershipVerified bool `json:"ownership_verified" yaml:"ownership_verified"`

	// Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

func (d *Domain) Clone() *Domain {
	if d == nil {
		return nil
	}
	clone := *d
	// if d.Metadata != nil {
	// 	clone.Metadata = make(map[string]string, len(d.Metadata))
	// 	maps.Copy(clone.Metadata, d.Metadata)
	// }
	return &clone
}
