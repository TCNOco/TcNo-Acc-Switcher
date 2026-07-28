import { get } from "svelte/store";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  createSteamGuardSettingsStore,
  type SteamGuardSettingsAdapter,
  type SteamGuardSettingsStatus,
} from "./steamGuardSettings";

const configuredStatus = (): SteamGuardSettingsStatus => ({
  vaultConfigured: true,
  unlocked: true,
  rememberPasswordForSession: false,
  folderPath: "C:\\Users\\tester\\AppData\\Roaming\\TcNo Account Switcher\\SteamGuard",
  lastVerifiedBackup: null,
});

function adapter(status = configuredStatus()): SteamGuardSettingsAdapter {
  return {
    getSettingsStatus: vi.fn().mockResolvedValue(status),
    setRememberPasswordForSession: vi.fn().mockResolvedValue(undefined),
    changePassword: vi.fn().mockResolvedValue(undefined),
    lockNow: vi.fn().mockResolvedValue(undefined),
    openFolder: vi.fn().mockResolvedValue(undefined),
    createVerifiedBackup: vi.fn().mockResolvedValue(undefined),
  };
}

describe("Steam Guard settings store", () => {
  let service: SteamGuardSettingsAdapter;

  beforeEach(() => {
    service = adapter();
  });

  it("starts unavailable with session password retention off", () => {
    const store = createSteamGuardSettingsStore();

    expect(get(store)).toMatchObject({
      availability: "unavailable",
      operation: null,
      status: {
        vaultConfigured: false,
        rememberPasswordForSession: false,
        folderPath: "",
      },
    });
  });

  it("loads authoritative status from the adapter", async () => {
    const store = createSteamGuardSettingsStore(service);

    await store.refresh();

    expect(service.getSettingsStatus).toHaveBeenCalledOnce();
    expect(get(store)).toMatchObject({
      availability: "ready",
      status: configuredStatus(),
    });
  });

  it("refreshes status after changing the session retention setting", async () => {
    const remembered = { ...configuredStatus(), rememberPasswordForSession: true };
    vi.mocked(service.getSettingsStatus)
      .mockResolvedValueOnce(configuredStatus())
      .mockResolvedValueOnce(remembered);
    const store = createSteamGuardSettingsStore(service);
    await store.refresh();

    await store.setRememberPasswordForSession(true);

    expect(service.setRememberPasswordForSession).toHaveBeenCalledWith(true);
    expect(get(store).status.rememberPasswordForSession).toBe(true);
  });

  it("does not report a failed backup as verified", async () => {
    const error = new Error("backup verification failed");
    vi.mocked(service.createVerifiedBackup).mockRejectedValue(error);
    const store = createSteamGuardSettingsStore(service);
    await store.refresh();

    await expect(store.createVerifiedBackup()).rejects.toBe(error);

    expect(get(store)).toMatchObject({
      operation: null,
      status: { lastVerifiedBackup: null },
      error,
    });
  });

  it("rejects mutations when no adapter has been installed", async () => {
    const store = createSteamGuardSettingsStore();

    await expect(store.lockNow()).rejects.toThrow("Steam Guard settings service is unavailable");
    expect(get(store).availability).toBe("unavailable");
  });

  it("rejects a second operation instead of reporting false success", async () => {
    let finishLock: (() => void) | undefined;
    vi.mocked(service.lockNow).mockImplementation(() => new Promise<void>((resolve) => {
      finishLock = resolve;
    }));
    const store = createSteamGuardSettingsStore(service);
    await store.refresh();

    const locking = store.lockNow();
    await expect(store.createVerifiedBackup()).rejects.toThrow("already in progress");
    expect(service.createVerifiedBackup).not.toHaveBeenCalled();

    finishLock?.();
    await locking;
  });
});
