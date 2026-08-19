import type { MenuItemDef } from "../stores/contextMenu";
import type { PlatformSortKind } from "../stores/platformListSort";
import type { SearchResultRow } from "./SearchOverlay.svelte";
import type { TagDefRow } from "../lib/accountTagsContext";

export type GameStatMetricDTO = {
  statValue: string;
  indicatorMarkup: string;
  /** Raw `Tooltip` from the game definition — plain text or `i18n:<Key>`. */
  tooltip?: string;
};

/**
 * Ban state painted onto the account name. `label` is already localised — it is
 * the tooltip and the screen-reader text, since the colour alone carries no
 * meaning for anyone who cannot see it.
 */
export type AccountNameStatus = { kind: "vac" | "limited"; label: string };

export interface SharedMenuItems {
  swapTo: MenuItemDef;
  changeName: MenuItemDef;
  createShortcut: MenuItemDef;
  changeImage: MenuItemDef;
  forget: MenuItemDef;
  notes: MenuItemDef;
  tags: MenuItemDef;
  gameStats: MenuItemDef | null;
}

export interface AccountRowProjection<TAccount> {
  id(a: TAccount): string;
  name(a: TAccount): string;
  imageUrl(a: TAccount): string | undefined;
  imagePending(a: TAccount): boolean;
  currentSession(a: TAccount): boolean;
  manualProfileImage(a: TAccount): boolean;
  savedDataBroken?(a: TAccount): boolean;
  /**
   * The Forget confirmation body, for a platform where forgetting an account
   * does more than the shared prompt describes. Returning nothing - which is
   * every platform, for every account that has nothing extra to say - leaves the
   * shared prompt in place.
   */
  forgetPrompt?(a: TAccount): string | undefined;
  nameStatus?(a: TAccount): AccountNameStatus | null;
  /**
   * Extra spoken status lines for things only the platform knows about. A
   * render-time snapshot, not a live region: rewriting the description every
   * second would make a screen reader unusable.
   */
  extraA11yStatus?(a: TAccount): string[];
  tags(a: TAccount): TagDefRow[] | undefined;
  note(a: TAccount): string;
  shouldShowNote(a: TAccount): boolean;
  shouldShowLastUsed(a: TAccount): boolean;
  lastUsed(a: TAccount): string;
  accountLogin(a: TAccount): string;
  visualKey(a: TAccount): string;
}

export interface AccountDataSource<TAccount> {
  loadAccountsList(): Promise<TAccount[]>;
  loadAccountsEnrichment(): Promise<TAccount[]>;
}

export interface AccountCommands {
  swapTo(id: string): Promise<void>;
  saveOrder(ids: string[]): Promise<void>;
  addNew(): Promise<void>;
  forget(id: string): Promise<void>;
  rename(id: string, name: string): Promise<void>;
  changeImage(id: string, path: string): Promise<void>;
  clearManualImage(id: string): Promise<void>;
  getNote(id: string): Promise<string>;
  setNote(id: string, note: string): Promise<void>;
  launch(): Promise<void>;
  /** Ends the platform, the way switching accounts does. */
  closePlatform(): Promise<void>;
}

export interface AccountPatchStream<TAccount> {
  updateEventName: string;
  /**
   * Additional per-account update events, each with its own payload shape.
   *
   * Separate from updateEventName because buildPatch funnels through one DTO:
   * a payload that is not that DTO would come out with every other field at its
   * zero value and the merge would apply them, blanking what it did not carry.
   */
  extraUpdateEvents?: {
    name: string;
    targetId(raw: unknown): string;
    apply(raw: unknown, account: TAccount): TAccount;
  }[];
  buildPatch(raw: unknown): unknown;
  applyPatch(patch: unknown, account: TAccount): TAccount;
  patchTargetId(patch: unknown): string;
}

export interface AccountSearchProjection<TAccount> {
  searchHay(account: TAccount, query: string): string;
}

export interface AccountLifecycle<TAccount> {
  onAfterLoad?(
    accounts: TAccount[],
    ctx?: { hadCachedAccounts: boolean; enrichChanged: boolean },
  ): void | Promise<void>;
  onCleanup?(): void;
}

export interface AccountPageExtensions {
  saveCurrent?(): Promise<boolean>;
  suggestedSaveName?(): Promise<string>;
  refreshOnWindowFocus?: boolean;
  refreshAccounts?(): Promise<void>;
  refreshAllProfileImages?(): Promise<void>;
  gameSearchRows?(query: string): SearchResultRow[];
  gameSearchHint?: string;
  loginAndLaunchGame?(accountId: string, appId: string): Promise<void>;
  extraSortKinds?: PlatformSortKind[];
}

export interface AccountMenuBuilder<TAccount> {
  buildMenu(account: TAccount, shared: SharedMenuItems): MenuItemDef[];
  /**
   * Right-click on empty space in the list, which belongs to no account. Return
   * nothing and no menu opens, which is the default for platforms that have
   * nothing to offer there.
   */
  buildBackgroundMenu?(): MenuItemDef[];
}

export interface PlatformAccountAdapter<TAccount>
  extends AccountRowProjection<TAccount>,
    AccountDataSource<TAccount>,
    AccountCommands,
    AccountPatchStream<TAccount>,
    AccountSearchProjection<TAccount>,
    AccountLifecycle<TAccount>,
    AccountPageExtensions,
    AccountMenuBuilder<TAccount> {
  /** Stable key identifying the platform (e.g. "Steam", "Battle.net"). */
  platformKey: string;

  /** Default placeholder image when no profile image is available. */
  profileFallback: string;
}
