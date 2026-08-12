import { describe, expect, it } from "vitest";
import { steamGuardBadge } from "./steamGuardBadge";
import type { SteamAccountRow } from "./types";

function account(over: Partial<SteamAccountRow> = {}): SteamAccountRow {
  return {
    steamId64: "1",
    hasSteamGuard: false,
    steamGuardPending: false,
    ...over,
  } as SteamAccountRow;
}

describe("steamGuardBadge", () => {
  it("draws nothing for an account the vault does not hold", () => {
    expect(steamGuardBadge(account())).toBeUndefined();
  });

  it.each([
    ["an authenticator", { hasSteamGuard: true }, "stored"],
    ["a pending enrollment", { steamGuardPending: true }, "pending"],
    ["a login-only record", { steamGuardLoginOnly: true }, "login-only"],
  ])("marks %s", (_label, over, variant) => {
    expect(steamGuardBadge(account(over))?.variant).toBe(variant);
  });

  it("prefers the strongest state when flags overlap", () => {
    const all = { hasSteamGuard: true, steamGuardPending: true, steamGuardLoginOnly: true };
    expect(steamGuardBadge(account(all))?.variant).toBe("stored");
    expect(steamGuardBadge(account({ ...all, hasSteamGuard: false }))?.variant).toBe("pending");
  });

  it("gives each state its own label", () => {
    const keys = [
      steamGuardBadge(account({ hasSteamGuard: true }))?.labelKey,
      steamGuardBadge(account({ steamGuardPending: true }))?.labelKey,
      steamGuardBadge(account({ steamGuardLoginOnly: true }))?.labelKey,
    ];
    expect(new Set(keys).size).toBe(3);
  });

  it("obeys the display setting, but only once it has arrived", () => {
    expect(steamGuardBadge(account({ steamGuardLoginOnly: true, showSteamGuardLock: false }))).toBeUndefined();
    expect(steamGuardBadge(account({ steamGuardLoginOnly: true }))?.variant).toBe("login-only");
  });
});
