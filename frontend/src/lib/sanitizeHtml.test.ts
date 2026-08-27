import { describe, expect, it } from "vitest";
import { safeUrl } from "./sanitizeHtml";

// sanitizeHtml() needs a DOM, which this suite does not have; safeUrl is the
// decision that mattered here and it is pure.
describe("sanitized attribute URLs", () => {
  // The backend rewrites the mini profile's avatar to a local path plus a
  // cache-busting query. Rejecting the query drops the src entirely.
  it("keeps a cache-busted local avatar", () => {
    const url = "/img/profiles/steam/76561198000000001.jpg?_tcv=1753440000000";
    expect(safeUrl(url, { image: true })).toBe(url);
  });

  it.each([
    "/img/profiles/steam/1.jpg",
    "img/icons/file.svg",
    "https://avatars.steamstatic.com/a.jpg",
  ])("keeps %s", (url) => {
    expect(safeUrl(url, { image: true })).toBeTruthy();
  });

  it.each([
    "javascript:alert(1)",
    "http://example.com/a.jpg",
    '/img/a.jpg" onerror="alert(1)',
    "/img/a.jpg?x=<script>",
    "",
  ])("drops %s", (url) => {
    expect(safeUrl(url, { image: true })).toBe("");
  });
});
