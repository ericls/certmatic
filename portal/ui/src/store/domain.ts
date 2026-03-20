import {
  getDomainInfo,
  ensureCert as apiEnsureCert,
  runDomainCheck as apiRunDomainCheck,
} from "../api/client";
import type { DomainInfo, DomainCheckReport, IssuedCert } from "../api/client";

export type DomainStoreState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "error"; error: string }
  | { status: "ready"; domain: DomainInfo };

let state: DomainStoreState = { status: "idle" };
const listeners = new Set<() => void>();

function setState(next: DomainStoreState): void {
  state = next;
  listeners.forEach((fn) => fn());
}

async function refresh(): Promise<void> {
  try {
    const domain = await getDomainInfo();
    setState({ status: "ready", domain });
  } catch (e) {
    setState({ status: "error", error: (e as Error).message });
  }
}

export const domainStore = {
  subscribe(listener: () => void): () => void {
    listeners.add(listener);
    return () => listeners.delete(listener);
  },

  getSnapshot(): DomainStoreState {
    return state;
  },

  // Idempotent initial fetch — safe to call on every mount
  load(): void {
    if (state.status === "loading" || state.status === "ready") return;
    setState({ status: "loading" });
    refresh();
  },

  // Mutations: each awaits refresh() before resolving
  async runDomainCheck(): Promise<DomainCheckReport> {
    const report = await apiRunDomainCheck();
    await refresh();
    return report;
  },

  async ensureCert(): Promise<IssuedCert> {
    const cert = await apiEnsureCert();
    await refresh();
    return cert;
  },
};
