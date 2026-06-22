import type { SearchResultRow } from "../components/SearchOverlay.svelte";
import { fuzzyWordsMatch } from "./searchFuzzy";

export type CommandPaletteCommand = {
  id: string;
  title: string;
  run: () => void | Promise<void>;
};

export const COMMAND_PREFIX = ">";

export function isCommandQuery(query: string): boolean {
  return query.trimStart().startsWith(COMMAND_PREFIX);
}

export function commandNeedle(query: string): string {
  const trimmed = query.trimStart();
  return trimmed.startsWith(COMMAND_PREFIX) ? trimmed.slice(1).trim() : trimmed.trim();
}

export function commandRows(
  query: string,
  commands: CommandPaletteCommand[],
  badge: string,
  max = 7,
): SearchResultRow[] {
  const needle = commandNeedle(query);
  const filtered = needle
    ? commands.filter((command) => fuzzyWordsMatch(needle, command.title))
    : commands;

  return filtered.slice(0, max).map((command) => ({
    key: `cmd:${command.id}`,
    title: command.title,
    badge,
  }));
}

export function runCommand(commands: CommandPaletteCommand[], key: string): void {
  const id = key.startsWith("cmd:") ? key.slice(4) : key;
  const command = commands.find((candidate) => candidate.id === id);
  if (command) {
    void command.run();
  }
}
