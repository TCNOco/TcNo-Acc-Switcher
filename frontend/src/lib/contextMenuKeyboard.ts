/**
 * Keyboard navigation for `.ctx-menu-root` menus (arrow keys, Enter/Space, Escape handled elsewhere).
 */

const SEL_ROW = ":scope > li:not(.row-hidden):not(.ctx-sep):not(.ctx-pagination-li)";

export function focusFirstNavigable(menuRoot: HTMLElement): void {
  const first = firstNavigableInMenu(menuRoot);
  first?.focus();
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
    const lab = li.querySelector<HTMLElement>(":scope > .ctx-menu__label");
    return lab ?? li;
  }
  const btn = li.querySelector("button.ctx-menu__btn") as HTMLButtonElement | null;
  return btn ?? li;
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
  if (!(rowLi instanceof HTMLElement) || rowLi.classList.contains("hasSubmenu")) {
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
    (active.classList.contains("ctx-menu__label") && active.parentElement === rowLi)
  );
}

/** Direct navigable targets in document order under this `ul` (one column). */
export function navigableTargetsInColumn(ul: HTMLUListElement): HTMLElement[] {
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
};

/**
 * Returns true if the event was consumed (caller should preventDefault/stopPropagation).
 */
export function handleContextMenuKeydown(ev: KeyboardEvent, menuRoot: HTMLElement, deps: KeyboardNavDeps): boolean {
  const menu = menuRoot.closest(".ctx-menu-root") ?? menuRoot;
  if (!(menu instanceof HTMLElement) || !menu.classList.contains("ctx-menu-root")) {
    return false;
  }

  const active = document.activeElement as HTMLElement | null;
  if (!active || !menu.contains(active)) {
    return false;
  }

  const key = ev.key;

  if (key === "Escape") {
    return false;
  }

  const searchInput = menu.querySelector<HTMLInputElement>("li.contextSearch input.ctx-menu__search");

  if (active === searchInput && (key === "ArrowDown" || key === "ArrowUp")) {
    ev.preventDefault();
    ev.stopPropagation();
    const rootUl = menu as HTMLUListElement;
    const col = navigableTargetsInColumn(rootUl);
    if (col.length === 0) {
      return true;
    }
    const i = col.indexOf(active);
    if (key === "ArrowDown") {
      col[i + 1]?.focus();
    } else {
      const prev = i <= 0 ? col[col.length - 1] : col[i - 1];
      prev?.focus();
    }
    return true;
  }

  if (key === "ArrowDown" || key === "ArrowUp") {
    ev.preventDefault();
    ev.stopPropagation();
    const list = parentMenuList(active);
    if (!list) {
      return true;
    }
    const items = navigableTargetsInColumn(list);
    const i = indexInList(items, active);
    if (i < 0) {
      return true;
    }
    const delta = key === "ArrowDown" ? 1 : -1;
    const next = items[i + delta];
    next?.focus();
    return true;
  }

  if (key === "ArrowRight") {
    const rowLi = active.closest("li");
    if (
      !(rowLi instanceof HTMLElement) ||
      !rowLi.classList.contains("hasSubmenu") ||
      !isSubmenuRowExpandTarget(active, rowLi)
    ) {
      return false;
    }
    ev.preventDefault();
    ev.stopPropagation();
    deps.expandSubmenuForLi(rowLi);
    requestAnimationFrame(() => {
      const inner = firstNavigableInSubmenu(rowLi);
      inner?.focus();
    });
    return true;
  }

  if (key === "ArrowLeft") {
    const ownerLi = owningHasSubmenuLi(active);
    if (!ownerLi) {
      return false;
    }
    ev.preventDefault();
    ev.stopPropagation();
    ownerLi.focus();
    return true;
  }

  if (key === "Enter" || key === " ") {
    if (active.tagName === "INPUT") {
      return false;
    }
    const leafBtn = leafButtonForActiveRow(active);
    if (leafBtn) {
      ev.preventDefault();
      ev.stopPropagation();
      leafBtn.click();
      return true;
    }
    const rowLi = active.closest("li");
    if (
      rowLi instanceof HTMLElement &&
      rowLi.classList.contains("hasSubmenu") &&
      isSubmenuRowExpandTarget(active, rowLi)
    ) {
      ev.preventDefault();
      ev.stopPropagation();
      deps.expandSubmenuForLi(rowLi);
      requestAnimationFrame(() => {
        const inner = firstNavigableInSubmenu(rowLi);
        inner?.focus();
      });
      return true;
    }
    return false;
  }

  if (key === "Home") {
    if (active.tagName === "INPUT") {
      return false;
    }
    ev.preventDefault();
    ev.stopPropagation();
    const list = parentMenuList(active);
    if (!list) {
      return true;
    }
    const items = navigableTargetsInColumn(list);
    items[0]?.focus();
    return true;
  }

  if (key === "End") {
    if (active.tagName === "INPUT") {
      return false;
    }
    ev.preventDefault();
    ev.stopPropagation();
    const list = parentMenuList(active);
    if (!list) {
      return true;
    }
    const items = navigableTargetsInColumn(list);
    items[items.length - 1]?.focus();
    return true;
  }

  return false;
}
