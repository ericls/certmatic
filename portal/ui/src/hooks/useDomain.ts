import { useSyncExternalStore } from "react";
import { domainStore } from "../store/domain";
import type { DomainStoreState } from "../store/domain";

export function useDomain(): DomainStoreState {
  return useSyncExternalStore(
    domainStore.subscribe,
    domainStore.getSnapshot,
    domainStore.getSnapshot,
  );
}
