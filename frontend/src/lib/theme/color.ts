import { DEFAULT_THEME_OPTION } from "./types";

export function normalizeHexColor(value: unknown): string | null {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  const match = trimmed.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i);
  if (!match) {
    return null;
  }
  const hex = match[1];
  if (hex.length === 3) {
    return `#${hex
      .split("")
      .map((ch) => `${ch}${ch}`)
      .join("")
      .toLowerCase()}`;
  }
  return `#${hex.toLowerCase()}`;
}

export function hexToRgb(hex: string): { r: number; g: number; b: number } {
  const normalized = normalizeHexColor(hex) ?? DEFAULT_THEME_OPTION.defaultAccentColor;
  return {
    r: Number.parseInt(normalized.slice(1, 3), 16),
    g: Number.parseInt(normalized.slice(3, 5), 16),
    b: Number.parseInt(normalized.slice(5, 7), 16),
  };
}

export function rgbToHsl({ r, g, b }: { r: number; g: number; b: number }): { h: number; s: number; l: number } {
  const nr = r / 255;
  const ng = g / 255;
  const nb = b / 255;
  const max = Math.max(nr, ng, nb);
  const min = Math.min(nr, ng, nb);
  const delta = max - min;

  let h = 0;
  if (delta !== 0) {
    switch (max) {
      case nr:
        h = ((ng - nb) / delta) % 6;
        break;
      case ng:
        h = (nb - nr) / delta + 2;
        break;
      default:
        h = (nr - ng) / delta + 4;
        break;
    }
  }
  h = Math.round(((h * 60) + 360) % 360);

  const l = (max + min) / 2;
  const s = delta === 0 ? 0 : delta / (1 - Math.abs((2 * l) - 1));

  return {
    h,
    s: Math.round(s * 100),
    l: Math.round(l * 100),
  };
}

export function buildAccentOverlayCss(color: string): string {
  const normalized = normalizeHexColor(color) ?? DEFAULT_THEME_OPTION.defaultAccentColor;
  const rgb = hexToRgb(normalized);
  const hsl = rgbToHsl(rgb);
  const textOnBrightBg = hsl.l >= 58 ? "#111111" : "#f8f8f2";
  return `
:root {
  --accent: ${normalized};
  --accentHS: ${hsl.h}, ${hsl.s}%;
  --accentL: ${hsl.l}%;
  --accentInt: ${rgb.r}, ${rgb.g}, ${rgb.b};
  --text-on-bright-bg: ${textOnBrightBg};
  --notification-info-border: var(--accent);
  --accent-border-deep: hsl(var(--accentHS), 100%, 9%);
  --accent-text-bright: hsl(var(--accentHS), calc(var(--accentL) + 32%));
  --accent-border-strong: hsl(var(--accentHS), 65%, 40%);
  --accent-type-dim: hsl(var(--accentHS), calc(var(--accentL) - 12%));
  --accent-overlay-border: rgba(var(--accentInt), 0.35);
  --accent-overlay-border-strong: rgba(var(--accentInt), 0.55);
  --accent-overlay-hover: rgba(var(--accentInt), 0.85);
  --accent-fill-very-soft: rgba(var(--accentInt), 0.12);
  --accent-fill-soft: rgba(var(--accentInt), 0.32);
  --accent-fill-medium: rgba(var(--accentInt), 0.42);
  --accent-fill-panel: rgba(var(--accentInt), 0.45);
  --accent-highlight-row: rgba(var(--accentInt), 0.9);
  --accent-text-on-selection: rgba(var(--accentInt), 0.95);
  --accent-text-heading: rgba(var(--accentInt), 0.98);
  --home-ico-gradient-end: rgba(var(--accentInt), 1);
  --miniprofile-radial-glow: rgba(var(--accentInt), 0.45);
  --miniprofile-radial-fade: rgba(var(--accentInt), 0.08);
  --miniprofile-linear-top: rgba(var(--accentInt), 0.55);
  --miniprofile-linear-mid: rgba(var(--accentInt), 0.14);
  --miniprofile-linear-low: rgba(var(--accentInt), 0.1);
}`;
}
