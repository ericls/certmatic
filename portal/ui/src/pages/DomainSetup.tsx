import { useEffect, useState } from "react";
import type { DomainInfo, DomainCheckReport } from "../api/client";
import { getDomainInfo, runDomainCheck } from "../api/client";
import { RequiredRecords } from "../components/RequiredRecords";
import { CertStatusCard } from "../components/CertStatusCard";
import { DoctorReport } from "../components/DoctorReport";
import { StatusBadge } from "../components/StatusBadge";

interface Props {
  onBackButton: (back: { url: string; text: string } | null) => void;
}

export function DomainSetup({ onBackButton }: Props) {
  const [domain, setDomain] = useState<DomainInfo | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [checkReport, setCheckReport] = useState<DomainCheckReport | null>(null);
  const [checking, setChecking] = useState(false);
  const [checkError, setCheckError] = useState<string | null>(null);

  useEffect(() => {
    getDomainInfo()
      .then((d) => {
        setDomain(d);
        onBackButton(d.back_url ? { url: d.back_url, text: d.back_text || "Back" } : null);
      })
      .catch((e: Error) => setLoadError(e.message));
  }, [onBackButton]);

  const handleRunCheck = async () => {
    setChecking(true);
    setCheckError(null);
    try {
      const report = await runDomainCheck();
      setCheckReport(report);
    } catch (e: unknown) {
      setCheckError(e instanceof Error ? e.message : "Unknown error");
    } finally {
      setChecking(false);
    }
  };

  if (loadError) {
    return (
      <div className="max-w-3xl mx-auto px-4 py-8">
        <div className="rounded-lg border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 p-5">
          <p className="font-semibold text-red-800 dark:text-red-300">Failed to load domain info</p>
          <p className="mt-1 text-sm text-red-600 dark:text-red-400">{loadError}</p>
        </div>
      </div>
    );
  }

  if (!domain) {
    return (
      <div className="p-6 text-gray-500 dark:text-gray-400 text-sm max-w-3xl mx-auto px-4 py-8 space-y-8">
        Loading…
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto px-4 py-8 space-y-8">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">{domain.hostname}</h1>
        <div className="mt-1 flex items-center gap-2">
          <span className="text-sm text-gray-500 dark:text-gray-400">Ownership:</span>
          <StatusBadge status={domain.ownership_verified ? "ok" : "pending"} />
          {!domain.ownership_verified &&
            domain.ownership_verification_mode === "provider_managed" &&
            domain.verify_ownership_url && (
              <a
                href={domain.verify_ownership_url}
                target="_blank"
                rel="noopener noreferrer"
                className="px-3 py-1 text-xs font-medium bg-blue-600 text-white rounded-lg hover:bg-blue-700"
              >
                {domain.verify_ownership_text ?? "Verify Ownership"}
              </a>
            )}
        </div>
      </div>

      {/* DNS Records */}
      <section>
        <h2 className="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-3">
          Required DNS Records
        </h2>
        <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
          Add the following records to your DNS provider to connect your domain.
        </p>
        <RequiredRecords records={domain.required_dns_records} />
      </section>

      {/* Ownership Verification Record (DNS challenge mode) */}
      {domain.ownership_txt_record && (
        <section>
          <h2 className="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-3">
            Ownership Verification Record
          </h2>
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
            Add this TXT record to prove ownership.
          </p>
          <RequiredRecords records={[domain.ownership_txt_record]} />
        </section>
      )}

      {/* Certificate Status */}
      <section>
        <h2 className="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-3">
          Certificate Status
        </h2>
        <CertStatusCard
          certStatus={domain.cert_status}
          ownershipVerified={domain.ownership_verified}
        />
      </section>

      {/* Setup Doctor */}
      <section>
        <h2 className="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-3">Setup Check</h2>
        <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
          Run a live check to verify your DNS records are correctly configured.
        </p>
        <button
          onClick={handleRunCheck}
          disabled={checking}
          className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {checking ? "Running check…" : "Run Setup Check"}
        </button>
        {checkError && <p className="mt-3 text-sm text-red-600 dark:text-red-400">{checkError}</p>}
        {checkReport && (
          <div className="mt-4">
            <DoctorReport report={checkReport} />
          </div>
        )}
      </section>
    </div>
  );
}
