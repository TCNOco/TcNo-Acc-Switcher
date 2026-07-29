import { get, writable, type Readable } from "svelte/store";

export type SteamGuardBackupStatus = {
  verifiedAt: string;
  path: string;
};

export type SteamGuardSettingsStatus = {
  vaultConfigured: boolean;
  unlocked: boolean;
  rememberPasswordForSession: boolean;
  folderPath: string;
  lastVerifiedBackup: SteamGuardBackupStatus | null;
};

/** One enrolled way into the vault. Requires lists the factors needed together. */
export type SteamGuardVaultFactor = {
  id: string;
  label: string;
  /** The thing the user holds, which is what the row is named after. */
  kind: string;
  requires: string[];
  /** Whether this way in needs the password alongside what it is named after. */
  requiresPassword: boolean;
  removable: boolean;
  /** Why it cannot be removed: "last", "lastInteractive" or "backupNeeded". */
  blocks: string;
  /** Removing this leaves a vault with no password at all. */
  lastPasswordWayIn: boolean;
};

export type SteamGuardFactorStatus = {
  factors: SteamGuardVaultFactor[];
  hasBackupKey: boolean;
  passwordOnly: boolean;
  /** Whether some enrolled way in needs nothing but the password. When false, a
   *  management action has to collect the other factors before it can proceed. */
  passwordOpens: boolean;
  hasKeyfile: boolean;
  keyfileCount: number;
  securityKeyCount: number;
  canRemoveAFactor: boolean;
};

export interface SteamGuardSettingsAdapter {
  getSettingsStatus(): Promise<SteamGuardSettingsStatus>;
  setRememberPasswordForSession(enabled: boolean): Promise<void>;
  changePassword(
    currentPassword: string,
    newPassword: string,
    keyfilePath: string,
    backupKey: string,
  ): Promise<void>;
  lockNow(): Promise<void>;
  openFolder(): Promise<void>;
  createVerifiedBackup(): Promise<void>;
  restoreFromBackup(): Promise<void>;
  listVaultFactors(): Promise<SteamGuardFactorStatus>;
  unlockForManagement(password: string, keyfilePath: string, backupKey: string): Promise<void>;
  pickKeyfile(): Promise<string>;
  createBackupKey(password: string): Promise<string>;
  saveBackupKey(code: string): Promise<string>;
  enrollKeyfile(password: string, keyfilePassword: string): Promise<string>;
  enrollSecurityKey(password: string, name: string, keyPassword: string): Promise<void>;
  enrollPassword(password: string, newPassword: string): Promise<void>;
  removeVaultFactor(password: string, factorId: string): Promise<void>;
  renameVaultFactor(password: string, factorId: string, name: string): Promise<void>;
  securityKeyAvailable(): Promise<{ available: boolean; reason: string }>;
}

export type SteamGuardSettingsOperation =
  | "remember"
  | "change-password"
  | "lock"
  | "open-folder"
  | "backup"
  | "restore"
  | "factors";

export type SteamGuardSettingsState = {
  availability: "loading" | "ready" | "unavailable" | "error";
  operation: SteamGuardSettingsOperation | null;
  status: SteamGuardSettingsStatus;
  error: unknown;
};

export interface SteamGuardSettingsStore extends Readable<SteamGuardSettingsState> {
  setAdapter(adapter: SteamGuardSettingsAdapter | null): void;
  refresh(): Promise<void>;
  setRememberPasswordForSession(enabled: boolean): Promise<void>;
  changePassword(
    currentPassword: string,
    newPassword: string,
    keyfilePath?: string,
    backupKey?: string,
  ): Promise<void>;
  lockNow(): Promise<void>;
  openFolder(): Promise<void>;
  createVerifiedBackup(): Promise<void>;
  restoreFromBackup(): Promise<void>;
  listVaultFactors(): Promise<SteamGuardFactorStatus>;
  /** Opens the vault with every factor the user can present, so the factor
   *  actions that follow work from an open vault. The unlock lease lapses long
   *  before the settings screen is reached, and a password alone cannot reopen a
   *  vault whose only way in needs a password and something else. */
  unlockForManagement(password: string, keyfilePath: string, backupKey: string): Promise<void>;
  /** Empty string when the user cancels the picker. */
  pickKeyfile(): Promise<string>;
  createBackupKey(password: string): Promise<string>;
  /** Writes the key to a file the user picks. Not guarded as an operation: it
   *  runs while the key dialog is open, which is itself an operation. */
  saveBackupKey(code: string): Promise<string>;
  enrollKeyfile(password: string, keyfilePassword: string): Promise<string>;
  enrollSecurityKey(password: string, name: string, keyPassword: string): Promise<void>;
  /** Adds a password that opens the vault on its own, for a vault that has none. */
  enrollPassword(password: string, newPassword: string): Promise<void>;
  removeVaultFactor(password: string, factorId: string): Promise<void>;
  renameVaultFactor(password: string, factorId: string, name: string): Promise<void>;
  securityKeyAvailable(): Promise<{ available: boolean; reason: string }>;
}

const emptyStatus = (): SteamGuardSettingsStatus => ({
  vaultConfigured: false,
  unlocked: false,
  rememberPasswordForSession: false,
  folderPath: "",
  lastVerifiedBackup: null,
});

