# Steam Guard threat model

Status: Accepted Phase 0 decision record. The inner vault and lease foundations exist in `internal/steamguard/vault`; protocol, outer wrapping, backup, and product UI work remain gated by this document.

## Assets

The protected asset is a Steam desktop-authenticator identity: shared secret, identity secret, revocation or recovery material, device identifiers, and any metadata needed to generate or approve Steam Guard challenges. The vault also protects its encrypted records, keyring, generation history, recovery wrapper, and lease state.

Web-session cookies and refresh tokens are separate assets. They can authorize account access without revealing the authenticator secret, but their loss still allows account actions. They must not be treated as ordinary account metadata.

The existing general vault is rooted by `internal/security/vault.go`; this threat model reserves a separate Steam Guard record format in `internal/steamguard/vault` rather than extending a platform backup archive in `internal/platform/backup.go`.

## Entry points and trust boundaries

| Entry point | Boundary | Required handling |
| --- | --- | --- |
| Steam Guard import, including SDA input | Untrusted file to authenticator record | Parse into bounded structures, verify the record authentication before use, and import legacy CBC only for migration. |
| Manual password entry | User intent to in-memory key material | Do not persist the vault password. Clear derived key material when the lease ends. |
| Login with QR | Visible Steam or `steamwebhelper` window, screenshot input, or selected monitor region to Go-only decoder | Scan verified visible Steam or `steamwebhelper` windows immediately; then accept a screenshot file or drop; then use native single-monitor region selection. Return only an opaque attempt handle after validation. |
| Local Steam sign-in form | Frontend form input to Go protocol adapter | Sign-in uses local forms and `internal/steamguard/protocol`. No remote Steam page is embedded in WebView2; unexpected remote navigation is blocked. |
| Backup import and restore | External archive to local vault generations | Authenticate the recovery wrapper before accepting an inner vault record, then use generation and rollback checks. |
| Plaintext SDA export | User-selected destination | Exclude web-session tokens by default and show that the export contains authenticator material. |

`internal/app/webview_cache.go` configures the Wails WebView2 data path and removes browser cache directories. It is not a Steam sign-in channel. The Steam Guard flow must block unexpected remote navigation and must not bridge vault keys, secrets, IPC methods, or arbitrary local files into WebView JavaScript.

## Actors

The account owner supplies passwords, imports, exports, recovery data, and Steam credentials. A local interactive attacker can read user-writable files, inspect the screen, read clipboard history, or invoke the app under the same Windows account. A malware attacker can alter vault or backup bytes, capture a selected QR region, or wait for an unlocked lease. A remote attacker controls phishing pages, modified QR payloads, and Steam responses on a compromised network path. Steam is a remote dependency whose private endpoints and formats can change without compatibility notice.

## Protected cases

The design protects stored authenticator secrets against an offline copy of the vault when the attacker lacks the Steam Guard vault password. When an app password exists, the outer AEAD uses a key from the app password or app master-key domain, never the inner Steam Guard password key, even if the ordinary saved-data encryption setting is disabled. Authenticated keyring entries, generation lineage, and the active pointer detect record substitution, truncation, and unauthenticated rollback within the retained local history.

Recovery backups remain self-contained: they carry the encrypted vault record and an authenticated recovery wrapper. Opening a backup requires the Steam Guard password. A double-layer backup also requires the app password; no third recovery secret exists.

## Unprotected cases and user-visible warnings

An unlocked process can generate codes or approve requests until its lease expires. Malware running as the same user during that lease, code execution inside the process, a compromised display, or an attacker who obtains the vault password are outside the confidentiality promise.

Desktop authenticator storage weakens factor separation. A computer that stores the Steam password, the Steam Guard vault password, and the decrypted authenticator in one user session is not equivalent to a separate physical authenticator. The UI must state this before enrollment and must not present local storage as independent multi-factor protection.

Screen capture, accessibility software, remote-desktop software, and the system clipboard can expose setup QR codes, generated codes, passwords, recovery strings, and browser content. The application can avoid copying secrets by default and clear its own transient views; it cannot prevent operating-system-level capture or clipboard history.

The local-form design does not remove host-process or OS-level risks. A hostile host process, screen capture, or Steam private-API change can still compromise a sign-in attempt. The app blocks unexpected remote navigation and never bridges vault secrets into WebView JavaScript.

## Backup and rollback boundary

Generation authentication detects a backup that claims to be a later generation without the required authenticated history. It cannot determine that an intact, older self-contained backup is stale if every local copy of newer authenticated state is lost. Restore therefore requires an explicit warning that restoring an older generation can reintroduce revoked or replaced authenticator material. Steam-side revocation remains authoritative after restore.

## Protocol dependency boundary

Steam does not publish a stable public protocol contract for all desktop-authenticator operations. The proposed adapter at `internal/steamguard/protocol` must isolate upstream request shapes, endpoints, and parsing. The candidate upstream is `github.com/lemon4ksan/g-man` `v0.13.0` at commit `48d2dc5c2942cc9ede5ba4cf189b1d898f6818e5`; admission requires the exact dependency gate in `docs/adr/steam-guard-vault-and-protocol.md` before code imports it. A failed or changed private endpoint is an expected operational failure, not evidence that local vault integrity has failed.
