import type { ComponentType, SvelteComponent } from "svelte";
import { get, writable } from "svelte/store";

type ModalBase = { id: number };

export type ModalBodyOptions = {
  body?: string;
  bodyComponent?: ComponentType<SvelteComponent>;
  bodyProps?: Record<string, unknown>;
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
    } & ModalBodyOptions)
  | (ModalBase & {
      kind: "folder";
      title: string;
      initialPath?: string;
      positiveLabel?: string;
      negativeLabel?: string;
      dirsOnly?: boolean;
      soughtFilename?: string;
    } & ModalBodyOptions);

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

export function openFolderPicker(
  opts: {
    title: string;
    initialPath?: string;
    positiveLabel?: string;
    negativeLabel?: string;
    dirsOnly?: boolean;
    soughtFilename?: string;
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
    });
  });
}

export function cancelActiveModal(): void {
  const m = get(activeModal);
  if (!m) return;
  if (m.kind === "alert" || m.kind === "alertNoButton") dismissModal();
  else if (m.kind === "confirm") dismissModal(false);
  else dismissModal(null);
}
