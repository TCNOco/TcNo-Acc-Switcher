import { relativeLuminance } from "./color";
import { resolveBackdropInk, sampleImageElement } from "./backdrop";
import type { BackdropSample, BackdropInk } from "./backdrop";

const POLARITY_ATTR = "data-backdrop";
const BUSY_ATTR = "data-backdrop-busy";

/** Scrims sit behind on-background text; each opposes its ink. */
const DARK_SCRIM = "rgba(0, 0, 0, 0.55)";
const LIGHT_SCRIM = "rgba(255, 255, 255, 0.62)";

function parseCssRgb(value: string): { r: number; g: number; b: number } | null {
  const match = value.match(/rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)(?:[\s,/]+([\d.]+%?))?/i);
  if (!match) {
    return null;
  }
  // A fully transparent fill tells us nothing about what is painted underneath.
  if (match[4] !== undefined && Number.parseFloat(match[4]) === 0) {
    return null;
  }
  return { r: Number.parseFloat(match[1]), g: Number.parseFloat(match[2]), b: Number.parseFloat(match[3]) };
}

/**
 * Luminance of the surface the background image is painted over.
 *
 * Walks outwards from the page root, because the elements nearest the image are
 * deliberately transparent when a background is showing; the first ancestor with
 * an actual fill is what the image is really composited against. Falls back to
 * dark, which matches the default theme and every other dark one.
 */
export function themeBaseLuma(): number {
  if (typeof document === "undefined" || typeof getComputedStyle === "undefined") {
    return 0.05;
  }
  const chain = [
    document.querySelector(".page"),
    document.body,
    document.documentElement,
  ];
  for (const node of chain) {
    if (!node) {
      continue;
    }
    const rgb = parseCssRgb(getComputedStyle(node).backgroundColor);
    if (rgb) {
      return relativeLuminance(rgb);
    }
  }
  return 0.05;
}

/**
 * Publish the chosen ink to CSS. Clearing it (null) is what puts a theme back in
 * charge of its own colours, so it must undo everything it sets.
 */
export function applyBackdropInk(ink: BackdropInk | null): void {
  if (typeof document === "undefined") {
    return;
  }
  const root = document.documentElement;

  if (!ink) {
    root.removeAttribute(POLARITY_ATTR);
    root.removeAttribute(BUSY_ATTR);
    root.style.removeProperty("--auto-ink");
    root.style.removeProperty("--auto-ink-scrim");
    root.style.removeProperty("--backdrop-luma");
    return;
  }

  root.setAttribute(POLARITY_ATTR, ink.polarity);
  root.setAttribute(BUSY_ATTR, ink.busy ? "true" : "false");
  root.style.setProperty("--auto-ink", ink.ink);
  root.style.setProperty("--auto-ink-scrim", ink.polarity === "light" ? DARK_SCRIM : LIGHT_SCRIM);
  root.style.setProperty("--backdrop-luma", ink.luma.toFixed(3));
}

/**
 * Work out and apply the ink for the background currently on screen.
 *
 * `sample` is what the backend measured. When it did not measure anything — a
 * theme's bundled background, or one chosen before measuring existed — and the
 * image element is available and decoded, it is sampled here instead.
 */
export function syncBackdropInk(
  sample: BackdropSample | null | undefined,
  opacity: number,
  img?: HTMLImageElement | null,
): BackdropInk | null {
  let measured = sample;
  if (!measured?.measured && img) {
    measured = sampleImageElement(img) ?? measured;
  }
  const ink = resolveBackdropInk(measured, opacity, themeBaseLuma());
  applyBackdropInk(ink);
  return ink;
}
