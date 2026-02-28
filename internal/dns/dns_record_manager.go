package dns

import (
	domain "github.com/ericls/certmatic/pkg/domain"
)

type ChallengeType string

const (
	ChallengeTypeHTTP01 ChallengeType = "http-01"
	ChallengeTypeDNS01  ChallengeType = "dns-01"
)

type DNSRecordManager struct {
	challengeType ChallengeType
	// For DNS01 challenge delegation.
	// If a host name is "shop.foo.com", and dnsDelegationDomain is "acme_delegation.saas.com",
	// then we will ask user to create a CNAME record for "_acme-challenge.shop.foo.com" pointing to "acme_delegation.saas.com".
	dnsDelegationDomain string
	cNameTarget         string
}

func NewDNSRecordManager(challengeType ChallengeType, dnsDelegationDomain, cNameTarget string) *DNSRecordManager {
	return &DNSRecordManager{
		challengeType:       challengeType,
		dnsDelegationDomain: dnsDelegationDomain,
		cNameTarget:         cNameTarget,
	}
}

func (m *DNSRecordManager) GetRequiredDNSRecords(hostname string) []domain.DNSRecord {
	records := []domain.DNSRecord{}
	pointingRecord, err := m.getPointingRecord(hostname)
	if err != nil {
		return records
	}
	records = append(records, pointingRecord)
	if m.challengeType == ChallengeTypeDNS01 {
		records = append(records, domain.DNSRecord{
			Type:  "TXT",
			Name:  "_acme-challenge." + hostname,
			Value: m.dnsDelegationDomain,
		})
	}
	return records
}

func (m *DNSRecordManager) supportCNameFlattening(hostname string) bool {
	provider := getDNSProvider(hostname)
	return providerSupportsCNAMEFlattening[provider]
}

// Get the DNS record to point domain to our ingress.
func (m *DNSRecordManager) getPointingRecord(hostname string) (domain.DNSRecord, error) {
	useCName := true
	if isETLDPlusOne(hostname) {
		if !m.supportCNameFlattening(hostname) {
			useCName = false
		}
	}
	if useCName {
		return domain.DNSRecord{
			Type:  "CNAME",
			Name:  hostname,
			Value: m.cNameTarget,
		}, nil
	}
	ip, err := nameToIP(m.cNameTarget)
	if err != nil {
		return domain.DNSRecord{}, err
	}
	return domain.DNSRecord{
		Type:  "A",
		Name:  hostname,
		Value: ip.String(),
	}, nil
}
