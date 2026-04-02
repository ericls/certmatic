package dns

import (
	"context"
	"net"
)

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

func DirectUDPLookup(nameserver string) Lookup {
	if nameserver == "" {
		nameserver = "1.1.1.1:53"
	}
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", nameserver)
		},
	}
	return directLookup{r: r}
}

type directLookup struct{ r *net.Resolver }

func (d directLookup) LookupNS(name string) ([]*net.NS, error) {
	return d.r.LookupNS(context.Background(), name)
}

func (d directLookup) LookupIP(name string) ([]net.IP, error) {
	addrs, err := d.r.LookupIPAddr(context.Background(), name)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, len(addrs))
	for i, a := range addrs {
		ips[i] = a.IP
	}
	return ips, nil
}

func (d directLookup) LookupCNAME(name string) (string, error) {
	return d.r.LookupCNAME(context.Background(), name)
}

func (d directLookup) LookupHost(name string) ([]string, error) {
	return d.r.LookupHost(context.Background(), name)
}

func (d directLookup) LookupTXT(name string) ([]string, error) {
	return d.r.LookupTXT(context.Background(), name)
}
