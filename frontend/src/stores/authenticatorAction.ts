import { writable } from "svelte/store";

export const authenticatorAction = writable({ platformKey: "", provider: "" });

export function publishAuthenticatorAction(platformKey: string, provider: string): void {
  authenticatorAction.set({ platformKey, provider });
}

export function clearAuthenticatorAction(platformKey: string): void {
  authenticatorAction.update((current) => current.platformKey === platformKey
    ? { platformKey: "", provider: "" }
    : current);
}
