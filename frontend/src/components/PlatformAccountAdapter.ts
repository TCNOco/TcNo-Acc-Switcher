import type { MenuItemDef } from "../stores/contextMenu";
import type { PlatformSortKind } from "../stores/platformListSort";
import type { SearchResultRow } from "./SearchOverlay.svelte";
import type { TagDefRow } from "../lib/accountTagsContext";

export type GameStatMetricDTO = { statValue: string; indicatorMarkup: string };

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

export interface PlatformAccountAdapter<TAccount> {
  /** Stable key identifying the platform (e.g. "Steam", "Battle.net"). */
  platformKey: string;

  /** Default placeholder image when no profile image is available. */
  profileFallback: string;

  // ---- Field accessors ----
  id(a: TAccount): string;
  name(a: TAccount): string;
  imageUrl(a: TAccount): string | undefined;
  imagePending(a: TAccount): boolean;
  currentSession(a: TAccount): boolean;
  manualProfileImage(a: TAccount): boolean;
  tags(a: TAccount): TagDefRow[] | undefined;
  note(a: TAccount): string;
  shouldShowNote(a: TAccount): boolean;
  shouldShowLastUsed(a: TAccount): boolean;
  lastUsed(a: TAccount): string;
  accountLogin(a: TAccount): string;

  // ---- I/O ----
  /** Fast list: ids, names, order, current session. */
  loadAccountsList(): Promise<TAccount[]>;
  /** Slower per-account metadata merged into the list after first paint. */
  loadAccountsEnrichment(): Promise<TAccount[]>;
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

  // ---- Context menu ----
  /** Receive shared base items; return the full menu in the desired order. */
  buildMenu(account: TAccount, shared: SharedMenuItems): MenuItemDef[];

  // ---- Real-time update events ----
  /** Wails event name subscriptions for account updates (e.g. "steam-account-updated"). */
  updateEventName: string;
  /** Construct a typed patch object from the raw event payload. */
  buildPatch(raw: unknown): unknown;
  /** Apply the patch to a single account row, returning the updated row. */
  applyPatch(patch: unknown, account: TAccount): TAccount;
  /** Extract the account ID that a patch targets. */
  patchTargetId(patch: unknown): string;

  // ---- Search ----
  /** Build the searchable text blob for an account. */
  searchHay(account: TAccount, query: string): string;

  // ---- Lifecycle ----
  onAfterLoad?(
    accounts: TAccount[],
    ctx?: { hadCachedAccounts: boolean; enrichChanged: boolean },
  ): void | Promise<void>;
  onCleanup?(): void;

  // ---- Optional extensions ----
  /** Resolves true when a new account was saved (false on cancel or failure). */
  saveCurrent?(): Promise<boolean>;
  suggestedSaveName?(): Promise<string>;
  gameSearchRows?(query: string): SearchResultRow[];
  gameSearchHint?: string;
  loginAndLaunchGame?(accountId: string, appId: string): Promise<void>;
  extraSortKinds?: PlatformSortKind[];
}
