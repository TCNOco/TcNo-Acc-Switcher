# Investigation: Epic / Riot / EA Desktop switching issues (code & filesystem only)

User reports: "Some users are reporting issues with Epic Games, Riot Games, EA Desktop (mostly Epic Games)." Consistent across a wide variety of platforms. Limited info from users.

The platform definitions in `Platforms.json` are well-tuned (filepaths, registry paths, globs, etc.). This investigation looks only at **code behaviour and filesystem/process interaction** — not at the path strings themselves.

Verdict legend:
- ✅ **Real & important** — code evidence supports the claim, likely contributes to user reports
- ⚠️ **Real but minor** — confirmed in code, low impact or edge case
- 🟦 **Speculative** — plausible but no direct code evidence
- ❌ **Likely no impact / not actionable**

---

## Background: how the switch works (so the findings make sense)

1. User clicks an account → `BasicService.SwapToAccount(name, id, [])` → `SwapTo` (`internal/basic/flow.go:721`).
2. `SwapTo` does, in order:
   1. `killPlatformExes` — kill the running platform's exes.
   2. `saveCurrentAfterKill` — copy the **live** login files / registry to a per-account cache.
   3. `ClearCurrentLogin` — delete the live login files.
   4. `Login` — copy the **cached** files back to the live paths.
   5. `launchBasicNoStatus` — launch the platform exe.
3. Process kill (`internal/winutil/process_kill.go:39`):
   - **Combined** (default): `WM_CLOSE` to all top-level HWNDs of the matching image → wait up to 5 s → non-force `taskkill /T /IM` → force `taskkill /F /T /IM`.
   - **Close**: same as Combined with a 12 s graceful window.
   - **TaskKill**: `taskkill /F /T /IM` immediately.
   - **Electron** (built for Electron apps, e.g. Discord): launch the exe first to surface a foreground window, then `Alt+F4` per Electron/Chromium root HWND (looks for `Chrome_WidgetWin_1` class — line 15 of `internal/winutil/window_electron.go`; up to 12 focus attempts + 950 ms tray-recover pause), then non-force taskkill, wait up to 35 s, then force taskkill.
4. Default `ClosingMethod` is per-platform-settings; the platform's descriptor can override it via `Extras.ClosingMethod` + `Extras.ForceClosingMethod`. None of Epic, Riot, or EA Desktop override it (`Platforms.json`), so they all use `"Combined"` with a 5 s window.
5. File copy uses `internal/basic/flow_fileops.go`:
   - `copyFile`: `os.Open` → `os.Create` → `io.Copy`. No retry on sharing violation.
   - `copyDir`: `filepath.WalkDir` + `copyFile`. Returns on first error.
6. `leveldbLockedDirROSnapshot` (`internal/basic/leveldb_snapshot_copy_windows.go:20`) is a workaround that copies a LevelDB directory to temp with `FILE_SHARE_READ|WRITE|DELETE` so the original can stay locked. **It is only called from `openReadOnlyLevelDBWithTempCopyFallback`, which is only called from the LevelDB read path (`acquire` / `readValueFresh`).** It is **not** used by the file copy in `Login`/`SaveCurrent`.

---

## A. Process kill behaviour (affects Epic, Riot, EA)

### A-1. `Combined` waits only 5 s before falling back to force kill ⚠️

- `Combined` (`process_kill.go:60-66`) sends `WM_CLOSE` to all top-level HWNDs of the matching image, waits up to 5 s (`gracefulCombinedExitMaxWait = 5 * time.Second`, line 41), then non-force `taskkill /T /IM`, then force `taskkill /F /T /IM`.
- 5 s is a hard-coded constant. There is no per-platform override, no environment-based adjustment, and no telemetry to know if 5 s was insufficient.
- The longer-waiting `Close` method has 12 s, the `Electron` method has 35 s (`electronExitMaxWait = 35 * time.Second`, `process_windows.go:18`). 5 s is the shortest of all.
- **Real but no clear fix**: changing the constant could help, but a single global value cannot fit all platforms. The per-platform `ClosingMethod` selector exists for this reason; the fix is to either (a) add a new "Combined-Long" method with a longer wait, or (b) increase the default and let users override.
- **Plausible contributing cause** of "the launcher didn't fully release files in time" reports.

