import { get } from "svelte/store";
import { t } from "../stores/i18n";
import { openSearchOverlay } from "../stores/searchOverlay";
import { triggerPlatformSort } from "../stores/platformListSort";
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

/**
 * Search and Sort By, shared by the action bar's Filter button and the right-click
 * menu on empty space in an account list. Pass an empty platform name for the home
 * platform grid, which sorts by name only.
 */
export function buildFilterMenuItems(platformName: string): MenuItemDef[] {
  const tr = get(t) as Translate;
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
