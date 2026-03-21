package dns

import (
	"net"
	"testing"
)

// mockLookup is a test double for Lookup. Each field is optional;
// unset methods return a no-such-host DNS error by default.
type mockLookup struct {
	lookupNSFn    func(name string) ([]*net.NS, error)
	lookupIPFn    func(name string) ([]net.IP, error)
	lookupCNAMEFn func(name string) (string, error)
	lookupHostFn  func(name string) ([]string, error)
	lookupTXTFn   func(name string) ([]string, error)
}

func (m *mockLookup) LookupNS(name string) ([]*net.NS, error) {
	if m.lookupNSFn != nil {
		return m.lookupNSFn(name)
	}
	return nil, &net.DNSError{Name: name, Err: "no such host"}
}

func (m *mockLookup) LookupIP(name string) ([]net.IP, error) {
	if m.lookupIPFn != nil {
		return m.lookupIPFn(name)
	}
	return nil, &net.DNSError{Name: name, Err: "no such host"}
}

func (m *mockLookup) LookupCNAME(name string) (string, error) {
	if m.lookupCNAMEFn != nil {
		return m.lookupCNAMEFn(name)
	}
	return "", &net.DNSError{Name: name, Err: "no such host"}
}

func (m *mockLookup) LookupHost(name string) ([]string, error) {
	if m.lookupHostFn != nil {
		return m.lookupHostFn(name)
	}
	return nil, &net.DNSError{Name: name, Err: "no such host"}
}

func (m *mockLookup) LookupTXT(name string) ([]string, error) {
	if m.lookupTXTFn != nil {
		return m.lookupTXTFn(name)
	}
	return nil, &net.DNSError{Name: name, Err: "no such host"}
}

// --- isETLDPlusOne ---

func TestIsETLDPlusOne(t *testing.T) {
	tests := []struct {
		hostname string
		want     bool
	}{
		{"example.com", true},
		{"sub.example.com", false},
		{"deep.sub.example.com", false},
		{"example.co.uk", true},
		{"sub.example.co.uk", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.hostname, func(t *testing.T) {
			got := isETLDPlusOne(tc.hostname)
			if got != tc.want {
				t.Errorf("isETLDPlusOne(%q) = %v, want %v", tc.hostname, got, tc.want)
			}
		})
	}
}

// --- detectProviderFromNS ---

func TestDetectProviderFromNS(t *testing.T) {
	tests := []struct {
		ns   string
		want string
	}{
		{"ns1.ns.cloudflare.com.", ProviderCloudflare},
		{"ns-1234.awsdns-56.com.", ProviderRoute53},
		{"ns1.domaincontrol.com.", ProviderGoDaddy},
		{"ns-cloud-a1.googledomains.com.", ProviderGoogleCloud},
		{"ns1-01.azure-dns.com.", ProviderAzure},
		{"ns1.digitalocean.com.", ProviderDigitalOcean},
		{"dns1.nsone.net.", ProviderNS1},
		{"ns1.vercel-dns.com.", ProviderVercel},
		{"ns1.registrar-servers.com.", ProviderNamecheap},
		{"ns1.dnsimple.com.", ProviderDNSimple},
		{"ns1.cloudns.net.", ProviderClouDNS},
		{"ns1.dnspod.net.", ProviderDNSPod},
		{"ns1.hostinger.com.", ProviderHostinger},
		{"ns1.wixdns.net.", ProviderWix},
		{"ns1.ui-dns.com.", ProviderIONOS},
		{"ns1.ovh.net.", ProviderOVH},
		{"ns1.anycast.me.", ProviderOVH},
		{"ns1.unknown-provider.net.", ""},
	}
	for _, tc := range tests {
		t.Run(tc.ns, func(t *testing.T) {
			got := detectProviderFromNS(tc.ns)
			if got != tc.want {
				t.Errorf("detectProviderFromNS(%q) = %q, want %q", tc.ns, got, tc.want)
			}
		})
	}
}

// --- GetRequiredDNSRecords (subdomain paths only — no live DNS) ---

func TestGetRequiredDNSRecords_HTTP01_Subdomain(t *testing.T) {
	m := NewDNSRecordManager(ChallengeTypeHTTP01, "", "proxy.saas.example.com", &mockLookup{})
	records := m.GetRequiredDNSRecords("sub.tenant.com")

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	r := records[0]
	if r.Type != "CNAME" {
		t.Errorf("expected CNAME, got %s", r.Type)
	}
	if r.Name != "sub.tenant.com" {
		t.Errorf("expected name %q, got %q", "sub.tenant.com", r.Name)
	}
	if r.Value != "proxy.saas.example.com" {
		t.Errorf("expected value %q, got %q", "proxy.saas.example.com", r.Value)
	}
}

