/**
 * Whether focus that is no longer on the node should be taken back.
 *
 * Anywhere inside the same dialog is somewhere the user could have put it, and
 * is left alone. Focus that has left the dialog entirely, or that nothing holds,
 * was taken by something else settling after the screen appeared.
 */
export function focusEscapedSurface(surface: Element | null, active: Element | null): boolean {
  if (!active) return true;
  if (!surface) return false;
  return !surface.contains(active);
}

/**
 * Delays, in ms, at which a lost caret is taken back. Long enough to outlast the
 * frame or two in which a modal, its focus trap and the context menu that opened
 * it all settle; short enough that nothing can move focus in between by hand.
 * The context menu answers the same problem the same way - see
 * scheduleMenuFocusAfterOpen in ContextMenu.svelte.
 */
const RECLAIM_DELAYS_MS = [60, 160];

/**
 * Puts the caret in a field as the screen it belongs to appears.
 *
 * Focusing from the screen transition instead means guessing when the field will
 * exist and when it will stop being disabled, and then holding the caret against
 * everything that moves focus over the next frame or two - the modal's own focus
 * trap, and the context menu restoring focus to the row it was opened from. This
 * waits for the field, so the caret lands whichever path opened the screen.
 *
 * The parameter is whether the field can take focus yet; pass `!busy` for one
 * that is disabled while work is in flight.
 */
export function focusOnShow(node: HTMLElement, enabled = true) {
  let armed = enabled;
  let claimed = false;
  let frame = 0;
  const timers: ReturnType<typeof setTimeout>[] = [];

  function canFocus(): boolean {
    return node.isConnected && !(node as Partial<HTMLInputElement>).disabled;
  }

  function claim(): void {
    if (!canFocus()) return;
    node.focus({ preventScroll: true });
    if (node instanceof HTMLInputElement || node instanceof HTMLTextAreaElement) {
      node.select();
    }
    claimed = true;
  }

  function reclaimIfLost(): void {
    const active = node.ownerDocument.activeElement;
    if (active === node) return;
    if (!focusEscapedSurface(node.closest("[role='dialog']"), active)) return;
    claim();
  }

  function start(): void {
    if (!armed || claimed) return;
    cancelAnimationFrame(frame);
    frame = requestAnimationFrame(() => {
      if (!canFocus()) return;
      claim();
      for (const delay of RECLAIM_DELAYS_MS) {
        timers.push(setTimeout(reclaimIfLost, delay));
      }
    });
  }

  start();

  return {
    update(next: boolean) {
      armed = next;
      start();
    },
    destroy() {
      cancelAnimationFrame(frame);
      timers.forEach((timer) => clearTimeout(timer));
    },
  };
}