### A-2. `waitForImageExit` only tracks PIDs that existed at first call ⚠️

- `process_snapshot.go:159-202`: the first iteration enumerates PIDs matching the image and caches them in `cachedPIDs`. Subsequent iterations only wait for those PIDs. **Any child process spawned after the first enumeration is not waited for.**
- For Chromium-based apps (Electron or CEF): when the main process is sent `WM_CLOSE`, the GPU process may be respawned (e.g. due to a watcher), or a background service (e.g. `EABackgroundService.exe`) may respawn the main process, or a new helper is launched for crash reporting. These new PIDs bypass the wait.
- Real but minor in practice — usually the parent process exits and OS reaps the children.

### A-3. `windows.OpenProcess(SYNCHRONIZE, ...)` errors other than `ERROR_ACCESS_DENIED` are silently skipped ⚠️

- `process_snapshot.go:184-192`: the loop opens each PID with `SYNCHRONIZE` access to wait on it. The error handling is:
  ```go
  if err != nil {
      if err == windows.ERROR_ACCESS_DENIED {
          stillRunning = true
          remaining = append(remaining, pid)
      }
      continue  // ← silently drops ALL other errors
  }
  ```
- For other errors (e.g. `ERROR_INVALID_PARAMETER` for protected processes, or the PID has been recycled), the PID is **silently dropped from the wait list**. If the platform has a protected process (EAC, Vanguard), the wait returns "all clear" prematurely.
- Real but the protected-process case is the only one where it matters; protected anti-cheats are usually not listed in `ExesToEnd` anyway.

### A-4. `taskkill` exit code is not propagated; "not running" silently returns success 🟦

- `process_kill.go:178-198`: the only way `taskKillIM` returns an error is if the command fails AND the output doesn't contain "not running" / "could not find" / "not found". If the platform is in a "zombie" state where `taskkill` returns 0 (succeeded) but the process is actually still alive (some Windows edge cases with `STATUS_PROCESS_IS_TERMINATING`), the caller proceeds to `copyFile` on files the zombie is still holding open.
- Plausible but edge case.

### A-5. `taskkill` is invoked via `cmd.exe` (extra fork) and the output pipe has no timeout 🟦

- `process_kill.go:181-186`: `exec.Command("cmd.exe", "/C", "taskkill", ...)`. If `cmd.exe` itself hangs (rare but documented for shared Win stations), the kill call hangs and the swap stalls.
- The combined-kill path runs all kills in parallel goroutines, so other targets' kills proceed. But for a single-platform swap (only one exe to kill), the entire swap blocks on `cmd.exe`.
- Speculative — only matters in pathological cases.

### A-6. `Electron` method's "launch then Alt+F4" requires the platform to be installable & launchable ⚠️

- `flow_launch.go:35-58`: when the closing method is `Electron`, `electronBeforeKillSynth` is passed to `KillByName`. It launches the platform exe first, waits for its foreground window, then issues `Alt+F4`. This requires the platform exe to actually launch successfully.
- If the user has a corrupted install (missing DLL, etc.), the launch step fails or hangs, the foreground-wait times out (20 s), and the swap stalls. The non-Electron methods don't have this risk because they just taskkill the existing process.
- Real but only for users on `Electron` (Discord only in practice). Doesn't affect Epic/Riot/EA.

---

## B. File copy / restore behaviour

### B-1. `copyFile` / `copyDir` do NOT retry on sharing violation (the most common Epic/Riot/EA failure) ✅

- `internal/basic/flow_fileops.go:9-37`:
  ```go
  in, err := os.Open(src)
  if err != nil { return fmt.Errorf("open %s: %w", src, err) }
  ```
- No retry. No `isSkippableCopyErr` check (that helper exists in `internal/platform/user_data_move_windows.go:17-26` but is **only** used by the user-data-move flow, not by `Login`/`SaveCurrent`).
- This means: if any file in the source tree is held open by a process that hasn't fully released it, `copyFile` fails with `ERROR_SHARING_VIOLATION` (errno 32) or `ERROR_LOCK_VIOLATION` (33). For Chromium LevelDB files, the OS can take 1-3 s to actually release handles after `taskkill` returns. For EAC/EOS background services, even longer.
- The `WaitForImageExit` already waits up to 5 s (`Combined`) for the process to die. **But** the OS may release the process handle to the kernel before fully flushing file-system buffers and closing the underlying File handles — so a 5 s wait is not sufficient on slow disks or under heavy load.
- The LevelDB snapshot workaround (`leveldbLockedDirROSnapshot`) already exists and works around this **in the read path**. It is not wired into the copy path.
- **Plausible primary cause** of "switch fails silently / file not found / launcher is in a half-state" reports.

