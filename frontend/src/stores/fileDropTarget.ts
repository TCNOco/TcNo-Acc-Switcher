import { writable, type Writable } from "svelte/store";

export type FileDropAcceptor = {
  labelKey: string;
  handle: (paths: string[]) => Promise<void>;
};

export const fileDropAcceptor: Writable<FileDropAcceptor | null> = writable(null);
