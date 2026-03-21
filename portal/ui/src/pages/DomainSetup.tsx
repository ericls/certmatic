import { useEffect, useState } from "react";
import { domainStore } from "../store/domain";
import { useDomain } from "../hooks/useDomain";
import { CertStatusCard } from "../components/CertStatusCard";
import { OwnershipStep } from "../components/OwnershipStep";
import { SetupCheck } from "../components/SetupCheck";
import { RequiredRecords } from "../components/RequiredRecords";
import { RecordsExport } from "../components/RecordsExport";
import { ExportModal } from "../components/ExportModal";
import { Button, StepCard } from "../ui";
import type { DNSRecord, EnsuredCert } from "../api/client";

interface Props {
  onBackButton: (back: { url: string; text: string } | null) => void;
}

export function DomainSetup({ onBackButton }: Props) {
  const storeState = useDomain();

  const [showExportModal, setShowExportModal] = useState(false);

  // Cert issuance state lives here so SetupCheck's auto-trigger shares it with the cert step UI
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
    } catch (e: unknown) {
      setPokeError(e instanceof Error ? e.message : "Unknown error");
    } finally {
      setPoking(false);
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

  const exportRecords: DNSRecord[] =
    isDnsChallenge && domain.ownership_txt_record
      ? [domain.ownership_txt_record, ...domain.required_dns_records]
      : domain.required_dns_records;

  const showTopLevelExport = isDnsChallenge && exportRecords.length > 0;

  return (
    <>
      {showExportModal && (
        <ExportModal records={exportRecords} onClose={() => setShowExportModal(false)} />
      )}
      <div className="max-w-3xl mx-auto px-4 py-8 space-y-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">{domain.hostname}</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Follow the steps below to connect your domain.
          </p>
        </div>

        {showTopLevelExport && (
          <p className="text-s text-gray-500 dark:text-gray-400">
            <span className="font-semibold">Tip:</span> both steps below require DNS changes.{" "}
            <Button variant="ghost" onClick={() => setShowExportModal(true)}>
              View all records at once
            </Button>
          </p>
        )}

        <StepCard n={1} title="Verify Ownership">
          <OwnershipStep
            domain={domain}
            isDnsChallenge={isDnsChallenge}
            ownershipDone={ownershipDone}
          />
        </StepCard>

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

        <StepCard n={3} title="Certificate">
          <div className="space-y-3">
            <CertStatusCard
              cert={domain.cert}
              ownershipVerified={domain.ownership_verified}
              isIssuing={poking}
            />
            {domain.ownership_verified && domain.cert === null && (
              <div>
                <Button onClick={handleEnsureCert} disabled={poking}>
                  {poking ? "Issuing…" : "Issue certificate"}
                </Button>
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

        <SetupCheck
          // Invalidates the setup check state when cert changes
          key={issuedCert?.not_after ?? "no-cert"}
          certIsNull={domain.cert === null}
          onEnsureCert={handleEnsureCert}
        />
      </div>
    </>
  );
}
