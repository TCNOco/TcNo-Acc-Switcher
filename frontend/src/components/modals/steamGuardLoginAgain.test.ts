import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const source = readFileSync(new URL("./SteamGuardModalBody.svelte", import.meta.url), "utf8");

/**
 * One function's body. The file is tab-indented, so the first line that closes a
 * brace at a single tab is the function's own close — bounding by that survives
 * edits inside the body, which a fixed character window does not.
 */
function functionSource(name: string): string {
  const start = source.indexOf(`function ${name}(`);
  expect(start).toBeGreaterThanOrEqual(0);
  const end = source.indexOf("\n\t}", start);
  expect(end).toBeGreaterThan(start);
  return source.slice(start, end);
}

describe("Steam Guard login again", () => {
  // Steam's access token lapses in about a day; the refresh token stored beside
  // it lasts months.
  it("spends the saved refresh token before asking for a password", () => {
    const body = functionSource("startLoginAgain");

    expect(body).toContain("controller.loginAgain");
    expect(body).toContain("steamLoginAgainNextStep(result)");

    const renewal = body.indexOf("controller.loginAgain");
    const credentials = body.indexOf('showCredentialForm("login_again"', renewal);
    expect(credentials).toBeGreaterThan(renewal);
  });

  // A refresh can mint a token for a session Steam then refuses, so success is
  // confirmed rather than assumed.
  it("confirms the renewed session with Steam before reporting success", () => {
    const body = functionSource("startLoginAgain");
    expect(body).toContain("renewedSessionWorks(currentAccount)");

    const probe = functionSource("renewedSessionWorks");
    expect(probe).toContain("controller.probeSteamSession");
    // An unavailable or failing probe must not manufacture a sign-in prompt.
    expect(probe).toContain("return true");
  });

  // Opening an account is the path most users arrive on; renewing only when they
  // press the button would still show every account as needing a sign-in.
  it("renews a lapsed session when an account is opened, before judging it", () => {
    const body = functionSource("checkSessionNeedsLogin");
    expect(body).toContain("controller.ensureFreshSession");

    const renewal = body.indexOf("controller.ensureFreshSession");
    const verdict = body.indexOf("needsLogin", renewal);
    expect(verdict).toBeGreaterThan(renewal);
  });

  // A renewal writes to the vault, which rotates its generation and invalidates
  // the capability in hand. Missing this makes every call after it fail.
  it("re-acquires the capability after a renewal has written to the vault", () => {
    const body = functionSource("checkSessionNeedsLogin");
    expect(body).toContain("capabilityRefreshRequired");
    expect(body).toContain("refreshCapabilityIfRequired(currentAccount, true)");
    expect(body).toContain("capability = capabilityFor(currentAccount)");
  });
});
