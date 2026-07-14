export const BACKGROUND_ALIGNMENTS = ["center", "left", "right", "top", "bottom"] as const;
export const BACKGROUND_FITS = ["cover", "contain", "fill", "none", "scale-down"] as const;

export type BackgroundAlignment = (typeof BACKGROUND_ALIGNMENTS)[number];
export type BackgroundFit = (typeof BACKGROUND_FITS)[number];

export function normalizeBackgroundAlignment(value: string | undefined): BackgroundAlignment {
  return BACKGROUND_ALIGNMENTS.includes(value as BackgroundAlignment)
    ? (value as BackgroundAlignment)
    : "center";
}

export function normalizeBackgroundFit(value: string | undefined): BackgroundFit {
  return BACKGROUND_FITS.includes(value as BackgroundFit) ? (value as BackgroundFit) : "cover";
}

export function backgroundObjectPosition(alignment: BackgroundAlignment): string {
  switch (alignment) {
    case "left":
      return "left center";
    case "right":
      return "right center";
    case "top":
      return "center top";
    case "bottom":
      return "center bottom";
    default:
      return "center center";
  }
}
