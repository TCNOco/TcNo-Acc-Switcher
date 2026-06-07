# Releasing TcNo Account Switcher

## Versioning scheme

**Standard semver `MAJOR.MINOR.PATCH`** starting at `4.0.0`.

```
v4.0.0  →  v4.0.1  (patch)
v4.1.0  →  v4.1.1  (minor)
v5.0.0               (major)
```

Pre-release suffixes are supported when needed (e.g. `v4.0.0-beta.1`). The auto-updater treats pre-releases as older than the corresponding release.

## Single source of truth

The version lives in **one place**: the Git tag.

At build time the CI writes the tag (stripped of `v` prefix) into `build/config.yml` (`info.version`), which is then embedded into the binary via `//go:embed`. Everything downstream reads from `buildinfo.Version()`:

```
git tag v4.0.0
  │
  ├─► CI writes "4.0.0" into build/config.yml
  │     └─► go:embed → buildinfo.Version()
  │           ├─► Wails auto-updater (compares against GitHub Releases)
  │           └─► tcno.co API update check (banner notification)
  │
  ├─► CI writes "4.0.0.0" into build/windows/info.json
  │     └─► wails3 generate syso → EXE VERSIONINFO resource
  │
  └─► CI passes to NSIS: VERSION=4.0.0.0  DISPLAY_VERSION=4.0.0
```

## How to release

### 1. Ensure `changes.md` is up to date

### 2. Tag the release

```bash
git tag v4.0.0
git push origin v4.0.0
```

Tags MUST use a `v` prefix. This is required by both semver convention and the Wails GitHub Releases provider.

### 3. CI (AppVeyor) does the rest

AppVeyor triggers automatically on tags to `main` or `go` branches:

1. Strips `v` from the tag → `4.0.0`
2. Updates `build/config.yml` with the version
3. Builds the Go binary with `-tags production -trimpath -ldflags="-w -s -H windowsgui"`
4. Signs the `.exe` via SignPath
5. Creates a `.7z` archive containing the EXE + `Platforms.json` + WebView2 bootstrapper
6. Generates `SHA256SUMS` with SHA-256 hashes
7. Builds the NSIS installer
8. Publishes a draft GitHub Release

### 4. Review and publish the release

Go to https://github.com/TCNOco/TcNo-Acc-Switcher/releases, review, and publish.

## Auto-updater

The Wails v3 self-updater checks GitHub Releases via the standard semver provider:

1. **Background** — every 4 hours. If a newer semver tag exists, shows the update window.
2. **Manual** — clicking the update banner opens the update window directly.

The updater downloads the matching `.exe`, verifies SHA-256 against `SHA256SUMS`, swaps the binary atomically, and relaunches.
