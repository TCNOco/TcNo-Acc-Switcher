import { get, writable } from "svelte/store";

type SearchOverlayCtrl = {
  open: boolean;
  /** Seed when overlay opens (e.g. first typed character). */
  initialQuery: string;
  nonce: number;
};

const initial: SearchOverlayCtrl = { open: false, initialQuery: "", nonce: 0 };

export const searchOverlayCtrl = writable<SearchOverlayCtrl>(initial);

export function openSearchOverlay(initialQuery = ""): void {
  searchOverlayCtrl.update((c) => ({
    open: true,
    initialQuery,
    nonce: c.nonce + 1,
  }));
}

export function closeSearchOverlay(): void {
  const c = get(searchOverlayCtrl);
  searchOverlayCtrl.set({ ...c, open: false });
}

/** When overlay is open but focus is not in the search field, App routes keys here. */
export const searchOverlayPendingAppend = writable<string | null>(null);
