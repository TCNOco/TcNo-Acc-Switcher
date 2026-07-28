import type { SteamGuardMenuRequest } from "./types";

type SteamGuardMenuRequestListener = (request: SteamGuardMenuRequest) => void;

const listeners = new Set<SteamGuardMenuRequestListener>();

/** Publishes non-secret account context for the Steam Guard modal host. */
export function emitSteamGuardMenuRequest(request: SteamGuardMenuRequest): void {
  for (const listener of listeners) listener(request);
}

export function onSteamGuardMenuRequest(listener: SteamGuardMenuRequestListener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}
