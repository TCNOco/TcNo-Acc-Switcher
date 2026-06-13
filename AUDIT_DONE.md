# TcNo-Acc-Switcher — Audit Items Completed

> Archive of items removed from `AUDIT_ACTION_PLAN.md` after being implemented.
> Each entry captures the original finding, the resolution, and care notes for future maintainers.

---

### 1.1 `crashlog.Capture` calls `os.Exit(1)` on any background panic
- **File:** `internal/crashlog/crashlog.go:67-92`, `main.go:110`
- **Impact:** A single panic in any background goroutine (profile refresh, image download, game-stats monitor, IPC accept loop) kills the entire app.
- **Risk:** Medium — must keep crash-dump writing and submission behavior intact; must not leave the user with a silently broken app.
- **Effort:** S
- **Resolution:** ✅ Done in `72defc73`.
  - `Capture()` became non-fatal: recover, write dump, emit a `toast` event so the user knows a background task failed, then return.
  - Added `CaptureFatal()` for the main goroutine only: recover, write dump, exit.
  - Changed `main.go` to use `CaptureFatal()`.
- **Care notes:**
  - Keep writing `CrashDump.json` and keep `SubmitPending` behavior.
  - Use `application.Get()` and emit the existing `toast` event; guard against `nil` (CLI/headless mode).
  - Keep the toast message actionable but not alarming: "A background task crashed (%v). Restart if the app behaves oddly."
  - Tests: `TestCapture_DoesNotExit` and `TestCaptureFatal_Exits`.

---

### 2.1 Frontend: Triple `loadAccounts` reload per mutation
- **File:** `frontend/src/components/PlatformAccountsBase.svelte:227-228`
- **Resolution:** ✅ Re-evaluated as **NO** — current code uses one immediate `loadAccounts()` for metadata mutations and one scheduled refresh for image-remove via the overlay. The only redundant path is the overlay remove button, which is not worth a dedicated fix.
- **Care notes:**
  - If image-remove via overlay is touched later, drop the duplicate `await loadAccounts()` and rely only on `scheduleAccountsRefresh()`.
  - Keep `scheduleAccountsRefresh()` for rapid-signal collapsing.

---

### 2.2 Frontend: `JSON.stringify` row-equality on every list refresh/patch
- **File:** `frontend/src/components/PlatformAccountsBase.svelte:188`, `frontend/src/lib/accounts/mergePipeline.ts:38`
- **Resolution:** ✅ Done. Added `visualKey(a)` to `PlatformAccountAdapter`; both basic and Steam adapters implement it. `accountRowEqual` in `mergePipeline.ts` and `PlatformAccountsBase.svelte` now compare `adapter.visualKey(a) === adapter.visualKey(b)`.
- **Care notes:**
  - Keep the key set in sync with the fields used by the row template. If a new adapter accessor is added to the template, add it to `visualKey` too.
  - `miniProfileHtml` is intentionally excluded from the Steam key (only used in the hover popover, not the list row).

---

### 2.3 Frontend: `accountMap` rebuilt on every reactive cycle
- **File:** `frontend/src/components/PlatformAccountsBase.svelte:89`
- **Resolution:** ✅ Done. `accountById` now uses `accountMap.get(id)` for O(1) lookup. The `$: accountMap = ...` reactive declaration still rebuilds when `accounts` is reassigned, but the in-place mutation from 2.6 means patch events no longer trigger a rebuild.
- **Care notes:**
  - Keep `accountMap` as the single source of truth for id→row lookups.

---

### 2.4 Frontend: `epochManager.shouldBumpEpoch` ignores `staticImageUrl` / `avatarFrameUrl`
- **File:** `frontend/src/lib/accounts/epochManager.ts:10-15`
- **Resolution:** ✅ Done. Added `staticImageUrl` and `avatarFrameUrl` to `EpochCheckRow` and to `shouldBumpEpoch`. Non-Steam rows leave these as `undefined`, so the `?? ""` fallback yields `"" === ""` and does not false-bump. `buildEpochMap` did not need changes because it operates on the same row shape.
- **Care notes:**
  - Keep the field list in sync with what `SteamAccountAvatar.svelte` actually renders. If a new image field is rendered there, add it to `EpochCheckRow` and `shouldBumpEpoch` too.
  - The check is by value; an empty-string → empty-string change is a no-op (correct).
  - Non-Steam adapters that gain a `staticImageUrl` or `avatarFrameUrl` field in the future will automatically participate — the comparison is opt-out by absence, not opt-in.

