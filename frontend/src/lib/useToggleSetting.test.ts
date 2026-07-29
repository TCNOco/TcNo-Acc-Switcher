import { describe, it, expect, vi } from "vitest";
import { get } from "svelte/store";

// useToggleSetting.ts pulls in toasts and i18n — mock them all
vi.mock("./formatWailsError", () => ({
  formatWailsError: vi.fn(() => "error"),
}));

vi.mock("../stores/toast", () => ({
  pushToast: vi.fn(),
}));

vi.mock("../stores/i18n", () => ({
  t: {
    subscribe: (run: (value: unknown) => void) => {
      run(() => "");
      return () => {};
    },
  },
}));

import { createToggle } from "./useToggleSetting";

describe("createToggle", () => {
  it("keeps the new value once the save succeeds", async () => {
    const controller = createToggle(
      async () => false,
      async () => {},
      "label",
    );

    await controller.toggle();

    expect(get(controller.value)).toBe(true);
  });

  it("reverts the value when the save fails", async () => {
    const controller = createToggle(
      async () => false,
      async () => {
        throw new Error("backend refused");
      },
      "label",
    );

    await controller.toggle();

    expect(get(controller.value)).toBe(false);
  });

  it("shows the new value while the save is still in flight", async () => {
    let release = (): void => {};
    const inFlight = new Promise<void>((resolve) => {
      release = resolve;
    });
    const controller = createToggle(async () => false, () => inFlight, "label");

    const pending = controller.toggle();
    expect(get(controller.value)).toBe(true);
    expect(get(controller.loading)).toBe(true);

    release();
    await pending;
    expect(get(controller.value)).toBe(true);
    expect(get(controller.loading)).toBe(false);
  });

  it("does nothing while a save is already running", async () => {
    const setter = vi.fn(async () => {});
    const controller = createToggle(async () => false, setter, "label");
    controller.loading.set(true);

    await controller.toggle();

    expect(setter).not.toHaveBeenCalled();
    expect(get(controller.value)).toBe(false);
  });
});
