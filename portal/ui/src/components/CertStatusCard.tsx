import type { CertInfo } from "../api/client";

interface Props {
  cert: CertInfo | null;
  ownershipVerified: boolean;
}

export function CertStatusCard({ cert, ownershipVerified }: Props) {
  if (cert !== null) {
    return (
      <div className="flex items-center gap-2 p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg">
        <span className="text-green-600 dark:text-green-400 text-lg">✓</span>
        <span className="text-sm text-green-800 dark:text-green-300 font-medium">
          Certificate is issued and active.
        </span>
      </div>
    );
  }

  if (!ownershipVerified) {
    return (
      <div className="flex items-center gap-2 p-3 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
        <span className="text-gray-400 text-lg">⏳</span>
        <span className="text-sm text-gray-600 dark:text-gray-400">
          Certificate will be issued once domain ownership is verified.
        </span>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-2 p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg">
      <span className="text-yellow-600 dark:text-yellow-400 text-lg">⏳</span>
      <span className="text-sm text-yellow-800 dark:text-yellow-300">
        Certificate issuance in progress…
      </span>
    </div>
  );
}
