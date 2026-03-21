package dns

import "net"

// Lookup abstracts all DNS resolution so it can be replaced in tests.
type Lookup interface {
	LookupNS(name string) ([]*net.NS, error)
	LookupIP(name string) ([]net.IP, error)
	LookupCNAME(name string) (string, error)
	LookupHost(name string) ([]string, error)
	LookupTXT(name string) ([]string, error)
}

// NetLookup returns the production Lookup backed by the standard library.
func NetLookup() Lookup { return netLookup{} }

type netLookup struct{}

func (netLookup) LookupNS(name string) ([]*net.NS, error)  { return net.LookupNS(name) }
func (netLookup) LookupIP(name string) ([]net.IP, error)   { return net.LookupIP(name) }
func (netLookup) LookupCNAME(name string) (string, error)  { return net.LookupCNAME(name) }
func (netLookup) LookupHost(name string) ([]string, error) { return net.LookupHost(name) }
func (netLookup) LookupTXT(name string) ([]string, error)  { return net.LookupTXT(name) }
