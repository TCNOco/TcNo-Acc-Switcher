import type { Action } from "svelte/action";

/** Text filter for the settings pages.
 *
 * Matches over the rendered DOM rather than threading a query through every
 * settings component. The controls come from a dozen files and keep growing; a
 * prop someone forgot to pass would silently drop that setting out of search
 * results, and nothing would fail loudly enough to notice.
 */

/** Elements a query is matched against individually. */
const UNIT_SELECTOR = [
  // Before .settings-toggle so a matched parent brings its dependent along:
  // "Share total switches" on its own says nothing about Discord.
  ".settings-stack",
  ".settings-toggle",
  ".settings-field",
  ".settings-card",
  ".settings-note",
  ".settings-links",
  ".settings-actions > button",
  ".theme-controls",
  ".bg-settings-row",
  "[data-search-item]",
].join(", ");

/** Containers that should disappear once every child of theirs is hidden. */
const WRAPPER_SELECTOR = ".settings-grid, .settings-actions";

const HIDDEN_ATTR = "data-filter-hidden";

/** Folded so "Zurücksetzen" is reachable by typing "zuruck". */
function fold(value: string): string {
  return value
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .toLowerCase()
    .replace(/\s+/g, " ")
    .trim();
}

export function queryTokens(query: string): string[] {
  return fold(query).split(" ").filter((token) => token.length > 0);
}

/** Every token must appear somewhere in `text`; order does not matter. */
export function matchesQuery(text: string, tokens: string[]): boolean {
  const haystack = fold(text);
  return tokens.every((token) => haystack.includes(token));
}

function setHidden(el: Element, hidden: boolean): void {
  el.toggleAttribute(HIDDEN_ATTR, hidden);
}

function clearFilter(root: HTMLElement): void {
  for (const el of root.querySelectorAll(`[${HIDDEN_ATTR}]`)) {
    el.removeAttribute(HIDDEN_ATTR);
  }
}

/** Drops units contained by another unit, so a group is matched at one level only. */
function outermost(units: HTMLElement[]): HTMLElement[] {
  return units.filter((unit) => !units.some((other) => other !== unit && other.contains(unit)));
}

function hideEmptyWrappers(group: HTMLElement): void {
  // Reversed document order visits descendants before their ancestors, so an
  // emptied inner wrapper still counts as hidden when the outer one is checked.
  const wrappers = [...group.querySelectorAll<HTMLElement>(WRAPPER_SELECTOR)].reverse();
  for (const wrapper of wrappers) {
    const hasVisibleChild = [...wrapper.children].some((child) => !child.hasAttribute(HIDDEN_ATTR));
    setHidden(wrapper, !hasVisibleChild);
  }
}

/** A subheading with nothing left under it is noise. */
function hideOrphanSubheaders(group: HTMLElement): void {
  for (const sub of group.querySelectorAll<HTMLElement>(".settings-subheader")) {
    let sibling = sub.nextElementSibling;
    let hasVisible = false;
    while (sibling && !sibling.classList.contains("settings-subheader")) {
      if (!sibling.hasAttribute(HIDDEN_ATTR)) {
        hasVisible = true;
        break;
      }
      sibling = sibling.nextElementSibling;
    }
    setHidden(sub, !hasVisible);
  }
}

function filterGroup(group: HTMLElement, tokens: string[]): boolean {
  const header = group.querySelector(".SettingsHeader");
  // A section whose own name matches keeps all of its contents: someone typing
  // "security" wants the section, not the one control that repeats the word.
  const headerMatches = matchesQuery(header?.textContent ?? "", tokens);
  const units = outermost([...group.querySelectorAll<HTMLElement>(UNIT_SELECTOR)]);

  if (units.length === 0) {
    const show = headerMatches || matchesQuery(group.textContent ?? "", tokens);
    setHidden(group, !show);
    return show;
  }

  let visible = 0;
  for (const unit of units) {
    const show = headerMatches || matchesQuery(unit.textContent ?? "", tokens);
    setHidden(unit, !show);
    if (show) visible += 1;
  }

  hideEmptyWrappers(group);
  hideOrphanSubheaders(group);
  setHidden(group, visible === 0);
  return visible > 0;
}

const isHeading = (el: Element): boolean => /^H[1-6]$/.test(el.tagName);
const isRule = (el: Element): boolean => el.tagName === "HR";

/**
 * Hides the page's own headings and dividers once the groups they introduce are
 * gone. These sit beside the groups rather than inside them — "Steam settings"
 * above a rule, "App settings" below it — so `filterGroup` never reaches them,
 * and a search matching one app-wide toggle used to leave both headings and the
 * rule stranded above it.
 */
function hidePageChrome(root: HTMLElement): void {
  const children = [...root.children] as HTMLElement[];

  // A heading owns the run of siblings up to the next heading or rule.
  for (const [i, el] of children.entries()) {
    if (!isHeading(el)) continue;
    let hasVisible = false;
    for (const next of children.slice(i + 1)) {
      if (isHeading(next) || isRule(next)) break;
      if (!next.hasAttribute(HIDDEN_ATTR)) {
        hasVisible = true;
        break;
      }
    }
    setHidden(el, !hasVisible);
  }

  // After the headings, so one that just went away cannot keep a rule alive.
  for (const [i, el] of children.entries()) {
    if (!isRule(el)) continue;
    const visible = (c: HTMLElement) => !c.hasAttribute(HIDDEN_ATTR);
    setHidden(el, !(children.slice(0, i).some(visible) && children.slice(i + 1).some(visible)));
  }
}

/** @returns number of sections still showing, or -1 when not filtering. */
export function applySettingsFilter(root: HTMLElement, query: string): number {
  clearFilter(root);
  const tokens = queryTokens(query);
  if (tokens.length === 0) return -1;

  let visibleGroups = 0;
  for (const group of root.querySelectorAll<HTMLElement>(".settings-group")) {
    if (filterGroup(group, tokens)) visibleGroups += 1;
  }
  hidePageChrome(root);
  return visibleGroups;
}

/**
 * Keeps `node`'s settings filtered to `query`, dispatching `filtered` with the
 * surviving section count so the page can offer an empty state.
 *
 * Re-runs on DOM changes: several sections (quarantines, platform settings,
 * backup availability) only appear once their backend call resolves.
 */
export const settingsFilter: Action<HTMLElement, string, { "on:filtered": (e: CustomEvent<number>) => void }> = (
  node,
  query = "",
) => {
  let current = query;
  let frame = 0;

  function run(): void {
    frame = 0;
    const visibleGroups = applySettingsFilter(node, current);
    node.dispatchEvent(new CustomEvent("filtered", { detail: visibleGroups }));
  }

  function schedule(): void {
    if (frame) return;
    frame = requestAnimationFrame(run);
  }

  // childList only: the filter marks elements with an attribute, so observing
  // attributes here would make every pass trigger another one.
  const observer = new MutationObserver(schedule);
  observer.observe(node, { childList: true, subtree: true });
  run();

  return {
    update(next: string) {
      current = next ?? "";
      run();
    },
    destroy() {
      observer.disconnect();
      if (frame) cancelAnimationFrame(frame);
    },
  };
};
