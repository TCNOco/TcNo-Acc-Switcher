<script lang="ts">
  import { scale } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { DUR, motionEnabled } from "../lib/animation";
  import { get } from "svelte/store";
  import { onDestroy, onMount, tick } from "svelte";
  import {
    contextMenu,
    closeContextMenu,
    submenuOpenPath,
    submenuExpandEnabled,
    type ContextMenuState,
    type ContextMenuAnchorRect,
  } from "../stores/contextMenu";
  import ContextMenuNest from "./ContextMenuNest.svelte";
  const CTX_MENU_DEBUG = false;
  function ctxMenuLog(...args: unknown[]): void {
    if (CTX_MENU_DEBUG) console.log("[ctx-menu]", ...args);
  }
  import {
    focusFirstNavigable,
    restoreFocus,
    handleContextMenuKeydown,
    handleContextMenuQuickFilter,
  } from "../lib/contextMenuKeyboard";
  import {
    submenuShouldFillRootHeight,
    submenuTopOffset,
  } from "../lib/contextMenuLayout";

  /** Viewport padding — keep menu fully inside the window. */
  const PAD = 8;

  let menuEl: HTMLUListElement | null = null;
  let resizeObs: ResizeObserver | null = null;
  let mutationObs: MutationObserver | null = null;

  let submenuObserveDebounce: ReturnType<typeof setTimeout> | null = null;
  let menuReady = false;
  let layoutEpoch = 0;

  /**
   * Run attachObservers + layoutAfterOpen only once per open — the reactive block can fire many
   * times if `menuEl` / bind:this churns; re-attaching observers retriggers layout and hot-loops.
   */
  let menuSetupFor: ContextMenuState | null = null;

  /** Element to restore focus when the menu closes (set when moving focus into the menu). */
  let priorFocusEl: HTMLElement | null = null;
  let flyoutRaf: number | null = null;
  let focusAfterOpenTimers: ReturnType<typeof setTimeout>[] = [];

  function capturePriorFocus(menuRoot: HTMLElement): HTMLElement | null {
    const a = document.activeElement;
    if (!(a instanceof HTMLElement)) {
      return null;
    }
    if (menuRoot.contains(a)) {
      return null;
    }
    return a;
  }

  function expandSubmenuForLi(el: HTMLElement): void {
    const path = submenuPathForLi(el);
    if (!path) {
      return;
    }
    submenuOpenPath.set(path);
  }

  function submenuPathForLi(el: HTMLElement): number[] | null {
    const li = el.closest("li.hasSubmenu") ?? (el.classList.contains("hasSubmenu") ? el : null);
    if (!li) {
      return null;
    }
    const raw = li.getAttribute("data-submenu-path");
    if (!raw) {
      return null;
    }
    try {
      return JSON.parse(raw) as number[];
    } catch {
      return null;
    }
  }

  function collapseSubmenuForLi(el: HTMLElement): void {
    const path = submenuPathForLi(el);
    if (!path) {
      return;
    }
    submenuOpenPath.set(path.slice(0, -1));
  }

  function focusMenuIfNeeded(force = false): void {
    if (!menuEl || !get(contextMenu)) {
      return;
    }
    const active = document.activeElement;
    if (force || !(active instanceof HTMLElement) || !menuEl.contains(active)) {
      focusFirstNavigable(menuEl);
    }
  }

  function clearFocusAfterOpenTimers(): void {
    focusAfterOpenTimers.forEach((timer) => clearTimeout(timer));
    focusAfterOpenTimers = [];
  }

  function scheduleMenuFocusAfterOpen(): void {
    clearFocusAfterOpenTimers();
    focusMenuIfNeeded(true);
    requestAnimationFrame(() => focusMenuIfNeeded());
    for (const delay of [0, 60, 160]) {
      focusAfterOpenTimers.push(setTimeout(() => focusMenuIfNeeded(), delay));
    }
  }

  function contextMenuKeyboardDeps() {
    return {
      expandSubmenuForLi,
      collapseSubmenuForLi,
    };
  }

  function onMenuKeydown(ev: KeyboardEvent): void {
    if (!menuEl) {
      return;
    }
    const handled = handleContextMenuKeydown(ev, menuEl, {
      ...contextMenuKeyboardDeps(),
    });
    if (handled) {
      ev.stopPropagation();
    }
  }

  function onDocPointerDown(ev: MouseEvent): void {
    const t = ev.target as Node | null;
    const menu = document.querySelector(".ctx-menu-root");
    if (menu && t && menu.contains(t)) {
      return;
    }
    closeContextMenu();
  }

  function onKey(ev: KeyboardEvent): void {
    if (ev.key === "Escape") {
      closeContextMenu();
      return;
    }
    if (menuEl && handleContextMenuKeydown(ev, menuEl, contextMenuKeyboardDeps())) {
      ev.stopPropagation();
      return;
    }
    if (handleContextMenuQuickFilter(ev, menuEl)) {
      ev.preventDefault();
      ev.stopPropagation();
    }
  }

  function clamp(v: number, lo: number, hi: number): number {
    return Math.max(lo, Math.min(hi, v));
  }

  function layoutFromRect(
    anchorRect: ContextMenuAnchorRect,
    mw: number,
    mh: number,
  ): { left: number; top: number } {
    const winW = window.innerWidth;
    const winH = window.innerHeight;
    const gap = 6;
    const preferRightEdge = anchorRect.right - mw;
    const preferAbove = anchorRect.top - mh - gap;
    let left = anchorRect.left;
    let top = anchorRect.bottom + gap;
    if (left + mw + PAD > winW) {
      left = preferRightEdge;
    }
    if (top + mh + PAD > winH) {
      top = preferAbove;
    }
    left = clamp(left, PAD, Math.max(PAD, winW - mw - PAD));
    top = clamp(top, PAD, Math.max(PAD, winH - mh - PAD));
    return { left, top };
  }

  /**
   * Initial placement from pointer — matches legacy wwwroot/js/context_menu.js
   * (viewport coords from clientX/Y; legacy page offsets omitted for Wails).
   */
  function layoutFromAnchor(ax: number, ay: number, mw: number, mh: number): { left: number; top: number } {
    const winW = window.innerWidth;
    const winH = window.innerHeight;
    const posX = ax - 14;
    const posY = ay;
    const hOffset = 42;
    const xOverflow = posX + mw + hOffset - winW;
    const yOverflow = posY + mh + hOffset - winH;
    let left = posX + (xOverflow > 0 ? -mw : 10);
    let top = posY + (yOverflow > 0 ? -yOverflow : 10);
    left = clamp(left, PAD, Math.max(PAD, winW - mw - PAD));
    top = clamp(top, PAD, Math.max(PAD, winH - mh - PAD));
    return { left, top };
  }

  function fitExpandedSubmenusToViewport(root: HTMLUListElement): void {
    root.querySelectorAll("ul.submenu").forEach((sub) => {
      if (sub instanceof HTMLElement) {
        sub.style.removeProperty("top");
        sub.style.removeProperty("height");
        sub.style.removeProperty("max-height");
      }
    });
    const expanded = root.querySelectorAll("li.hasSubmenu.submenu-expanded > ul.submenu");
    if (expanded.length === 0) {
      return;
    }
    const rootRect = root.getBoundingClientRect();
    expanded.forEach((sub) => {
      if (!(sub instanceof HTMLElement)) {
        return;
      }
      const li = sub.parentElement;
      if (!(li instanceof HTMLElement) || !li.classList.contains("hasSubmenu")) {
        return;
      }
      const rect = sub.getBoundingClientRect();
      const topOffset = submenuTopOffset({
        naturalTop: rect.top,
        naturalBottom: rect.bottom,
        rootTop: rootRect.top,
        rootBottom: rootRect.bottom,
      });
      const rows = Array.from(sub.children).filter((child): child is HTMLElement => child instanceof HTMLElement);
      const itemHeights = rows
        .filter((row) =>
          !row.classList.contains("contextSearch") &&
          !row.classList.contains("ctx-create-li") &&
          !row.classList.contains("ctx-pagination-li")
        )
        .map((row) => row.getBoundingClientRect().height)
        .filter((height) => height > 0);
      const hasPagination = rows.some((row) => row.classList.contains("ctx-pagination-li"));
      const rowHeight = itemHeights.length > 0 ? Math.min(...itemHeights) : 0;
      if (hasPagination && submenuShouldFillRootHeight({
        naturalTop: rect.top,
        topOffset,
        naturalHeight: rect.height,
        rootTop: rootRect.top,
        rootHeight: rootRect.height,
        rowHeight,
      })) {
        sub.style.top = `${rootRect.top - rect.top}px`;
        sub.style.height = `${rootRect.height}px`;
        return;
      }
      sub.style.top = `${topOffset}px`;
    });
  }

  function refreshFlyoutLayout(root: HTMLUListElement): void {
    fitExpandedSubmenusToViewport(root);
    nudgeRootForSubmenus(root);
  }

  function onSubmenuPlanned(ev: Event): void {
    if (
      menuEl &&
      ev.target instanceof Node &&
      menuEl.contains(ev.target) &&
      get(contextMenu)
    ) {
      refreshFlyoutLayout(menuEl);
    }
  }

  function nudgeRootForSubmenus(el: HTMLUListElement): void {
    const subs = el.querySelectorAll(".submenu");
    let shiftLeft = 0;
    subs.forEach((sub) => {
      const r = sub.getBoundingClientRect();
      const rs = r.right - window.innerWidth;
      if (rs > 0) shiftLeft = Math.max(shiftLeft, rs + 40);
    });
    if (shiftLeft <= 0) {
      return;
    }
    const cur = el.getBoundingClientRect();
    const nextLeft = clamp(
      cur.left - shiftLeft,
      PAD,
      Math.max(PAD, window.innerWidth - cur.width - PAD),
    );
    el.style.left = `${nextLeft}px`;
  }

  function applyAnchorLayout(): void {
    const st = get(contextMenu);
    if (!st || !menuEl) {
      return;
    }
    const mw = menuEl.offsetWidth;
    const mh = menuEl.offsetHeight;
    if (mw === 0 || mh === 0) {
      return;
    }
    const { left, top } = st.anchorRect
      ? layoutFromRect(st.anchorRect, mw, mh)
      : layoutFromAnchor(st.x, st.y, mw, mh);
    menuEl.style.left = `${left}px`;
    menuEl.style.top = `${top}px`;
  }

  async function layoutAfterOpen(): Promise<void> {
    ctxMenuLog("layoutAfterOpen: start");
    const st = get(contextMenu);
    if (!st || !menuEl) return;
    menuEl.style.left = `${st.x}px`;
    menuEl.style.top = `${st.y}px`;
    menuReady = true;
    await tick();
    await new Promise<void>((r) => requestAnimationFrame(() => r()));
    if (!get(contextMenu) || !menuEl) {
      ctxMenuLog("layoutAfterOpen: aborted (no context or menuEl)");
      return;
    }
    applyAnchorLayout();
    void menuEl.offsetHeight;
    refreshFlyoutLayout(menuEl);
    submenuExpandEnabled.set(true);
    ctxMenuLog("layoutAfterOpen: done → submenuExpandEnabled=true", {
      submenuExpandEnabled: get(submenuExpandEnabled),
      menuRect: menuEl.getBoundingClientRect(),
    });
    priorFocusEl = capturePriorFocus(menuEl);
    scheduleMenuFocusAfterOpen();
  }

  function onWinResize(): void {
    if (get(contextMenu)) {
      layoutEpoch++;
      applyAnchorLayout();
      if (menuEl) {
        refreshFlyoutLayout(menuEl);
      }
    }
  }

  function detachObservers(): void {
    clearFocusAfterOpenTimers();
    resizeObs?.disconnect();
    resizeObs = null;
    mutationObs?.disconnect();
    mutationObs = null;
    if (submenuObserveDebounce !== null) {
      clearTimeout(submenuObserveDebounce);
      submenuObserveDebounce = null;
    }
  }

  function attachObservers(el: HTMLUListElement): void {
    detachObservers();
    resizeObs = new ResizeObserver(() => {
      /* Never applyAnchorLayout here: root/subtree size changes (e.g. submenu pagination)
         would snap the menu back to the pointer anchor and undo nudge from flyouts. */
      refreshFlyoutLayout(el);
    });
    resizeObs.observe(el);

    mutationObs = new MutationObserver(() => {
      if (submenuObserveDebounce !== null) {
        clearTimeout(submenuObserveDebounce);
      }
      submenuObserveDebounce = setTimeout(() => {
        submenuObserveDebounce = null;
        if (!resizeObs || !menuEl || !get(contextMenu)) {
          return;
        }
        refreshFlyoutLayout(menuEl);
      }, 40);
    });
    mutationObs.observe(el, { subtree: true, childList: true });
  }

  $: if ($contextMenu && menuEl) {
    if (menuSetupFor !== $contextMenu) {
      layoutEpoch++;
      menuSetupFor = $contextMenu;
      ctxMenuLog("setup once / open", {
        items: $contextMenu.items.length,
        submenuExpandEnabledBefore: get(submenuExpandEnabled),
      });
      attachObservers(menuEl);
      void layoutAfterOpen();
    }
  }

  $: if (!$contextMenu) {
    menuSetupFor = null;
    menuReady = false;
    ctxMenuLog("context menu cleared → detachObservers");
    detachObservers();
    const restore = priorFocusEl;
    priorFocusEl = null;
    restoreFocus(restore);
  }

  onMount(() => {
    window.addEventListener("pointerdown", onDocPointerDown, true);
    window.addEventListener("keydown", onKey, true);
    window.addEventListener("resize", onWinResize);
    document.addEventListener("ctxsubmenuplanned", onSubmenuPlanned);

    const unsub = submenuOpenPath.subscribe(() => {
      if (!menuEl || !get(contextMenu)) {
        return;
      }
      if (flyoutRaf !== null) {
        cancelAnimationFrame(flyoutRaf);
      }
      flyoutRaf = requestAnimationFrame(() => {
        flyoutRaf = null;
        if (!menuEl || !get(contextMenu)) {
          return;
        }
        refreshFlyoutLayout(menuEl);
      });
    });

    return () => {
      window.removeEventListener("pointerdown", onDocPointerDown, true);
      window.removeEventListener("keydown", onKey, true);
      window.removeEventListener("resize", onWinResize);
      document.removeEventListener("ctxsubmenuplanned", onSubmenuPlanned);
      unsub();
    };
  });

  onDestroy(() => {
    if (flyoutRaf !== null) {
      cancelAnimationFrame(flyoutRaf);
    }
    detachObservers();
    closeContextMenu();
  });
</script>

{#if $contextMenu}
  <ul
    bind:this={menuEl}
    class="ctx-menu-root contextmenu"
    class:ctx-menu-root--ready={menuReady}
    role="menu"
    aria-label="Context menu"
    tabindex="-1"
    in:scale={{ start: 0.97, duration: motionEnabled() ? DUR.fast : 0, easing: cubicOut }}
    on:keydown={onMenuKeydown}
  >
    <ContextMenuNest
      items={$contextMenu.items}
      {layoutEpoch}
      depth={1}
      pathPrefix={[]}
    />
  </ul>
{/if}

<style lang="scss">
  /* fixed + clientX/Y; overrides global .contextmenu { position: absolute } */
  ul.ctx-menu-root.contextmenu {
    position: fixed;
    left: 0;
    top: 0;
    margin: 0;
    z-index: 999999;
  }

  ul.ctx-menu-root.contextmenu:not(.ctx-menu-root--ready) {
    visibility: hidden;
  }
</style>
