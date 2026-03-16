import type { DomainCheckReport } from "../api/client";
import { StatusBadge } from "./StatusBadge";

const checkDisplayNames: Record<string, string> = {
  cname_record: "CNAME Record",
  a_record: "A Record",
  txt_record: "TXT Record",
  ownership_verified: "Ownership Verified",
  certificate: "Certificate",
};

function displayName(name: string): string {
  return checkDisplayNames[name] ?? name;
}

interface Props {
  report: DomainCheckReport;
}

export function DoctorReport({ report }: Props) {
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-3">
        <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
          Overall status:
        </span>
        <StatusBadge status={report.overall} />
      </div>
      <div className="space-y-2">
        {report.checks.map((check, i) => (
          <div
            key={i}
            className="border border-gray-200 dark:border-gray-700 rounded-lg p-3 bg-white dark:bg-gray-800"
          >
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                {displayName(check.name)}
              </span>
              <StatusBadge status={check.status} />
            </div>
            <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">{check.message}</p>
            {(check.expected || check.actual) && (
              <div className="mt-2 text-xs font-mono text-gray-500 dark:text-gray-400 space-y-0.5">
                {check.expected && (
                  <div>
                    <span className="text-gray-400 dark:text-gray-500">expected:</span>{" "}
                    {check.expected}
                  </div>
                )}
                {check.actual && (
                  <div>
                    <span className="text-gray-400 dark:text-gray-500">actual:</span> {check.actual}
                  </div>
                )}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
