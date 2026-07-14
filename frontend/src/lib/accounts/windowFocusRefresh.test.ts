import { describe, expect, it, vi } from "vitest";
import {
  createLatestRequestGuard,
  registerWindowFocusAccountRefresh,
} from "./windowFocusRefresh";

function deferred(): { promise: Promise<void>; resolve: () => void } {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => { resolve = done; });
  return { promise, resolve };
}

describe("window focus account refresh", () => {
  it("rejects an older account response after a newer reload starts", () => {
    const guard = createLatestRequestGuard();
    const oldRequest = guard.begin();
    const currentRequest = guard.begin();

    expect(guard.isCurrent(oldRequest)).toBe(false);
    expect(guard.isCurrent(currentRequest)).toBe(true);
  });

  it("reloads accounts when the Wails window regains focus", async () => {
    let focusHandler: (() => void) | undefined;
    const unsubscribe = vi.fn();
    const refresh = vi.fn(async () => {});
    const stop = registerWindowFocusAccountRefresh(
      (handler) => { focusHandler = handler; return unsubscribe; },
      refresh,
    );

    focusHandler?.();
    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));

    stop();
    expect(unsubscribe).toHaveBeenCalledTimes(1);
  });

  it("coalesces repeated focus events while a reload is still running", async () => {
    let focusHandler: (() => void) | undefined;
    const first = deferred();
    const refresh = vi.fn()
      .mockImplementationOnce(() => first.promise)
      .mockResolvedValue(undefined);
    registerWindowFocusAccountRefresh(
      (handler) => { focusHandler = handler; return () => {}; },
      refresh,
    );

    focusHandler?.();
    focusHandler?.();
    focusHandler?.();
    expect(refresh).toHaveBeenCalledTimes(1);

    first.resolve();
    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(2));
  });
});
