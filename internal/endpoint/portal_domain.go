package endpoint

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ericls/certmatic/internal/certman"
	"github.com/ericls/certmatic/internal/dns"
	"github.com/ericls/certmatic/pkg/domain"
	pkgsession "github.com/ericls/certmatic/pkg/session"
	"github.com/ericls/certmatic/pkg/webhook"
)

type portalDomainEndpoint struct {
	domainRepo        domain.DomainRepo
	dnsRecordManager  *dns.DNSRecordManager
	certMan           certman.CertMan
	certWaitTimeout   time.Duration
	certPollInterval  time.Duration
	lookup            dns.Lookup
	webhookDispatcher webhook.Dispatcher
}

// --- GET /portal/api/domain ---

type certInfoResponse struct {
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
	Issuer    string    `json:"issuer"`
}

type portalDomainResponse struct {
	Hostname                  string                               `json:"hostname"`
	OwnershipVerified         bool                                 `json:"ownership_verified"`
	RequiredDNSRecords        []domain.DNSRecord                   `json:"required_dns_records"`
	Cert                      *certInfoResponse                    `json:"cert"`
	BackURL                   string                               `json:"back_url,omitempty"`
	BackText                  string                               `json:"back_text,omitempty"`
	OwnershipVerificationMode pkgsession.OwnershipVerificationMode `json:"ownership_verification_mode,omitempty"`
	OwnershipTXTRecord        *domain.DNSRecord                    `json:"ownership_txt_record,omitempty"`
	VerifyOwnershipURL        string                               `json:"verify_ownership_url,omitempty"`
	VerifyOwnershipText       string                               `json:"verify_ownership_text,omitempty"`
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

		var cert *certInfoResponse
		if e.certMan != nil {
			if info, _ := e.certMan.GetCertInfo(r.Context(), session.Hostname); info != nil {
				cert = &certInfoResponse{
					NotBefore: info.NotBefore,
					NotAfter:  info.NotAfter, Issuer: info.Issuer,
				}
			}
		}

		resp := portalDomainResponse{
			Hostname:                  sd.Domain.Hostname,
			OwnershipVerified:         sd.Domain.OwnershipVerified,
			RequiredDNSRecords:        e.dnsRecordManager.GetRequiredDNSRecords(session.Hostname),
			Cert:                      cert,
			BackURL:                   session.BackURL,
			BackText:                  session.BackText,
			OwnershipVerificationMode: session.OwnershipVerificationMode,
		}
		switch session.OwnershipVerificationMode {
		case pkgsession.OwnershipVerificationModeDNSChallenge:
			resp.OwnershipTXTRecord = &domain.DNSRecord{
				Type:  "TXT",
				Name:  "_certmatic-verify." + session.Hostname,
				Value: sd.Domain.VerificationToken,
			}
		case pkgsession.OwnershipVerificationModeProviderManaged:
			resp.VerifyOwnershipURL = session.VerifyOwnershipURL
			resp.VerifyOwnershipText = session.VerifyOwnershipText
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

type portalEnsureCertResponse struct {
	Hostname string `json:"hostname"`
	certInfoResponse
}

func (e *portalDomainEndpoint) handleEnsureCert() http.HandlerFunc {
	return JSONHandler(http.StatusOK, func(r *http.Request, _ struct{}) (portalEnsureCertResponse, error) {
		session := sessionFromContext(r.Context())
		if session == nil {
			return portalEnsureCertResponse{}, HTTPError{
				Status: http.StatusUnauthorized, Message: "session required",
			}
		}
		if e.certMan == nil {
			return portalEnsureCertResponse{}, HTTPError{
				Status: http.StatusServiceUnavailable, Message: "cert manager not available",
			}
		}
		hostname := session.Hostname
		if err := e.certMan.PokeCert(r.Context(), hostname); err != nil {
			return portalEnsureCertResponse{}, HTTPError{
				Status: http.StatusInternalServerError, Message: fmt.Sprintf(
					"error requesting certificate: %v", err),
			}
		}
		timeout := time.After(e.certWaitTimeout)
		waitDuration := e.certPollInterval
		for {
			certInfo, err := e.certMan.GetCertInfo(r.Context(), hostname)
			if err != nil {
				return portalEnsureCertResponse{}, HTTPError{
					Status:  http.StatusInternalServerError,
					Message: fmt.Sprintf("error checking certificate: %v", err),
				}
			}
			if certInfo != nil && certInfo.NotAfter.After(time.Now()) && certInfo.NotBefore.Before(time.Now()) {
				return portalEnsureCertResponse{
					Hostname: certInfo.Hostname,
					certInfoResponse: certInfoResponse{
						NotBefore: certInfo.NotBefore,
						NotAfter:  certInfo.NotAfter, Issuer: certInfo.Issuer,
					},
				}, nil
			}
			select {
			case <-timeout:
				return portalEnsureCertResponse{}, HTTPError{
					Status: http.StatusGatewayTimeout, Message: "timed out waiting for certificate to be issued",
				}
			case <-r.Context().Done():
				return portalEnsureCertResponse{}, HTTPError{
					Status: http.StatusServiceUnavailable, Message: "request cancelled",
				}
			default:
				time.Sleep(waitDuration)
				if waitDuration < 10*time.Second {
					waitDuration += e.certPollInterval
				}
			}
		}
	})
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
				checks = append(checks, checkCNAME(e.lookup, rec.Name, rec.Value))
			case "A":
				checks = append(checks, checkARecord(e.lookup, rec.Name, rec.Value))
			case "TXT":
				checks = append(checks, checkTXTRecord(e.lookup, rec.Name, rec.Value))
			}
		}

		// DNS challenge ownership check.
		if session.OwnershipVerificationMode == pkgsession.OwnershipVerificationModeDNSChallenge {
			ownershipCheck := checkOwnershipTXTRecord(e.lookup, hostname, sd.Domain.VerificationToken)
			checks = append(checks, ownershipCheck)
			if ownershipCheck.Status == checkStatusOK && !sd.Domain.OwnershipVerified {
				verified := true
				if err := e.domainRepo.Patch(r.Context(), hostname, domain.DomainPatch{
					OwnershipVerified: &verified,
				}); err != nil {
					return domainCheckResponse{}, err
				}
				sd.Domain.OwnershipVerified = true // update local copy for downstream checks in this response
				e.webhookDispatcher.Dispatch(webhook.Event{
					Type:      webhook.EventDomainVerified,
					Timestamp: time.Now(),
					Data:      map[string]any{"hostname": hostname},
				})
			}
		}

		// Ownership verified check.
		if sd.Domain.OwnershipVerified {
			checks = append(checks, domainCheck{
				Name:    checkNameOwnershipVerified,
				Status:  checkStatusOK,
				Message: "Domain ownership is verified.",
			})
		} else if session.OwnershipVerificationMode ==
			pkgsession.OwnershipVerificationModeProviderManaged {
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

func checkCNAME(r dns.Lookup, name, expected string) domainCheck {
	cname, err := r.LookupCNAME(name)
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

func checkARecord(r dns.Lookup, name, expected string) domainCheck {
	addrs, err := r.LookupHost(name)
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

func checkTXTRecord(r dns.Lookup, name, expected string) domainCheck {
	txts, err := r.LookupTXT(name)
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

func checkOwnershipTXTRecord(r dns.Lookup, hostname, token string) domainCheck {
	name := "_certmatic-verify." + hostname
	txts, err := r.LookupTXT(name)
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
