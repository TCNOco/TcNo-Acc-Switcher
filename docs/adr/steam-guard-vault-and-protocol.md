# ADR: Steam Guard vault and protocol

Status: Accepted for Phase 0. Scope: Steam Guard work adjacent to the existing general vault in `internal/security/vault.go`. The inner vault, immutable-write, lease, OTP, and maFile codec foundations are implemented; the outer wrapper, protocol adapter, backup format, QR decoder, and product UI remain gated work.

## Decisions

### Vault passwords and encryption layers

Every Steam Guard vault requires a Steam Guard vault password. The inner vault record always uses independent AEAD encryption. This inner layer does not depend on the application's ordinary saved-data encryption toggle.

When saved-account encryption is enabled, the stored vault record receives a second, outer AEAD layer using a key from the app master-key domain, never from the inner Steam Guard password key. An app password with saved-account encryption disabled remains an access gate and does not add the outer storage layer. Key derivation inputs, AEAD nonces, algorithm identifiers, and record format version are authenticated in `internal/steamguard/vault`.

Decision SG-CRYPTO-BASELINE selects Argon2id with 64 MiB memory, 3 passes, 4 lanes, a 16-byte salt, and a 32-byte result. It selects AES-256-GCM for the inner and outer AEAD layers. The record header authenticates the Argon2id parameters, salt, AEAD nonce, algorithm identifiers, layer presence, and format version.

The vault unlock lease is fixed at five minutes by Decision SG-LEASE-5M. A process-session lease is optional and must require a separate explicit setting. Both lease forms hold only the minimum in-memory key material needed to access the active record; closing the process clears the process-session lease.

### Keyring, generations, backup, and rollback

The vault uses an authenticated keyring, authenticated generation lineage, and an authenticated active pointer. A keyring entry binds the active key and generation metadata to the vault identity. A generation record uses a random identifier and binds its predecessor reference or root marker into authenticated data. The implementation target is `internal/steamguard/vault`; it must not be folded into `internal/platform/backup.go`.

Backup is self-contained. The backup package carries encrypted vault material plus an authenticated recovery wrapper. Opening a backup requires the Steam Guard password. Opening a double-layer backup also requires the app password, from the app password or app master-key domain. Restore validates the recovery wrapper, record authentication, generation lineage, and active pointer before replacing local state. It warns when the selected backup generation is older than retained local state. The threat-model rollback boundary remains: an intact old backup cannot be identified as stale after loss of every newer local generation.

### Import and export compatibility

Plaintext SDA export excludes web-session tokens by default. An export that includes such tokens, if later approved, needs a separate opt-in and a field-level warning. Legacy CBC input is import-only for migrating existing SDA data. No new Steam Guard record, backup wrapper, or export format may use CBC.

On Windows, the vault and its temporary, generation, and recovery-wrapper paths require an owner-restrictive ACL. If the ACL cannot be created or verified, creation, import, restore, and write operations fail closed. The permission modes in `internal/security/vault.go` do not replace this Windows ACL rule.

### Protocol adapter and dependency admission

The adapter is pure Go and has no UI or WebView dependency. The proposed interface boundary is `internal/steamguard/protocol`, with provider-specific code below that package. Local sign-in forms call this adapter. It accepts validated values from import and sign-in flows, and QR attempt handles from the Go-only QR flow. It returns typed protocol errors and does not expose raw requests to the frontend. No remote Steam page is embedded in WebView2; unexpected remote navigation is blocked.

The current candidate is `github.com/lemon4ksan/g-man` `v0.13.0` at commit `48d2dc5c2942cc9ede5ba4cf189b1d898f6818e5`, with BSD-3-Clause as the candidate license, behind that interface. The candidate is not admitted until all of the following gate items are checked in the dependency change:

- [ ] `github.com/lemon4ksan/g-man` `v0.13.0` resolves to commit `48d2dc5c2942cc9ede5ba4cf189b1d898f6818e5`, and BSD-3-Clause is verified against the upstream source.
- [ ] The exact package import path and exported API compile in a minimal Windows-targeted package.
- [ ] A source review confirms that required Steam authenticator operations do not shell out, start a service, or require browser automation.
- [ ] The module's transitive dependencies, update behavior, and security advisories are recorded with the dependency change.
- [ ] Adapter contract tests cover malformed input and a private-API change response without importing the candidate outside `internal/steamguard/protocol`.

Steam private API behavior is unstable. The adapter owns endpoint changes and response-shape changes. The vault format must remain readable when an adapter provider is replaced.

### QR intake

Clicking Login with QR immediately scans visible, verified Steam or `steamwebhelper` windows. If that scan does not produce a valid result, the flow accepts a screenshot file or drop. If that also fails, it opens native single-monitor region selection. The QR decoder is Go-only. It validates the decoded payload and returns an opaque attempt handle to the frontend; neither the payload nor vault keys cross the frontend boundary.

## App and Steam Guard password policy

Status: Accepted 2026-07-23.

New and changed app and Steam Guard passwords accept any non-empty, valid UTF-8 value. The app does not impose minimum length, maximum length, common-password, character-class, or composition rules. Paste and password-manager input remain supported. Password fields use the appropriate `current-password` or `new-password` autocomplete hint and do not block paste or inspect password clipboard contents.

The KDF receives the exact password. Whitespace is significant, and the app performs no Unicode normalization. The backend rejects only empty or invalid UTF-8 values; the frontend mirrors the non-empty check.

The policy applies to setup and change operations. Existing app and Steam Guard passwords retain exact unlock compatibility. When an operation accepts both passwords for a double-layer vault, they must not be identical. The KDFs use independent salts and domain-separated wrappers.

Plaintext maFile export excludes access, refresh, and other bearer-session tokens by default. Token inclusion requires an explicit export option after fresh authentication and a field-level warning. The first release does not emit legacy SDA CBC output.

Security-sensitive storage fails closed when the destination cannot enforce the platform's owner-only protection. The app does not silently fall back to weaker permissions or stage plaintext in a temporary directory. A read-only import source may be inspected, but the native vault or export destination must be secured before committing secret data. A platform without an enforceable equivalent to the Windows protected owner/SYSTEM DACL or POSIX owner-only mode reports the operation as unsupported.
