import type { ComponentType, SvelteComponent } from "svelte";
import { get, writable } from "svelte/store";
import type {
  SteamGuardAccountRef,
  SteamGuardAccountSummary,
  SteamGuardModalController,
  SteamGuardModalEntry,
} from "../lib/steamGuardModal";

type ModalBase = { id: number };

export type ModalBodyOptions = {
  body?: string;
  bodyComponent?: ComponentType<SvelteComponent>;
  bodyProps?: Record<string, unknown>;
};

export type PasswordSetupResult = { password: string };
export type PromptWithCheckboxResult = { value: string; checked: boolean };
export type TagExpiryResult = {
  scope: "account" | "all";
  expiresAt: string;
};

type ActiveModal =
  | (ModalBase & { kind: "alert"; title: string; dismissLabel?: string } & ModalBodyOptions)
  | (ModalBase & { kind: "alertNoButton"; title: string } & ModalBodyOptions)
  | (ModalBase & {
      kind: "confirm";
      title: string;
      style: "yesno" | "okcancel";
      positiveLabel?: string;
      negativeLabel?: string;
    } & ModalBodyOptions)
  | (ModalBase & {
      kind: "prompt";
      title: string;
      inputType: "text" | "password";
      /** When true (text only), uses a textarea; Enter inserts a newline instead of submitting. */
      multiline?: boolean;
      initialValue?: string;
      positiveLabel?: string;
      negativeLabel?: string;
      checkboxLabel?: string;
      checkboxInitial?: boolean;
      returnCheckboxResult?: boolean;
    } & ModalBodyOptions)
  | (ModalBase & {
      kind: "passwordSetup";
      title: string;
      positiveLabel?: string;
      negativeLabel?: string;
    })
  | (ModalBase & {
      kind: "tagExpiry";
      title: string;
      tagName: string;
      initialScope?: "account" | "all";
      initialDate?: string;
      initialTime?: string;
      positiveLabel?: string;
      negativeLabel?: string;
    })
  | (ModalBase & {
      kind: "folder";
      title: string;
      initialPath?: string;
      positiveLabel?: string;
      negativeLabel?: string;
      dirsOnly?: boolean;
      soughtFilename?: string;
      suggestedFolder?: string;
      showPortableButton?: boolean;
    } & ModalBodyOptions)
  | (ModalBase & {
      kind: "update";
      message: string;
      downloadUrl: string;
    })
  | (ModalBase & {
      kind: "feedback";
      mode: "issue" | "suggestion";
      platform?: string;
    })
  | (ModalBase & {
      kind: "crashReport";
    })
  | (ModalBase & {
      kind: "steamGuard";
      title: string;
      account: SteamGuardAccountRef;
      entry: SteamGuardModalEntry;
      knownAccounts: SteamGuardAccountSummary[];
      controller: SteamGuardModalController;
    });

export type CrashReportChoice = "no" | "yes" | "always";

let resolver: ((value: unknown) => void) | null = null;
let nextModalId = 0;

function bumpId(): number {
  nextModalId += 1;
  return nextModalId;
}

export const activeModal = writable<ActiveModal | null>(null);

export function dismissModal(result?: unknown): void {
  const r = resolver;
  resolver = null;
  activeModal.set(null);
  r?.(result);
}

export function openAlert(
  opts: {
    title: string;
    dismissLabel?: string;
  } & ModalBodyOptions,
): Promise<void> {
  return new Promise((resolve) => {
    resolver = () => resolve();
    activeModal.set({ id: bumpId(), kind: "alert", ...opts });
  });
}

export function openAlertNoButton(
  opts: {
    title: string;
  } & ModalBodyOptions,
): Promise<void> {
  return new Promise((resolve) => {
    resolver = () => resolve();
    activeModal.set({ id: bumpId(), kind: "alertNoButton", ...opts });
  });
}

