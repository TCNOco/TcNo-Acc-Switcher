import type { SearchResultRow } from "../components/SearchOverlay.svelte";
import { fuzzyWordsMatch } from "./searchFuzzy";

type Translator = (key: string) => string;

export type HomeSearchPick =
  | { kind: "command"; value: string }
  | { kind: "platform"; value: string }
  | { kind: "disabled"; value: string }
  | { kind: "unknown"; value: string };

export function buildHomePrimaryRows(
  homeOrder: string[],
  disabledPlatformNames: string[],
  query: string,
  tr: Translator,
  max: number,
): SearchResultRow[] {
  const disabled = new Set(disabledPlatformNames.map((name) => name.trim().toLowerCase()));
  const enabled = homeOrder.filter((name) => !disabled.has(name.trim().toLowerCase()));
  const trimmed = query.trim();
  let matches = trimmed
    ? enabled.filter((name) => fuzzyWordsMatch(trimmed, name))
    : enabled.slice(0, max);
  if (trimmed) {
    matches = matches.slice(0, max);
  }

  return matches.map((name) => ({
    key: `p:${name}`,
    title: name.toUpperCase(),
    badge: tr("Search_Section_Platform"),
    platformIconName: name,
  }));
}

export function buildHomeDisabledRows(
  disabledPlatformNames: string[],
  query: string,
  tr: Translator,
  max: number,
): SearchResultRow[] {
  const trimmed = query.trim();
  if (!trimmed) {
    return [];
  }

  return disabledPlatformNames
    .filter((name) => fuzzyWordsMatch(trimmed, name))
    .slice(0, max)
    .map((name) => ({
      key: `d:${name}`,
      title: name.toUpperCase(),
      badge: tr("Search_Section_DisabledPlatform"),
      platformIconName: name,
      isCategory: true,
    }));
}

export function classifyHomeSearchPick(key: string): HomeSearchPick {
  if (key.startsWith("cmd:")) {
    return { kind: "command", value: key };
  }
  if (key.startsWith("p:")) {
    return { kind: "platform", value: key.slice(2) };
  }
  if (key.startsWith("d:")) {
    return { kind: "disabled", value: key.slice(2) };
  }
  return { kind: "unknown", value: key };
}
