import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const pageSource = readFileSync(new URL("../../pages/PlatformSettings.svelte", import.meta.url), "utf8");
const sectionSource = readFileSync(new URL("./PlatformSettingsSteamSection.svelte", import.meta.url), "utf8");

describe("Steam Offline Mode setting", () => {
  it("derives its checked state from Steam's saved launch arguments", () => {
    expect(pageSource).toContain("ARG_OFFLINE");
    expect(pageSource).toMatch(/\$: steamOfflineOn\s*=/);
    expect(pageSource).toContain("{steamOfflineOn}");
  });

  it("adds and removes the offline flag through an accessible settings toggle", () => {
    expect(sectionSource).toContain('id="ps-steam-offline"');
    expect(sectionSource).toContain("checked={steamOfflineOn}");
    expect(sectionSource).toContain('label={$t("Steam_OfflineMode")}');
    expect(sectionSource).toContain(
      'withLaunchArgFlag(steamSettings.LaunchArguments ?? "", ARG_OFFLINE, !steamOfflineOn)',
    );
  });

  it("saves the offline flag immediately instead of waiting for the settings debounce", () => {
    expect(sectionSource).toMatch(/id="ps-steam-offline"[\s\S]*?dispatch\("saveImmediate"\)/);
    expect(pageSource).toContain("function onSteamSaveImmediate(): void");
    expect(pageSource).toMatch(
      /function onSteamSaveImmediate\(\): void \{[\s\S]*?steamSavePending = true;[\s\S]*?void flushSteamSave\(\);/,
    );
    expect(pageSource).toContain("on:saveImmediate={onSteamSaveImmediate}");
    expect(pageSource).toContain("steamSwitchCoordinator.saveSettings(steamSettings)");
  });
});
