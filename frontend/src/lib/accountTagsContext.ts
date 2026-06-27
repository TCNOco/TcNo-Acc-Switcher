import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
import { openContextMenu, type MenuItemDef } from "../stores/contextMenu";

export type TagFilterMode =
  | { kind: "all" }
  | { kind: "untagged" }
  | { kind: "tag"; id: string; name: string };

export type TagDefRow = { id: string; name: string; color: string };

export function textColorForTagBackground(hex: string): string {
  const m = /^#([0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) {
    return "#fff";
  }
  const n = parseInt(m[1], 16);
  const r = (n >> 16) & 255;
  const g = (n >> 8) & 255;
  const b = n & 255;
  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
  return luminance > 0.55 ? "#111" : "#fff";
}

export function buildTagsSectionMenuItem(opts: {
  platformKey: string;
  uniqueId: string;
  assignedTags: TagDefRow[];
  tagDefs: TagDefRow[];
  tr: (k: string, vars?: Record<string, string | number>) => string;
  afterChange: () => Promise<void> | void;
  onSuccess: () => void;
  onError: (e: unknown) => void;
}): MenuItemDef {
  const { platformKey, uniqueId, assignedTags, tagDefs, tr, afterChange, onSuccess, onError } = opts;
  const assignedIds = new Set(assignedTags.map((t) => t.id));
  const tagList = tagDefs;

  const wrap = async (fn: () => Promise<void>) => {
    try {
      await fn();
      await afterChange();
      onSuccess();
    } catch (e) {
      onError(e);
    }
  };

  const addChildren: MenuItemDef[] = [
    {
      type: "search",
      label: tr("Tags_SearchPlaceholder"),
      alwaysShowSearch: true,
      onSearchCanCreate: (q) => {
        const w = q.trim();
        if (!w) {
          return false;
        }
        return !tagList.some((d) => d.name.trim().toLowerCase() === w.toLowerCase());
      },
      onSearchCreate: (q) => {
        void wrap(async () => {
          const w = q.trim();
          if (!w) {
            return;
          }
          await BasicService.CreateTagAndAddToAccount(platformKey, uniqueId, w);
        });
      },
      onSearchEnter: (q) => {
        void wrap(async () => {
          const w = q.trim();
          if (!w) {
            return;
          }
          const allHit = tagList.find((d) => d.name.trim().toLowerCase() === w.toLowerCase());
          if (allHit) {
            if (assignedIds.has(allHit.id)) {
              return;
            }
            await BasicService.AddTagToAccount(platformKey, uniqueId, allHit.id);
          } else {
            await BasicService.CreateTagAndAddToAccount(platformKey, uniqueId, w);
          }
        });
      },
    },
    ...tagList
      .filter((d) => !assignedIds.has(d.id))
      .map(
        (d): MenuItemDef => ({
          label: d.name,
          action: () =>
            void wrap(async () => {
              await BasicService.AddTagToAccount(platformKey, uniqueId, d.id);
            }),
        }),
      ),
  ];

  const removeChildren: MenuItemDef[] =
    assignedTags.length === 0
      ? [{ label: tr("Tags_RemoveEmpty"), disabled: true }]
      : assignedTags.map(
          (tg): MenuItemDef => ({
            label: tg.name,
            action: () =>
              void wrap(async () => {
                await BasicService.RemoveTagFromAccount(platformKey, uniqueId, tg.id);
              }),
          }),
        );
  const removeAllTags =
    assignedTags.length === 0
      ? undefined
      : () =>
          void wrap(async () => {
            for (const tg of assignedTags) {
              await BasicService.RemoveTagFromAccount(platformKey, uniqueId, tg.id);
            }
          });

  return {
    label: tr("Tags_Section"),
    children: [
      { label: tr("Tags_Add"), children: addChildren },
      { label: tr("Tags_Remove"), action: removeAllTags, children: removeChildren },
    ],
  };
}

export function openTagFilterMenu(opts: {
  ev: MouseEvent;
  tagDefs: TagDefRow[];
  tr: (k: string, vars?: Record<string, string | number>) => string;
  onPick: (mode: TagFilterMode) => void;
}): void {
  const { ev, tagDefs, tr, onPick } = opts;
  ev.preventDefault();
  const t = ev.target;
  const iconEl =
    t instanceof Element ? (t.closest(".tag-filter-bar__icon") as HTMLElement | null) : null;
  let x = ev.clientX;
  let y = ev.clientY;
  if (iconEl) {
    const r = iconEl.getBoundingClientRect();
    x = r.right;
    y = r.bottom;
  }
  const items: MenuItemDef[] = [
    { label: tr("Tags_Filter_All"), action: () => onPick({ kind: "all" }) },
    { label: tr("Tags_Filter_Untagged"), action: () => onPick({ kind: "untagged" }) },
    ...tagDefs.map(
      (d): MenuItemDef => ({
        label: d.name,
        action: () => onPick({ kind: "tag", id: d.id, name: d.name }),
      }),
    ),
  ];
  openContextMenu(x, y, items);
}
