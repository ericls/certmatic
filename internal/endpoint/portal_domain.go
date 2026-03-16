package endpoint

import (
	"net"
	"net/http"
	"strings"

	"github.com/ericls/certmatic/internal/certman"
	"github.com/ericls/certmatic/internal/dns"
	"github.com/ericls/certmatic/internal/portal"
	"github.com/ericls/certmatic/pkg/domain"
)

type portalDomainEndpoint struct {
	domainRepo       domain.DomainRepo
	dnsRecordManager *dns.DNSRecordManager
	certMan          certman.CertMan
}

// --- GET /portal/api/domain ---

type certStatusResponse struct {
	HasCert bool `json:"has_cert"`
}

type portalDomainResponse struct {
	Hostname                  string                           `json:"hostname"`
	OwnershipVerified         bool                             `json:"ownership_verified"`
	RequiredDNSRecords        []domain.DNSRecord               `json:"required_dns_records"`
	CertStatus                certStatusResponse               `json:"cert_status"`
	BackURL                   string                           `json:"back_url,omitempty"`
	BackText                  string                           `json:"back_text,omitempty"`
	OwnershipVerificationMode portal.OwnershipVerificationMode `json:"ownership_verification_mode,omitempty"`
	OwnershipTXTRecord        *domain.DNSRecord                `json:"ownership_txt_record,omitempty"`
	VerifyOwnershipURL        string                           `json:"verify_ownership_url,omitempty"`
	VerifyOwnershipText       string                           `json:"verify_ownership_text,omitempty"`
}

func (e *portalDomainEndpoint) handleGetDomain() http.HandlerFunc {
	return JSONHandler(http.StatusOK, func(r *http.Request, _ struct{}) (portalDomainResponse, error) {
		session := sessionFromContext(r.Context())
		if session == nil {
			return portalDomainResponse{}, HTTPError{Status: http.StatusUnauthorized, Message: "session required"}
		}

		sd, err := e.domainRepo.Get(r.Context(), session.Hostname)
		if err == domain.ErrNotFound {
			return portalDomainResponse{}, HTTPError{Status: http.StatusNotFound, Message: "domain not found"}
		}
		if err != nil {
			return portalDomainResponse{}, err
		}

		hasCert := false
		if e.certMan != nil {
			hasCert, _ = e.certMan.HasCert(r.Context(), session.Hostname)
		}

		resp := portalDomainResponse{
			Hostname:                  sd.Domain.Hostname,
			OwnershipVerified:         sd.Domain.OwnershipVerified,
			RequiredDNSRecords:        e.dnsRecordManager.GetRequiredDNSRecords(session.Hostname),
			CertStatus:                certStatusResponse{HasCert: hasCert},
			BackURL:                   session.BackURL,
			BackText:                  session.BackText,
			OwnershipVerificationMode: session.OwnershipVerificationMode,
		}
		switch session.OwnershipVerificationMode {
		case portal.OwnershipVerificationModeDNSChallenge:
			resp.OwnershipTXTRecord = &domain.DNSRecord{
				Type:  "TXT",
				Name:  "_certmatic-verify." + session.Hostname,
				Value: sd.Domain.VerificationToken,
			}
		case portal.OwnershipVerificationModeProviderManaged:
			resp.VerifyOwnershipURL = session.VerifyOwnershipURL
			resp.VerifyOwnershipText = session.VerifyOwnershipText
			if resp.VerifyOwnershipText == "" {
				resp.VerifyOwnershipText = "Verify Ownership"
			}
		}
		return resp, nil
	})
}

// --- POST /portal/api/domain/check ---

type checkStatus string

const (
	checkStatusOK      checkStatus = "ok"
	checkStatusFail    checkStatus = "fail"
	checkStatusPending checkStatus = "pending"
)

type checkName string

const (
	checkNameCNAMERecord        checkName = "cname_record"
	checkNameARecord            checkName = "a_record"
	checkNameTXTRecord          checkName = "txt_record"
	checkNameOwnershipVerified  checkName = "ownership_verified"
	checkNameOwnershipTXTRecord checkName = "ownership_txt_record"
	checkNameCertificate        checkName = "certificate"
)

