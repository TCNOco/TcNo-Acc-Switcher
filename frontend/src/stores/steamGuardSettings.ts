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

export interface SteamGuardSettingsAdapter {
  getSettingsStatus(): Promise<SteamGuardSettingsStatus>;
  setRememberPasswordForSession(enabled: boolean): Promise<void>;
  changePassword(currentPassword: string, newPassword: string): Promise<void>;
  lockNow(): Promise<void>;
  openFolder(): Promise<void>;
  createVerifiedBackup(): Promise<void>;
}

export type SteamGuardSettingsOperation =
  | "remember"
  | "change-password"
  | "lock"
  | "open-folder"
  | "backup";

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
  changePassword(currentPassword: string, newPassword: string): Promise<void>;
  lockNow(): Promise<void>;
  openFolder(): Promise<void>;
  createVerifiedBackup(): Promise<void>;
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
    changePassword(currentPassword, newPassword) {
      return run("change-password", (service) => service.changePassword(currentPassword, newPassword));
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
  };
}

export const steamGuardSettings = createSteamGuardSettingsStore();

export function configureSteamGuardSettingsAdapter(adapter: SteamGuardSettingsAdapter | null): void {
  steamGuardSettings.setAdapter(adapter);
  if (adapter) {
    void steamGuardSettings.refresh().catch(() => undefined);
  }
}
