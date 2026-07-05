/**
 * Keyboard navigation for `.ctx-menu-root` menus (arrow keys, Enter/Space, Escape handled elsewhere).
 */

const SEL_ROW = ":scope > li:not(.row-hidden):not(.ctx-sep):not(.ctx-pagination-li)";

export function focusFirstNavigable(menuRoot: HTMLElement): HTMLElement | null {
  const first = firstNavigableInMenu(menuRoot);
  first?.focus();
  return first;
}

export function restoreFocus(el: HTMLElement | null): void {
  if (!el || typeof el.focus !== "function") {
    return;
  }
  try {
    el.focus({ preventScroll: true });
  } catch {
    el.focus();
  }
}

/** Focus target for a row `li`: search row → input; submenu row → label `span` (or `li`); leaf → `button`. */
function focusTargetForRowLi(li: HTMLElement): HTMLElement | null {
  if (li.classList.contains("contextSearch")) {
    return li.querySelector<HTMLInputElement>("input.ctx-menu__search");
  }
  if (li.classList.contains("hasSubmenu")) {
    const btn = li.querySelector<HTMLElement>(":scope > button.ctx-menu__parent-action");
    if (btn) {
      return btn;
    }
    const lab = li.querySelector<HTMLElement>(":scope > .ctx-menu__label");
    return lab ?? li;
  }
  const btn = li.querySelector("button.ctx-menu__btn") as HTMLButtonElement | null;
  if (btn?.disabled) {
    return null;
  }
  return btn ?? li;
}

function focusTargetForParentSubmenuLi(li: HTMLElement): HTMLElement | null {
  if (!li.classList.contains("hasSubmenu")) {
    return null;
  }
  const target = focusTargetForRowLi(li);
  return target === li ? null : target;
}

/**
 * Leaf `.ctx-menu__btn` for keyboard activation — works when focus is on the `li` or on a
 * non-button child (e.g. label `span`). Returns null for `.hasSubmenu` rows.
 */
function leafButtonForActiveRow(active: HTMLElement): HTMLButtonElement | null {
  if (active instanceof HTMLButtonElement && active.classList.contains("ctx-menu__btn")) {
    return active;
  }
  const rowLi = active.closest("li");
  if (!(rowLi instanceof HTMLElement)) {
    return null;
  }
  const parentAct = rowLi.querySelector(":scope > button.ctx-menu__parent-action");
  if (parentAct instanceof HTMLButtonElement) {
    return parentAct;
  }
  if (rowLi.classList.contains("hasSubmenu")) {
    return null;
  }
  const b = rowLi.querySelector(":scope > button.ctx-menu__btn");
  return b instanceof HTMLButtonElement ? b : null;
}

/**
 * True when focus is on the parent row of a flyout (the `li` or its `.ctx-menu__label`), not
 * inside that row’s nested `ul.submenu` (avoids matching an ancestor `li.hasSubmenu` from deep
 * descendants — `closest("li.hasSubmenu")` alone is wrong there).
 */
function isSubmenuRowExpandTarget(active: HTMLElement, rowLi: HTMLElement): boolean {
  if (!rowLi.classList.contains("hasSubmenu")) {
    return false;
  }
  const sub = rowLi.querySelector(":scope > ul.submenu");
  if (sub instanceof HTMLElement && (active === sub || sub.contains(active))) {
    return false;
  }
  return (
    active === rowLi ||
    (active.classList.contains("ctx-menu__label") && active.parentElement === rowLi) ||
    (active.classList.contains("ctx-menu__parent-action") && active.parentElement === rowLi)
  );
}

/** Direct navigable targets in document order under this `ul` (one column). */
function navigableTargetsInColumn(ul: HTMLUListElement): HTMLElement[] {
  const rows = ul.querySelectorAll<HTMLElement>(SEL_ROW);
  const out: HTMLElement[] = [];
  rows.forEach((li) => {
    const t = focusTargetForRowLi(li);
    if (t) {
      out.push(t);
    }
  });
  return out;
}

function firstNavigableInMenu(menuRoot: HTMLElement): HTMLElement | null {
  const raw = menuRoot.matches("ul.ctx-menu-root") ? menuRoot : menuRoot.querySelector("ul.ctx-menu-root");
  const rootUl = raw instanceof HTMLUListElement ? raw : null;
  if (!rootUl) {
    return null;
  }
  const col = navigableTargetsInColumn(rootUl);
  return col[0] ?? null;
}

