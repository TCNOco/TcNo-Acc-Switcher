import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
import { openContextMenu, type MenuItemDef } from "../stores/contextMenu";
import { openTagExpiryModal } from "../stores/modal";

export type TagFilterMode =
  | { kind: "all" }
  | { kind: "untagged" }
  | { kind: "tag"; id: string; name: string }
  // The inverse of "tag". Answering "which accounts are NOT on cooldown" needs a
  // negative filter; a positive one can only ever list the affected accounts.
  | { kind: "notTag"; id: string; name: string };

export type TagDefRow = {
  id: string;
  name: string;
  color: string;
  expiresAt?: string;
  specialID?: string;
};

export const SPECIAL_TAG_CS2_DROP_CLAIMED_ID = "cs2-drop-claimed";
export const SPECIAL_TAG_CS2_DROP_CLAIMED_NAME = "CS2 Drop Claimed";
export const SPECIAL_TAG_CS2_DROP_CLAIMED_BUBBLE_NAME = "Drop Claimed";

type BasicServiceWithTagExpiry = typeof BasicService & {
  RemoveTagFromAllAccounts?: (platformKey: string, tagID: string) => Promise<void>;
  SetTagExpiry?: (
    platformKey: string,
    uniqueID: string,
    tagID: string,
    scope: "account" | "all",
    expiresAt: string,
  ) => Promise<void>;
  ApplySpecialTag?: (platformKey: string, uniqueID: string, specialID: string) => Promise<void>;
};

const tagService = BasicService as BasicServiceWithTagExpiry;

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

export function accountTagBubbleLabel(name: string): string {
  return name.trim() === SPECIAL_TAG_CS2_DROP_CLAIMED_NAME ? SPECIAL_TAG_CS2_DROP_CLAIMED_BUBBLE_NAME : name;
}

/**
 * Tags the app manages itself. Matched by name because the Go tag DTO carries no
 * special id — the same duplication SPECIAL_TAG_CS2_DROP_CLAIMED_NAME already
 * lives with.
 */
export const MANAGED_TAG_CS2_COOLDOWN_NAME = "CS2 Cooldown";
export const MANAGED_TAG_CS2_PRIME_NAME = "CS2 Prime";
export const MANAGED_TAG_CS2_NON_PRIME_NAME = "CS2 Non-Prime";

const MANAGED_TAG_NAMES: ReadonlySet<string> = new Set([
  MANAGED_TAG_CS2_COOLDOWN_NAME,
  MANAGED_TAG_CS2_PRIME_NAME,
  MANAGED_TAG_CS2_NON_PRIME_NAME,
]);

/**
 * App-managed tags, which the user cannot remove by hand.
 *
 * Narrower than isSpecialTag: "CS2 Drop Claimed" is applied by the user and so
 * stays removable, while this one is applied and withdrawn by the cooldown
 * sweep. Removing it would only make it vanish until the next refresh, so it is
 * turned off in Steam settings instead - which clears it from every account.
 */
export function isManagedTag(tag: Pick<TagDefRow, "name">): boolean {
  return MANAGED_TAG_NAMES.has(tag.name.trim());
}

export function isSpecialTag(tag: Pick<TagDefRow, "name" | "specialID">): boolean {
  return (
    tag.specialID === SPECIAL_TAG_CS2_DROP_CLAIMED_ID ||
    tag.name.trim() === SPECIAL_TAG_CS2_DROP_CLAIMED_NAME ||
    // Kept out of the Modify submenu: the sweep reapplies these, so editing one
    // by hand would just be fighting the next refresh.
    isManagedTag(tag)
  );
}

export function tagExpiryMs(expiresAt: string | undefined): number | null {
  const raw = (expiresAt ?? "").trim();
  if (!raw) {
    return null;
  }
  const ms = Date.parse(raw);
  return Number.isFinite(ms) ? ms : null;
}

export function tagHasExpiry(tag: Pick<TagDefRow, "expiresAt">): boolean {
  return tagExpiryMs(tag.expiresAt) !== null;
}

export function formatTagExpiryCountdown(expiresAt: string | undefined, nowMs = Date.now()): string {
  const expiryMs = tagExpiryMs(expiresAt);
  if (expiryMs == null) {
    return "";
  }
  const totalSeconds = Math.ceil((expiryMs - nowMs) / 1000);
  if (totalSeconds <= 0) {
    return "";
  }
  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (days > 0) {
    return `${days}d ${hours}h ${minutes}m ${seconds}s`;
  }
  if (hours > 0) {
    return `${hours}h ${minutes}m ${seconds}s`;
  }
  if (minutes > 0) {
    return `${minutes}m ${seconds}s`;
  }
  return `${seconds}s`;
}

