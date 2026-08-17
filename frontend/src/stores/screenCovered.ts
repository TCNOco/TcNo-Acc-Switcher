import { derived, writable } from "svelte/store";
import { Events } from "@wailsio/runtime";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

/** A fullscreen application from another process is in front. */
export const screenCovered = writable(false);

/** A game is running, windowed or not. */
export const gameRunning = writable(false);

/**
 * Whether the account list should hold its animated avatar frames still.
 *
 * Steam's frames are multi-megabyte APNGs and GIFs that keep decoding for as
 * long as they are in the document — around a quarter of a core on a list of a
 * dozen accounts — and neither CSS nor JS can pause an animated <img>. While a
 * game has the machine, that work is worth nothing, so the list freezes each
 * frame onto a canvas instead. The decoration stays put; it just stops moving.
 */
export const animationsSuspended = derived(
  [screenCovered, gameRunning],
  ([$covered, $game]) => $covered || $game,
);

let started = false;

/** Hydrate from the backend, then follow it. Safe to call more than once. */
export function initScreenCovered(): void {
  if (started) {
    return;
  }
  started = true;

  // Both events can fire before any page is listening, so ask for the current
  // answer once rather than assuming "running freely" until the next change.
  void PlatformService.GetScreenCovered()
    .then((covered) => screenCovered.set(covered === true))
    .catch(() => {
      /* No backend (browser preview); leaving it false keeps animations on. */
    });
  void PlatformService.GetGameRunning()
    .then((running) => gameRunning.set(running === true))
    .catch(() => {
      /* Same. */
    });

  Events.On("screen-covered", (ev) => {
    screenCovered.set(ev.data === true);
  });
  Events.On("game-running", (ev) => {
    gameRunning.set(ev.data === true);
  });
}
