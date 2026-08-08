import { readable, type Readable } from "svelte/store";
import { STEAM_GUARD_CODE_LIFETIME_MS } from "../steamGuardModal";

const TICK_MS = 100;

/**
 * End of the code window the given instant falls in. Steam's periods are counted
 * off the Unix epoch, so the boundary is absolute: it can be drawn without
 * unlocking the vault or asking for a code.
 */
export function steamGuardCodePeriodExpiry(now: number): number {
  return Math.floor(now / STEAM_GUARD_CODE_LIFETIME_MS) * STEAM_GUARD_CODE_LIFETIME_MS
    + STEAM_GUARD_CODE_LIFETIME_MS;
}

/** Fraction of the current window still to run, 1 immediately after a reset. */
export function steamGuardCodePeriodProgress(now: number): number {
  if (!Number.isFinite(now)) return 0;
  return (steamGuardCodePeriodExpiry(now) - now) / STEAM_GUARD_CODE_LIFETIME_MS;
}

/**
 * Live countdown of the current code window. One shared timer that only runs
 * while a menu row is on screen.
 */
export const steamGuardCodeRemaining: Readable<number> = readable(
  steamGuardCodePeriodProgress(Date.now()),
  (set) => {
    const tick = (): void => set(steamGuardCodePeriodProgress(Date.now()));
    tick();
    const timer = setInterval(tick, TICK_MS);
    return () => clearInterval(timer);
  },
);
