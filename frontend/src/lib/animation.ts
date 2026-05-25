import { fly, scale } from "svelte/transition";
import { cubicOut, quartOut } from "svelte/easing";
import { get } from "svelte/store";
import { animationsEnabled } from "../stores/animationSettings";

export const DUR = {
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
function noOpTransition() {
  return { duration: 0, css: () => "" };
}

/** Fade + slight upward drift for toasts, dropdowns, etc. */
export function fadeUp(node: Element, params?: { delay?: number; duration?: number }) {
  if (!motionEnabled()) return noOpTransition();
  return fly(node, {
    y: 10,
    duration: params?.duration ?? DUR.normal,
    delay: params?.delay ?? 0,
    easing: EASE.default,
    opacity: 0,
  });
}

/** Scale + fade for modals, menus, dialogs. */
export function scaleFade(node: Element, params?: { delay?: number; duration?: number }) {
  if (!motionEnabled()) return noOpTransition();
  return scale(node, {
    start: 0.96,
    duration: params?.duration ?? DUR.normal,
    delay: params?.delay ?? 0,
    easing: EASE.default,
    opacity: 0,
  });
}

/** Staggered entrance delay helper. */
export function staggerDelay(index: number, baseMs = 30, maxMs = 400): number {
  return Math.min(index * baseMs, maxMs);
}
