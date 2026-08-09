import { get } from "svelte/store";
import { t } from "../stores/i18n";
import { openSearchOverlay } from "../stores/searchOverlay";
import { focusSteamGamesSearch } from "../stores/steamGamesSearch";
import { triggerPlatformSort } from "../stores/platformListSort";
import { steamPageTab } from "../stores/steamPageTab";
import type { MenuItemDef } from "../stores/contextMenu";

type Translate = (key: string, params?: Record<string, unknown>) => string;

/** Name ordering with nothing nested under either direction. */
function alphaLeafEntries(tr: Translate): MenuItemDef[] {
  return [
    {
      label: tr("Filter_Sort_AZ"),
      action: () => triggerPlatformSort("alpha_asc"),
    },
    {
      label: tr("Filter_Sort_ZA"),
      action: () => triggerPlatformSort("alpha_desc"),
    },
  ];
}

/**
 * Steam accounts carry both a display name and an account name, so its alphabetical
 * entries stay clickable as-is and nest the by-username variant underneath.
 */
function alphaSortChildren(platformName: string, tr: Translate): MenuItemDef[] {
  if (platformName !== "Steam") {
    return alphaLeafEntries(tr);
  }
  return [
    {
      label: tr("Filter_Sort_AZ"),
      action: () => triggerPlatformSort("alpha_asc"),
      children: [
        {
          label: tr("Filter_Sort_AZ"),
          action: () => triggerPlatformSort("alpha_asc"),
        },
        {
          label: tr("Filter_Sort_Username"),
          action: () => triggerPlatformSort("steam_user_asc"),
        },
      ],
    },
    {
      label: tr("Filter_Sort_ZA"),
      action: () => triggerPlatformSort("alpha_desc"),
      children: [
        {
          label: tr("Filter_Sort_ZA"),
          action: () => triggerPlatformSort("alpha_desc"),
        },
        {
          label: tr("Filter_Sort_Username"),
          action: () => triggerPlatformSort("steam_user_desc"),
        },
      ],
    },
  ];
}

/** Only game rows carry an owner count, so this entry stays off the account lists. */
function ownedCountSortEntry(tr: Translate): MenuItemDef {
  return {
    label: tr("Filter_Sort_OwnedCount"),
    children: [
      {
        label: tr("Filter_Sort_Ascending"),
        action: () => triggerPlatformSort("owned_count_asc"),
      },
      {
        label: tr("Filter_Sort_Descending"),
        action: () => triggerPlatformSort("owned_count_desc"),
      },
    ],
  };
}

/**
 * A game row has a name and an owner count and nothing else: no account name to nest
 * under A-Z/Z-A, and no timestamp for last-used ordering to read. Offering either
 * would put entries in the menu that leave the list exactly as it was.
 */
function gamesSortChildren(tr: Translate): MenuItemDef[] {
  return [...alphaLeafEntries(tr), ownedCountSortEntry(tr)];
}

/**
 * Search and Sort By, shared by the action bar's Filter button and the right-click
 * menu on empty space in an account list. Pass an empty platform name for the home
 * platform grid, which sorts by name only. Steam's page swaps its whole body between
 * the switcher and the games list, so the tab store — not the caller — decides which
 * of the two the menu is being raised over; that keeps the action bar's one Filter
 * button correct on both tabs without it having to know which list is mounted.
 */
export function buildFilterMenuItems(platformName: string): MenuItemDef[] {
  const tr = get(t) as Translate;
  const onGamesTab = platformName === "Steam" && get(steamPageTab) === "games";
  let sortChildren: MenuItemDef[];
  if (platformName === "") {
    sortChildren = alphaLeafEntries(tr);
  } else if (onGamesTab) {
    sortChildren = gamesSortChildren(tr);
  } else {
    sortChildren = [
      ...alphaSortChildren(platformName, tr),
      {
        label: tr("Filter_Sort_LastUsed"),
        children: [
          {
            label: tr("Filter_Sort_NewOld"),
            action: () => triggerPlatformSort("lastused_new_old"),
          },
          {
            label: tr("Filter_Sort_OldNew"),
            action: () => triggerPlatformSort("lastused_old_new"),
          },
        ],
      },
    ];
  }

  return [
    {
      label: tr("Filter_Search"),
      // The games list filters in place through its own bar; the overlay is not
      // mounted on that tab, so this focuses the bar instead of opening nothing.
      action: () => {
        if (focusSteamGamesSearch()) return;
        openSearchOverlay("");
      },
    },
    {
      label: tr("Filter_SortBy"),
      children: sortChildren,
    },
  ];
}
