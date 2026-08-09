import { get } from "svelte/store";
import { t } from "../stores/i18n";
import { openSearchOverlay } from "../stores/searchOverlay";
import { triggerPlatformSort } from "../stores/platformListSort";
import { steamPageTab } from "../stores/steamPageTab";
import type { MenuItemDef } from "../stores/contextMenu";

type Translate = (key: string, params?: Record<string, unknown>) => string;

/** Home sorts the platform grid, which has no last-used or username ordering. */
function homeSortChildren(tr: Translate): MenuItemDef[] {
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
 * Steam rows carry both a display name and an account name, so its alphabetical
 * entries stay clickable as-is and nest the by-username variant underneath.
 */
function alphaSortChildren(platformName: string, tr: Translate): MenuItemDef[] {
  if (platformName !== "Steam") {
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
 * Search and Sort By, shared by the action bar's Filter button and the right-click
 * menu on empty space in an account list. Pass an empty platform name for the home
 * platform grid, which sorts by name only. Steam's page swaps its whole body between
 * the switcher and the games list, so the tab store — not the caller — decides which
 * of the two the menu is being raised over.
 */
export function buildFilterMenuItems(platformName: string): MenuItemDef[] {
  const tr = get(t) as Translate;
  const onGamesTab = platformName === "Steam" && get(steamPageTab) === "games";
  const sortChildren: MenuItemDef[] = platformName === ""
    ? homeSortChildren(tr)
    : [
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
        ...(onGamesTab ? [ownedCountSortEntry(tr)] : []),
      ];

  return [
    {
      label: tr("Filter_Search"),
      action: () => openSearchOverlay(""),
    },
    {
      label: tr("Filter_SortBy"),
      children: sortChildren,
    },
  ];
}
