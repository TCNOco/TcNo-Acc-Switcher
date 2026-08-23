import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const source = readFileSync(new URL("./SteamGuardModalBody.svelte", import.meta.url), "utf8");

/** One screen's markup, bounded by the next screen's branch. */
function screenSource(screen: string): string {
  const start = source.indexOf(`state.screen === "${screen}"}`);
  expect(start).toBeGreaterThanOrEqual(0);
  const end = source.indexOf("{:else if state.screen ===", start + 1);
  expect(end).toBeGreaterThan(start);
  return source.slice(start, end);
}

const BROWSE_ROW = 'class="steam-guard__browse"';

/** openBrowser's body, bounded by the next function declared beside it. */
function openBrowserSource(): string {
  const start = source.indexOf("async function openBrowser(");
  expect(start).toBeGreaterThanOrEqual(0);
  const end = source.indexOf("\n  async function ", start + 1);
  expect(end).toBeGreaterThan(start);
  return source.slice(start, end);
}

describe("Steam Guard browse buttons", () => {
  // They ride on the account's stored Steam session, so they belong only on the
  // screens of a record that holds one.
  it("are offered on the two screens whose account has a session", () => {
    expect(screenSource("account-code")).toContain(BROWSE_ROW);
    expect(screenSource("login-only")).toContain(BROWSE_ROW);
  });

  // The setup page is for an account the vault does not hold at all: no record,
  // no session, and openBrowser has no account to act on. They were offered here
  // and did nothing when pressed.
  it("are absent from the page for an account with no vault record", () => {
    expect(screenSource("setup")).not.toContain(BROWSE_ROW);
  });

  // A window opened on a lapsed session lands on a signed-out page, which is
  // worse than not offering it: the account screen already points at Login Again.
  it("appear only once the session has been found to work", () => {
    const rows = source.split(BROWSE_ROW).length - 1;
    const gated = source.split("controller.openSteamBrowser && canBrowse").length - 1;
    expect(rows).toBeGreaterThan(0);
    expect(gated).toBe(rows);
  });

  // "Undecided" must not read as "fine". The check leaves the verdict unknown
  // whenever it could not reach an answer, and that has to hide the buttons.
  it("treats an undecided session as unusable", () => {
    expect(source).toContain('$: canBrowse = sessionVerdict === "valid"');
  });

  // A page opening in its own window has no account state to protect, so it must
  // not take the modal-wide lock: that one disables the close button, swallows
  // Escape and the backdrop, and leaves no way out until the window is up.
  it("does not take the modal-wide lock while a window opens", () => {
    const body = openBrowserSource();
    expect(body).not.toMatch(/\bbusy\s*=[^=]/);
    expect(body).toContain("openingBrowserSite = site;");
  });

  // Every browse button answers to the in-flight flag, so one added later cannot
  // silently let a second window through while the first is still opening.
  it("gates every browse button on the in-flight flag", () => {
    const clicks = source.split('on:click={() => openBrowser("').length - 1;
    const gated = source.split("disabled={busy || openingBrowserSite !== null}").length - 1;
    expect(clicks).toBeGreaterThan(0);
    expect(gated).toBe(clicks);
  });

  // The modal stays usable during the open, so the user can be on a different
  // account by the time a lapsed session reports back. Sending that one to the
  // sign-in screen would renew the wrong record.
  it("only sends the account it started from to the sign-in screen", () => {
    expect(openBrowserSource()).toContain("steamGuardAccountForState(state)?.id === account.id");
  });
});
