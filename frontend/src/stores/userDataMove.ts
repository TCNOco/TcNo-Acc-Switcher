import { writable } from "svelte/store";

export type UserDataMoveOverlayState = {
  active: boolean;
  phase: string;
  done: number;
  total: number;
};

export const userDataMoveOverlay = writable<UserDataMoveOverlayState>({
  active: false,
  phase: "",
  done: 0,
  total: 0,
});

export function showUserDataMoveOverlay(): void {
  userDataMoveOverlay.set({ active: true, phase: "copying", done: 0, total: 0 });
}

export function hideUserDataMoveOverlay(): void {
  userDataMoveOverlay.set({ active: false, phase: "", done: 0, total: 0 });
}

export function applyUserDataMoveProgress(payload: {
  phase?: string;
  done?: number;
  total?: number;
}): void {
  userDataMoveOverlay.update((s) => ({
    ...s,
    active: true,
    phase: payload.phase ?? s.phase,
    done: payload.done ?? s.done,
    total: payload.total ?? s.total,
  }));
}
