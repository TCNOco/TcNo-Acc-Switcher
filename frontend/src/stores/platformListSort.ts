import { writable } from "svelte/store";

export type PlatformSortKind =
  | "alpha_asc"
  | "alpha_desc"
  | "steam_user_asc"
  | "steam_user_desc"
  | "lastused_new_old"
  | "lastused_old_new"
  | "date_new_old"
  | "date_old_new";

export type PlatformListSortSignal = { id: number; kind: PlatformSortKind };

export const platformListSort = writable<PlatformListSortSignal | null>(null);

let sortSeq = 0;

export function triggerPlatformSort(kind: PlatformSortKind): void {
  sortSeq += 1;
  platformListSort.set({ id: sortSeq, kind });
}
