import { derived, writable } from "svelte/store";
import { Events } from "@wailsio/runtime";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

/** A fullscreen application from another process is in front. */
export const screenCovered = writable(false);

/** A game is running, windowed or not. */
export const gameRunning = writable(false);

/** Whether the switcher's own window is the one the user is looking at. */
export const windowFocused = writable(true);

/**
 * Whether the account list should hold its animated avatar frames still.
 *
 * Steam's frames are multi-megabyte APNGs and GIFs that keep decoding for as
 * long as they are in the document — around a quarter of a core on a list of a
 * dozen accounts — and neither CSS nor JS can pause an animated <img>. While a
 * game has the machine, that work is worth nothing, so the list freezes each
 * frame onto a canvas instead. The decoration stays put; it just stops moving.
 *
 * A running game only counts while the user is somewhere else. gameRunning is
 * read from Steam and says nothing about what is in front, so on its own it kept
 * the list frozen after the user tabbed back to it — the animations paused when
 * the game started and never came back while it was still open. Being looked at
 * is the whole justification for spending the core.
 *
 * Coverage is not qualified the same way. A fullscreen window from another
 * process being in front already means we are not, and trusting the page's own
 * idea of focus over that would risk animating behind a game, which is the exact
 * waste this exists to avoid.
 */
export const animationsSuspended = derived(
  [screenCovered, gameRunning, windowFocused],
  ([$covered, $game, $focused]) => $covered || ($game && !$focused),
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

  trackWindowFocus();
}

/**
 * Follow whether the page is the thing being looked at.
 *
 * Read from the document rather than from a window event on the Go side: focus
 * is the browser's own answer, it needs no round trip, and hasFocus is already
 * false for every route out of the window — alt-tab, another monitor, a game
 * taking the foreground — without each of them having to be enumerated.
 */
function trackWindowFocus(): void {
  if (typeof document === "undefined" || typeof window === "undefined") {
    return;
  }
  const sync = () => windowFocused.set(document.hasFocus() && !document.hidden);
  window.addEventListener("focus", sync);
  window.addEventListener("blur", sync);
  document.addEventListener("visibilitychange", sync);
  // Backstop for coming back. Activating the host window does not always land
  // focus inside the webview, and hasFocus would then still say no - leaving the
  // list frozen in front of the user, which is the bug this is here to fix.
  // Losing focus needs no equivalent: blur is reliable, and erring towards
  // animating something nobody asked to be frozen is the cheaper mistake.
  Events.On("common:WindowFocus", () => windowFocused.set(true));
  sync();
}
