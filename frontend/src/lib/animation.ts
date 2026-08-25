import { fade, fly, scale } from "svelte/transition";
import { cubicOut, quartOut } from "svelte/easing";
import type { TransitionConfig } from "svelte/transition";
import { get } from "svelte/store";
import { animationsEnabled } from "../stores/animationSettings";

export const DUR = {
  /** Feedback that must not read as a delay — scrims, hover-scale swaps. */
  instant: 90,
  fast: 120,
  normal: 200,
  slow: 300,
} as const;

export const EASE = {
  default: cubicOut,
  snappy: quartOut,
} as const;

/**
 * Matched once: the list stays live, so a change to the OS setting is picked up
 * without a reload. Undefined where matchMedia is not implemented (tests).
 */
const reducedMotionQuery =
  typeof window !== "undefined" && typeof window.matchMedia === "function"
    ? window.matchMedia("(prefers-reduced-motion: reduce)")
    : null;

/**
 * Global guard: true when motion is enabled.
 *
 * The OS preference counts as well as the app setting. animations.scss already
 * neutralises CSS animations under `prefers-reduced-motion`, and every
 * transition here compiles down to one - but Svelte still holds a leaving
 * element for the length of the duration it was handed. Answering false here is
 * what actually removes it on the frame it was told to go.
 */
export function motionEnabled(): boolean {
  if (reducedMotionQuery?.matches) return false;
  return get(animationsEnabled);
}

/** No-op transition for when motion is disabled. */
function noOpTransition(): TransitionConfig {
  return { duration: 0, css: () => "" };
}

type MotionParams = { delay?: number; duration?: number };

/** Opacity only. Scrims, backdrops, anything that must not move the layout. */
export function fadeMotion(node: Element, params?: MotionParams): TransitionConfig {
  if (!motionEnabled()) return noOpTransition();
  return fade(node, {
    duration: params?.duration ?? DUR.fast,
    delay: params?.delay ?? 0,
    easing: EASE.default,
  });
}

/** Fade + slight upward drift for toasts, dropdowns, list rows. */
export function fadeUp(node: Element, params?: MotionParams & { y?: number }): TransitionConfig {
  if (!motionEnabled()) return noOpTransition();
  return fly(node, {
    y: params?.y ?? 10,
    duration: params?.duration ?? DUR.normal,
    delay: params?.delay ?? 0,
    easing: EASE.default,
    opacity: 0,
  });
}

/** Scale + fade for modals, menus, dialogs, floating panels. */
export function scaleFade(
  node: Element,
  params?: MotionParams & { start?: number },
): TransitionConfig {
  if (!motionEnabled()) return noOpTransition();
  return scale(node, {
    start: params?.start ?? 0.96,
    duration: params?.duration ?? DUR.normal,
    delay: params?.delay ?? 0,
    easing: EASE.default,
    opacity: 0,
  });
}

/**
 * Height collapse for bars and sections that open in place. Padding, margin and
 * border travel with the height so the surrounding layout never jumps at the
 * endpoints.
 *
 * Not `svelte/transition`'s `slide`: that one animates `height` alone, and a bar
 * carrying its own `min-height` - the action bar does - simply refuses to
 * shrink past it. Zeroing `min-height` for the duration is the whole difference.
 */
export function collapse(node: Element, params?: MotionParams): TransitionConfig {
  if (!motionEnabled()) return noOpTransition();
  const style = getComputedStyle(node);
  const opacity = Number(style.opacity);
  const height = parseFloat(style.height);
  const paddingTop = parseFloat(style.paddingTop);
  const paddingBottom = parseFloat(style.paddingBottom);
  const marginTop = parseFloat(style.marginTop);
  const marginBottom = parseFloat(style.marginBottom);
  const borderTop = parseFloat(style.borderTopWidth);
  const borderBottom = parseFloat(style.borderBottomWidth);
  return {
    delay: params?.delay ?? 0,
    duration: params?.duration ?? DUR.normal,
    easing: EASE.default,
    css: (t) =>
      "overflow: hidden;" +
      "min-height: 0;" +
      `opacity: ${Math.min(t * 20, 1) * opacity};` +
      `height: ${t * height}px;` +
      `padding-top: ${t * paddingTop}px;` +
      `padding-bottom: ${t * paddingBottom}px;` +
      `margin-top: ${t * marginTop}px;` +
      `margin-bottom: ${t * marginBottom}px;` +
      `border-top-width: ${t * borderTop}px;` +
      `border-bottom-width: ${t * borderBottom}px;`,
  };
}

type TileParams = MotionParams & { enabled?: boolean; y?: number; start?: number };

/**
 * Grid tiles arriving: rise a little and settle to full size. `scale` alone
 * reads as a pop and `fly` alone as a slide; together they read as the tile
 * coming forward into place.
 */
export function tileIn(node: Element, params?: TileParams): TransitionConfig {
  if (!motionEnabled() || params?.enabled === false) return noOpTransition();
  const y = params?.y ?? 6;
  const start = params?.start ?? 0.94;
  return {
    delay: params?.delay ?? 0,
    duration: params?.duration ?? DUR.normal,
    easing: EASE.default,
    css: (t, u) =>
      `opacity: ${t}; transform: translateY(${u * y}px) scale(${start + (1 - start) * t});`,
  };
}

/** Grid tiles leaving: shrink out of the way, quicker than they arrived. */
export function tileOut(node: Element, params?: TileParams): TransitionConfig {
  if (!motionEnabled() || params?.enabled === false) return noOpTransition();
  const start = params?.start ?? 0.9;
  return {
    delay: params?.delay ?? 0,
    duration: params?.duration ?? DUR.fast,
    easing: EASE.default,
    css: (t) => `opacity: ${t}; transform: scale(${start + (1 - start) * t});`,
  };
}

/**
 * Staggered entrance delay. Capped so a long list still finishes promptly
 * rather than trickling in for seconds.
 */
export function staggerDelay(index: number, baseMs = 22, maxMs = 240): number {
  if (!motionEnabled()) return 0;
  return Math.min(Math.max(index, 0) * baseMs, maxMs);
}

/**
 * `animate:flip` parameters for reflowing grids. Duration scales with how far
 * the tile actually travels, so a one-slot nudge stays crisp while a jump
 * across the grid remains readable.
 */
export function flipMotion(params?: { enabled?: boolean }): {
  duration: number | ((len: number) => number);
  easing: typeof cubicOut;
} {
  const on = motionEnabled() && params?.enabled !== false;
  return {
    duration: on ? (len: number) => Math.min(DUR.slow, 60 + Math.sqrt(len) * 14) : 0,
    easing: EASE.default,
  };
}
