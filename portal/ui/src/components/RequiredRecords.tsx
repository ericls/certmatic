import type { DNSRecord } from "../api/client";
import { CopyButton } from "../ui";

interface Props {
  records: DNSRecord[];
}

export function RequiredRecords({ records }: Props) {
  if (records.length === 0) {
    return <p className="text-sm text-gray-500 dark:text-gray-400">No DNS records required.</p>;
  }

  return (
    <div className="overflow-x-auto">
      <table className="min-w-full text-sm border border-gray-200 dark:border-gray-700 rounded-lg">
        <thead className="bg-gray-50 dark:bg-gray-700">
          <tr>
            <th className="px-4 py-2 text-left font-medium text-gray-600 dark:text-gray-300">
              Type
            </th>
            <th className="px-4 py-2 text-left font-medium text-gray-600 dark:text-gray-300">
              Name
            </th>
            <th className="px-4 py-2 text-left font-medium text-gray-600 dark:text-gray-300">
              Value
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
          {records.map((rec, i) => (
            <tr key={i} className="bg-white dark:bg-gray-800">
              <td className="px-4 py-2 font-mono font-semibold text-gray-700 dark:text-gray-300">
                {rec.type}
              </td>
              <td className="px-4 py-2 font-mono text-gray-700 dark:text-gray-300">
                {rec.name}
                <CopyButton value={rec.name} />
              </td>
              <td className="px-4 py-2 font-mono text-gray-700 dark:text-gray-300">
                {rec.value}
                <CopyButton value={rec.value} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
