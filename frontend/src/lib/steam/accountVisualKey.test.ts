import { describe, expect, it } from "vitest";
import { steamAccountVisualKey } from "./accountVisualKey";
import type { SteamAccountRow } from "./types";

function row(overrides: Partial<SteamAccountRow> = {}): SteamAccountRow {
  return {
    steamId64: "76561198000000000",
    personaName: "Test",
    displayName: "Test",
    accountName: "test",
    currentSession: false,
    showShortNotes: false,
    note: "",
    ...overrides,
  } as SteamAccountRow;
}

describe("steamAccountVisualKey", () => {
  it.each([
    ["mini-profile preference", { showMiniProfile: true }],
    ["mini-profile content", { miniProfileHtml: "<div>profile</div>" }],
    ["avatar-frame preference", { showAvatarFrame: true }],
    ["last-login preference", { showLastLogin: true }],
    ["Steam ID preference", { showSteamId: true }],
    // Anything the tile draws has to be in the key: applyAccountPatch bails
    // when it is unchanged, so a missing field silently never repaints.
    ["Steam Guard lock preference", { showSteamGuardLock: true }],
    ["CS2 cooldown", { cs2CooldownExpiresAt: "2026-08-14T09:30:00Z" }],
    ["permanent CS2 cooldown", { cs2CooldownPermanent: true }],
    ["CS2 cooldown preference", { showCs2Cooldown: true }],
  ] as const)("changes for %s", (_name, change) => {
    expect(steamAccountVisualKey(row(change))).not.toBe(steamAccountVisualKey(row()));
  });
});
