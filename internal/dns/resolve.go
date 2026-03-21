package dns

import (
	"fmt"
	"net"
	"strings"

	"golang.org/x/net/publicsuffix"
)

const (
	ProviderCloudflare   = "cloudflare"
	ProviderRoute53      = "route53"
	ProviderGoDaddy      = "godaddy"
	ProviderGoogleCloud  = "googlecloud"
	ProviderAzure        = "azure"
	ProviderDigitalOcean = "digitalocean"
	ProviderNS1          = "ns1"
	ProviderVercel       = "vercel"
	ProviderNamecheap    = "namecheap"
	ProviderDNSimple     = "dnsimple"
	ProviderClouDNS      = "cloudns"
	ProviderDNSPod       = "dnspod"
	ProviderHostinger    = "hostinger"
	ProviderWix          = "wix"
	ProviderIONOS        = "ionos"
	ProviderOVH          = "ovh"
)

// providerSupportsCNAMEFlattening indicates whether a provider supports
// CNAME records at the zone apex (eTLD+1) via flattening or ALIAS/ANAME records.
var providerSupportsCNAMEFlattening = map[string]bool{
	ProviderCloudflare:  true, // invented CNAME flattening
	ProviderRoute53:     true, // Alias records
	ProviderGoogleCloud: true, // synthetic ANAME-style records
	ProviderAzure:       true, // Alias records
	ProviderNS1:         true, // ALIAS records
	ProviderDNSimple:    true, // ALIAS records
	ProviderClouDNS:     true, // ALIAS records
	ProviderGoDaddy:     true, // ALIAS / forwarding at apex
}

// nsProviderPatterns maps substrings in NS hostnames to provider names.
// More specific patterns are listed first to avoid false matches.
var nsProviderPatterns = []struct {
	contains string
	provider string
}{
	{"ns.cloudflare.com", ProviderCloudflare},
	{"awsdns-", ProviderRoute53},
	{"domaincontrol.com", ProviderGoDaddy},
	{"googledomains.com", ProviderGoogleCloud},
	{"ns-cloud-", ProviderGoogleCloud},
	{"azure-dns.com", ProviderAzure},
	{"azure-dns.net", ProviderAzure},
	{"azure-dns.org", ProviderAzure},
	{"azure-dns.info", ProviderAzure},
	{"digitalocean.com", ProviderDigitalOcean},
	{"nsone.net", ProviderNS1},
	{"vercel-dns.com", ProviderVercel},
	{"registrar-servers.com", ProviderNamecheap},
	{"dnsimple.com", ProviderDNSimple},
	{"cloudns.net", ProviderClouDNS},
	{"dnspod.net", ProviderDNSPod},
	{"hostinger.com", ProviderHostinger},
	{"wixdns.net", ProviderWix},
	{"ui-dns.", ProviderIONOS},
	{"ovh.net", ProviderOVH},
	{"anycast.me", ProviderOVH},
}

func detectProviderFromNS(ns string) string {
	ns = strings.ToLower(strings.TrimSuffix(ns, "."))
	for _, p := range nsProviderPatterns {
		if strings.Contains(ns, p.contains) {
			return p.provider
		}
	}
	return ""
}

// lookupNSForZone walks up the domain hierarchy to find NS records,
// since NS records are typically only set at the zone apex.
func lookupNSForZone(hostname string, l Lookup) ([]*net.NS, error) {
	hostname = strings.ToLower(strings.TrimSuffix(hostname, "."))
	labels := strings.Split(hostname, ".")
	for i := 0; i < len(labels)-1; i++ {
		candidate := strings.Join(labels[i:], ".")
		nss, err := l.LookupNS(candidate)
		if err == nil && len(nss) > 0 {
			return nss, nil
		}
	}
	return nil, fmt.Errorf("no NS records found for %s", hostname)
}

// getDNSProvider returns the primary DNS provider for the given hostname
// by inspecting its authoritative NS records. Returns empty string if unknown.
// When NS records point to multiple providers, returns the one with the most records.
func getDNSProvider(hostname string, l Lookup) string {
	nss, err := lookupNSForZone(hostname, l)
	if err != nil || len(nss) == 0 {
		return ""
	}

	votes := make(map[string]int)
	for _, ns := range nss {
		if provider := detectProviderFromNS(ns.Host); provider != "" {
			votes[provider]++
		}
	}

	best, bestCount := "", 0
	for provider, count := range votes {
		if count > bestCount {
			best = provider
			bestCount = count
		}
	}
	return best
}

func isETLDPlusOne(hostname string) bool {
	hostname = strings.ToLower(strings.TrimSuffix(hostname, "."))
	if hostname == "" {
		return false
	}
	e, err := publicsuffix.EffectiveTLDPlusOne(hostname)
	if err != nil {
		return false
	}
	return e == hostname
}

// nameToIP resolves a hostname to its primary IP address, preferring IPv4.
// This function expects pre-defined, stable hostnames.
func nameToIP(name string, l Lookup) (net.IP, error) {
	ips, err := l.LookupIP(name)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4, nil
		}
	}
	if len(ips) > 0 {
		return ips[0], nil
	}
	return nil, fmt.Errorf("no IPs found for %s", name)
}
