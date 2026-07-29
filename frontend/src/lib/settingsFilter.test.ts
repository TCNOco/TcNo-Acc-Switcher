import { describe, it, expect } from "vitest";
import { matchesQuery, queryTokens } from "./settingsFilter";

describe("queryTokens", () => {
  it("splits on runs of whitespace", () => {
    expect(queryTokens("  start   centered ")).toEqual(["start", "centered"]);
  });

  it("returns nothing for a blank query", () => {
    expect(queryTokens("   ")).toEqual([]);
  });

  it("folds diacritics so translated labels stay reachable from a plain keyboard", () => {
    expect(queryTokens("Zurücksetzen")).toEqual(["zurucksetzen"]);
  });
});

describe("matchesQuery", () => {
  it("requires every token, in any order", () => {
    const tokens = queryTokens("centered start");
    expect(matchesQuery("Start program centered", tokens)).toBe(true);
    expect(matchesQuery("Start program", tokens)).toBe(false);
  });

  it("ignores case on both sides", () => {
    expect(matchesQuery("Offline Mode (No network activity)", queryTokens("OFFLINE"))).toBe(true);
  });

  it("matches folded text against a folded query", () => {
    expect(matchesQuery("Einstellungen zurücksetzen", queryTokens("zuruck"))).toBe(true);
  });

  it("matches everything when there are no tokens", () => {
    expect(matchesQuery("anything", [])).toBe(true);
  });
});