export function openConfirm(
  opts: {
    title: string;
    style?: "yesno" | "okcancel";
    positiveLabel?: string;
    negativeLabel?: string;
  } & ModalBodyOptions,
): Promise<boolean> {
  return new Promise((resolve) => {
    resolver = resolve as (value: unknown) => void;
    activeModal.set({
      id: bumpId(),
      kind: "confirm",
      title: opts.title,
      style: opts.style ?? "yesno",
      positiveLabel: opts.positiveLabel,
      negativeLabel: opts.negativeLabel,
      body: opts.body,
      bodyComponent: opts.bodyComponent,
      bodyProps: opts.bodyProps,
    });
  });
}

export function openPrompt(
  opts: {
    title: string;
    inputType?: "text" | "password";
    multiline?: boolean;
    initialValue?: string;
    positiveLabel?: string;
    negativeLabel?: string;
  } & ModalBodyOptions,
): Promise<string | null> {
  return new Promise((resolve) => {
    resolver = resolve as (value: unknown) => void;
    activeModal.set({
      id: bumpId(),
      kind: "prompt",
      title: opts.title,
      body: opts.body,
      bodyComponent: opts.bodyComponent,
      bodyProps: opts.bodyProps,
      inputType: opts.inputType ?? "text",
      multiline: opts.multiline,
      initialValue: opts.initialValue,
      positiveLabel: opts.positiveLabel,
      negativeLabel: opts.negativeLabel,
    });
  });
}

export function openPromptWithCheckbox(
  opts: {
    title: string;
    inputType?: "text" | "password";
    initialValue?: string;
    positiveLabel?: string;
    negativeLabel?: string;
    checkboxLabel: string;
    checkboxInitial?: boolean;
  } & ModalBodyOptions,
): Promise<PromptWithCheckboxResult | null> {
  return new Promise((resolve) => {
    resolver = resolve as (value: unknown) => void;
    activeModal.set({
      id: bumpId(),
      kind: "prompt",
      title: opts.title,
      body: opts.body,
      bodyComponent: opts.bodyComponent,
      bodyProps: opts.bodyProps,
      inputType: opts.inputType ?? "text",
      initialValue: opts.initialValue,
      positiveLabel: opts.positiveLabel,
      negativeLabel: opts.negativeLabel,
      checkboxLabel: opts.checkboxLabel,
      checkboxInitial: opts.checkboxInitial ?? false,
      returnCheckboxResult: true,
    });
  });
}

export function openPasswordSetupModal(opts: {
  title: string;
  positiveLabel?: string;
  negativeLabel?: string;
}): Promise<PasswordSetupResult | null> {
  return new Promise((resolve) => {
    resolver = resolve as (value: unknown) => void;
    activeModal.set({
      id: bumpId(),
      kind: "passwordSetup",
      title: opts.title,
      positiveLabel: opts.positiveLabel,
      negativeLabel: opts.negativeLabel,
    });
  });
}

export function openTagExpiryModal(opts: {
  title: string;
  tagName: string;
  initialScope?: "account" | "all";
  initialDate?: string;
  initialTime?: string;
  positiveLabel?: string;
  negativeLabel?: string;
}): Promise<TagExpiryResult | null> {
  return new Promise((resolve) => {
    resolver = resolve as (value: unknown) => void;
    activeModal.set({
      id: bumpId(),
      kind: "tagExpiry",
      title: opts.title,
      tagName: opts.tagName,
      initialScope: opts.initialScope,
      initialDate: opts.initialDate,
      initialTime: opts.initialTime,
      positiveLabel: opts.positiveLabel,
      negativeLabel: opts.negativeLabel,
    });
  });
}

/** Returned from [openFolderPicker] when the user chooses portable mode. */
export const FOLDER_PICKER_PORTABLE = "__portable__";
/** Returned from [openFolderPicker] when the user chooses the default AppData location. */
export const FOLDER_PICKER_APPDATA = "__appdata__";

