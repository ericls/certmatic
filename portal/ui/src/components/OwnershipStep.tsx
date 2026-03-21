import { useState } from "react";
import { sanitizeUrl } from "../utils/sanitizeUrl";
import { domainStore } from "../store/domain";
import type { DomainCheckReport, DomainInfo } from "../api/client";
import { RequiredRecords } from "./RequiredRecords";
import { DoctorReport } from "./DoctorReport";
import { StatusBadge } from "./StatusBadge";
import { Button } from "../ui";

const OWNERSHIP_CHECK_NAMES = new Set(["ownership_txt_record", "ownership_verified"]);

interface Props {
  domain: DomainInfo;
  isDnsChallenge: boolean;
  ownershipDone: boolean;
}

export function OwnershipStep({ domain, isDnsChallenge, ownershipDone }: Props) {
  const [verifyReport, setVerifyReport] = useState<DomainCheckReport | null>(null);
  const [verifying, setVerifying] = useState(false);
  const [verifyError, setVerifyError] = useState<string | null>(null);

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

  return (
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
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Verify ownership of this domain through your provider dashboard.
            </p>
          )}

          <div className="flex flex-wrap gap-2">
            {!isDnsChallenge && domain.verify_ownership_url && (
              <a
                href={sanitizeUrl(domain.verify_ownership_url)}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-block px-4 py-2 text-sm font-medium bg-primary-600 text-white rounded-lg hover:bg-primary-700"
              >
                {domain.verify_ownership_text ?? "Verify ownership"}
              </a>
            )}
            <Button variant="outline" onClick={handleCheckVerification} disabled={verifying}>
              {verifying ? "Checking…" : "Check verification"}
            </Button>
          </div>

          {verifyError && <p className="text-sm text-red-600 dark:text-red-400">{verifyError}</p>}
          {verifyReport && <DoctorReport report={verifyReport} />}
        </>
      )}
    </div>
  );
}