### B-2. `copyDir` returns on the first error; the rest of the tree is skipped ✅

- `flow_fileops.go:41-57`:
  ```go
  func copyDir(src, dst string) error {
      return filepath.WalkDir(src, func(path string, de os.DirEntry, err error) error {
          if err != nil {
              return fmt.Errorf("walk %s: %w", path, err)  // ← aborts entire copy
          }
          ...
      })
  }
  ```
- If any single file in the tree fails (sharing violation, permission, transient I/O), `WalkDir` returns the error and the entire copy aborts. The destination ends up with a **partial** tree.
- This affects **both** save and restore. On save: the cached snapshot is partial → on restore, the partial snapshot overwrites the live state with a partial state. On restore: live state is partial → launcher sees corrupt cookies/sessions.
- **Plausible primary cause** of "switch happens but the launcher is broken afterwards" reports.

### B-3. `copyFile` does not fsync the destination ✅

- `flow_fileops.go:9-37`:
  ```go
  out, err := os.Create(dst)
  ...
  if _, err = io.Copy(out, in); err != nil { ... }
  return nil
  ```
- The destination file is closed (via `defer out.Close()`) but not explicitly `Sync()`'d. On Windows, `os.File.Close()` does call `FlushFileBuffers` for files opened with `os.O_CREATE` and not `os.O_TRUNC` (Go wraps the HANDLE with default flags) — but **only on the local file system**. On network drives, removable media, or with certain AV filters, the data may not be on disk when `Close` returns.
- The next swap or the launcher's first read could see a zero-length file or stale data.
- Real but rare.

### B-4. `copyFile` opens destination with default flags (truncates, no `FILE_SHARE_READ`) ⚠️

