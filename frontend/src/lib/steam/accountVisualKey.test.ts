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
  ] as const)("changes for %s", (_name, change) => {
    expect(steamAccountVisualKey(row(change))).not.toBe(steamAccountVisualKey(row()));
  });
});
