import { fade, fly, scale, slide } from "svelte/transition";
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

/** Global guard: returns true when motion is enabled. */
export function motionEnabled(): boolean {
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
 * Height collapse for sections that open in place. Padding and margin travel
 * with the height, so the surrounding layout never jumps at the endpoints.
 */
export function collapse(node: Element, params?: MotionParams): TransitionConfig {
  if (!motionEnabled()) return noOpTransition();
  return slide(node, {
    duration: params?.duration ?? DUR.normal,
    delay: params?.delay ?? 0,
    easing: EASE.default,
  });
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
