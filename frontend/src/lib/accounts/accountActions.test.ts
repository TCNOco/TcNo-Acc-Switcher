import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  pushToast,
  reportLaunchFailure,
  offerRestartIfNeedsAdmin,
  isNeedsAdminError,
  formatToastWithError,
  platformActionBusySet,
  actionBarStatusSet,
} = vi.hoisted(() => ({
  pushToast: vi.fn(),
  reportLaunchFailure: vi.fn(),
  offerRestartIfNeedsAdmin: vi.fn(),
  isNeedsAdminError: vi.fn(() => false),
  formatToastWithError: vi.fn((message: string, error: unknown) => `${message}: ${String(error)}`),
  platformActionBusySet: vi.fn(),
  actionBarStatusSet: vi.fn(),
}));

vi.mock("../../stores/toast", () => ({
  pushToast,
}));

vi.mock("../../stores/platformPage", () => ({
  platformActionBusy: {
    set: platformActionBusySet,
  },
}));

vi.mock("../../stores/fileDrop", () => ({
  actionBarStatus: {
    set: actionBarStatusSet,
  },
}));

vi.mock("../formatWailsError", () => ({
  formatToastWithError,
}));

vi.mock("../adminFlow", () => ({
  isNeedsAdminError,
  offerRestartIfNeedsAdmin,
  reportLaunchFailure,
}));

vi.mock("../../stores/i18n", () => ({
  t: {
    subscribe(run: (translator: (key: string) => string) => void) {
      run((key: string) => key);
      return () => {};
    },
  },
}));

import {
  launchPlatformForSelection,
  swapToLogin,
} from "./accountActions";

type FakeAccount = {
  id: string;
  current: boolean;
};

function createContext(overrides: Partial<Parameters<typeof swapToLogin>[0]> = {}) {
  const accounts = new Map<string, FakeAccount>([
    ["a", { id: "a", current: false }],
    ["b", { id: "b", current: true }],
  ]);
  const adapter = {
    swapTo: vi.fn(() => Promise.resolve()),
    launch: vi.fn(() => Promise.resolve()),
    addNew: vi.fn(() => Promise.resolve()),
    forget: vi.fn(() => Promise.resolve()),
    rename: vi.fn(() => Promise.resolve()),
    changeImage: vi.fn(() => Promise.resolve()),
    clearManualImage: vi.fn(() => Promise.resolve()),
    getNote: vi.fn(() => Promise.resolve("")),
    setNote: vi.fn(() => Promise.resolve()),
    saveOrder: vi.fn(() => Promise.resolve()),
    loadAccountsList: vi.fn(() => Promise.resolve([])),
    loadAccountsEnrichment: vi.fn(() => Promise.resolve([])),
    buildPatch: vi.fn(),
    applyPatch: vi.fn(),
    patchTargetId: vi.fn(),
    searchHay: vi.fn(),
    currentSession: vi.fn((account: FakeAccount) => account.current),
  };

  return {
    name: "Steam",
    adapter,
    selectedId: "a",
    isActionBusyValue: false,
    accountById: (id: string) => accounts.get(id),
    scheduleAccountsRefresh: vi.fn(),
    touchStatus: vi.fn(),
    setIsActionBusy: vi.fn(),
    ...overrides,
  };
}

describe("accountActions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("swaps to the selected account and schedules a refresh", async () => {
    const ctx = createContext();

    await swapToLogin(ctx);

    expect(ctx.adapter.swapTo).toHaveBeenCalledWith("a");
    expect(ctx.scheduleAccountsRefresh).toHaveBeenCalledTimes(1);
    expect(pushToast).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "success",
        message: "Toast_AccountSwitched",
      }),
    );
  });

  it("launches the selected account after switching when it is not the current session", async () => {
    const ctx = createContext();

    await launchPlatformForSelection(ctx);

    expect(ctx.adapter.swapTo).toHaveBeenCalledWith("a");
    expect(ctx.adapter.launch).toHaveBeenCalledTimes(1);
    expect(ctx.scheduleAccountsRefresh).toHaveBeenCalledTimes(1);
  });

  it("launches directly when the selected account is already active", async () => {
    const ctx = createContext({ selectedId: "b" });

    await launchPlatformForSelection(ctx);

    expect(ctx.adapter.swapTo).not.toHaveBeenCalled();
    expect(ctx.adapter.launch).toHaveBeenCalledTimes(1);
  });
});
