import * as BasicService from "../../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
import type { AccountRowProjection } from "../../components/PlatformAccountAdapter";
import { openAlertNoButton } from "../../stores/modal";
import { get } from "svelte/store";
import { t } from "../../stores/i18n";
import GameStatsSetupModalBody from "../../components/modals/GameStatsSetupModalBody.svelte";
import type { TagDefRow } from "../accountTagsContext";

export async function loadTagDefs(name: string): Promise<TagDefRow[]> {
  try {
    const rows = await BasicService.ListTagDefinitions(name);
    return (rows as { id: string; name: string; color: string }[]).map((r) => ({ id: r.id, name: r.name, color: r.color }));
  } catch { return []; }
}

export async function refreshGameStatsMarkup(
  name: string,
  acctIds: string[],
): Promise<Record<string, Record<string, Record<string, { statValue: string; indicatorMarkup: string }>>>> {
  if (!name.trim() || acctIds.length === 0) { return {}; }
  try {
    const pairs = await Promise.all(
      acctIds.map(async (uid) => {
        try { const m = await BasicService.GetUserStatsAllGamesMarkup(name, uid); return [uid, m ?? {}] as const; }
        catch { return [uid, {}] as const; }
      }),
    );
    return Object.fromEntries(pairs) as Record<string, Record<string, Record<string, { statValue: string; indicatorMarkup: string }>>>;
  } catch { return {}; }
}

export async function refreshGameStatsSupport(name: string): Promise<boolean> {
  try { const games = await BasicService.GetAvailableGames(name); return (games?.length ?? 0) > 0; }
  catch { return false; }
}

export function openGameStatsModal(
  rowId: string,
  rows: Pick<AccountRowProjection<unknown>, "name">,
  name: string,
  accountById: (id: string) => unknown | undefined,
  onApplied: () => void,
): void {
  const acc = accountById(rowId);
  void openAlertNoButton({
    title: get(t)("Context_ManageGameStats"),
    bodyComponent: GameStatsSetupModalBody,
    bodyProps: {
      platformKey: name,
      uniqueId: rowId,
      displayName: (acc ? rows.name(acc) : rowId).trim(),
      onApplied,
    },
  });
}
