// Formats DNS records as a BIND zone file (RFC 1035 §5, master file format).
// Rules applied:
//   - Names and CNAME targets get a trailing dot to mark them as absolute FQDNs.
//   - TXT RDATA is wrapped in double quotes as required by RFC 1035 §3.3.14.
//   - TTL is omitted; importers use their default or the zone's $TTL.
import type { DNSRecord } from "../api/client";

function formatValue(type: string, value: string): string {
  if (type === "TXT") return `"${value}"`;
  // CNAME and domain targets must be absolute (trailing dot) in zone file format
  if (type === "CNAME") return value.endsWith(".") ? value : `${value}.`;
  return value;
}

export function formatZoneFile(records: DNSRecord[]): string {
  return records
    .map((r) => {
      const name = r.name.endsWith(".") ? r.name : `${r.name}.`;
      return `${name}  IN  ${r.type}  ${formatValue(r.type, r.value)}`;
    })
    .join("\n");
}
