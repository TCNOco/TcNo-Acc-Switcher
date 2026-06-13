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

  /** Viewport padding — keep menu fully inside the window. */
  const PAD = 8;

  let menuEl: HTMLUListElement | null = null;
  let resizeObs: ResizeObserver | null = null;
  let mutationObs: MutationObserver | null = null;

  let submenuObserveDebounce: ReturnType<typeof setTimeout> | null = null;
  let menuReady = false;

  /**
   * Run attachObservers + layoutAfterOpen only once per open — the reactive block can fire many
   * times if `menuEl` / bind:this churns; re-attaching observers retriggers layout and hot-loops.
   */
  let menuSetupFor: ContextMenuState | null = null;

  /** Element to restore focus when the menu closes (set when moving focus into the menu). */
  let priorFocusEl: HTMLElement | null = null;
  let flyoutRaf: number | null = null;

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
    const li = el.closest("li.hasSubmenu") ?? (el.classList.contains("hasSubmenu") ? el : null);
    if (!li) {
      return;
    }
    const raw = li.getAttribute("data-submenu-path");
    if (!raw) {
      return;
    }
    try {
      submenuOpenPath.set(JSON.parse(raw) as number[]);
    } catch {
      /* ignore malformed path */
    }
  }

  function onMenuKeydown(ev: KeyboardEvent): void {
    if (!menuEl) {
      return;
    }
    const handled = handleContextMenuKeydown(ev, menuEl, {
      expandSubmenuForLi,
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
    if (handleContextMenuQuickFilter(ev, menuEl)) {
      ev.preventDefault();
      ev.stopPropagation();
    }
  }

  function clamp(v: number, lo: number, hi: number): number {
    return Math.max(lo, Math.min(hi, v));
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

  /**
   * When a submenu extends past the viewport, shift the root menu (moveContextMenu in legacy).
   */

  function alignExpandedSubmenusToRoot(root: HTMLUListElement): void {
    root.querySelectorAll("ul.submenu").forEach((sub) => {
      if (sub instanceof HTMLElement) {
        sub.style.removeProperty("top");
      }
    });
    const expanded = root.querySelectorAll("li.hasSubmenu.submenu-expanded > ul.submenu");
    if (expanded.length === 0) {
      return;
    }
    const rt = root.getBoundingClientRect().top;
    expanded.forEach((sub) => {
      if (!(sub instanceof HTMLElement)) {
        return;
      }
      const li = sub.parentElement;
      if (!(li instanceof HTMLElement) || !li.classList.contains("hasSubmenu")) {
        return;
      }
      const liT = li.getBoundingClientRect().top;
      sub.style.top = `${Math.round(rt - liT)}px`;
    });
  }

  /** Primary column height for `.submenu:has(.ctx-pagination-li)` (see context_menu.scss). */
  function syncCtxRootColumnHeightVar(root: HTMLUListElement): void {
    const h = root.offsetHeight;
    if (h > 0) {
      root.style.setProperty("--ctx-root-column-height", `${h}px`);
    }
  }

  function refreshFlyoutLayout(root: HTMLUListElement): void {
    nudgeRootForSubmenus(root);
    alignExpandedSubmenusToRoot(root);
    syncCtxRootColumnHeightVar(root);
  }

  function nudgeRootForSubmenus(el: HTMLUListElement): void {
    const subs = el.querySelectorAll(".submenu");
    let shiftLeft = 0;
    let shiftUp = 0;
    subs.forEach((sub) => {
      const r = sub.getBoundingClientRect();
      const rs = r.right - window.innerWidth;
      const bs = r.bottom - window.innerHeight;
      if (rs > 0) shiftLeft = Math.max(shiftLeft, rs + 40);
      if (bs > 0) shiftUp = Math.max(shiftUp, bs + 10);
    });
    if (shiftLeft <= 0 && shiftUp <= 0) {
      return;
    }
    const cur = el.getBoundingClientRect();
    const nextLeft = clamp(
      cur.left - shiftLeft,
      PAD,
      Math.max(PAD, window.innerWidth - cur.width - PAD),
    );
    const nextTop = clamp(
      cur.top - shiftUp,
      PAD,
      Math.max(PAD, window.innerHeight - cur.height - PAD),
    );
    el.style.left = `${nextLeft}px`;
    el.style.top = `${nextTop}px`;
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
    const { left, top } = layoutFromAnchor(st.x, st.y, mw, mh);
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
    observeSubmenus(menuEl);
    refreshFlyoutLayout(menuEl);
    submenuExpandEnabled.set(true);
    ctxMenuLog("layoutAfterOpen: done → submenuExpandEnabled=true", {
      submenuExpandEnabled: get(submenuExpandEnabled),
      menuRect: menuEl.getBoundingClientRect(),
    });
    priorFocusEl = capturePriorFocus(menuEl);
    focusFirstNavigable(menuEl);
  }

  function onWinResize(): void {
    if (get(contextMenu)) {
      applyAnchorLayout();
      if (menuEl) {
        refreshFlyoutLayout(menuEl);
      }
    }
  }

  function detachObservers(): void {
    if (menuEl) {
      menuEl.style.removeProperty("--ctx-root-column-height");
    }
    resizeObs?.disconnect();
    resizeObs = null;
    mutationObs?.disconnect();
    mutationObs = null;
    if (submenuObserveDebounce !== null) {
      clearTimeout(submenuObserveDebounce);
      submenuObserveDebounce = null;
    }
  }

  /** ResizeObserver does not support subtree; observe root + each .submenu (legacy submenu1/submenu2). */
  function observeSubmenus(el: HTMLUListElement): void {
    if (!resizeObs) {
      return;
    }
    el.querySelectorAll(".submenu").forEach((sub) => {
      resizeObs!.observe(sub);
    });
  }

  function attachObservers(el: HTMLUListElement): void {
    detachObservers();
    resizeObs = new ResizeObserver(() => {
      /* Never applyAnchorLayout here: root/subtree size changes (e.g. submenu pagination)
         would snap the menu back to the pointer anchor and undo nudge from flyouts. */
      refreshFlyoutLayout(el);
    });
    resizeObs.observe(el);
    observeSubmenus(el);

    mutationObs = new MutationObserver(() => {
      if (submenuObserveDebounce !== null) {
        clearTimeout(submenuObserveDebounce);
      }
      submenuObserveDebounce = setTimeout(() => {
        submenuObserveDebounce = null;
        if (!resizeObs || !menuEl || !get(contextMenu)) {
          return;
        }
        observeSubmenus(menuEl);
        refreshFlyoutLayout(menuEl);
      }, 40);
    });
    mutationObs.observe(el, { subtree: true, childList: true });
  }

  $: if ($contextMenu && menuEl) {
    if (menuSetupFor !== $contextMenu) {
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
    window.addEventListener("keydown", onKey);
    window.addEventListener("resize", onWinResize);

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
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("resize", onWinResize);
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
    tabindex="-1"
    in:scale={{ start: 0.97, duration: motionEnabled() ? DUR.fast : 0, easing: cubicOut }}
    on:keydown={onMenuKeydown}
  >
    <ContextMenuNest items={$contextMenu.items} depth={1} pathPrefix={[]} />
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
