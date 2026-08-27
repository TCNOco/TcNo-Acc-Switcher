import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const source = readFileSync(new URL("./SteamGuardModalBody.svelte", import.meta.url), "utf8");

/**
 * One function's body, bounded by the first line that closes a brace at a single
 * tab — the file is tab-indented, so that is the function's own close, and it
 * survives edits inside the body where a fixed character window would not.
 */
function functionSource(name: string): string {
  const start = source.indexOf(`function ${name}(`);
  expect(start).toBeGreaterThanOrEqual(0);
  const end = source.indexOf("\n\t}", start);
  expect(end).toBeGreaterThan(start);
  return source.slice(start, end);
}

describe("Steam Guard QR scanning", () => {
  // These wait on the user — a file picker, a Steam window — so they are the
  // flows a background vault write is most likely to land in the middle of.
  it.each([
    ["scanSteamWindow", "captureQrFromSteam"],
    ["chooseQrScreenshot", "decode("],
    ["decodeDroppedQRScreenshot", "decode("],
  ])("recovers a superseded capability in %s", (handler, call) => {
    const body = functionSource(handler);
    expect(body).toContain("withCapability(currentAccount");
    expect(body).toContain(call);
    expect(body).not.toContain("ensureCapability");
  });

  // Picking the file is not part of the retry: reopening the native picker to
  // recover a capability would make the user choose the same screenshot twice.
  it("picks the screenshot once, outside the retry", () => {
    const body = functionSource("chooseQrScreenshot");
    const pick = body.indexOf("controller.pickQrScreenshot()");
    const retry = body.indexOf("withCapability(currentAccount");
    expect(pick).toBeGreaterThanOrEqual(0);
    expect(retry).toBeGreaterThan(pick);
  });

  // The region is the deliberate exception: its capability is checked again
  // after the drag, so retrying means putting the overlay back up and asking
  // for the same box again. It says what happened instead of blaming the pixels.
  it("tells the user the vault moved rather than retrying the region drag", () => {
    const body = functionSource("selectQrRegion");
    expect(body).not.toContain("withCapability(");
    expect(body).toContain("isStaleCapabilityError(error)");
    expect(body).toContain("SteamGuard_QR_VaultChanged");
  });
});
