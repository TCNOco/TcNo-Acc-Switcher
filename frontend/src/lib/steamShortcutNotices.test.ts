import { describe, expect, it } from "vitest";
import { addToSteamNotices } from "./steamShortcutNotices";
import type { SteamShortcutState } from "../../bindings/TcNo-Acc-Switcher/internal/steam/models.js";

function state(over: Partial<SteamShortcutState> = {}): SteamShortcutState {
  return {
    steamInstalled: true,
    enabled: true,
    present: true,
    users: 2,
    steamRunning: false,
    flatpakSteam: false,
    ...over,
  } as SteamShortcutState;
}

describe("addToSteamNotices", () => {
  it("says nothing when the entry is written and Steam is closed", () => {
    expect(addToSteamNotices(state())).toEqual([]);
  });

  it("says nothing while the toggle is off", () => {
    expect(addToSteamNotices(state({ enabled: false, present: false, steamRunning: true }))).toEqual([]);
  });

  it("says nothing before the state has loaded", () => {
    expect(addToSteamNotices(null)).toEqual([]);
  });

  it("reports a Steam that has never been signed into on its own", () => {
    // Nothing else is worth saying when there is nowhere to write the entry.
    expect(addToSteamNotices(state({ users: 0, present: false, steamRunning: true }))).toEqual([
      "Settings_AddToSteam_NoUsers",
    ]);
  });

  it("asks for a Steam restart while Steam is running", () => {
    expect(addToSteamNotices(state({ steamRunning: true }))).toEqual([
      "Settings_AddToSteam_RestartSteam",
    ]);
  });

  it("names the Flatpak permission alongside the restart", () => {
    expect(addToSteamNotices(state({ steamRunning: true, flatpakSteam: true }))).toEqual([
      "Settings_AddToSteam_RestartSteam",
      "Settings_AddToSteam_Flatpak",
    ]);
  });

  it("reports an entry that is on but not on disk", () => {
    expect(addToSteamNotices(state({ present: false }))).toEqual(["Settings_AddToSteam_Missing"]);
  });
});
