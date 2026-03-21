import type { DNSRecord } from "../api/client";

export function formatZoneFile(records: DNSRecord[]): string {
  return records.map((r) => `${r.name}  IN  ${r.type}  ${r.value}`).join("\n");
}