function parentMenuList(el: HTMLElement): HTMLUListElement | null {
  const li = el.closest("li");
  if (!li) {
    return null;
  }
  const parentUl = li.parentElement;
  return parentUl instanceof HTMLUListElement ? parentUl : null;
}

function indexInList(list: HTMLElement[], el: HTMLElement): number {
  return list.findIndex((x) => x === el);
}

/** First navigable item inside the nested `ul.submenu` of this `li.hasSubmenu`. */
function firstNavigableInSubmenu(liHasSubmenu: HTMLElement): HTMLElement | null {
  const sub = liHasSubmenu.querySelector<HTMLUListElement>("ul.submenu");
  if (!sub) {
    return null;
  }
  const items = navigableTargetsInColumn(sub);
  return items[0] ?? null;
}

/** Parent `li.hasSubmenu` that owns the submenu list containing `el`. */
function owningHasSubmenuLi(el: HTMLElement): HTMLElement | null {
  const list = parentMenuList(el);
  if (!list || !list.classList.contains("submenu")) {
    return null;
  }
  const owner = list.closest("li.hasSubmenu");
  return owner instanceof HTMLElement ? owner : null;
}

export type KeyboardNavDeps = {
  /** Expand submenu path for row index at current depth — caller maps index from `li`. */
  expandSubmenuForLi: (liHasSubmenu: HTMLElement) => void;
  /** Collapse the submenu owned by this row, if the caller tracks open submenu state. */
  collapseSubmenuForLi?: (liHasSubmenu: HTMLElement) => void;
};

function navigateArrow(active: HTMLElement, menu: HTMLElement, key: "ArrowDown" | "ArrowUp"): void {
  const searchInput = menu.querySelector<HTMLInputElement>("li.contextSearch input.ctx-menu__search");
  if (active === searchInput) {
    const col = navigableTargetsInColumn(menu as HTMLUListElement);
    if (col.length === 0) return;
    const i = col.indexOf(active);
    if (key === "ArrowDown") col[i + 1]?.focus();
    else {
      const prev = i <= 0 ? col[col.length - 1] : col[i - 1];
      prev?.focus();
    }
    return;
  }

  const list = parentMenuList(active);
  if (!list) return;
  const items = navigableTargetsInColumn(list);
  const i = indexInList(items, active);
  if (i < 0) return;
  const delta = key === "ArrowDown" ? 1 : -1;
  items[i + delta]?.focus();
}

function tryExpandSubmenu(active: HTMLElement, deps: KeyboardNavDeps): boolean {
  const rowLi = active.closest("li");
  if (!(rowLi instanceof HTMLElement) || !rowLi.classList.contains("hasSubmenu") || !isSubmenuRowExpandTarget(active, rowLi))
    return false;
  deps.expandSubmenuForLi(rowLi);
  requestAnimationFrame(() => {
    const inner = firstNavigableInSubmenu(rowLi);
    inner?.focus();
  });
  return true;
}

function tryActivateRow(active: HTMLElement, deps: KeyboardNavDeps): boolean {
  const leafBtn = leafButtonForActiveRow(active);
  if (leafBtn) {
    leafBtn.click();
    return true;
  }
  return tryExpandSubmenu(active, deps);
}

function jumpToEdge(active: HTMLElement, key: "Home" | "End"): void {
  const list = parentMenuList(active);
  if (!list) return;
  const items = navigableTargetsInColumn(list);
  const target = key === "Home" ? items[0] : items[items.length - 1];
  target?.focus();
}

type KeyHandler = (ev: KeyboardEvent, active: HTMLElement, menu: HTMLElement, deps: KeyboardNavDeps) => boolean;

function handleArrow(ev: KeyboardEvent, active: HTMLElement, menu: HTMLElement, key: "ArrowDown" | "ArrowUp"): boolean {
  ev.preventDefault();
  ev.stopPropagation();
  navigateArrow(active, menu, key);
  return true;
}

function handleActivation(ev: KeyboardEvent, active: HTMLElement, deps: KeyboardNavDeps): boolean {
  if (active.tagName === "INPUT") return false;
  if (!tryActivateRow(active, deps)) return false;
  ev.preventDefault();
  ev.stopPropagation();
  return true;
}

function handleEdgeKey(ev: KeyboardEvent, active: HTMLElement, key: "Home" | "End"): boolean {
  if (active.tagName === "INPUT") return false;
  ev.preventDefault();
  ev.stopPropagation();
  jumpToEdge(active, key);
  return true;
}