const initialState = (): SteamGuardSettingsState => ({
  availability: "unavailable",
  operation: null,
  status: emptyStatus(),
  error: null,
});

export function createSteamGuardSettingsStore(
  initialAdapter: SteamGuardSettingsAdapter | null = null,
): SteamGuardSettingsStore {
  let adapter = initialAdapter;
  const state = writable<SteamGuardSettingsState>(initialState());

  function requireAdapter(): SteamGuardSettingsAdapter {
    if (!adapter) {
      throw new Error("Steam Guard settings service is unavailable");
    }
    return adapter;
  }

  async function refresh(): Promise<void> {
    if (!adapter) {
      state.set(initialState());
      return;
    }
    const previous = get(state);
    state.set({ ...previous, availability: "loading", error: null });
    try {
      const status = await adapter.getSettingsStatus();
      state.set({ availability: "ready", operation: null, status, error: null });
    } catch (error) {
      state.set({ ...previous, availability: "error", operation: null, error });
      throw error;
    }
  }

  async function run(
    operation: SteamGuardSettingsOperation,
    action: (service: SteamGuardSettingsAdapter) => Promise<void>,
    refreshAfter = true,
  ): Promise<void> {
    const service = requireAdapter();
    const current = get(state);
    if (current.operation) {
      throw new Error("A Steam Guard settings operation is already in progress");
    }
    state.set({ ...current, operation, error: null });
    try {
      await action(service);
      if (refreshAfter) {
        const status = await service.getSettingsStatus();
        state.set({ availability: "ready", operation: null, status, error: null });
      } else {
        state.update((value) => ({ ...value, operation: null }));
      }
    } catch (error) {
      state.update((value) => ({ ...value, operation: null, error }));
      throw error;
    }
  }

  // Same single-operation guard as run, for actions that return something. A
  // backup key is returned once and never again, so it cannot be re-fetched by
  // refreshing afterwards.
  async function runValue<T>(
    operation: SteamGuardSettingsOperation,
    action: (service: SteamGuardSettingsAdapter) => Promise<T>,
    refreshAfter = true,
  ): Promise<T> {
    const service = requireAdapter();
    const current = get(state);
    if (current.operation) {
      throw new Error("A Steam Guard settings operation is already in progress");
    }
    state.set({ ...current, operation, error: null });
    try {
      const result = await action(service);
      if (refreshAfter) {
        const status = await service.getSettingsStatus();
        state.set({ availability: "ready", operation: null, status, error: null });
      } else {
        state.update((value) => ({ ...value, operation: null }));
      }
      return result;
    } catch (error) {
      state.update((value) => ({ ...value, operation: null, error }));
      throw error;
    }
  }

  return {
    subscribe: state.subscribe,
    setAdapter(nextAdapter) {
      adapter = nextAdapter;
      state.set(initialState());
    },
    refresh,
    setRememberPasswordForSession(enabled) {
      return run("remember", (service) => service.setRememberPasswordForSession(enabled));
    },
    changePassword(currentPassword, newPassword, keyfilePath = "", backupKey = "") {
      return run("change-password", (service) =>
        service.changePassword(currentPassword, newPassword, keyfilePath, backupKey));
    },
    lockNow() {
      return run("lock", (service) => service.lockNow());
    },
    openFolder() {
      return run("open-folder", (service) => service.openFolder(), false);
    },
    createVerifiedBackup() {
      return run("backup", (service) => service.createVerifiedBackup());
    },
    restoreFromBackup() {
      return run("restore", (service) => service.restoreFromBackup());
    },
    listVaultFactors() {
      return runValue("factors", (service) => service.listVaultFactors(), false);
    },
    unlockForManagement(password, keyfilePath, backupKey) {
      return run("factors", (service) =>
        service.unlockForManagement(password, keyfilePath, backupKey));
    },
    pickKeyfile() {
      return requireAdapter().pickKeyfile();
    },
    createBackupKey(password) {
      return runValue("factors", (service) => service.createBackupKey(password), false);
    },
    saveBackupKey(code) {
      return requireAdapter().saveBackupKey(code);
    },
    enrollKeyfile(password, keyfilePassword) {
      return runValue("factors", (service) => service.enrollKeyfile(password, keyfilePassword), false);
    },
    enrollPassword(password, newPassword) {
      return run("factors", (service) => service.enrollPassword(password, newPassword), false);
    },
    renameVaultFactor(password, factorId, name) {
      return run("factors", (service) => service.renameVaultFactor(password, factorId, name), false);
    },
    removeVaultFactor(password, factorId) {
      return run("factors", (service) => service.removeVaultFactor(password, factorId), false);
    },
    securityKeyAvailable() {
      return requireAdapter().securityKeyAvailable();
    },
    enrollSecurityKey(password, name, keyPassword) {
      return run("factors", (service) => service.enrollSecurityKey(password, name, keyPassword), false);
    },
  };
}

export const steamGuardSettings = createSteamGuardSettingsStore();

export function configureSteamGuardSettingsAdapter(adapter: SteamGuardSettingsAdapter | null): void {
  steamGuardSettings.setAdapter(adapter);
  if (adapter) {
    void steamGuardSettings.refresh().catch(() => undefined);
  }
}
