import { writable, type Writable } from "svelte/store";

/** If set, runs before shortcut file-drop handling; return true if the drop was consumed. */
export type FileDropInterceptor = (paths: string[]) => Promise<boolean>;

export const fileDropInterceptor: Writable<FileDropInterceptor | null> = writable(null);
