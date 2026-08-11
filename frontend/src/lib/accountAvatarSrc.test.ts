import { describe, expect, it, vi } from "vitest";
import { accountAvatarSrc, type AccountAvatarInput } from "./accountAvatarSrc";

// jsdom has no canvas, so the generator returns "" and the resolver falls through
// to the platform placeholder. That is the behaviour worth pinning: a missing
// canvas must never leave an account tile with a broken image.
vi.mock("./generatedAvatar", () => ({
  accountAvatar: (salt: string, platform: string, key: string) =>
    key ? `generated:${salt}:${platform}:${key}` : "",
}));

const base: AccountAvatarInput = {
  streamer: false,
  salt: "salt",
  platformKey: "Steam",
  accountKey: "76561198000000001",
  imageUrl: "https://avatars.example/real.jpg",
  pending: false,
  epoch: 0,
  offline: false,
  fallback: "/img/BasicDefault.webp",
};

describe("accountAvatarSrc", () => {
  it("shows the real avatar when there is nothing to hide", () => {
    expect(accountAvatarSrc(base)).toContain("https://avatars.example/real.jpg");
  });

  it("never shows the real avatar in streamer mode", () => {
    const src = accountAvatarSrc({ ...base, streamer: true });
    expect(src).toBe("generated:salt:Steam:76561198000000001");
  });

  it("generates a stand-in for an account that has no avatar at all", () => {
    expect(accountAvatarSrc({ ...base, imageUrl: "" })).toBe("generated:salt:Steam:76561198000000001");
  });

  it("leaves an in-flight avatar on the platform placeholder", () => {
    // Swapping a generated face in mid-refresh and back out again reads as a bug.
    expect(accountAvatarSrc({ ...base, imageUrl: "", pending: true })).toBe("/img/BasicDefault.webp");
  });

  it("generates rather than falling back when offline blocks a remote avatar", () => {
    expect(accountAvatarSrc({ ...base, offline: true })).toBe("generated:salt:Steam:76561198000000001");
  });

  it("falls back to the placeholder when generation is unavailable", () => {
    expect(accountAvatarSrc({ ...base, imageUrl: "", accountKey: "" })).toBe("/img/BasicDefault.webp");
  });

  it("still censors when generation is unavailable", () => {
    const src = accountAvatarSrc({ ...base, streamer: true, accountKey: "" });
    expect(src).toBe("/img/BasicDefault.webp");
  });
});
