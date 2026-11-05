import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import { describe, expect, it } from "vitest";

const script = readFileSync(new URL("../../../internal/steambrowser/battlenet_login_identity.js", import.meta.url), "utf8");
function read(inputs: { type: string; value?: string }[] = [], text = "", origin = "https://account.battle.net", pathname = "/login/en/password") {
  const window: { top?: unknown } = {};
  window.top = window;
  const document = {
    querySelectorAll: (selector: string) => {
      expect(selector).toBe('input#accountName, input[name="accountName"], input[name="email"], input[type="email"]');
      return inputs;
    },
    querySelector: (selector: string) => {
      expect(selector).toBe("#login-wrapper, .login-wrapper");
      return { innerText: text };
    },
  };
  return runInNewContext(script, { window, document, location: { origin, pathname } }) as { email: string; name: string } | null;
}

describe("Battle.net login email capture", () => {
  it("reads only the account identifier and never a password value", () => {
    const password = { type: "password", get value(): string { throw new Error("password read"); } };
    expect(read([password, { type: "text", value: " test@example.invalid " }])?.email).toBe("test@example.invalid");
  });
  it("reads the email shown on the password screen", () => {
    expect(read([], "Log in\ntest@example.invalid\nPassword\nContinue")?.email).toBe("test@example.invalid");
  });
  it("refuses ambiguous, masked, partial and phone identifiers", () => {
    for (const value of ["test@example.invalid extra", "te***@example.invalid", "+123456789"]) {
      expect(read([{ type: "text", value }])?.email).toBe("");
    }
    expect(read([], "first@example.invalid second@example.invalid")?.email).toBe("");
  });
  it.each(["/login/en/", "/login/en/password", "/login/en-us/password", "/login/"])("accepts %s on a regional login host", path => {
    expect(read([{ type: "email", value: "test@example.invalid" }], "", "https://eu.account.battle.net", path)?.email).toBe("test@example.invalid");
  });
  it("ignores other hosts and account pages", () => {
    expect(read([], "", "https://account.battle.net.evil.test")).toBeNull();
    expect(read([], "", "https://account.battle.net", "/security")).toBeNull();
  });
});
