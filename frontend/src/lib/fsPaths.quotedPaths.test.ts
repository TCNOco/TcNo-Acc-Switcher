import { describe, expect, it } from "vitest";
import { normalizeDisplayPath, parentDisplayPath, sameFsPath, stripSurroundingQuotes } from "./fsPaths";

// Explorer's "Copy as path" hands over a quoted path. The quotes reached the
// Go stat, which only trims and cleans, so every pasted path was rejected -
// and the picker is the only way back for a platform that was not auto-found.
describe("quoted paths", () => {
  it("takes the quotes off a path pasted from Explorer", () => {
    expect(stripSurroundingQuotes('"C:\\Games\\Delta Force\\game.exe"')).toBe(
      "C:\\Games\\Delta Force\\game.exe",
    );
  });

  it("leaves an unquoted path alone", () => {
    expect(stripSurroundingQuotes("C:\\Games\\game.exe")).toBe("C:\\Games\\game.exe");
  });

  it("keeps a lone quote, which can be part of a name", () => {
    expect(stripSurroundingQuotes('/home/tcno/it"s')).toBe('/home/tcno/it"s');
    expect(stripSurroundingQuotes('"unterminated')).toBe('"unterminated');
  });

  it("still recognises Windows paths once the quote is gone", () => {
    expect(normalizeDisplayPath('"C:/Games//Delta Force/game.exe"')).toBe(
      "C:\\Games\\Delta Force\\game.exe",
    );
  });

  it("treats a quoted and an unquoted path as the same place", () => {
    expect(sameFsPath('"C:\\Games\\game.exe"', "C:\\Games\\game.exe")).toBe(true);
  });

  it("finds the parent of a quoted path", () => {
    expect(parentDisplayPath('"C:\\Games\\Delta Force\\game.exe"')).toBe("C:\\Games\\Delta Force");
  });
});