- `flow_fileops.go:25`: `out, err := os.Create(dst)`. This translates to `OpenFile(dst, O_WRONLY|O_CREATE|O_TRUNC, 0o644)`. On Windows, the resulting HANDLE has no `FILE_SHARE_READ` share mode. If the destination file is being read concurrently by the platform process (e.g. it's mid-shutdown and still flushing), the `Create` call fails with sharing violation.
- Should be opened with `FILE_SHARE_READ` so concurrent readers don't conflict. This is a code-level bug that the same `leveldbLockedDirROSnapshot` already knows about (line 75-80 of `leveldb_snapshot_copy_windows.go` uses `CreateFile` with explicit share flags).

### B-5. `copyFile` does not handle `os.ErrNotExist` gracefully for globbed sources ⚠️

- `flow.go:251-260` (glob branch in `SaveCurrent`):
  ```go
  matches, _ := filepath.Glob(src)
  globDestRoot := globDestinationRoot(destRoot, cacheRel)
  if err := os.MkdirAll(globDestRoot, 0o755); err != nil { ... }
  for _, m := range matches {
      st, err := os.Stat(m)
      if err != nil { ... continue }
      ...
  }
  ```
- If `src` is a glob and matches a LevelDB directory but the directory was just deleted (e.g. by `ClearCurrentLogin` from a previous swap, or by the platform's uninstaller), the destination directory is created with `MkdirAll` but is empty. The error is silently logged. The restore step then copies this empty directory over the live one.
- Real but rare in normal usage.

### B-6. `globDestinationRoot` semantics is fragile when `cacheRel` contains glob characters ⚠️

- (Helper definition not in the read scope but inferred.) For each entry in `LoginFiles` whose `liveKey` contains a glob, the code derives a "root" by stripping the glob. If `cacheRel` is also a glob-shaped string, the root derivation is unstable.
- Real but mostly affects descriptor authoring; the shipped descriptors are fine.

### B-7. LoginFiles is iterated in random map order, so parent + child directories can be restored in the wrong order 🟦

- `flow.go` `Login` and `saveCurrentAfterKill` iterate `fc.Descriptor.LoginFiles` (a `map[string]string`). Go map iteration is **non-deterministic**; the runtime may shuffle keys differently on every call.
- The descriptors for Riot (and any other platform with both parent and child directories) will get processed in random order. If the child is processed first, the parent is then `copyDir`'d on top of it, which can clobber the child's files with the parent's contents. If the parent is first, the child is `copyDir`'d into a subdirectory of the parent.
- **Plausible cause** of "switch results are inconsistent across runs" reports. Mitigation would be to sort keys before iteration, or to enforce a separate "save order" / "restore order" list in the descriptor.

### B-8. The LevelDB-snapshot-with-locked-files workaround exists but is not used in the file copy path ✅

- `leveldb_snapshot_copy_windows.go:20-66` implements `leveldbLockedDirROSnapshot` that copies a locked LevelDB directory using `CreateFile` with `FILE_SHARE_READ|WRITE|DELETE`. This is the *correct* way to read files that the platform process still has open.
- It is only called from `openReadOnlyLevelDBWithTempCopyFallback` (`leveldb_resolver.go:153-176`), which is in turn only called by `readValueFresh` and `acquire` (the LevelDB-key read path).
- It is **not** called from `Login`/`saveCurrentAfterKill`. So when the switcher copies cookies/sessions LevelDB directories, it uses the standard `copyFile`/`copyDir` path that does not share-open the source — and fails on locked files.
- **Plausible primary cause** of "Riot/EA cookies are not switched" — the LevelDB copy fails silently or partially.

### B-9. `closeSharedLevelDBHandles` is called at the *start* of every flow, but the platform's own handles are not closed by it ✅

- `flow.go:108-109, 452-453, 724-725, 803-804`: each flow function calls `closeSharedLevelDBHandles("…begin")` at start. This closes the **switcher's own** cached LevelDB handles (the ones opened for reading `user.username` etc. via the `leveldb:` reference syntax).
- It does **not** close any handles the platform process is holding on its own data. Those are only released by killing the platform process.
- The point of mentioning this: the switcher assumes "I closed my handles, so I can copy now". It forgets that the platform process's handles are still alive and will block the copy.
- Real, but explaining the relationship: the switcher's only path to a clean copy is to (a) wait for the platform process to fully die, OR (b) use the locked-file snapshot workaround (B-8).

### B-10. The swap may skip saving the current session if the unique-ID read fails ✅

- `flow.go:746-757`:
  ```go
  ids, err := readIDs(platformKey)
  if err != nil { return err }
  if curErr == nil && strings.TrimSpace(cur) != "" {
      if prevName, ok := ids[cur]; ok { ... save ... }
  }
  ```
- If `curErr != nil` (e.g. the unique-ID file is missing, the regex failed, the LevelDB couldn't be opened), `cur` is the empty string and the `if` is skipped. **The previous account's session is never saved.** The user switches to a new account, and the previous account's session data on disk is now lost.
- This is a code bug: a failure to *read* the current unique ID should not be silently treated as "no previous account". The correct behaviour is either to fail the swap, or to ask the user.
- **Plausible cause** of "I switched to a new account and the old one is now corrupted / missing" reports. Particularly relevant for EA's `UniqueIdRegex =([^\r\n]*)` on `telemetry.ini` (see C-1) which can fail if the first `=value` line changes between launches.

### B-11. `wrapNeedsAdminIfPermission` only recognises `os.IsPermission` (access denied), not sharing violation ⚠️

- `flow_helpers.go:33-43`:
  ```go
  if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "access is denied") { ... }
  ```
- A sharing violation (errno 32) is reported as "The process cannot access the file because it is being used by another process." `os.IsPermission` does not match this (Go's `os.IsPermission` checks for `ERROR_ACCESS_DENIED` only). The error is not wrapped as a "needs admin" error, so the user sees a generic failure toast, not a clear "file in use" message.
- Real but UX-level, not a correctness issue.

### B-12. `RemoveAllWithRetry` is used in two specific places, not in the general `PathListToClear` path ⚠️

- `flow.go:157, 173` use `fsutil.RemoveAllWithRetry(..., 2*time.Second, os.RemoveAll)`.
- `ClearCurrentLogin` (`flow.go:684-689`) uses bare `os.RemoveAll`:
  ```go
  _ = os.RemoveAll(target)
  ```
- If a target of `PathListToClear` is a LevelDB directory with one locked file, the `RemoveAll` will fail to remove the locked file but will remove everything else. The directory will be left as a partial remnant. The subsequent `copyDir` (in `Login`) will then place the cached copy *into* this partial remnant, producing a merged (broken) live state.
- Real but only affects `PathListToClear` entries where the file is locked at clear time.

---

## C. Unique ID resolution (affects EA Desktop most, others secondary)

### C-1. EA Desktop's `UniqueIdRegex =([^\r\n]*)` on `telemetry.ini` returns an unstable identifier ✅

- `Platforms.json:332`: `"UniqueIdRegex": "=([^\\r\\n]*)"`.
- The code path: `uniqueFromFileRegex` (`internal/basic/unique_id.go:138-163`) reads the whole file, applies the regex, returns `m[1]` (the first capture group).
- The regex has no start anchor. `FindStringSubmatch` returns the first match anywhere in the file. The first `=` in `telemetry.ini` is whatever EA happened to write first, on whatever key. EA updates `telemetry.ini` on each app launch, and the order of keys within the file is not stable.
- The "unique ID" for the account is therefore unstable across launches. This causes:
  - "My EA account list keeps growing / duplicates appearing".
  - "I switched, the launcher still shows the old account" (because the cached account was looked up by the wrong UID).
  - "I have 5 EA accounts but I only own 2".
- **Plausible primary cause** of EA-specific reports. The fix is to use a stable source (e.g. `user.userid=` from `user_*.ini`, which the `ea_desktop.go` parser already extracts for profile image, but not for unique ID).

### C-2. `UniqueIdFile` for EA is `telemetry.ini` which is rewritten on every EA launch ⚠️

- This is the same root cause as C-1, but framed as "the file is volatile". `telemetry.ini` is updated by the EA app on every launch and periodically. A cached unique ID may not match the file on the next swap.
- Real, part of the EA issue.

### C-3. `ReadUniqueID` returns an empty string with no error when the file is missing ⚠️

- `unique_id.go:139-145`:
  ```go
  data, err := os.ReadFile(p)
  if err != nil { return "", fmt.Errorf("read unique id file %s: %w", p, err) }
  ```
- Actually this **does** return an error. (My initial concern was wrong — the code is fine here.)
- ❌ **Drop — not a real issue.**

### C-4. `UniqueIdMethod=CREATE_ID_FILE` writes a random 16-hex ID to a file in the platform's own directory 🟦

- For Riot: the file is at `%LocalAppData%\Riot Games\Riot Client\TcNoAccSwitcher-ID.instance` (Riot's own directory). If Riot's uninstaller or a user cleanup removes the directory, the file goes away. Next save creates a new one, and the previous account is orphaned (its cached files reference the old UID).
- Self-healing in the sense that the file is recreated, but the **previous account's cached data is now orphaned** because the saved IDs map has the new UID as the key.
- Speculative on impact; only affects users who nuke Riot's local app data.

### C-5. The "current session" detection (used to decide whether to save) is fragile ⚠️

- `flow.go:739-744`:
  ```go
  cur, curErr := ReadUniqueID(platformKey, d, folder)
  if curErr == nil && strings.TrimSpace(cur) != "" && ... { return nil }  // no-op if same UID
  ```
- If the unique ID read fails (C-1, C-2), the "no-op same account" early-return is bypassed and the swap proceeds as a fresh switch. This compounds with B-10: previous account not saved, switch proceeds anyway.

---

## D. Cross-cutting (all three platforms)

### D-1. None of Epic, Riot, or EA Desktop override `ClosingMethod` to `Electron` ⚠️

- `Platforms.json` lines 121, 174, 224 etc.: only Discord and its variants have `"ClosingMethod": "Electron"`. For all other platforms, the descriptor default is `"Combined"` (`internal/platform/platformsettings.go:121-141`).
- The `Electron` method is designed and named for **Electron** apps (UI label: `"Electron / Discord (recommended)"` in `frontend/src/lib/platformSettingsShared.ts:12`). Its Alt+F4 logic looks for `Chrome_WidgetWin_1` class HWNDs, which Electron apps use because Electron is built on Chromium. Whether the same logic works reliably for **non-Electron Chromium-based apps** (i.e. apps using CEF, which uses the same widget class) is not directly evidenced in code; the constants, class checks, and timing values were tuned for Electron/Discord.
- **Observation only — no fix recommendation**: pointing Epic/Riot/EA to `Electron` is speculative, not code-supported. If the underlying issue is the 5 s `Combined` window (A-1), the right fix is to lengthen the wait, not to switch methods.

### D-2. `taskkill /F` does not synchronously release file handles ⚠️

- `taskkill /F` with `/T` should kill the process tree, but the **OS file handle table** is not synchronously flushed. On Windows, a killed process's handles are released asynchronously; another process opening the same file may see `ERROR_SHARING_VIOLATION` for a brief window after the kill.
- This is OS-level, not switcher-level. The switcher's only mitigation is to wait or use the locked-file snapshot (B-8).
- Real, OS-implementation-detail.

### D-3. `waitForImageExit` only tracks PIDs at first call, not new ones (A-2 above, applies to all three) ⚠️

- Already covered.

### D-4. The platform's child processes (EAC, EOS, EABackgroundService) keep running after the main process is killed ✅

- `taskkill /T /IM <main>.exe` cascades **only** to descendants of the named process. If the child was launched by a *different* parent (e.g. `services.exe` → `EABackgroundService.exe` → `EADesktop.exe`), then `taskkill /T /IM EADesktop.exe` kills only `EADesktop.exe`'s descendants, not `EABackgroundService.exe`.
- The descriptors do list `EABackgroundService.exe`, `RiotClientUxRender.exe`, etc. in `ExesToEnd`, so they are killed by separate `taskkill /IM <name>` calls. But the kill is **racy**: `EABackgroundService.exe` may be in the process of *respawning* `EADesktop.exe` (it does this on user demand) and the respawned `EADesktop.exe` is not in any PID list.
- **Plausible cause** of "the platform came back to life" reports after a switch.

### D-5. No synchronization between killing the platform and starting the new one ✅

- `SwapTo` calls `killPlatformExes` (which waits for processes to die) → file operations → `launchBasicNoStatus`. There's no explicit "wait for the previous launch's residual state to settle" step.
- The file operations start immediately after `killPlatformExes` returns. If the OS hasn't fully released handles (D-2), the copy fails.
- The launch happens *after* the file operations, so on the launch side there's no race. The race is purely on the file side.
- **Plausible cause** of "the platform launches and immediately sees stale data".

### D-6. `launchBasicNoStatus` doesn't wait for the new platform to be ready before returning ⚠️

- `flow_launch.go:62-102`: the launch just calls `winutil.Start(exe, args, opts)`. For non-Electron closing methods, there's no `WaitForegroundForExe` call after the launch.
- This is fine for the switch flow (the user is back in the switcher UI). But for the next swap, the next `killPlatformExes` may run while the previous launch is still spawning.
- Real but the window is small.

### D-7. `electronBeforeKillSynth` only runs when `ClosingMethod == "Electron"` ⚠️

- `flow_launch.go:35-58`: the `beforeElectronSynth` callback is only registered when the closing method is `Electron`. For `Combined`, no such hook exists.
- If Epic/Riot/EA were on `Electron`, the launcher would be re-launched first, focused, then Alt+F4'd — which is how the Electron method reliably flushes Electron apps. On `Combined`, this re-launch step does not happen.
- Real observation, but does **not** imply Epic/Riot/EA should be switched to `Electron` — that method's behaviour was tuned for Electron, not for these other apps. If a "relaunch-then-graceful-close" behaviour is needed for them, it would need separate tuning.

---

## E. Things I considered but did NOT find strong evidence for

- **Antivirus locking the leveldb files** — possible but no code-side mitigation. Windows Defender can be configured per-folder; not in the switcher's scope.
- **Crash on swap** — would need stack traces from the user.
- **Network-based auth (token expiry)** — Riot and EA both rely on local cookies/tokens that expire. If the user switches and the token is expired, the platform re-prompts for login. Not a switcher bug per se.
- **Multiple Riot accounts via the same Riot account family** — not a switcher issue.

---

## F. Quick wins (concrete actions to test/apply)

These are ordered by leverage. None of them require touching the platform definitions in `Platforms.json` (paths are correct).

1. **(A-1) Lengthen the `Combined` graceful wait for slow-to-shut platforms** — `gracefulCombinedExitMaxWait = 5 * time.Second` in `process_kill.go:41` is too short for some launchers. A configurable per-platform override (or a new "Combined-Long" preset at, e.g., 12-15 s) would help. **No recommendation to switch these platforms to `Electron`** — that method is designed for Electron apps and may not behave the same way for CEF/non-Electron apps.
2. **(B-1, B-2, B-8) Wire `leveldbLockedDirROSnapshot` into the file copy path** — `copyDir` should attempt the locked-file snapshot for any directory containing `LOCK` or `*.ldb` files. The helper is already implemented; it's just not called.
3. **(B-1, B-2) Add retry-on-sharing-violation to `copyFile` and `copyDir`** — `os.IsPermission` / `errno == 32 || errno == 33` detection + bounded exponential backoff (the `RemoveAllWithRetry` helper is already in `internal/fsutil/remove.go`; copy the pattern).
4. **(B-4) Open destination files with `FILE_SHARE_READ` so concurrent readers don't conflict** — use the same `CreateFile` API as `leveldbLockedDirROSnapshot`.
5. **(B-7) Sort `LoginFiles` keys before iteration** — or accept an explicit `Order` field in the descriptor. The non-determinism is a real source of inconsistency.
6. **(B-10, C-1) Don't silently skip save on `curErr != nil`** — either fail the swap or fall back to a heuristic (e.g. read `user_*.ini` for the EA-specific path).
7. **(C-1, C-2) Use a stable unique ID source for EA** — e.g. `user.userid=` from `user_*.ini`, which the profile-image parser already extracts. Or use the file mtime of the most recently modified `user_*.ini` (less reliable but stable).
8. **(B-12) Use `RemoveAllWithRetry` in `ClearCurrentLogin`** — instead of bare `os.RemoveAll` on the `PathListToClear` targets.
9. **(B-11) Add sharing-violation detection to `wrapNeedsAdminIfPermission`** — give the user a clearer error message ("file in use" rather than generic failure).
10. **(B-3) `Sync` after `copyFile`** — explicit `out.Sync()` to ensure data is on disk before the next operation. Low cost.

---

# Verification pass

After writing the items above, I re-checked each against the source once more. Items below are the same findings with a final verdict.

- ⚠️ **A-1** `Combined` waits only 5 s — code confirmed (`process_kill.go:41`, `60-66`). **Real, low-to-medium impact.** Earlier draft called this a "real and important" fix-via-Electron; that conclusion is **dropped** — `Electron` method is designed for Electron apps, not a drop-in for any Chromium-based platform. The fix (if any) is to lengthen the wait.
- ⚠️ **D-1** None of Epic, Riot, or EA Desktop override `ClosingMethod` to `Electron` — code confirmed (`Platforms.json` shows only Discord and its variants use `Electron`; `platformsettings.go:121-141` shows the default is `Combined`). **Real observation only** — earlier draft recommended switching these platforms to `Electron`; **dropped** — `Electron` is named and tuned for Electron apps, not a generic Chromium/CEF fix.
- ⚠️ **A-2** `waitForImageExit` only tracks initial PIDs — code confirmed (`process_snapshot.go:159-202`). **Real but minor.**
- ⚠️ **A-3** `OpenProcess(SYNCHRONIZE)` error handling — code confirmed (`process_snapshot.go:184-192`). **Real but only matters for protected processes.**
- 🟦 **A-4** `taskkill` exit code semantic — speculative. Confirmed code returns nil on success but doesn't verify process actually died.
- 🟦 **A-5** `cmd.exe` extra fork and no timeout — speculative, edge case.
- ⚠️ **A-6** Electron method requires launchable exe — real, but only affects users on Electron (Discord only in practice).
- ✅ **B-1** `copyFile`/`copyDir` no retry on sharing violation — code confirmed (`flow_fileops.go:9-37, 41-57`). **Real and important.**
- ✅ **B-2** `copyDir` returns on first error — code confirmed (`flow_fileops.go:46-48`). **Real and important.**
- ✅ **B-3** `copyFile` no fsync — code confirmed. **Real.**
- ⚠️ **B-4** Default `os.Create` no `FILE_SHARE_READ` — code confirmed. **Real but specific to NTFS share modes.**
- ⚠️ **B-5** Glob source not handled gracefully — code confirmed. **Real but rare.**
- ⚠️ **B-6** `globDestinationRoot` fragility — speculative on impact, descriptor-specific.
- 🟦 **B-7** LoginFiles map iteration non-deterministic — code confirmed (`Go spec: map iteration order is undefined`). **Real and likely impactful for platforms with parent+child dirs.**
- ✅ **B-8** LevelDB snapshot not used in copy path — code confirmed (`leveldb_snapshot_copy_windows.go:20-66` is only called by `leveldb_resolver.go:153, 187, 222`, not by `flow.go`). **Real and important.**
- ✅ **B-9** `closeSharedLevelDBHandles` doesn't close platform's handles — code confirmed. **Real, framing finding.**
- ✅ **B-10** Swap may skip save on `curErr != nil` — code confirmed (`flow.go:746-757`). **Real and important.**
- ⚠️ **B-11** `wrapNeedsAdminIfPermission` doesn't catch sharing violation — code confirmed. **Real but UX-level.**
- ⚠️ **B-12** `RemoveAllWithRetry` not used in `ClearCurrentLogin` — code confirmed (`flow.go:684-689`). **Real.**
- ✅ **C-1** EA UniqueIdRegex captures arbitrary first `=value` — code confirmed (`unique_id.go:138-163`, regex `=([^\r\n]*)` has no anchor). **Real and important.**
- ⚠️ **C-2** EA `telemetry.ini` volatile — part of C-1, framing.
- ❌ **C-3** `ReadUniqueID` returns empty on missing file with no error — **drop**, code does return error correctly.
- 🟦 **C-4** Riot CREATE_ID_FILE in platform dir — speculative, edge case.
- ⚠️ **C-5** Unique-ID read failure bypasses same-account check — code confirmed (`flow.go:739-744`). **Real, related to B-10.**

### Final "real & important" list (sorted by impact)

1. **B-1, B-2, B-8**: File copy in `Login`/`SaveCurrent` does not use the existing locked-file snapshot workaround. Sharing violations cause partial copies. The workaround (`leveldbLockedDirROSnapshot`) is already implemented and working in the read path; it just isn't wired into the copy path.
2. **B-10**: A failure to read the current unique ID silently skips saving the previous account, and the swap proceeds, orphaning the previous account's cached data.
3. **C-1**: EA's unique-ID regex captures the first `=value` line in `telemetry.ini`, which is unstable. Causes duplicate / missing accounts.
4. **B-7**: Non-deterministic Go map iteration over `LoginFiles` makes the restore order non-deterministic. Real source of inconsistent behaviour.
5. **B-12**: `ClearCurrentLogin` uses bare `os.RemoveAll` instead of `RemoveAllWithRetry`, leaving partial directories on locked files. The subsequent `Login` then merges into the partial directory, producing a broken state.
6. **D-4**: The platform's child processes (EAC, EOS, EABackgroundService) are killed by separate `taskkill /IM` calls, racing with the main process kill. Respawned processes (e.g. `EABackgroundService` respawning `EADesktop`) can be missed.
7. **A-1** (low-medium impact): `Combined`'s 5 s wait may be insufficient. Not a "single-line config fix" — would need a configurable per-platform override or a new preset.

### Hallucinations / inaccuracies / no-impact items

- **C-3**: Wrong; `ReadUniqueID` does return an error for missing files.
- **B-6**: Speculative on impact, descriptor authoring issue, not a code bug per se.
- **A-4, A-5**: Edge cases without evidence of real-world impact.
- **A-6**: Doesn't affect Epic/Riot/EA (they use Combined, not Electron).
- **C-4**: Edge case, self-healing.
- **E (speculative items)**: kept in the file as "things I considered" but not actionable.
- **A-1, D-1, D-7, F.1, "real & important" #2 (earlier draft)**: Wrong framing. The `Electron` closing method is **designed and named for Electron apps** (UI label: "Electron / Discord (recommended)"). It is **not** a generic fix for CEF/Chromium platforms, and earlier drafts that recommended "defaulting Epic/Riot/EA to Electron" were conflating Electron with CEF because they share the `Chrome_WidgetWin_1` widget class. While the method's `Alt+F4` logic may *incidentally* work for some CEF apps, there is no code evidence that it was tested or tuned for them. Dropped the recommendation; A-1 reframed as a "5 s wait is short" observation with no easy fix.