function requiredMethod<T>(fn: T | undefined, name: string): T {
  if (!fn) {
    throw new Error(`Missing binding: BasicService.${name}`);
  }
  return fn;
}

function buildDefaultExpiryValues(): { date: string; time: string } {
  const next = new Date(Date.now() + 60 * 60 * 1000);
  next.setSeconds(0, 0);
  return {
    date: `${next.getFullYear()}-${String(next.getMonth() + 1).padStart(2, "0")}-${String(next.getDate()).padStart(2, "0")}`,
    time: `${String(next.getHours()).padStart(2, "0")}:${String(next.getMinutes()).padStart(2, "0")}`,
  };
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
  // tagList stays complete: it backs the "does this name already exist" check,
  // and hiding a managed tag from that would let the user create a second tag
  // with the same name. Only the rows offered for adding are filtered.
  const tagList = tagDefs;
  const editableAssignedTags = assignedTags.filter((tg) => !isSpecialTag(tg));
  const defaultExpiry = buildDefaultExpiryValues();

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
    // Managed tags are never offered: the app applies and withdraws them on its
    // own, so adding one by hand would only last until the next sweep. The
    // Steam settings checkbox is their only control.
    ...tagList
      .filter((d) => !assignedIds.has(d.id) && !isManagedTag(d))
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

  // Managed tags are not offered for removal, here or under Remove all - the
  // sweep would reapply them. Special tags the user applied themselves stay
  // removable; only the app-managed ones are withheld.
  const removableTags = assignedTags.filter((tg) => !isManagedTag(tg));
  const removeChildren: MenuItemDef[] = removableTags.map(
    (tg): MenuItemDef => ({
      label: tg.name,
      action: () =>
        void wrap(async () => {
          await BasicService.RemoveTagFromAccount(platformKey, uniqueId, tg.id);
        }),
    }),
  );
  const removeAllTags =
    removableTags.length === 0
      ? undefined
      : () =>
          void wrap(async () => {
            for (const tg of removableTags) {
              await BasicService.RemoveTagFromAccount(platformKey, uniqueId, tg.id);
            }
          });

  const modifyChildren: MenuItemDef[] = editableAssignedTags.map(
    (tg): MenuItemDef => ({
      label: tg.name,
      children: [
        {
          label: tr("Tags_AddExpiry"),
          action: async () => {
            const result = await openTagExpiryModal({
              title: tr("Tags_AddExpiry"),
              tagName: tg.name,
              initialScope: "account",
              initialDate: defaultExpiry.date,
              initialTime: defaultExpiry.time,
              positiveLabel: tr("Tags_SaveExpiry"),
              negativeLabel: tr("Button_Cancel"),
            });
            if (!result) {
              return;
            }
            void wrap(async () => {
              await requiredMethod(tagService.SetTagExpiry, "SetTagExpiry")(
                platformKey,
                uniqueId,
                tg.id,
                result.scope,
                result.expiresAt,
              );
            });
          },
        },
        {
          label: tr("Tags_RemoveAll"),
          action: () =>
            void wrap(async () => {
              await requiredMethod(tagService.RemoveTagFromAllAccounts, "RemoveTagFromAllAccounts")(
                platformKey,
                tg.id,
              );
            }),
        },
      ],
    }),
  );

  const specialChildren: MenuItemDef[] = [
    {
      label: SPECIAL_TAG_CS2_DROP_CLAIMED_NAME,
      action: () =>
        void wrap(async () => {
          await requiredMethod(tagService.ApplySpecialTag, "ApplySpecialTag")(
            platformKey,
            uniqueId,
            SPECIAL_TAG_CS2_DROP_CLAIMED_ID,
          );
        }),
    },
  ];

  return {
    label: tr("Tags_Section"),
    children: [
      { label: tr("Tags_Add"), children: addChildren },
      { label: tr("Tags_AddSpecial"), children: specialChildren },
      // Nothing assigned means nothing to modify or remove — hide rather than show dead entries.
      ...(modifyChildren.length > 0
        ? [{ label: tr("Tags_Modify"), children: modifyChildren }]
        : []),
      ...(removeChildren.length > 0
        ? [{ label: tr("Tags_Remove"), action: removeAllTags, children: removeChildren }]
        : []),
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
    // Grouped under one submenu rather than doubling the flat list: with several
    // tags the inverses would otherwise bury the ordinary ones.
    ...(tagDefs.length > 0
      ? [
          {
            label: tr("Tags_Filter_Without"),
            children: tagDefs.map(
              (d): MenuItemDef => ({
                label: d.name,
                action: () => onPick({ kind: "notTag", id: d.id, name: d.name }),
              }),
            ),
          } satisfies MenuItemDef,
        ]
      : []),
  ];
  openContextMenu(x, y, items);
}
