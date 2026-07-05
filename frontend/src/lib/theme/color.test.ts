import { describe, expect, it } from "vitest";
import { buildAccentOverlayCss, hexToRgb } from "./color";

function cssVar(css: string, name: string): string {
  const match = css.match(new RegExp(`${name}:\\s*([^;]+);`));
  if (!match) {
    throw new Error(`Missing CSS variable ${name}`);
  }
  return match[1].trim();
}

function relativeLuminance(hex: string): number {
  const { r, g, b } = hexToRgb(hex);
  const channel = (value: number): number => {
    const normalized = value / 255;
    return normalized <= 0.03928 ? normalized / 12.92 : ((normalized + 0.055) / 1.055) ** 2.4;
  };

  return (0.2126 * channel(r)) + (0.7152 * channel(g)) + (0.0722 * channel(b));
}

function contrastRatio(first: string, second: string): number {
  const lighter = Math.max(relativeLuminance(first), relativeLuminance(second));
  const darker = Math.min(relativeLuminance(first), relativeLuminance(second));
  return (lighter + 0.05) / (darker + 0.05);
}

function textOnBrightBg(accent: string): string {
  return cssVar(buildAccentOverlayCss(accent), "--text-on-bright-bg");
}

describe("buildAccentOverlayCss", () => {
  it("uses light text for dark custom accents", () => {
    const accent = "#123456";
    const textColor = textOnBrightBg(accent);

    expect(textColor).toBe("#f8f8f2");
    expect(contrastRatio(accent, textColor)).toBeGreaterThanOrEqual(4.5);
  });

  it("uses dark text for light custom accents", () => {
    const accent = "#f5d76e";
    const textColor = textOnBrightBg(accent);

    expect(textColor).toBe("#111111");
    expect(contrastRatio(accent, textColor)).toBeGreaterThanOrEqual(4.5);
  });

  it("uses higher-contrast dark text for highly saturated mid-light accents", () => {
    const accent = "#ff0000";
    const textColor = textOnBrightBg(accent);

    expect(textColor).toBe("#111111");
    expect(contrastRatio(accent, textColor)).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio(accent, textColor)).toBeGreaterThan(contrastRatio(accent, "#f8f8f2"));
  });
});
