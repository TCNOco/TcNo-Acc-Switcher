import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import { describe, expect, it } from "vitest";

const script = readFileSync(new URL("../../../internal/steambrowser/battlenet_account_name.js", import.meta.url), "utf8");
const node = (text: string, visible = true) => ({ innerText: text, textContent: text, getClientRects: () => visible ? [{}] : [] });
function root(dedicated: unknown[] = [], headers: unknown[] = [], children: unknown[] = [], parts: unknown[] = []) {
  return { querySelectorAll: (selector: string) => {
    const nodes = [...dedicated, ...headers, ...children, ...parts];
    if (selector === "*") return nodes;
    if (selector.startsWith('[part="text"]')) return parts;
    return selector.startsWith("[data-testid") ? dedicated : headers;
  } };
}
function navigation(text: string) {
  const tooltip = root([], [], [], [node(text, false)]);
  const link = { tagName: "BLZ-NAV-LINK", shadowRoot: tooltip };
  const menu = root([], [], [link]);
  return root([], [], [{ tagName: "BLZ-NAV-BATTLENET", shadowRoot: menu }]);
}
function read(document: ReturnType<typeof root>, origin = "https://account.battle.net", pathname = "/security") {
  const window: { top?: unknown } = {};
  window.top = window;
  return runInNewContext(script, { window, document, location: { origin, pathname } }) as { name: string; email: string } | null;
}

describe("Battle.net Security account name capture", () => {
  it("reads the rendered account tooltip while the menu is closed", () => {
    expect(read(navigation("Example\n#1234\ntest@example.invalid"))?.name).toBe("Example#1234");
  });
  it("reads the regional Battle.net navigation component without opening its hover menu", () => {
    expect(read(navigation("Example\n#1234\ntest@example.invalid"), "https://eu.account.battle.net")?.name).toBe("Example#1234");
  });
  it("reads the full email from a tooltip without a layout box", () => {
    const identity = read(navigation("Example\n#1234\ntest@example.invalid"), "https://eu.account.battle.net");
    expect(identity?.name).toBe("Example#1234");
    expect(identity?.email).toBe("test@example.invalid");
  });
  it("does not read account-like text in unrelated component internals", () => {
    const shadowRoot = root([], [], [], [node("Example#1234 test@example.invalid")]);
    expect(read(root([], [], [{ tagName: "UNRELATED-COMPONENT", shadowRoot }]))?.name).toBe("");
  });
  it.each(["eu", "us", "kr", "tw", "cn"])("accepts the %s account Security origin", region => {
    expect(read(root([], [node("Example#1234")]), `https://${region}.account.battle.net`)?.name).toBe("Example#1234");
  });
  it("prefers the BattleTag over the email in the account header", () => {
    expect(read(root([], [node("Example#1234 test@example.invalid Manage account")]))?.name).toBe("Example#1234");
  });
  it("reads the account menu inside a web component shadow root", () => {
    const shadowRoot = root([node("Example#1234")]);
    expect(read(root([], [], [{ shadowRoot }]))?.name).toBe("Example#1234");
  });
  it("joins a BattleTag whose discriminator is rendered in another element", () => {
    expect(read(root([], [node("Example\n #1234\nManage account")]))?.name).toBe("Example#1234");
  });
  it("accepts an email or a dedicated username when no BattleTag is displayed", () => {
    expect(read(root([], [node("test@example.invalid Manage account")]))?.name).toBe("test@example.invalid");
    expect(read(root([node("Example")]))?.name).toBe("Example");
  });
  it("ignores login pages, unrelated hosts, hidden elements, and body text", () => {
    const header = root([], [node("Example#1234")]);
    expect(read(header, "https://account.battle.net.evil.test")).toBeNull();
    expect(read(header, "https://eu.account.battle.net.evil.test")).toBeNull();
    expect(read(header, "https://unknown.account.battle.net")).toBeNull();
    expect(read(header, "http://eu.account.battle.net")).toBeNull();
    expect(read(header, "https://account.battle.net", "/login/en/")).toBeNull();
    expect(read(root([], [node("Example#1234", false)], [node("Body example: Example#9999")] ))?.name).toBe("");
  });
  it("waits instead of picking one of several visible account identities", () => {
    expect(read(root([], [node("First#1234 Second#5678")]))?.name).toBe("");
    expect(read(root([], [node("first@example.invalid second@example.invalid")]))?.name).toBe("");
    expect(read(root([node("BattleTag")]))?.name).toBe("");
  });
});
