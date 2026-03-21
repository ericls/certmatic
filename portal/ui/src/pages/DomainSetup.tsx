import { useEffect, useState } from "react";
import { sanitizeUrl } from "../utils/sanitizeUrl";
import { domainStore } from "../store/domain";
import { useDomain } from "../hooks/useDomain";
import { RequiredRecords } from "../components/RequiredRecords";
import { CertStatusCard } from "../components/CertStatusCard";
import { DoctorReport } from "../components/DoctorReport";
import { StatusBadge } from "../components/StatusBadge";
import type { DNSRecord, DomainCheckReport, EnsuredCert } from "../api/client";

interface Props {
  onBackButton: (back: { url: string; text: string } | null) => void;
}

function StepCard({ n, title, children }: { n: number; title: string; children: React.ReactNode }) {
  return (
    <div className="border border-gray-200 dark:border-gray-700 rounded-xl overflow-hidden">
      <div className="flex items-center gap-3 px-5 py-4 bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
        <span className="flex-shrink-0 w-7 h-7 rounded-full bg-blue-600 text-white text-sm font-bold flex items-center justify-center">
          {n}
        </span>
        <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100">{title}</h2>
      </div>
      <div className="px-5 py-5 bg-white dark:bg-gray-900">{children}</div>
    </div>
  );
}

const OWNERSHIP_CHECK_NAMES = new Set(["ownership_txt_record", "ownership_verified"]);

function formatZoneFile(records: DNSRecord[]): string {
  return records.map((r) => `${r.name}  IN  ${r.type}  ${r.value}`).join("\n");
}

function ExportModal({ records, onClose }: { records: DNSRecord[]; onClose: () => void }) {
  const [copied, setCopied] = useState(false);
  const zoneText = formatZoneFile(records);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(zoneText);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={onClose}
    >
      <div
        className="w-full max-w-lg rounded-xl bg-white dark:bg-gray-900 shadow-xl overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100">
            All DNS Records
          </h3>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 text-lg leading-none"
          >
            ×
          </button>
        </div>
        <div className="px-5 py-4">
          <pre className="text-xs font-mono text-gray-700 dark:text-gray-300 bg-gray-50 dark:bg-gray-800 rounded-lg p-3 overflow-x-auto whitespace-pre">
            {zoneText}
          </pre>
        </div>
        <div className="px-5 pb-4">
          <button
            onClick={handleCopy}
            className="px-4 py-2 text-sm font-medium bg-blue-600 text-white rounded-lg hover:bg-blue-700"
          >
            {copied ? "Copied!" : "Copy all"}
          </button>
        </div>
      </div>
    </div>
  );
}

function RecordsExport({ records }: { records: DNSRecord[] }) {
  const [copied, setCopied] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const zoneText = formatZoneFile(records);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(zoneText);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="mt-4 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
      <div className="flex items-center gap-2 px-4 py-3 bg-gray-50 dark:bg-gray-800">
        <span className="text-xs font-medium text-gray-600 dark:text-gray-300 flex-1">
          All records as zone file
        </span>
        <button
          onClick={() => setExpanded((v) => !v)}
          className="text-xs text-blue-600 dark:text-blue-400 hover:underline"
        >
          {expanded ? "Hide" : "Show"}
        </button>
        <button
          onClick={handleCopy}
          className="px-3 py-1 text-xs font-medium bg-blue-600 text-white rounded-md hover:bg-blue-700"
        >
          {copied ? "Copied!" : "Copy all"}
        </button>
      </div>
      {expanded && (
        <pre className="px-4 py-3 text-xs font-mono text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-900 overflow-x-auto whitespace-pre">
          {zoneText}
        </pre>
      )}
    </div>
  );
}

