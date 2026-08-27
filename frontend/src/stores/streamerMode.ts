import { derived, get, writable } from "svelte/store";
import { Events } from "@wailsio/runtime";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

/** Mirrors streamer.State from the backend. */
export interface StreamerState {
  manual: boolean;
  autoEnabled: boolean;
  autoActive: boolean;
  detectedExe: string;
  effective: boolean;
  avatarSalt: string;
}

const EMPTY: StreamerState = {
  manual: false,
  autoEnabled: false,
  autoActive: false,
  detectedExe: "",
  effective: false,
  avatarSalt: "",
};

export const streamerState = writable<StreamerState>(EMPTY);

/** The only question most callers have: should this screen be safe to broadcast? */
export const streamerMode = derived(streamerState, (s) => s.effective);

/** Machine-local seed material for generated avatars. */
export const avatarSalt = derived(streamerState, (s) => s.avatarSalt);

/**
 * Anything that looks like an email address collapses to the part before the @.
 * Account names are routinely the login email, and the domain is the half that
 * identifies a person to a chat full of strangers.
 */
const EMAIL_PATTERN = /([A-Za-z0-9._%+-]+)@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+/g;

export function shortenEmails(text: string): string {
  return text.replace(EMAIL_PATTERN, "$1");
}

/** Display name as it should appear right now. */
export function censorName(text: string, on: boolean): string {
  return on ? shortenEmails(text) : text;
}

/**
 * `$censoredName(...)` in a template: re-renders when streamer mode flips, the same
 * way `$t(...)` re-renders on a language change.
 */
export const censoredName = derived(
  streamerMode,
  (on) => (text: string | null | undefined) => censorName(String(text ?? ""), on),
);

function applyState(next: StreamerState): void {
  streamerState.set(next);
  if (typeof document !== "undefined") {
    // One class drives every purely-visual rule (hidden IDs, suppressed hover
    // cards), so components only handle what CSS cannot express.
    document.documentElement.classList.toggle("streamer-mode", next.effective);
  }
}

function normalize(raw: unknown): StreamerState {
  const s = (raw ?? {}) as Partial<StreamerState>;
  return {
    manual: s.manual === true,
    autoEnabled: s.autoEnabled === true,
    autoActive: s.autoActive === true,
    detectedExe: String(s.detectedExe ?? ""),
    effective: s.effective === true,
    // The salt is fixed for the process; a payload without one must not blank it.
    avatarSalt: String(s.avatarSalt ?? "") || get(streamerState).avatarSalt,
  };
}

/** Hydrates from the backend and keeps up with detection. Returns an unsubscriber. */
export async function initStreamerMode(): Promise<() => void> {
  const off = Events.On("streamer-mode-changed", (ev: { data?: unknown }) => {
    applyState(normalize(ev?.data));
  });
  try {
    applyState(normalize(await PlatformService.GetStreamerState()));
  } catch {
    applyState(EMPTY);
  }
  return off;
}

/* Neither setter recomputes the effective answer: the backend owns that rule and
   emits the new state. */

export async function setStreamerMode(enabled: boolean): Promise<void> {
  await PlatformService.SetStreamerMode(enabled);
}

export async function setAutoStreamerMode(enabled: boolean): Promise<void> {
  await PlatformService.SetAutoStreamerMode(enabled);
}
