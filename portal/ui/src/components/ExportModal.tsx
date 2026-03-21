import { useState } from "react";
import type { DNSRecord } from "../api/client";
import { Button } from "../ui";
import { formatZoneFile } from "../utils/formatZoneFile";

interface Props {
  records: DNSRecord[];
  onClose: () => void;
}

export function ExportModal({ records, onClose }: Props) {
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
          <Button onClick={handleCopy}>{copied ? "Copied!" : "Copy all"}</Button>
        </div>
      </div>
    </div>
  );
}