/** Search input in the rightmost/deepest expanded flyout column, if present. */
function searchInputInLatestSection(menuRoot: HTMLElement): HTMLInputElement | null {
  const expanded = menuRoot.querySelectorAll<HTMLUListElement>("li.hasSubmenu.submenu-expanded > ul.submenu");
  const latest = expanded.length > 0 ? expanded[expanded.length - 1]! : menuRoot;
  return latest.querySelector<HTMLInputElement>("li.contextSearch input.ctx-menu__search");
}

function isQuickFilterTypingKey(ev: KeyboardEvent): boolean {
  if (ev.ctrlKey || ev.altKey || ev.metaKey || ev.isComposing) {
    return false;
  }
  if (ev.key === " ") {
    return false;
  }
  return ev.key.length === 1;
}

/** Focus the latest section's search field and insert the typed character (quick-filter). */
function redirectTypingToSearch(ev: KeyboardEvent, active: HTMLElement, menuRoot: HTMLElement): boolean {
  if (!isQuickFilterTypingKey(ev)) {
    return false;
  }
  if (active instanceof HTMLInputElement && active.classList.contains("ctx-menu__search")) {
    return false;
  }

  const searchInput = searchInputInLatestSection(menuRoot);
  if (!searchInput) {
    return false;
  }

  ev.preventDefault();
  ev.stopPropagation();

  const char = ev.key;
  searchInput.focus();
  const start = searchInput.selectionStart ?? searchInput.value.length;
  const end = searchInput.selectionEnd ?? searchInput.value.length;
  searchInput.value = searchInput.value.slice(0, start) + char + searchInput.value.slice(end);
  searchInput.dispatchEvent(new Event("input", { bubbles: true }));
  const pos = start + char.length;
  searchInput.setSelectionRange(pos, pos);
  return true;
}

const KEY_HANDLERS: Record<string, KeyHandler> = {
  ArrowDown: (ev, a, m) => handleArrow(ev, a, m, "ArrowDown"),
  ArrowUp: (ev, a, m) => handleArrow(ev, a, m, "ArrowUp"),
  ArrowRight: (ev, a, _m, d) => {
    if (!tryExpandSubmenu(a, d)) return false;
    ev.preventDefault();
    ev.stopPropagation();
    return true;
  },
  ArrowLeft: (ev, a, _m, d) => {
    const ownerLi = owningHasSubmenuLi(a);
    if (!ownerLi) return false;
    ev.preventDefault();
    ev.stopPropagation();
    d.collapseSubmenuForLi?.(ownerLi);
    focusTargetForParentSubmenuLi(ownerLi)?.focus();
    return true;
  },
  Enter: (ev, a, _m, d) => handleActivation(ev, a, d),
  " ": (ev, a, _m, d) => handleActivation(ev, a, d),
  Home: (ev, a) => handleEdgeKey(ev, a, "Home"),
  End: (ev, a) => handleEdgeKey(ev, a, "End"),
};

/** Quick-filter when the menu is open but focus has not moved into it (e.g. mouse-expanded flyout). */
export function handleContextMenuQuickFilter(ev: KeyboardEvent, menuRoot: HTMLElement | null): boolean {
  if (!menuRoot) {
    return false;
  }
  const menu = menuRoot.closest(".ctx-menu-root") ?? menuRoot;
  if (!(menu instanceof HTMLElement) || !menu.classList.contains("ctx-menu-root")) {
    return false;
  }
  const active = document.activeElement as HTMLElement | null;
  if (active && menu.contains(active)) {
    return false;
  }
  return redirectTypingToSearch(ev, active ?? document.body, menu);
}

export function handleContextMenuKeydown(ev: KeyboardEvent, menuRoot: HTMLElement, deps: KeyboardNavDeps): boolean {
  const menu = menuRoot.closest(".ctx-menu-root") ?? menuRoot;
  if (!(menu instanceof HTMLElement) || !menu.classList.contains("ctx-menu-root")) return false;

  const active = document.activeElement as HTMLElement | null;
  if (!active || !menu.contains(active)) {
    if (ev.key === "ArrowDown" || ev.key === "ArrowUp" || ev.key === "ArrowRight" || ev.key === "ArrowLeft") {
      ev.preventDefault();
      ev.stopPropagation();
      focusFirstNavigable(menu);
      return true;
    }
    return false;
  }

  if (ev.key === "Escape") return false;

  if (redirectTypingToSearch(ev, active, menu)) {
    return true;
  }

  const handler = KEY_HANDLERS[ev.key];
  return handler ? handler(ev, active, menu, deps) : false;
}
