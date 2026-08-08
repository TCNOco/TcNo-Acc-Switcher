import { readable } from "svelte/store";

/**
 * Wall clock at one-second resolution, shared by everything that counts down.
 *
 * readable's start/stop notifier is the whole lifecycle: the interval exists
 * only while something is subscribed, so a list that re-keys its rows cannot
 * leak a timer, and every subscriber ticks off the same instant rather than
 * drifting apart. One timer for the page, not one per row.
 */
export const nowSecond = readable(Date.now(), (set) => {
  const timer = setInterval(() => set(Date.now()), 1_000);
  return () => clearInterval(timer);
});
