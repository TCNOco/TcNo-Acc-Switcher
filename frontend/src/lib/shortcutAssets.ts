export type ShortcutAssetRow = {
  fileName?: unknown;
  FileName?: unknown;
  iconUrl?: unknown;
  IconURL?: unknown;
  iconURL?: unknown;
};

function offlineSafeAssetSrc(offline: boolean, url: string | null | undefined, fallback: string): string {
  const trimmed = (url ?? "").trim();
  if (!trimmed) return fallback;
  if (offline && /^https?:\/\//i.test(trimmed)) return fallback;
  return trimmed;
}

export function safeShortcutFolderName(platformKey: string): string {
  let out = "";
  for (const char of platformKey.trim().toLowerCase()) {
    if (char === " " || char === "/" || char === "\\") out += "_";
    else if (/[a-z0-9\-_]/.test(char)) out += char;
  }
  return out || "unknown";
}

export function normalizeGameSearchKey(value: string): string {
  return value
    .toLowerCase()
    .replace(/[™©®]/g, "")
    .replace(/[^a-z0-9]+/g, " ")
    .trim()
    .replace(/\s+/g, " ");
}

export function shortcutStem(fileName: string): string {
  return fileName.replace(/\.(lnk|url)$/i, "").trim();
}

export function shortcutIconUrl(
  row: ShortcutAssetRow,
  platformFolder: string,
  fallback: string,
  offline: boolean,
): string {
  const fileName = String(row.fileName ?? row.FileName ?? "").trim();
  const stem = shortcutStem(fileName).toLowerCase();
  let iconRaw = String(row.iconUrl ?? row.IconURL ?? row.iconURL ?? "").trim();
  if (!iconRaw && stem) iconRaw = `/img/shortcuts/${platformFolder}/${stem}.png`;
  return offlineSafeAssetSrc(offline, iconRaw, fallback);
}

export function shortcutIconIndexes(
  rows: unknown[],
  platformKey: string,
  fallback: string,
  offline: boolean,
): { byAppId: Record<string, string>; byStemKey: Record<string, string> } {
  const byAppId: Record<string, string> = {};
  const byStemKey: Record<string, string> = {};
  const platformFolder = safeShortcutFolderName(platformKey);
  for (const raw of rows) {
    const row = (raw ?? {}) as ShortcutAssetRow;
    const fileName = String(row.fileName ?? row.FileName ?? "").trim();
    if (!fileName) continue;
    const stem = shortcutStem(fileName);
    const iconUrl = shortcutIconUrl(row, platformFolder, fallback, offline);
    if (/^\d+$/.test(stem)) byAppId[stem] = iconUrl;
    const normalized = normalizeGameSearchKey(stem);
    if (normalized) byStemKey[normalized] = iconUrl;
  }
  return { byAppId, byStemKey };
}

export function steamGameIconUrl(
  game: { appId: string; name: string },
  platformKey: string,
  indexes: { byAppId: Record<string, string>; byStemKey: Record<string, string> },
  fallback: string,
  offline: boolean,
): string {
  const id = String(game.appId).trim();
  if (indexes.byAppId[id]) return indexes.byAppId[id];
  const normalized = normalizeGameSearchKey(game.name);
  if (indexes.byStemKey[normalized]) return indexes.byStemKey[normalized];
  return offlineSafeAssetSrc(
    offline,
    `/img/shortcuts/${safeShortcutFolderName(platformKey)}/${id.toLowerCase()}.png`,
    fallback,
  );
}
