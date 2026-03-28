package endpoint

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ericls/certmatic/internal/certman"
	"github.com/ericls/certmatic/internal/dns"
	domainrepo "github.com/ericls/certmatic/internal/repo/domain"
	"github.com/ericls/certmatic/pkg/domain"
	pkgsession "github.com/ericls/certmatic/pkg/session"
	"github.com/ericls/certmatic/pkg/webhook"
)

// withSession injects a portal session into a request's context.
func withSession(r *http.Request, session *pkgsession.Session) *http.Request {
	ctx := context.WithValue(r.Context(), sessionContextKey, session)
	return r.WithContext(ctx)
}

// newPortalTestEndpoint creates a portalDomainEndpoint wired for testing.
// certPollInterval=0 and a short timeout make polling tests fast.
// Uses a mock DNS lookup (returns errors) — safe for tests that don't need real DNS.
func newPortalTestEndpoint(repo domain.DomainRepo, cm certman.CertMan) *portalDomainEndpoint {
	return newPortalTestEndpointWithLookup(repo, cm, &mockLookup{})
}

func newPortalTestEndpointWithLookup(repo domain.DomainRepo, cm certman.CertMan, l dns.Lookup) *portalDomainEndpoint {
	return &portalDomainEndpoint{
		domainRepo:        repo,
		dnsRecordManager:  dns.NewDNSRecordManager(dns.ChallengeTypeHTTP01, "", "proxy.saas.internal", l),
		certMan:           cm,
		certWaitTimeout:   20 * time.Millisecond,
		certPollInterval:  0,
		lookup:            l,
		webhookDispatcher: webhook.NoopDispatcher{},
	}
}

// mockLookup lets tests control DNS lookup results.
// Unset methods return a no-such-host DNS error by default.
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

// portalSession returns a minimal session for sub.tenant.com.
// Uses ProviderManaged mode to avoid triggering live DNS lookups in handleDomainCheck.
func portalSession(hostname string) *pkgsession.Session {
	return &pkgsession.Session{
		SessionID:                 "aaaabbbb-0000-0000-0000-000000000001",
		Hostname:                  hostname,
		ExpiresAt:                 time.Now().Add(time.Hour),
		OwnershipVerificationMode: pkgsession.OwnershipVerificationModeProviderManaged,
	}
}

// --- overallStatus (pure function, no I/O) ---

