import { get, writable } from "svelte/store";

type ModalBase = { id: number };

export type ActiveModal =
  | (ModalBase & { kind: "alert"; title: string; body: string; dismissLabel?: string })
  | (ModalBase & {
      kind: "confirm";
      title: string;
      body: string;
      style: "yesno" | "okcancel";
      positiveLabel?: string;
      negativeLabel?: string;
    })
  | (ModalBase & {
      kind: "prompt";
      title: string;
      body?: string;
      inputType: "text" | "password";
      initialValue?: string;
      positiveLabel?: string;
      negativeLabel?: string;
    })
  | (ModalBase & {
      kind: "folder";
      title: string;
      body?: string;
      initialPath?: string;
      positiveLabel?: string;
      negativeLabel?: string;
      dirsOnly?: boolean;
      /** Shown under the path input; row turns red if the path lacks this substring (case-insensitive). */
      soughtFilename?: string;
    });

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

export function openAlert(opts: {
  title: string;
  body: string;
  dismissLabel?: string;
}): Promise<void> {
  return new Promise((resolve) => {
    resolver = () => resolve();
    activeModal.set({ id: bumpId(), kind: "alert", ...opts });
  });
}

export function openConfirm(opts: {
  title: string;
  body: string;
  style?: "yesno" | "okcancel";
  positiveLabel?: string;
  negativeLabel?: string;
}): Promise<boolean> {
  return new Promise((resolve) => {
    resolver = resolve as (value: unknown) => void;
    activeModal.set({
      id: bumpId(),
      kind: "confirm",
      title: opts.title,
      body: opts.body,
      style: opts.style ?? "yesno",
      positiveLabel: opts.positiveLabel,
      negativeLabel: opts.negativeLabel,
    });
  });
}

export function openPrompt(opts: {
  title: string;
  body?: string;
  inputType?: "text" | "password";
  initialValue?: string;
  positiveLabel?: string;
  negativeLabel?: string;
}): Promise<string | null> {
  return new Promise((resolve) => {
    resolver = resolve as (value: unknown) => void;
    activeModal.set({
      id: bumpId(),
      kind: "prompt",
      title: opts.title,
      body: opts.body,
      inputType: opts.inputType ?? "text",
      initialValue: opts.initialValue,
      positiveLabel: opts.positiveLabel,
      negativeLabel: opts.negativeLabel,
    });
  });
}

export function openFolderPicker(opts: {
  title: string;
  body?: string;
  initialPath?: string;
  positiveLabel?: string;
  negativeLabel?: string;
  dirsOnly?: boolean;
  soughtFilename?: string;
}): Promise<string | null> {
  return new Promise((resolve) => {
    resolver = resolve as (value: unknown) => void;
    activeModal.set({
      id: bumpId(),
      kind: "folder",
      title: opts.title,
      body: opts.body,
      initialPath: opts.initialPath,
      positiveLabel: opts.positiveLabel,
      negativeLabel: opts.negativeLabel,
      dirsOnly: opts.dirsOnly ?? true,
      soughtFilename: opts.soughtFilename,
    });
  });
}

/** Backdrop / Escape / secondary actions for the current modal. */
export function cancelActiveModal(): void {
  const m = get(activeModal);
  if (!m) return;
  if (m.kind === "alert") dismissModal();
  else if (m.kind === "confirm") dismissModal(false);
  else dismissModal(null);
}
