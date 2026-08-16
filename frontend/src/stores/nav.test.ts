import { get } from "svelte/store";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js", () => ({
  GetStartup: vi.fn(),
}));

import { previousPage, route } from "./nav";

/**
 * These guard the Svelte 5 remount loop: pages set their own route on mount,
 * and App rebuilds the page whenever this store changes. A fresh object for the
 * route we are already on used to tear the page down, whose onMount set the
 * route again, without bound. Dropping no-op writes is what stops it, so the
 * store must stay silent when the destination has not moved.
 */
describe("route store", () => {
  beforeEach(() => {
    route.set({ page: "home" });
  });

  it("stays silent when set to the route it already holds", () => {
    route.set({ page: "platform", platformName: "Steam" });

    let notifications = 0;
    const stop = route.subscribe(() => {
      notifications += 1;
    });
    expect(notifications).toBe(1); // subscribing always delivers the current value

    route.set({ page: "platform", platformName: "Steam" });

    expect(notifications).toBe(1);
    stop();
  });

  it("keeps the held object identical across a no-op write", () => {
    const first = { page: "platform", platformName: "Steam" } as const;
    route.set(first);
    route.set({ page: "platform", platformName: "Steam" });

    // Identity is the signal consumers key off; a new object means "navigated".
    expect(get(route)).toBe(first);
  });

  it("notifies when the page changes", () => {
    let notifications = 0;
    const stop = route.subscribe(() => {
      notifications += 1;
    });

    route.set({ page: "settings" });

    expect(notifications).toBe(2);
    expect(get(route)).toEqual({ page: "settings" });
    stop();
  });

  it("notifies when only the platform name changes", () => {
    route.set({ page: "platform", platformName: "Steam" });

    let notifications = 0;
    const stop = route.subscribe(() => {
      notifications += 1;
    });

    route.set({ page: "platform", platformName: "Discord" });

    expect(notifications).toBe(2);
    expect(get(route)).toEqual({ page: "platform", platformName: "Discord" });
    stop();
  });

  it("drops an update() that resolves to the same route", () => {
    const first = { page: "platform", platformName: "Steam" } as const;
    route.set(first);

    route.update((cur) => ({ ...cur }));

    expect(get(route)).toBe(first);
  });
});

describe("previousPage store", () => {
  it("stays silent when set to the route it already holds", () => {
    previousPage.set({ page: "home" });

    let notifications = 0;
    const stop = previousPage.subscribe(() => {
      notifications += 1;
    });

    previousPage.set({ page: "home" });

    expect(notifications).toBe(1);
    stop();
  });

  it("still accepts a real change, including back to null", () => {
    previousPage.set({ page: "home" });
    previousPage.set({ page: "platform", platformName: "Steam" });
    expect(get(previousPage)).toEqual({ page: "platform", platformName: "Steam" });

    previousPage.set(null);
    expect(get(previousPage)).toBeNull();
  });
});
