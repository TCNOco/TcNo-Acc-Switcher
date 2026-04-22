import { writable } from "svelte/store";

/** Public URL from GetPlatformExeIcon, or "" */
export const platformExeIconUrl = writable<string>("");

export type PlatformActionKind = "login" | "addNew" | "launch" | "saveCurrent";

/** Fired when the ActionBar requests an action on the platform page (increment id each time). */
export const platformAction = writable<{ id: number; kind: PlatformActionKind } | null>(null);

/** Currently selected account on the active platform page (for shortcut swap-before-launch). */
export const selectedAccount = writable<{ platformKey: string; uniqueId: string }>({
  platformKey: "",
  uniqueId: "",
});

let actionSeq = 0;

export function triggerPlatformAction(kind: PlatformActionKind): void {
  actionSeq += 1;
  platformAction.set({ id: actionSeq, kind });
}
