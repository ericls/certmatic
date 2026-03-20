export interface DNSRecord {
  type: string;
  name: string;
  value: string;
}

export interface CertStatus {
  has_cert: boolean;
}

export interface DomainInfo {
  hostname: string;
  ownership_verified: boolean;
  required_dns_records: DNSRecord[];
  cert_status: CertStatus;
  back_url?: string;
  back_text?: string;
  ownership_verification_mode?: "dns_challenge" | "provider_managed" | "";
  ownership_txt_record?: DNSRecord;
  verify_ownership_url?: string;
  verify_ownership_text?: string;
}

export type CheckStatus = "ok" | "fail" | "pending";

export interface DomainCheck {
  name: string;
  status: CheckStatus;
  expected?: string;
  actual?: string;
  message: string;
}

export interface DomainCheckReport {
  hostname: string;
  checks: DomainCheck[];
  overall: CheckStatus;
}

interface ApiResponse<T> {
  data: T;
  errors: { message: string; field?: string }[];
}

// Extract the session ID from the URL path: /portal/{sessionID}/...
function getApiBase(): string {
  const parts = window.location.pathname.split("/").filter(Boolean);
  // parts[0] = "portal", parts[1] = sessionID
  const sessionID = parts[1] ?? "";
  return `/portal/${sessionID}/api`;
}

async function apiRequest<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  const body: ApiResponse<T> = await res.json();
  if (!res.ok) {
    const msg = body.errors?.[0]?.message ?? `HTTP ${res.status}`;
    throw new Error(msg);
  }
  return body.data;
}

export function getDomainInfo(): Promise<DomainInfo> {
  return apiRequest<DomainInfo>(`${getApiBase()}/domain`);
}

export function runDomainCheck(): Promise<DomainCheckReport> {
  return apiRequest<DomainCheckReport>(`${getApiBase()}/domain/check`, {
    method: "POST",
  });
}

export async function pokeCert(): Promise<void> {
  await apiRequest<unknown>(`${getApiBase()}/domain/cert`, { method: "POST" });
}