type domainCheck struct {
	Name     checkName   `json:"name"`
	Status   checkStatus `json:"status"`
	Expected string      `json:"expected,omitempty"`
	Actual   string      `json:"actual,omitempty"`
	Message  string      `json:"message"`
}

type domainCheckResponse struct {
	Hostname string        `json:"hostname"`
	Checks   []domainCheck `json:"checks"`
	Overall  checkStatus   `json:"overall"`
}

func (e *portalDomainEndpoint) handleDomainCheck() http.HandlerFunc {
	return JSONHandler(http.StatusOK, func(r *http.Request, _ struct{}) (domainCheckResponse, error) {
		session := sessionFromContext(r.Context())
		if session == nil {
			return domainCheckResponse{}, HTTPError{Status: http.StatusUnauthorized, Message: "session required"}
		}

		hostname := session.Hostname

		sd, err := e.domainRepo.Get(r.Context(), hostname)
		if err == domain.ErrNotFound {
			return domainCheckResponse{}, HTTPError{Status: http.StatusNotFound, Message: "domain not found"}
		}
		if err != nil {
			return domainCheckResponse{}, err
		}

		required := e.dnsRecordManager.GetRequiredDNSRecords(hostname)

		var checks []domainCheck

		// Check each required DNS record.
		for _, rec := range required {
			switch rec.Type {
			case "CNAME":
				checks = append(checks, checkCNAME(rec.Name, rec.Value))
			case "A":
				checks = append(checks, checkARecord(rec.Name, rec.Value))
			case "TXT":
				checks = append(checks, checkTXTRecord(rec.Name, rec.Value))
			}
		}

		// DNS challenge ownership check.
		if session.OwnershipVerificationMode == portal.OwnershipVerificationModeDNSChallenge {
			ownershipCheck := checkOwnershipTXTRecord(hostname, sd.Domain.VerificationToken)
			checks = append(checks, ownershipCheck)
			if ownershipCheck.Status == checkStatusOK && !sd.Domain.OwnershipVerified {
				sd.Domain.OwnershipVerified = true
				if err := e.domainRepo.Set(r.Context(), sd.Domain); err != nil {
					return domainCheckResponse{}, err
				}
			}
		}

		// Ownership verified check.
		if sd.Domain.OwnershipVerified {
			checks = append(checks, domainCheck{
				Name:    checkNameOwnershipVerified,
				Status:  checkStatusOK,
				Message: "Domain ownership is verified.",
			})
		} else if session.OwnershipVerificationMode == portal.OwnershipVerificationModeProviderManaged {
			checks = append(checks, domainCheck{
				Name:    checkNameOwnershipVerified,
				Status:  checkStatusFail,
				Message: "Use the verify button to complete verification.",
			})
		} else {
			checks = append(checks, domainCheck{
				Name:    checkNameOwnershipVerified,
				Status:  checkStatusFail,
				Message: "Domain ownership not yet verified. Ensure DNS records are set, then wait up to 5 minutes.",
			})
		}

		// Certificate check.
		hasCert := false
		if e.certMan != nil {
			hasCert, _ = e.certMan.HasCert(r.Context(), hostname)
		}
		if hasCert {
			checks = append(checks, domainCheck{
				Name:    checkNameCertificate,
				Status:  checkStatusOK,
				Message: "Certificate is issued and valid.",
			})
		} else if !sd.Domain.OwnershipVerified {
			checks = append(checks, domainCheck{
				Name:    checkNameCertificate,
				Status:  checkStatusPending,
				Message: "Certificate not yet issued. Ownership verification is required first.",
			})
		} else {
			checks = append(checks, domainCheck{
				Name:    checkNameCertificate,
				Status:  checkStatusPending,
				Message: "Certificate issuance in progress.",
			})
		}

		overall := overallStatus(checks)
		return domainCheckResponse{
			Hostname: hostname,
			Checks:   checks,
			Overall:  overall,
		}, nil
	})
}