---

### 2.5 Frontend: `applyPatch` overwrites boolean fields with `undefined`
- **File:** `frontend/src/pages/PlatformSteam.svelte:216-227`, `frontend/src/pages/Platform.svelte:92-107`
- **Resolution:** ✅ Done in `1461feaa`. `PlatformSteam.svelte` `applyPatch` now uses `typeof` guards for booleans and `!= null` checks for `imageUrl`; `Platform.svelte` got the equivalent fix.
- **Care notes:**
  - Use `typeof p.x === "boolean" ? p.x : account.x` for booleans.
  - Use a sentinel/null check for `imageUrl` rather than `||`.
  - Add unit tests for partial patches if not already present.

---

### 2.6 Frontend: `applyPatchFromEvent` rebuilds entire `accounts` array for one-row change
- **File:** `frontend/src/components/PlatformAccountsBase.svelte:474-476`
- **Resolution:** ✅ Done in `1461feaa`. `applyPatchFromEvent` now mutates the single row in place and bumps a per-row `rowVersions[id]`; the row template is keyed on `${id}-${rowVersions[id]}` so only that row re-renders.
- **Care notes:**
  - Keep a per-row version map and key `{#each}` on `${id}-${epoch}`.
  - Only mutate the single row in place; Svelte's keyed each will re-render only that row.
  - Ensure selection state stays consistent.

---

### 2.8 Frontend: `buildAccountRows` runs full fuzzy search per keystroke
- **File:** `frontend/src/components/PlatformAccountsBase.svelte:131`
- **Resolution:** ✅ Done. Added `searchHayCache: Map<id, {v, text}>` keyed by `rowVersions[id]`. Hay is lowercased once at cache-fill time. `queryWords` is hoisted out of the filter loop and `fuzzyWordsMatch` is inlined as `words.every(w => text.includes(w))`. Cache is invalidated automatically when `applyPatchFromEvent` bumps `rowVersions`. Removed the now-unused `fuzzyWordsMatch` import from this file (still used by `PlatformSteam.svelte` game list and `Home.svelte` where the lists are small).
- **Care notes:**
  - Cache entries for accounts no longer in `accounts` are not eagerly pruned; the Map is bounded by the user's account count, which is small.
  - If a new searchable field is added to an adapter's `searchHay`, the cache for that row is stale until the next patch. Acceptable: clearing the cache on `onAfterLoad` or per-account swap would also work, but a tiny stale risk is cheaper than the rebuild cost.
  - Adapters (`Platform.svelte`, `PlatformSteam.svelte`) do not lowercase their `searchHay` output; lowercasing is centralized in the cache fill so the match loop never re-lowercases.

---

### 2.9 Backend: `LoadPlatformsJSON` deep-copies cached bytes on every call
- **File:** `internal/platform/platforms_json.go:42-95`
- **Resolution:** ✅ Done in `90fffe07`. The cached bytes are now shared across callers; the slice is treated as read-only.
- **Care notes:**
  - Return the cached slice; document that callers must not mutate.
  - Or wrap in a `bytes.Buffer`/reader if callers need a reader.
  - Audit all callers to ensure none mutate the slice.

---

### 2.16 Backend: `ResolveUserDataDir` does `os.Stat` on every settings read
- **File:** `internal/platform/user_data.go:43-58`
- **Resolution:** ✅ Done in `90fffe07`. Result is cached after `InitDataPaths` and only invalidated when the user explicitly changes the data dir.
- **Care notes:**
  - Cache the result after `InitDataPaths`.
  - Invalidate only when user explicitly changes data dir.

---

### 2.25 Backend: `Stats.SetStatsCollectionEnabled` never called at startup
- **File:** `internal/stats/gate.go:22-29`, `internal/platform/service.go:178-196`
- **Resolution:** ✅ Done. Added `stats.SetStatsCollectionEnabled(startupSettings.StatsEnabled)` in `main()` immediately after `loadStartupSettings` and `syncOfflineModeFromSettings`. The default in `settings.go` is `true`, so the cache is now always primed before any `stats.Increment*` call.
- **Care notes:**
  - If a future code path mutates `StatsEnabled` outside the platform setters, that path must also call `SetStatsCollectionEnabled` to keep the cache coherent.
  - The existing setters (`SetStatsEnabled`, `UpdateSettings` with `StatsEnabled`) already call this; no other call sites need updating.

---

*Archive last updated: 2026-06-13*
