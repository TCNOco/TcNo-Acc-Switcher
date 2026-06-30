import { writable, type Writable, derived } from "svelte/store";

export type FileDropTargetDetails = {
  elementId: string;
  classList: string[];
  x: number;
  y: number;
};

export type FileDropContext = {
  target?: FileDropTargetDetails;
};

export type FileDropAcceptor = {
  labelKey: string;
  handle: (paths: string[], context?: FileDropContext) => Promise<void>;
};

export type FileDropInterceptor = (paths: string[], context?: FileDropContext) => Promise<boolean>;

export type BgZoneDropInterceptor = (paths: string[], context?: FileDropContext) => Promise<boolean>;

export const actionBarStatus = writable("");

export const accountProfileImageDropActive = writable(false);

export const backgroundZoneInterceptor: Writable<BgZoneDropInterceptor | null> = writable(null);

export const fileDropInterceptor: Writable<FileDropInterceptor | null> = writable(null);

export const fileDropAcceptor: Writable<FileDropAcceptor | null> = writable(null);

export function resetDropInterceptors() {
  fileDropInterceptor.set(null);
  backgroundZoneInterceptor.set(null);
  fileDropAcceptor.set(null);
  accountProfileImageDropActive.set(false);
}

export const isFileDropHandled = derived(
  [fileDropInterceptor, backgroundZoneInterceptor, fileDropAcceptor],
  ([$interceptor, $bgInterceptor, $acceptor]) => {
    return $interceptor !== null || $bgInterceptor !== null || $acceptor !== null;
  },
);
