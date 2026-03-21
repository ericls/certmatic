import { useState } from "react";
import type { DNSRecord } from "../api/client";
import { Button } from "../ui";
import { formatZoneFile } from "../utils/formatZoneFile";

interface Props {
  records: DNSRecord[];
}

export function RecordsExport({ records }: Props) {
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
        <Button variant="ghost" size="sm" onClick={() => setExpanded((v) => !v)}>
          {expanded ? "Hide" : "Show"}
        </Button>
        <Button size="sm" onClick={handleCopy}>
          {copied ? "Copied!" : "Copy all"}
        </Button>
      </div>
      {expanded && (
        <pre className="px-4 py-3 text-xs font-mono text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-900 overflow-x-auto whitespace-pre">
          {zoneText}
        </pre>
      )}
    </div>
  );
}
