import type { CheckStatus } from "../api/client";

interface Props {
  status: CheckStatus;
}

const statusConfig: Record<CheckStatus, { label: string; className: string }> = {
  ok: {
    label: "OK",
    className: "bg-green-100 dark:bg-green-900/40 text-green-800 dark:text-green-300",
  },
  fail: {
    label: "Fail",
    className: "bg-red-100 dark:bg-red-900/40 text-red-800 dark:text-red-300",
  },
  pending: {
    label: "Pending",
    className: "bg-yellow-100 dark:bg-yellow-900/40 text-yellow-800 dark:text-yellow-300",
  },
};

export function StatusBadge({ status }: Props) {
  const { label, className } = statusConfig[status];
  return (
    <span
      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${className}`}
    >
      {label}
    </span>
  );
}
