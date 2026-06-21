import { writable, type Writable, derived } from "svelte/store";

export type FileDropAcceptor = {
  labelKey: string;
  handle: (paths: string[]) => Promise<void>;
};

export type FileDropInterceptor = (paths: string[]) => Promise<boolean>;

export type BgZoneDropInterceptor = (paths: string[]) => Promise<boolean>;

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
