import { beforeEach, describe, expect, it } from "vitest";
import { get } from "svelte/store";
import { animationsSuspended, gameRunning, screenCovered, windowFocused } from "./screenCovered";

describe("animationsSuspended", () => {
  beforeEach(() => {
    screenCovered.set(false);
    gameRunning.set(false);
    windowFocused.set(true);
  });

  it("lets the list animate when nothing is in the way", () => {
    expect(get(animationsSuspended)).toBe(false);
  });

  // The reported bug: the animations paused when the game started and never came
  // back while it was still open, however many times the user tabbed across.
  it("resumes once the user tabs back, with the game still running", () => {
    gameRunning.set(true);
    windowFocused.set(false);
    expect(get(animationsSuspended)).toBe(true);

    windowFocused.set(true);
    expect(get(animationsSuspended)).toBe(false);
  });

  it("freezes again when the user leaves for the running game", () => {
    gameRunning.set(true);
    windowFocused.set(true);
    expect(get(animationsSuspended)).toBe(false);

    windowFocused.set(false);
    expect(get(animationsSuspended)).toBe(true);
  });

  // Coverage is not qualified by focus: a fullscreen window from another process
  // being in front already means we are not, and animating behind a game is the
  // waste the whole thing exists to avoid.
  it("stays frozen while a fullscreen app covers the screen", () => {
    screenCovered.set(true);
    windowFocused.set(true);
    expect(get(animationsSuspended)).toBe(true);
  });

  it("does not freeze an unfocused window when no game is running", () => {
    windowFocused.set(false);
    expect(get(animationsSuspended)).toBe(false);
  });
});
