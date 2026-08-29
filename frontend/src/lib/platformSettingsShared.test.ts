import { describe, expect, it } from "vitest";
import {
  ARG_OFFLINE,
  hasLaunchArgFlag,
  withLaunchArgFlag,
} from "./platformSettingsShared";

describe("Steam offline launch argument", () => {
  it("uses Steam's offline launch flag", () => {
    expect(ARG_OFFLINE).toBe("-offline");
  });

  it("can be enabled and disabled without disturbing other launch arguments", () => {
    const enabled = withLaunchArgFlag("-silent -vgui", ARG_OFFLINE, true);
    expect(enabled).toBe("-silent -vgui -offline");
    expect(hasLaunchArgFlag(enabled, ARG_OFFLINE)).toBe(true);

    const disabled = withLaunchArgFlag(enabled, ARG_OFFLINE, false);
    expect(disabled).toBe("-silent -vgui");
    expect(hasLaunchArgFlag(disabled, ARG_OFFLINE)).toBe(false);
  });

  it("normalises duplicate and mixed-case offline flags", () => {
    expect(withLaunchArgFlag("-OFFLINE -silent -offline", ARG_OFFLINE, true)).toBe(
      "-silent -offline",
    );
  });
});
