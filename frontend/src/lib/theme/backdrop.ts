import { hexToRgb, relativeLuminance, contrastRatio } from "./color";

/**
 * What the backend measured about a background image, or `measured: false` when
 * nobody has looked at it yet (a theme's bundled background, or one chosen
 * before the app started measuring).
 */
export type BackdropSample = {
  measured?: boolean;
  mean?: number;
  low?: number;
  high?: number;
};

export type BackdropInk = {
  /** Effective luminance of what on-background text actually sits on, 0..1. */
  luma: number;
  /** Effective p90-p10 gap. Wide means no single text colour covers the image. */
  spread: number;
  ink: string;
  /** True when the backdrop is too varied to trust one ink colour. */
  busy: boolean;
  polarity: "light" | "dark";
};

/** Text colours to choose between. Not pure black/white — those ring on a photo. */
export const LIGHT_INK = "#f5f7fa";
export const DARK_INK = "#10151b";

/**
 * Spread above which we stop trusting a single ink colour. A bright-sky-over-dark-
 * ground photo lands here: its mean says "mid grey" while both halves would fail
 * a different text colour, so the UI leans on a scrim instead of flipping ink.
 */
export const BUSY_SPREAD = 0.34;

/** Contrast a colour must clear against the backdrop before it counts as legible. */
const MIN_CONTRAST = 4.5;

/**
 * Blend an image's luminance with the surface it is painted over.
 *
 * The image sits at `opacity` over the theme's own page colour, so neither alone
 * is what text lands on. This blends in luminance space rather than compositing
 * per channel and re-deriving: the exact answer needs the mean *colour*, not just
 * the mean brightness, and for a light-or-dark decision the approximation is well
 * inside the margin. Blur is ignored deliberately — it is a low-pass filter, so it
 * leaves the mean untouched (it does narrow the spread, which `resolveBackdropInk`
 * accounts for).
 */
export function compositeLuma(imageLuma: number, opacity: number, baseLuma: number): number {
  const a = Math.min(1, Math.max(0, opacity));
  return (imageLuma * a) + (baseLuma * (1 - a));
}

/** Whichever of the two inks has more contrast against `luma`. */
function bestInk(luma: number): { ink: string; contrast: number; polarity: "light" | "dark" } {
  // A grey of this luminance stands in for the backdrop; contrastRatio only needs
  // the luminance, and building a grey is the cheapest way to hand it one.
  const channel = Math.round(255 * (luma <= 0.0031308 ? luma * 12.92 : (1.055 * luma ** (1 / 2.4)) - 0.055));
  const backdrop = { r: channel, g: channel, b: channel };

  const light = contrastRatio(backdrop, hexToRgb(LIGHT_INK));
  const dark = contrastRatio(backdrop, hexToRgb(DARK_INK));

  return light >= dark
    ? { ink: LIGHT_INK, contrast: light, polarity: "light" }
    : { ink: DARK_INK, contrast: dark, polarity: "dark" };
}

/**
 * Decide what colour text sitting directly on the background should be.
 *
 * Returns null when there is nothing to adapt to — no background image, or one
 * whose brightness nobody has measured. Callers leave the theme's own colours
 * alone in that case rather than guessing.
 */
export function resolveBackdropInk(
  sample: BackdropSample | null | undefined,
  opacity: number,
  baseLuma: number,
): BackdropInk | null {
  if (!sample?.measured || typeof sample.mean !== "number") {
    return null;
  }

  const a = Math.min(1, Math.max(0, opacity));
  const luma = compositeLuma(sample.mean, a, baseLuma);

  // The theme colour showing through at (1 - opacity) is flat, so it dilutes the
  // image's variation by the same factor it dilutes its brightness.
  const rawSpread = Math.max(0, (sample.high ?? sample.mean) - (sample.low ?? sample.mean));
  const spread = rawSpread * a;

  const { ink, contrast, polarity } = bestInk(luma);

  return {
    luma,
    spread,
    ink,
    // Busy either because the image varies too much for one colour, or because
    // even the better of the two inks does not clear the readability bar.
    busy: spread > BUSY_SPREAD || contrast < MIN_CONTRAST,
    polarity,
  };
}

/**
 * Sample a loaded image in the browser, for backgrounds the backend never measured.
 * Returns null if the image cannot be read (not yet decoded, or a tainted canvas).
 */
export function sampleImageElement(img: HTMLImageElement, grid = 32): BackdropSample | null {
  if (!img.complete || img.naturalWidth === 0 || img.naturalHeight === 0) {
    return null;
  }

  let data: Uint8ClampedArray;
  try {
    const canvas = document.createElement("canvas");
    canvas.width = grid;
    canvas.height = grid;
    const ctx = canvas.getContext("2d", { willReadFrequently: true });
    if (!ctx) {
      return null;
    }
    // Drawing the whole image into a grid-sized canvas averages each cell for us.
    ctx.drawImage(img, 0, 0, grid, grid);
    data = ctx.getImageData(0, 0, grid, grid).data;
  } catch {
    // Tainted canvas — cross-origin image. Nothing to report.
    return null;
  }

  const values: number[] = [];
  let sum = 0;
  for (let i = 0; i < data.length; i += 4) {
    if (data[i + 3] === 0) {
      continue;
    }
    const lum = relativeLuminance({ r: data[i], g: data[i + 1], b: data[i + 2] });
    values.push(lum);
    sum += lum;
  }
  if (values.length === 0) {
    return null;
  }

  values.sort((x, y) => x - y);
  const at = (p: number): number => values[Math.min(values.length - 1, Math.max(0, Math.floor(p * (values.length - 1))))];

  return { measured: true, mean: sum / values.length, low: at(0.1), high: at(0.9) };
}
