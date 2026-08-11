import { describe, expect, it } from "vitest";
import { accountAvatar, generatedAvatar, makeRng, resetGeneratedAvatarCache } from "./generatedAvatar";

const draw = (seed: string, n = 24): number[] => {
  const rng = makeRng(seed);
  return Array.from({ length: n }, () => rng.next());
};

describe("avatar seeding", () => {
  it("paints the same picture for the same seed", () => {
    // No disk cache backs these avatars: an account keeps its face only because
    // the same seed replays the same draws, every run.
    expect(draw("salt Steam 76561198000000001")).toEqual(draw("salt Steam 76561198000000001"));
  });

  it("gives neighbouring account IDs unrelated pictures", () => {
    // Consecutive SteamID64s differ in one digit; a weak hash would put them in
    // the same palette and composition and make a list look duplicated.
    const a = draw("salt Steam 76561198000000001");
    const b = draw("salt Steam 76561198000000002");
    expect(a).not.toEqual(b);
    expect(a.filter((v, i) => Math.abs(v - b[i]) < 0.01).length).toBeLessThan(4);
  });

  it("gives the same account a different picture on a different machine", () => {
    // The point of the machine salt: a stream viewer cannot reproduce an avatar
    // by guessing the SteamID.
    expect(draw("saltA Steam 7656119800000001")).not.toEqual(draw("saltB Steam 7656119800000001"));
  });

  it("keeps values inside the unit interval", () => {
    for (const value of draw("range check", 200)) {
      expect(value).toBeGreaterThanOrEqual(0);
      expect(value).toBeLessThan(1);
    }
  });
});

describe("accountAvatar", () => {
  it("refuses to seed from an empty account key", () => {
    // A blank key would hand every unidentified account the same face.
    expect(accountAvatar("salt", "Steam", "   ")).toBe("");
  });

  it("returns an empty string rather than throwing where there is no canvas", () => {
    // jsdom has no 2D context; callers fall back to the platform placeholder.
    resetGeneratedAvatarCache();
    expect(generatedAvatar("anything")).toBe("");
  });
});
