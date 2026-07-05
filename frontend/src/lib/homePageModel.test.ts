import { describe, expect, it } from "vitest";
import {
  buildHomeDisabledRows,
  buildHomePrimaryRows,
  classifyHomeSearchPick,
} from "./homePageModel";

const tr = (key: string) => key;

describe("homePageModel", () => {
  it("keeps disabled platforms out of the primary activation rows", () => {
    const rows = buildHomePrimaryRows(
      ["Steam", "Epic Games", "GOG Galaxy"],
      ["epic games"],
      "",
      tr,
      5,
    );

    expect(rows).toEqual([
      {
        key: "p:Steam",
        title: "STEAM",
        badge: "Search_Section_Platform",
        platformIconName: "Steam",
      },
      {
        key: "p:GOG Galaxy",
        title: "GOG GALAXY",
        badge: "Search_Section_Platform",
        platformIconName: "GOG Galaxy",
      },
    ]);
  });

  it("surfaces disabled platforms separately as enable-first results", () => {
    const rows = buildHomeDisabledRows(
      ["Steam", "Epic Games", "GOG Galaxy"],
      "epic",
      tr,
      5,
    );

    expect(rows).toEqual([
      {
        key: "d:Epic Games",
        title: "EPIC GAMES",
        badge: "Search_Section_DisabledPlatform",
        platformIconName: "Epic Games",
        isCategory: true,
      },
    ]);
  });

  it("classifies home picks into command, enabled platform, and disabled platform flows", () => {
    expect(classifyHomeSearchPick("cmd:open-settings")).toEqual({
      kind: "command",
      value: "cmd:open-settings",
    });
    expect(classifyHomeSearchPick("p:Steam")).toEqual({
      kind: "platform",
      value: "Steam",
    });
    expect(classifyHomeSearchPick("d:Epic Games")).toEqual({
      kind: "disabled",
      value: "Epic Games",
    });
  });
});
