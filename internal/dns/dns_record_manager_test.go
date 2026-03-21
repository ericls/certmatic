package dns

import "testing"

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
	m := NewDNSRecordManager(ChallengeTypeHTTP01, "", "proxy.saas.example.com")
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
	m := NewDNSRecordManager(ChallengeTypeDNS01, "acme-delegate.saas.example.com", "proxy.saas.example.com")
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
	m := NewDNSRecordManager(ChallengeTypeHTTP01, "", "ingress.saas.example.com")
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