export function DomainSetup({ onBackButton }: Props) {
  const storeState = useDomain();

  // Full setup check state
  const [checkReport, setCheckReport] = useState<DomainCheckReport | null>(null);
  const [checking, setChecking] = useState(false);
  const [checkError, setCheckError] = useState<string | null>(null);

  // Ownership verification check state
  const [verifyReport, setVerifyReport] = useState<DomainCheckReport | null>(null);
  const [verifying, setVerifying] = useState(false);
  const [verifyError, setVerifyError] = useState<string | null>(null);

  // Export modal state
  const [showExportModal, setShowExportModal] = useState(false);

  // Certificate issuing state
  const [poking, setPoking] = useState(false);
  const [pokeError, setPokeError] = useState<string | null>(null);
  const [issuedCert, setIssuedCert] = useState<EnsuredCert | null>(null);

  useEffect(() => {
    domainStore.load();
  }, []);

  useEffect(() => {
    if (storeState.status !== "ready") return;
    const d = storeState.domain;
    onBackButton(d.back_url ? { url: d.back_url, text: d.back_text || "Back" } : null);
  }, [storeState, onBackButton]);

  const handleEnsureCert = async () => {
    setPoking(true);
    setPokeError(null);
    setIssuedCert(null);
    try {
      const cert = await domainStore.ensureCert();
      setIssuedCert(cert);
      setCheckReport(null);
    } catch (e: unknown) {
      setPokeError(e instanceof Error ? e.message : "Unknown error");
    } finally {
      setPoking(false);
    }
  };

  const handleCheckVerification = async () => {
    setVerifying(true);
    setVerifyError(null);
    try {
      const report = await domainStore.runDomainCheck();
      setVerifyReport({
        ...report,
        checks: report.checks.filter((c) => OWNERSHIP_CHECK_NAMES.has(c.name)),
      });
    } catch (e: unknown) {
      setVerifyError(e instanceof Error ? e.message : "Unknown error");
    } finally {
      setVerifying(false);
    }
  };

  const handleRunCheck = async () => {
    setChecking(true);
    setCheckError(null);
    try {
      const report = await domainStore.runDomainCheck();
      setChecking(false);
      setCheckReport(report);
      if (report.overall === "pending" && storeState.domain?.cert === null) {
        await handleEnsureCert();
      }
    } catch (e: unknown) {
      setCheckError(e instanceof Error ? e.message : "Unknown error");
    } finally {
      setChecking(false);
    }
  };

  if (storeState.status === "error") {
    return (
      <div className="max-w-3xl mx-auto px-4 py-8">
        <div className="rounded-lg border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 p-5">
          <p className="font-semibold text-red-800 dark:text-red-300">Failed to load domain info</p>
          <p className="mt-1 text-sm text-red-600 dark:text-red-400">{storeState.error}</p>
        </div>
      </div>
    );
  }

  if (storeState.status !== "ready") {
    return (
      <div className="p-6 text-gray-500 dark:text-gray-400 text-sm max-w-3xl mx-auto px-4 py-8">
        Loading…
      </div>
    );
  }

  const domain = storeState.domain;
  const isDnsChallenge = domain.ownership_verification_mode === "dns_challenge";
  const ownershipDone = domain.ownership_verified || domain.cert !== null;

  // Records to include in bulk export: if dns_challenge, include ownership TXT too
  const exportRecords: DNSRecord[] =
    isDnsChallenge && domain.ownership_txt_record
      ? [domain.ownership_txt_record, ...domain.required_dns_records]
      : domain.required_dns_records;

  // Show the export at the top level (above steps) when both ownership and pointing
  // records require DNS changes — so users can copy everything in one shot without
  // having to read through each step individually.
  const showTopLevelExport = isDnsChallenge && exportRecords.length > 0;

  return (
    <>
      {/* Render modal outside the space-y-6 container to avoid inherited top margin */}
      {showExportModal && (
        <ExportModal records={exportRecords} onClose={() => setShowExportModal(false)} />
      )}
      <div className="max-w-3xl mx-auto px-4 py-8 space-y-6">
        {/* Page header */}
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">{domain.hostname}</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Follow the steps below to connect your domain.
          </p>
        </div>

        {/* Tip (dns_challenge only): both steps need DNS changes, offer a modal to view all at once */}
        {showTopLevelExport && (
          <p className="text-s text-gray-500 dark:text-gray-400">
            <span className="font-semibold">Tip:</span> both steps below require DNS changes.{" "}
            <button
              onClick={() => setShowExportModal(true)}
              className="text-blue-600 dark:text-blue-400 hover:underline"
            >
              View all records at once
            </button>
          </p>
        )}

        {/* Step 1: Verify Ownership */}
        <StepCard n={1} title="Verify Ownership">
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <span className="text-sm text-gray-600 dark:text-gray-400">Status:</span>
              <StatusBadge status={ownershipDone ? "ok" : "pending"} />
            </div>

            {isDnsChallenge && domain.ownership_txt_record && (
              <div>
                {!ownershipDone && (
                  <p className="text-sm text-gray-600 dark:text-gray-400 mb-3">
                    Add this TXT record to your DNS provider to prove ownership of this domain.
                  </p>
                )}
                <RequiredRecords records={[domain.ownership_txt_record]} />
              </div>
            )}

            {!ownershipDone && (
              <>
                {!isDnsChallenge && domain.verify_ownership_url && (
                  <div>
                    <p className="text-sm text-gray-600 dark:text-gray-400 mb-3">
                      Verify ownership of this domain through your provider dashboard.
                    </p>
                    <a
                      href={sanitizeUrl(domain.verify_ownership_url)}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-block px-4 py-2 text-sm font-medium bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                    >
                      {domain.verify_ownership_text ?? "Verify Ownership"}
                    </a>
                  </div>
                )}

                <div>
                  <button
                    onClick={handleCheckVerification}
                    disabled={verifying}
                    className="px-4 py-2 text-sm font-medium border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {verifying ? "Checking…" : "Check Verification"}
                  </button>
                  {verifyError && (
                    <p className="mt-2 text-sm text-red-600 dark:text-red-400">{verifyError}</p>
                  )}
                  {verifyReport && (
                    <div className="mt-3">
                      <DoctorReport report={verifyReport} />
                    </div>
                  )}
                </div>
              </>
            )}
          </div>
        </StepCard>

        {/* Step 2: Configure DNS Records */}
        <StepCard n={2} title="Configure DNS Records">
          <div className="space-y-4">
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Add the following records to your DNS provider to point your domain.
            </p>

            {domain.required_dns_records.length > 0 ? (
              <RequiredRecords records={domain.required_dns_records} />
            ) : (
              <p className="text-sm text-gray-500 dark:text-gray-400">No DNS records required.</p>
            )}

            {!showTopLevelExport && exportRecords.length > 0 && (
              <RecordsExport records={exportRecords} />
            )}
          </div>
        </StepCard>

        {/* Step 3: Certificate */}
        <StepCard n={3} title="Certificate">
          <div className="space-y-3">
            <CertStatusCard cert={domain.cert} ownershipVerified={domain.ownership_verified} isIssuing={poking} />
            {domain.ownership_verified && domain.cert === null && (
              <div>
                <button
                  onClick={handleEnsureCert}
                  disabled={poking}
                  className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {poking ? "Issuing…" : "Issue Certificate"}
                </button>
                {pokeError && (
                  <p className="mt-2 text-sm text-red-600 dark:text-red-400">{pokeError}</p>
                )}
                {issuedCert && (
                  <p className="mt-2 text-sm text-green-600 dark:text-green-400">
                    Certificate issued. Valid until{" "}
                    {new Date(issuedCert.not_after).toLocaleDateString()}.
                  </p>
                )}
              </div>
            )}
          </div>
        </StepCard>

        {/* Setup Check */}
        <section className="pt-2">
          <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-3">
            Setup Check
          </h2>
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-3">
            Run a live check to verify all DNS records and certificate status.
          </p>
          <button
            onClick={handleRunCheck}
            disabled={checking}
            className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {checking ? "Running check…" : "Run Setup Check"}
          </button>
          {checkError && (
            <p className="mt-3 text-sm text-red-600 dark:text-red-400">{checkError}</p>
          )}
          {checkReport && (
            <div className="mt-4">
              <DoctorReport report={checkReport} />
            </div>
          )}
        </section>
      </div>
    </>
  );
}