export function openFolderPicker(
  opts: {
    title: string;
    initialPath?: string;
    positiveLabel?: string;
    negativeLabel?: string;
    dirsOnly?: boolean;
    soughtFilename?: string;
    /** Folder-name pattern to highlight, e.g. `TcNo-Acc-Switcher-SteamGuard*`. */
    suggestedFolder?: string;
    showPortableButton?: boolean;
  } & ModalBodyOptions,
): Promise<string | null> {
  return new Promise((resolve) => {
    resolver = resolve as (value: unknown) => void;
    activeModal.set({
      id: bumpId(),
      kind: "folder",
      title: opts.title,
      body: opts.body,
      bodyComponent: opts.bodyComponent,
      bodyProps: opts.bodyProps,
      initialPath: opts.initialPath,
      positiveLabel: opts.positiveLabel,
      negativeLabel: opts.negativeLabel,
      dirsOnly: opts.dirsOnly ?? true,
      soughtFilename: opts.soughtFilename,
      suggestedFolder: opts.suggestedFolder,
      showPortableButton: opts.showPortableButton,
    });
  });
}

export function openUpdateDialog(opts: { message: string; downloadUrl: string }): Promise<void> {
  return new Promise((resolve) => {
    resolver = () => resolve();
    activeModal.set({
      id: bumpId(),
      kind: "update",
      message: opts.message,
      downloadUrl: opts.downloadUrl,
    });
  });
}

export function openFeedbackModal(opts: {
  mode: "issue" | "suggestion";
  platform?: string;
}): Promise<string | null> {
  return new Promise((resolve) => {
    resolver = resolve as (value: unknown) => void;
    activeModal.set({
      id: bumpId(),
      kind: "feedback",
      mode: opts.mode,
      platform: opts.platform,
    });
  });
}

export function openCrashReportPrompt(): Promise<CrashReportChoice> {
  return new Promise((resolve) => {
    resolver = resolve as (value: unknown) => void;
    activeModal.set({
      id: bumpId(),
      kind: "crashReport",
    });
  });
}

export type OpenSteamGuardModalOptions = {
  account: SteamGuardAccountRef;
  controller: SteamGuardModalController;
  entry?: SteamGuardModalEntry;
  knownAccounts?: SteamGuardAccountSummary[];
  title?: string;
};

export function openSteamGuardModal(opts: OpenSteamGuardModalOptions): Promise<void> {
  return new Promise((resolve) => {
    resolver = () => resolve();
    activeModal.set({
      id: bumpId(),
      kind: "steamGuard",
      title: opts.title ?? "Steam Guard",
      account: opts.account,
      entry: opts.entry ?? "account",
      knownAccounts: opts.knownAccounts ?? [],
      controller: opts.controller,
    });
  });
}

export function openSteamGuardForAccount(
  account: SteamGuardAccountRef,
  controller: SteamGuardModalController,
  knownAccounts: SteamGuardAccountSummary[] = [],
): Promise<void> {
  return openSteamGuardModal({ account, controller, knownAccounts, entry: "account" });
}

export function openSteamGuardImport(
  account: SteamGuardAccountRef,
  controller: SteamGuardModalController,
): Promise<void> {
  return openSteamGuardModal({ account, controller, entry: "import" });
}

export function openSteamGuardEnrollment(
  account: SteamGuardAccountRef,
  controller: SteamGuardModalController,
): Promise<void> {
  return openSteamGuardModal({ account, controller, entry: "enrollment" });
}

export function openSteamGuardQrLogin(
  account: SteamGuardAccountRef,
  controller: SteamGuardModalController,
): Promise<void> {
  return openSteamGuardModal({ account, controller, entry: "qr" });
}

export function cancelActiveModal(): void {
  const m = get(activeModal);
  if (!m) return;
  if (m.kind === "alert" || m.kind === "alertNoButton" || m.kind === "update") dismissModal();
  else if (m.kind === "confirm") dismissModal(false);
  else if (m.kind === "crashReport") dismissModal("no" satisfies CrashReportChoice);
  else dismissModal(null);
}
