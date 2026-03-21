import { useState } from "react";
import { domainStore } from "../store/domain";
import type { DomainCheckReport } from "../api/client";
import { DoctorReport } from "./DoctorReport";
import { Button } from "../ui";

interface Props {
  certIsNull: boolean;
  onEnsureCert: () => Promise<void>;
}

export function SetupCheck({ certIsNull, onEnsureCert }: Props) {
  const [checkReport, setCheckReport] = useState<DomainCheckReport | null>(null);
  const [checking, setChecking] = useState(false);
  const [checkError, setCheckError] = useState<string | null>(null);

  const handleRunCheck = async () => {
    setChecking(true);
    setCheckError(null);
    try {
      const report = await domainStore.runDomainCheck();
      setChecking(false);
      setCheckReport(report);
      if (report.overall === "pending" && certIsNull) {
        await onEnsureCert();
      }
    } catch (e: unknown) {
      setCheckError(e instanceof Error ? e.message : "Unknown error");
    } finally {
      setChecking(false);
    }
  };

  return (
    <section className="pt-2">
      <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-3">
        Setup Check
      </h2>
      <p className="text-sm text-gray-600 dark:text-gray-400 mb-3">
        Run a live check to verify all DNS records and certificate status.
      </p>
      <Button onClick={handleRunCheck} disabled={checking}>
        {checking ? "Running check…" : "Run setup check"}
      </Button>
      {checkError && <p className="mt-3 text-sm text-red-600 dark:text-red-400">{checkError}</p>}
      {checkReport && (
        <div className="mt-4">
          <DoctorReport report={checkReport} />
        </div>
      )}
    </section>
  );
}
