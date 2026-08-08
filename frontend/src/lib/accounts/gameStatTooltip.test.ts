import { describe, expect, it } from "vitest";
import { resolveGameStatTooltip } from "./gameStatTooltip";

const messages: Record<string, string> = {
  Tooltip_Cs2PremierRating: "Bewertung im Premier-Modus",
};
const translate = (key: string) => messages[key] ?? key;

describe("resolveGameStatTooltip", () => {
  it("passes plain text through", () => {
    expect(resolveGameStatTooltip("This shows you X", translate)).toBe("This shows you X");
  });

  it("translates an i18n key into the active language", () => {
    expect(resolveGameStatTooltip("i18n:Tooltip_Cs2PremierRating", translate)).toBe(
      "Bewertung im Premier-Modus",
    );
  });

  it("falls back to the key when it is not in the catalogue", () => {
    expect(resolveGameStatTooltip("i18n:Tooltip_Typo", translate)).toBe("Tooltip_Typo");
  });

  it("yields nothing for missing, blank or keyless values", () => {
    expect(resolveGameStatTooltip(undefined, translate)).toBe("");
    expect(resolveGameStatTooltip("   ", translate)).toBe("");
    expect(resolveGameStatTooltip("i18n:", translate)).toBe("");
  });
});