func TestGetRequiredDNSRecords_DNS01_Subdomain(t *testing.T) {
	m := NewDNSRecordManager(ChallengeTypeDNS01, "acme-delegate.saas.example.com",
		"proxy.saas.example.com", &mockLookup{})
	records := m.GetRequiredDNSRecords("sub.tenant.com")

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	cname := records[0]
	if cname.Type != "CNAME" {
		t.Errorf("expected first record to be CNAME, got %s", cname.Type)
	}
	if cname.Name != "sub.tenant.com" {
		t.Errorf("expected CNAME name %q, got %q", "sub.tenant.com", cname.Name)
	}

	txt := records[1]
	if txt.Type != "TXT" {
		t.Errorf("expected second record to be TXT, got %s", txt.Type)
	}
	if txt.Name != "_acme-challenge.sub.tenant.com" {
		t.Errorf("expected TXT name %q, got %q", "_acme-challenge.sub.tenant.com", txt.Name)
	}
	if txt.Value != "acme-delegate.saas.example.com" {
		t.Errorf("expected TXT value %q, got %q", "acme-delegate.saas.example.com", txt.Value)
	}
}

func TestGetRequiredDNSRecords_DeepSubdomain(t *testing.T) {
	m := NewDNSRecordManager(ChallengeTypeHTTP01, "", "ingress.saas.example.com", &mockLookup{})
	records := m.GetRequiredDNSRecords("a.b.tenant.com")

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Type != "CNAME" {
		t.Errorf("expected CNAME, got %s", records[0].Type)
	}
	if records[0].Value != "ingress.saas.example.com" {
		t.Errorf("unexpected value %q", records[0].Value)
	}
}

// --- GetRequiredDNSRecords — apex-domain paths (previously required live DNS) ---

// cloudflareNS returns an NS record that matches the Cloudflare provider pattern.
func cloudflareNS() []*net.NS {
	return []*net.NS{{Host: "ns1.ns.cloudflare.com."}}
}

func TestGetRequiredDNSRecords_ApexDomain_KnownFlatteningProvider_ReturnsCNAME(t *testing.T) {
	l := &mockLookup{
		lookupNSFn: func(_ string) ([]*net.NS, error) { return cloudflareNS(), nil },
	}
	m := NewDNSRecordManager(ChallengeTypeHTTP01, "", "proxy.saas.internal", l)
	records := m.GetRequiredDNSRecords("tenant.com") // apex domain

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Type != "CNAME" {
		t.Errorf("expected CNAME for Cloudflare apex, got %s", records[0].Type)
	}
	if records[0].Value != "proxy.saas.internal" {
		t.Errorf("unexpected CNAME value %q", records[0].Value)
	}
}

func TestGetRequiredDNSRecords_ApexDomain_UnknownProvider_ReturnsARecord(t *testing.T) {
	l := &mockLookup{
		lookupNSFn: func(_ string) ([]*net.NS, error) {
			return []*net.NS{{Host: "ns1.unknown-provider.net."}}, nil
		},
		// nameToIP resolves the cNameTarget to an IP
		lookupIPFn: func(_ string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("1.2.3.4")}, nil
		},
	}
	m := NewDNSRecordManager(ChallengeTypeHTTP01, "", "proxy.saas.internal", l)
	records := m.GetRequiredDNSRecords("tenant.com")

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Type != "A" {
		t.Errorf("expected A record for unknown-provider apex, got %s", records[0].Type)
	}
	if records[0].Value != "1.2.3.4" {
		t.Errorf("unexpected A record value %q", records[0].Value)
	}
}

func TestGetRequiredDNSRecords_ApexDomain_NSLookupFails_ReturnsEmpty(t *testing.T) {
	// mockLookup returns DNS errors by default, so no NS records found.
	m := NewDNSRecordManager(ChallengeTypeHTTP01, "", "proxy.saas.internal", &mockLookup{})
	records := m.GetRequiredDNSRecords("tenant.com")

	// nameToIP also fails (no lookupIPFn), so getPointingRecord errors → empty slice.
	if len(records) != 0 {
		t.Errorf("expected empty records when NS lookup fails, got %d", len(records))
	}
}
