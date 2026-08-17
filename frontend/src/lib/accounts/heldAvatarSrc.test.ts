import { beforeEach, describe, expect, it } from "vitest";
import { heldAvatarSrc, resetHeldAvatars, setAvatarPreloader } from "./heldAvatarSrc";

/** A preloader whose outcome each test decides, and settles on demand. */
function controllablePreloader() {
  const pending: Array<{ src: string; resolve: () => void; reject: () => void }> = [];
  setAvatarPreloader(
    (src) =>
      new Promise<void>((resolve, reject) => {
        pending.push({ src, resolve: () => resolve(), reject: () => reject(new Error("failed")) });
      }),
  );
  return {
    pending,
    async settle(src: string, ok: boolean) {
      const entry = pending.find((p) => p.src === src);
      if (!entry) throw new Error(`nothing preloading ${src}`);
      if (ok) entry.resolve();
      else entry.reject();
      // Let the .then callbacks run.
      await Promise.resolve();
      await Promise.resolve();
    },
  };
}

describe("heldAvatarSrc", () => {
  beforeEach(() => {
    resetHeldAvatars();
    setAvatarPreloader(null);
  });

  it("paints the first src immediately, since there is nothing to preserve", () => {
    expect(heldAvatarSrc("acc", "/img/a.jpg")).toBe("/img/a.jpg");
  });

  it("keeps the old src on screen while the new one loads", async () => {
    const loader = controllablePreloader();
    heldAvatarSrc("acc", "/img/a.jpg");

    expect(heldAvatarSrc("acc", "/img/b.jpg")).toBe("/img/a.jpg");
    await loader.settle("/img/b.jpg", true);
    expect(heldAvatarSrc("acc", "/img/b.jpg")).toBe("/img/b.jpg");
  });

  // The case this exists for: a refresh points at a file that is still
  // downloading, or was refused. The face already on screen beats a placeholder.
  it("keeps the old src when the new one fails to load", async () => {
    const loader = controllablePreloader();
    heldAvatarSrc("acc", "/img/a.jpg");

    expect(heldAvatarSrc("acc", "/img/b.jpg")).toBe("/img/a.jpg");
    await loader.settle("/img/b.jpg", false);
    expect(heldAvatarSrc("acc", "/img/b.jpg")).toBe("/img/a.jpg");
  });

  // avatarPending swaps the desired src to the platform placeholder. Holding the
  // real face through it is the whole point.
  it("does not fall back to the placeholder mid-refresh", () => {
    controllablePreloader();
    heldAvatarSrc("acc", "/img/a.jpg");
    expect(heldAvatarSrc("acc", "/img/BasicDefault.webp")).toBe("/img/a.jpg");
  });

  it("ignores a load that finished after a newer src was asked for", async () => {
    const loader = controllablePreloader();
    heldAvatarSrc("acc", "/img/a.jpg");
    heldAvatarSrc("acc", "/img/b.jpg");
    heldAvatarSrc("acc", "/img/c.jpg");

    await loader.settle("/img/b.jpg", true);
    expect(heldAvatarSrc("acc", "/img/c.jpg")).toBe("/img/a.jpg");
    await loader.settle("/img/c.jpg", true);
    expect(heldAvatarSrc("acc", "/img/c.jpg")).toBe("/img/c.jpg");
  });

  it("holds each account separately", async () => {
    const loader = controllablePreloader();
    heldAvatarSrc("one", "/img/one-a.jpg");
    heldAvatarSrc("two", "/img/two-a.jpg");

    expect(heldAvatarSrc("one", "/img/one-b.jpg")).toBe("/img/one-a.jpg");
    await loader.settle("/img/one-b.jpg", true);
    expect(heldAvatarSrc("one", "/img/one-b.jpg")).toBe("/img/one-b.jpg");
    expect(heldAvatarSrc("two", "/img/two-a.jpg")).toBe("/img/two-a.jpg");
  });

  // An Image cannot report whether a webm loaded, so there is nothing to wait on.
  it("swaps a video straight in", () => {
    controllablePreloader();
    heldAvatarSrc("acc", "/img/a.jpg");
    expect(heldAvatarSrc("acc", "/img/a.webm")).toBe("/img/a.webm");
  });

  it("passes an empty src through rather than holding anything", () => {
    heldAvatarSrc("acc", "/img/a.jpg");
    expect(heldAvatarSrc("acc", "")).toBe("");
  });
});
