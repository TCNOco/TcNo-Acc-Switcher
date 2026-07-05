import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  openConfirm,
  openPrompt,
  pushToast,
  createAccountShortcut,
} = vi.hoisted(() => ({
  openConfirm: vi.fn(),
  openPrompt: vi.fn(),
  pushToast: vi.fn(),
  createAccountShortcut: vi.fn(),
}));

vi.mock("../../stores/modal", () => ({
  openConfirm,
  openPrompt,
}));

vi.mock("../../stores/toast", () => ({
  pushToast,
}));

vi.mock("../formatWailsError", () => ({
  formatToastWithError: (message: string, error: unknown) => `${message}: ${String(error)}`,
}));

vi.mock("../accountTagsContext", () => ({
  buildTagsSectionMenuItem: vi.fn(() => ({ label: "Tags" })),
}));

vi.mock("wails-shortcuts-service", () => ({
  CreateAccountShortcut: createAccountShortcut,
}));

import { buildSharedItems } from "./contextMenuBuilder";
import type { ContextMenuContext } from "./contextMenuBuilder";

type FakeAccount = {
  name: string;
  broken: boolean;
};

function createContext(broken: boolean) {
  const adapter = {
    swapTo: vi.fn(),
    saveOrder: vi.fn(),
    addNew: vi.fn(),
    forget: vi.fn(),
    rename: vi.fn(),
    changeImage: vi.fn(),
    clearManualImage: vi.fn(),
    getNote: vi.fn(),
    setNote: vi.fn(),
    launch: vi.fn(),
    name: (account: FakeAccount) => account.name,
    imageUrl: () => "",
    manualProfileImage: () => false,
    tags: () => [],
    accountLogin: () => "login-name",
    savedDataBroken: (account: FakeAccount) => account.broken,
  };
  return {
    name: "Steam",
    adapter,
    isActionBusy: false,
    hasGameStatsSupport: false,
    tr: (key: string) => key,
    tagDefs: [],
    openImagePick: vi.fn(),
    swapToLogin: vi.fn(() => Promise.resolve()),
    loadAccounts: vi.fn(() => Promise.resolve()),
    scheduleAccountsRefresh: vi.fn(),
    loadTagDefs: vi.fn(() => Promise.resolve()),
    openGameStatsModal: vi.fn(),
    onSelectedIdChanged: vi.fn(),
    account: { name: "Primary", broken },
  } satisfies ContextMenuContext & { account: FakeAccount };
}

describe("buildSharedItems", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("disables swap-to for broken accounts", async () => {
    const ctx = createContext(true);
    const items = buildSharedItems(ctx.account, "acc-1", ctx);

    expect(items.swapTo.label).toBe("Security_AccountDataBroken");
    expect(items.swapTo.disabled).toBe(true);

    await items.swapTo.action!();

    expect(ctx.onSelectedIdChanged).not.toHaveBeenCalled();
    expect(ctx.swapToLogin).not.toHaveBeenCalled();
  });

  it("selects and swaps when the account is healthy", async () => {
    const ctx = createContext(false);
    const items = buildSharedItems(ctx.account, "acc-2", ctx);

    expect(items.swapTo.label).toBe("Context_SwapTo");
    expect(items.swapTo.disabled).toBe(false);

    await items.swapTo.action!();

    expect(ctx.onSelectedIdChanged).toHaveBeenCalledWith("acc-2");
    expect(ctx.swapToLogin).toHaveBeenCalledTimes(1);
  });
});