func TestOverallStatus(t *testing.T) {
	tests := []struct {
		name   string
		checks []domainCheck
		want   checkStatus
	}{
		{"all ok", []domainCheck{{Status: checkStatusOK}, {Status: checkStatusOK}}, checkStatusOK},
		{"has fail", []domainCheck{{Status: checkStatusOK}, {Status: checkStatusFail}}, checkStatusFail},
		{"pending only", []domainCheck{{Status: checkStatusOK}, {Status: checkStatusPending}}, checkStatusPending},
		{"fail beats pending", []domainCheck{{Status: checkStatusFail}, {
			Status: checkStatusPending,
		}}, checkStatusFail},
		{"empty", []domainCheck{}, checkStatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := overallStatus(tc.checks)
			if got != tc.want {
				t.Errorf("overallStatus = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- handleGetDomain ---

func TestPortalGetDomain_NoSession(t *testing.T) {
	e := newPortalTestEndpoint(domainrepo.NewInMemoryDomainRepo("test"), nil)
	rec := httptest.NewRecorder()
	e.handleGetDomain().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestPortalGetDomain_DomainNotFound(t *testing.T) {
	e := newPortalTestEndpoint(domainrepo.NewInMemoryDomainRepo("test"), nil)
	req := withSession(httptest.NewRequest(http.MethodGet, "/", nil), portalSession("missing.tenant.com"))
	rec := httptest.NewRecorder()
	e.handleGetDomain().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestPortalGetDomain_Found_NoCertManager(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{
		Hostname:          "sub.tenant.com",
		OwnershipVerified: true,
	})
	e := newPortalTestEndpoint(repo, nil)

	req := withSession(httptest.NewRequest(http.MethodGet, "/", nil), portalSession("sub.tenant.com"))
	rec := httptest.NewRecorder()
	e.handleGetDomain().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	resp := decodeData[portalDomainResponse](t, rec)
	if resp.Hostname != "sub.tenant.com" {
		t.Errorf("expected hostname %q, got %q", "sub.tenant.com", resp.Hostname)
	}
	if !resp.OwnershipVerified {
		t.Error("expected ownership_verified=true")
	}
	if resp.Cert != nil {
		t.Error("expected no cert when cert manager is nil")
	}
}

func TestPortalGetDomain_WithCert(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{Hostname: "sub.tenant.com"})

	cm := &mockCertMan{
		getCertInfoFn: func(_ context.Context, _ string) (*certman.CertInfo, error) {
			return &certman.CertInfo{
				Hostname:  "sub.tenant.com",
				NotBefore: time.Now().Add(-time.Hour),
				NotAfter:  time.Now().Add(time.Hour),
				Issuer:    "Test CA",
			}, nil
		},
	}
	e := newPortalTestEndpoint(repo, cm)

	req := withSession(httptest.NewRequest(http.MethodGet, "/", nil), portalSession("sub.tenant.com"))
	rec := httptest.NewRecorder()
	e.handleGetDomain().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	resp := decodeData[portalDomainResponse](t, rec)
	if resp.Cert == nil {
		t.Fatal("expected cert info in response")
	}
	if resp.Cert.Issuer != "Test CA" {
		t.Errorf("expected issuer %q, got %q", "Test CA", resp.Cert.Issuer)
	}
}

func TestPortalGetDomain_DNSChallengeMode_OwnershipTXTRecord(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{
		Hostname:          "sub.tenant.com",
		VerificationToken: "verify-token-abc",
	})
	e := newPortalTestEndpoint(repo, nil)

	session := &pkgsession.Session{
		SessionID:                 "test-session",
		Hostname:                  "sub.tenant.com",
		ExpiresAt:                 time.Now().Add(time.Hour),
		OwnershipVerificationMode: pkgsession.OwnershipVerificationModeDNSChallenge,
	}
	req := withSession(httptest.NewRequest(http.MethodGet, "/", nil), session)
	rec := httptest.NewRecorder()
	e.handleGetDomain().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	resp := decodeData[portalDomainResponse](t, rec)
	if resp.OwnershipTXTRecord == nil {
		t.Fatal("expected ownership_txt_record for dns_challenge mode")
	}
	if resp.OwnershipTXTRecord.Value != "verify-token-abc" {
		t.Errorf("expected TXT value %q, got %q", "verify-token-abc", resp.OwnershipTXTRecord.Value)
	}
	if resp.OwnershipTXTRecord.Name != "_certmatic-verify.sub.tenant.com" {
		t.Errorf("unexpected TXT name %q", resp.OwnershipTXTRecord.Name)
	}
}

func TestPortalGetDomain_ProviderManagedMode_VerifyURL(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{Hostname: "sub.tenant.com"})
	e := newPortalTestEndpoint(repo, nil)

	session := &pkgsession.Session{
		SessionID:                 "test-session",
		Hostname:                  "sub.tenant.com",
		ExpiresAt:                 time.Now().Add(time.Hour),
		OwnershipVerificationMode: pkgsession.OwnershipVerificationModeProviderManaged,
		VerifyOwnershipURL:        "https://app.example.com/verify",
		VerifyOwnershipText:       "Verify Ownership",
	}
	req := withSession(httptest.NewRequest(http.MethodGet, "/", nil), session)
	rec := httptest.NewRecorder()
	e.handleGetDomain().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	resp := decodeData[portalDomainResponse](t, rec)
	if resp.VerifyOwnershipURL != "https://app.example.com/verify" {
		t.Errorf("expected verify URL %q, got %q", "https://app.example.com/verify", resp.VerifyOwnershipURL)
	}
	if resp.OwnershipTXTRecord != nil {
		t.Error("expected no ownership_txt_record for provider_managed mode")
	}
}

// --- handleEnsureCert ---

func TestPortalEnsureCert_NoSession(t *testing.T) {
	e := newPortalTestEndpoint(domainrepo.NewInMemoryDomainRepo("test"), nil)
	rec := httptest.NewRecorder()
	e.handleEnsureCert().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestPortalEnsureCert_NoCertManager(t *testing.T) {
	e := newPortalTestEndpoint(domainrepo.NewInMemoryDomainRepo("test"), nil)
	req := withSession(httptest.NewRequest(http.MethodPost, "/", nil), portalSession("sub.tenant.com"))
	rec := httptest.NewRecorder()
	e.handleEnsureCert().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestPortalEnsureCert_Success(t *testing.T) {
	cm := &mockCertMan{
		pokeCertFn: func(_ context.Context, _ string) error { return nil },
		getCertInfoFn: func(_ context.Context, _ string) (*certman.CertInfo, error) {
			return &certman.CertInfo{
				Hostname:  "sub.tenant.com",
				NotBefore: time.Now().Add(-time.Hour),
				NotAfter:  time.Now().Add(time.Hour),
				Issuer:    "Test CA",
			}, nil
		},
	}
	e := newPortalTestEndpoint(domainrepo.NewInMemoryDomainRepo("test"), cm)

	req := withSession(httptest.NewRequest(http.MethodPost, "/", nil), portalSession("sub.tenant.com"))
	rec := httptest.NewRecorder()
	e.handleEnsureCert().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	resp := decodeData[portalEnsureCertResponse](t, rec)
	if resp.Hostname != "sub.tenant.com" {
		t.Errorf("expected hostname %q, got %q", "sub.tenant.com", resp.Hostname)
	}
	if resp.Issuer != "Test CA" {
		t.Errorf("expected issuer %q, got %q", "Test CA", resp.Issuer)
	}
}

func TestPortalEnsureCert_PokeError(t *testing.T) {
	cm := &mockCertMan{
		pokeCertFn: func(_ context.Context, _ string) error { return errors.New("poke failed") },
	}
	e := newPortalTestEndpoint(domainrepo.NewInMemoryDomainRepo("test"), cm)

	req := withSession(httptest.NewRequest(http.MethodPost, "/", nil), portalSession("sub.tenant.com"))
	rec := httptest.NewRecorder()
	e.handleEnsureCert().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestPortalEnsureCert_Timeout(t *testing.T) {
	cm := &mockCertMan{
		pokeCertFn:    func(_ context.Context, _ string) error { return nil },
		getCertInfoFn: func(_ context.Context, _ string) (*certman.CertInfo, error) { return nil, nil },
	}
	e := newPortalTestEndpoint(domainrepo.NewInMemoryDomainRepo("test"), cm)

	req := withSession(httptest.NewRequest(http.MethodPost, "/", nil), portalSession("sub.tenant.com"))
	rec := httptest.NewRecorder()
	e.handleEnsureCert().ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("expected %d, got %d", http.StatusGatewayTimeout, rec.Code)
	}
}

// --- handleDomainCheck ---
//
// Tests use OwnershipVerificationModeProviderManaged to avoid live DNS lookups
// from checkOwnershipTXTRecord. The CNAME check on "sub.tenant.com" will still
// call net.LookupCNAME, but will fail quickly since the domain is not real;
// the tests assert on ownership/cert checks that are independent of DNS results.
//
// The DNS-challenge auto-ownership-verification path (checkOwnershipTXTRecord
// returning OK → setting OwnershipVerified) is not covered here without injecting
// a mock DNS resolver.

func TestPortalDomainCheck_NoSession(t *testing.T) {
	e := newPortalTestEndpoint(domainrepo.NewInMemoryDomainRepo("test"), nil)
	rec := httptest.NewRecorder()
	e.handleDomainCheck().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestPortalDomainCheck_DomainNotFound(t *testing.T) {
	e := newPortalTestEndpoint(domainrepo.NewInMemoryDomainRepo("test"), nil)
	req := withSession(httptest.NewRequest(http.MethodPost, "/", nil), portalSession("missing.tenant.com"))
	rec := httptest.NewRecorder()
	e.handleDomainCheck().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func findCheck(checks []domainCheck, name checkName) *domainCheck {
	for _, c := range checks {
		if c.Name == name {
			cp := c
			return &cp
		}
	}
	return nil
}

func TestPortalDomainCheck_OwnershipAlreadyVerified(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{
		Hostname:          "sub.tenant.com",
		OwnershipVerified: true,
	})
	cm := &mockCertMan{
		hasCertFn: func(_ context.Context, _ string) (bool, error) { return false, nil },
	}
	e := newPortalTestEndpointWithLookup(repo, cm, &mockLookup{})

	req := withSession(httptest.NewRequest(http.MethodPost, "/", nil), portalSession("sub.tenant.com"))
	rec := httptest.NewRecorder()
	e.handleDomainCheck().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	resp := decodeData[domainCheckResponse](t, rec)
	if resp.Hostname != "sub.tenant.com" {
		t.Errorf("expected hostname %q, got %q", "sub.tenant.com", resp.Hostname)
	}

	c := findCheck(resp.Checks, checkNameOwnershipVerified)
	if c == nil {
		t.Fatal("expected ownership_verified check in response")
	}
	if c.Status != checkStatusOK {
		t.Errorf("expected ownership_verified=ok, got %q", c.Status)
	}
}

func TestPortalDomainCheck_ProviderManaged_NotVerified(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{
		Hostname:          "sub.tenant.com",
		OwnershipVerified: false,
	})
	cm := &mockCertMan{
		hasCertFn: func(_ context.Context, _ string) (bool, error) { return false, nil },
	}
	e := newPortalTestEndpointWithLookup(repo, cm, &mockLookup{})

	req := withSession(httptest.NewRequest(http.MethodPost, "/", nil), portalSession("sub.tenant.com"))
	rec := httptest.NewRecorder()
	e.handleDomainCheck().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	resp := decodeData[domainCheckResponse](t, rec)

	c := findCheck(resp.Checks, checkNameOwnershipVerified)
	if c == nil {
		t.Fatal("expected ownership_verified check")
	}
	if c.Status != checkStatusFail {
		t.Errorf("expected fail, got %q", c.Status)
	}
	if c.Message != "Use the verify button to complete verification." {
		t.Errorf("unexpected message %q", c.Message)
	}
}

func TestPortalDomainCheck_CertPresent(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{
		Hostname:          "sub.tenant.com",
		OwnershipVerified: true,
	})
	cm := &mockCertMan{
		hasCertFn: func(_ context.Context, _ string) (bool, error) { return true, nil },
	}
	e := newPortalTestEndpointWithLookup(repo, cm, &mockLookup{})

	req := withSession(httptest.NewRequest(http.MethodPost, "/", nil), portalSession("sub.tenant.com"))
	rec := httptest.NewRecorder()
	e.handleDomainCheck().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	resp := decodeData[domainCheckResponse](t, rec)

	c := findCheck(resp.Checks, checkNameCertificate)
	if c == nil {
		t.Fatal("expected certificate check")
	}
	if c.Status != checkStatusOK {
		t.Errorf("expected cert=ok, got %q", c.Status)
	}
}

func TestPortalDomainCheck_CertPending_OwnershipNotVerified(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{
		Hostname:          "sub.tenant.com",
		OwnershipVerified: false,
	})
	cm := &mockCertMan{
		hasCertFn: func(_ context.Context, _ string) (bool, error) { return false, nil },
	}
	e := newPortalTestEndpointWithLookup(repo, cm, &mockLookup{})

	req := withSession(httptest.NewRequest(http.MethodPost, "/", nil), portalSession("sub.tenant.com"))
	rec := httptest.NewRecorder()
	e.handleDomainCheck().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	resp := decodeData[domainCheckResponse](t, rec)

	c := findCheck(resp.Checks, checkNameCertificate)
	if c == nil {
		t.Fatal("expected certificate check")
	}
	if c.Status != checkStatusPending {
		t.Errorf("expected cert=pending, got %q", c.Status)
	}
	if c.Message != "Certificate not yet issued. Ownership verification is required first." {
		t.Errorf("unexpected message %q", c.Message)
	}
}

func TestPortalDomainCheck_CertPending_OwnershipVerified(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{
		Hostname:          "sub.tenant.com",
		OwnershipVerified: true,
	})
	cm := &mockCertMan{
		hasCertFn: func(_ context.Context, _ string) (bool, error) { return false, nil },
	}
	e := newPortalTestEndpointWithLookup(repo, cm, &mockLookup{})

	req := withSession(httptest.NewRequest(http.MethodPost, "/", nil), portalSession("sub.tenant.com"))
	rec := httptest.NewRecorder()
	e.handleDomainCheck().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	resp := decodeData[domainCheckResponse](t, rec)

	c := findCheck(resp.Checks, checkNameCertificate)
	if c == nil {
		t.Fatal("expected certificate check")
	}
	if c.Status != checkStatusPending {
		t.Errorf("expected cert=pending, got %q", c.Status)
	}
	if c.Message != "Certificate issuance in progress." {
		t.Errorf("unexpected message %q", c.Message)
	}
}

// --- checkCNAME / checkARecord / checkTXTRecord / checkOwnershipTXTRecord ---

func TestCheckCNAME_Match(t *testing.T) {
	r := &mockLookup{
		lookupCNAMEFn: func(_ string) (string, error) { return "proxy.saas.internal.", nil },
	}
	c := checkCNAME(r, "sub.tenant.com", "proxy.saas.internal")
	if c.Status != checkStatusOK {
		t.Errorf("expected ok, got %q: %s", c.Status, c.Message)
	}
}

func TestCheckCNAME_WrongDestination(t *testing.T) {
	r := &mockLookup{
		lookupCNAMEFn: func(_ string) (string, error) { return "other.host.", nil },
	}
	c := checkCNAME(r, "sub.tenant.com", "proxy.saas.internal")
	if c.Status != checkStatusFail {
		t.Errorf("expected fail, got %q", c.Status)
	}
	if c.Actual != "other.host" {
		t.Errorf("expected actual %q, got %q", "other.host", c.Actual)
	}
}

func TestCheckCNAME_LookupError(t *testing.T) {
	r := &mockLookup{}
	c := checkCNAME(r, "sub.tenant.com", "proxy.saas.internal")
	if c.Status != checkStatusFail {
		t.Errorf("expected fail, got %q", c.Status)
	}
}

func TestCheckARecord_Match(t *testing.T) {
	r := &mockLookup{
		lookupHostFn: func(_ string) ([]string, error) {
			return []string{"1.2.3.4", "5.6.7.8"}, nil
		},
	}
	c := checkARecord(r, "sub.tenant.com", "1.2.3.4")
	if c.Status != checkStatusOK {
		t.Errorf("expected ok, got %q", c.Status)
	}
}

func TestCheckARecord_WrongIP(t *testing.T) {
	r := &mockLookup{
		lookupHostFn: func(_ string) ([]string, error) { return []string{"9.9.9.9"}, nil },
	}
	c := checkARecord(r, "sub.tenant.com", "1.2.3.4")
	if c.Status != checkStatusFail {
		t.Errorf("expected fail, got %q", c.Status)
	}
	if c.Actual != "9.9.9.9" {
		t.Errorf("expected actual %q, got %q", "9.9.9.9", c.Actual)
	}
}

func TestCheckARecord_LookupError(t *testing.T) {
	r := &mockLookup{}
	c := checkARecord(r, "sub.tenant.com", "1.2.3.4")
	if c.Status != checkStatusFail {
		t.Errorf("expected fail, got %q", c.Status)
	}
}

func TestCheckTXTRecord_Match(t *testing.T) {
	r := &mockLookup{
		lookupTXTFn: func(_ string) ([]string, error) {
			return []string{"wrong-value", "expected-value"}, nil
		},
	}
	c := checkTXTRecord(r, "_acme-challenge.sub.tenant.com", "expected-value")
	if c.Status != checkStatusOK {
		t.Errorf("expected ok, got %q", c.Status)
	}
}

func TestCheckTXTRecord_WrongValue(t *testing.T) {
	r := &mockLookup{
		lookupTXTFn: func(_ string) ([]string, error) { return []string{"other-value"}, nil },
	}
	c := checkTXTRecord(r, "_acme-challenge.sub.tenant.com", "expected-value")
	if c.Status != checkStatusFail {
		t.Errorf("expected fail, got %q", c.Status)
	}
	if c.Actual != "other-value" {
		t.Errorf("expected actual %q, got %q", "other-value", c.Actual)
	}
}

func TestCheckOwnershipTXTRecord_Match(t *testing.T) {
	r := &mockLookup{
		lookupTXTFn: func(name string) ([]string, error) {
			if name == "_certmatic-verify.sub.tenant.com" {
				return []string{"my-token"}, nil
			}
			return nil, &net.DNSError{Name: name, Err: "no such host"}
		},
	}
	c := checkOwnershipTXTRecord(r, "sub.tenant.com", "my-token")
	if c.Status != checkStatusOK {
		t.Errorf("expected ok, got %q", c.Status)
	}
}

func TestCheckOwnershipTXTRecord_NoRecord(t *testing.T) {
	r := &mockLookup{}
	c := checkOwnershipTXTRecord(r, "sub.tenant.com", "my-token")
	if c.Status != checkStatusFail {
		t.Errorf("expected fail, got %q", c.Status)
	}
}

// --- DNS-challenge auto-ownership-verification in handleDomainCheck ---

func TestPortalDomainCheck_DNSChallenge_AutoVerifiesOwnership(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{
		Hostname:          "sub.tenant.com",
		OwnershipVerified: false,
		VerificationToken: "secret-token",
	})
	cm := &mockCertMan{
		hasCertFn: func(_ context.Context, _ string) (bool, error) { return false, nil },
	}
	r := &mockLookup{
		// CNAME check on the hostname itself (the required DNS record)
		lookupCNAMEFn: func(_ string) (string, error) { return "proxy.saas.internal.", nil },
		// Ownership TXT record returns the correct token
		lookupTXTFn: func(_ string) ([]string, error) { return []string{"secret-token"}, nil },
	}
	e := newPortalTestEndpointWithLookup(repo, cm, r)

	session := &pkgsession.Session{
		SessionID:                 "test-session",
		Hostname:                  "sub.tenant.com",
		ExpiresAt:                 time.Now().Add(time.Hour),
		OwnershipVerificationMode: pkgsession.OwnershipVerificationModeDNSChallenge,
	}
	req := withSession(httptest.NewRequest(http.MethodPost, "/", nil), session)
	rec := httptest.NewRecorder()
	e.handleDomainCheck().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	resp := decodeData[domainCheckResponse](t, rec)

	// Ownership TXT check should be ok
	txtCheck := findCheck(resp.Checks, checkNameOwnershipTXTRecord)
	if txtCheck == nil {
		t.Fatal("expected ownership_txt_record check")
	}
	if txtCheck.Status != checkStatusOK {
		t.Errorf("expected ownership_txt_record=ok, got %q", txtCheck.Status)
	}

	// ownership_verified check should now be ok (auto-patched during this request)
	ownershipCheck := findCheck(resp.Checks, checkNameOwnershipVerified)
	if ownershipCheck == nil {
		t.Fatal("expected ownership_verified check")
	}
	if ownershipCheck.Status != checkStatusOK {
		t.Errorf("expected ownership_verified=ok after auto-verify, got %q", ownershipCheck.Status)
	}

	// Verify the domain was actually patched in the repo
	stored, err := repo.Get(context.Background(), "sub.tenant.com")
	if err != nil {
		t.Fatalf("repo.Get failed: %v", err)
	}
	if !stored.Domain.OwnershipVerified {
		t.Error("expected OwnershipVerified=true in repo after auto-verify")
	}
}

func TestPortalDomainCheck_DNSChallenge_WrongToken_NoAutoVerify(t *testing.T) {
	repo := domainrepo.NewInMemoryDomainRepo("test")
	repo.Set(context.Background(), &domain.Domain{
		Hostname:          "sub.tenant.com",
		OwnershipVerified: false,
		VerificationToken: "secret-token",
	})
	cm := &mockCertMan{
		hasCertFn: func(_ context.Context, _ string) (bool, error) { return false, nil },
	}
	r := &mockLookup{
		lookupCNAMEFn: func(_ string) (string, error) { return "proxy.saas.internal.", nil },
		lookupTXTFn:   func(_ string) ([]string, error) { return []string{"wrong-token"}, nil },
	}
	e := newPortalTestEndpointWithLookup(repo, cm, r)

	session := &pkgsession.Session{
		SessionID:                 "test-session",
		Hostname:                  "sub.tenant.com",
		ExpiresAt:                 time.Now().Add(time.Hour),
		OwnershipVerificationMode: pkgsession.OwnershipVerificationModeDNSChallenge,
	}
	req := withSession(httptest.NewRequest(http.MethodPost, "/", nil), session)
	rec := httptest.NewRecorder()
	e.handleDomainCheck().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	resp := decodeData[domainCheckResponse](t, rec)

	txtCheck := findCheck(resp.Checks, checkNameOwnershipTXTRecord)
	if txtCheck == nil {
		t.Fatal("expected ownership_txt_record check")
	}
	if txtCheck.Status != checkStatusFail {
		t.Errorf("expected ownership_txt_record=fail for wrong token, got %q", txtCheck.Status)
	}

	// Repo must NOT have been patched
	stored, _ := repo.Get(context.Background(), "sub.tenant.com")
	if stored.Domain.OwnershipVerified {
		t.Error("expected OwnershipVerified to remain false when token doesn't match")
	}
}