func checkCNAME(name, expected string) domainCheck {
	cname, err := net.LookupCNAME(name)
	if err != nil {
		return domainCheck{
			Name:     checkNameCNAMERecord,
			Status:   checkStatusFail,
			Expected: expected,
			Message:  "CNAME record not found: " + err.Error(),
		}
	}
	actual := strings.TrimSuffix(cname, ".")
	if strings.EqualFold(actual, expected) {
		return domainCheck{
			Name:     checkNameCNAMERecord,
			Status:   checkStatusOK,
			Expected: expected,
			Actual:   actual,
			Message:  "CNAME record is correctly configured.",
		}
	}
	return domainCheck{
		Name:     checkNameCNAMERecord,
		Status:   checkStatusFail,
		Expected: expected,
		Actual:   actual,
		Message:  "CNAME record points to wrong destination.",
	}
}

func checkARecord(name, expected string) domainCheck {
	addrs, err := net.LookupHost(name)
	if err != nil {
		return domainCheck{
			Name:     checkNameARecord,
			Status:   checkStatusFail,
			Expected: expected,
			Message:  "A record not found: " + err.Error(),
		}
	}
	for _, addr := range addrs {
		if addr == expected {
			return domainCheck{
				Name:     checkNameARecord,
				Status:   checkStatusOK,
				Expected: expected,
				Actual:   addr,
				Message:  "A record is correctly configured.",
			}
		}
	}
	actual := ""
	if len(addrs) > 0 {
		actual = addrs[0]
	}
	return domainCheck{
		Name:     checkNameARecord,
		Status:   checkStatusFail,
		Expected: expected,
		Actual:   actual,
		Message:  "A record does not match expected IP.",
	}
}

func checkTXTRecord(name, expected string) domainCheck {
	txts, err := net.LookupTXT(name)
	if err != nil {
		return domainCheck{
			Name:     checkNameTXTRecord,
			Status:   checkStatusFail,
			Expected: expected,
			Message:  "TXT record not found: " + err.Error(),
		}
	}
	for _, txt := range txts {
		if txt == expected {
			return domainCheck{
				Name:     checkNameTXTRecord,
				Status:   checkStatusOK,
				Expected: expected,
				Actual:   txt,
				Message:  "TXT record is correctly configured.",
			}
		}
	}
	actual := ""
	if len(txts) > 0 {
		actual = txts[0]
	}
	return domainCheck{
		Name:     checkNameTXTRecord,
		Status:   checkStatusFail,
		Expected: expected,
		Actual:   actual,
		Message:  "TXT record value does not match.",
	}
}

func checkOwnershipTXTRecord(hostname, token string) domainCheck {
	name := "_certmatic-verify." + hostname
	txts, err := net.LookupTXT(name)
	if err != nil {
		return domainCheck{
			Name:     checkNameOwnershipTXTRecord,
			Status:   checkStatusFail,
			Expected: token,
			Message:  "Ownership TXT record not found: " + err.Error(),
		}
	}
	for _, txt := range txts {
		if txt == token {
			return domainCheck{
				Name:     checkNameOwnershipTXTRecord,
				Status:   checkStatusOK,
				Expected: token,
				Actual:   txt,
				Message:  "Ownership TXT record is correctly configured.",
			}
		}
	}
	actual := ""
	if len(txts) > 0 {
		actual = txts[0]
	}
	return domainCheck{
		Name:     checkNameOwnershipTXTRecord,
		Status:   checkStatusFail,
		Expected: token,
		Actual:   actual,
		Message:  "Ownership TXT record value does not match.",
	}
}

func overallStatus(checks []domainCheck) checkStatus {
	hasFail := false
	hasPending := false
	for _, c := range checks {
		switch c.Status {
		case checkStatusFail:
			hasFail = true
		case checkStatusPending:
			hasPending = true
		}
	}
	if hasFail {
		return checkStatusFail
	}
	if hasPending {
		return checkStatusPending
	}
	return checkStatusOK
}
