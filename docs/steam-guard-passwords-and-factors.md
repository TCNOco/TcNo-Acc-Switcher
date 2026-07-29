# Steam Guard passwords and factors

This page describes what protects the Steam Guard vault, what each option costs, and what happens when something is lost. Read it before enrolling anything beyond a password.

## Password requirements

A Steam Guard password must be at least five characters. There are no other rules: no required digit, no required symbol, no required mixed case. Composition rules push people towards shorter passwords built from predictable patterns, which is the opposite of what protects a vault.

Length is what matters. A passphrase of several ordinary words is stronger than a short string with a digit and a symbol appended, and easier to remember.

The same minimum applies to a new app password. A password already in use is never re-checked against the current rules, so raising the minimum does not lock anyone out of a vault or an app they can already open. Changing that password requires the new one to meet the current minimum.

## What encrypts what

The vault holds a random key that encrypts the account records. That key never changes for the life of the vault. What changes is how it is wrapped.

Each enrolled way of opening the vault is a slot, and each slot holds its own wrapped copy of that key. Adding or removing a slot rewrites only the header. No account record is re-encrypted, and no code or secret changes.

Slots are alternatives. Any single slot opens the vault. Within one slot, every factor it lists is required together. That distinction decides what enrolling something actually means:

| Enrolment | Result |
|---|---|
| Keyfile added with no password of its own | Either the password or the keyfile opens the vault |
| Keyfile added with a password of its own | That keyfile needs its password; the vault password still opens the vault by itself |
| Password removed afterwards | Only the remaining factors open the vault |

Enrolling never takes anything away. Adding a keyfile or a security key adds a way in beside the ones you already have, and the password keeps working exactly as it did. Making a factor mandatory is a second, deliberate step: remove the password once the new factor is enrolled and proven.

Steam Guard settings lists the ways in under "Ways to open this vault". Read that list rather than assuming: entries listed separately are alternatives, not requirements. An entry that also needs the password says so.

## Factors

**Password.** Present unless you remove it. Typed from memory, so it is the factor a memory-hard derivation protects. A vault can have none - a security key alone is a legitimate setup - and "Add password" puts one back.

**Keyfile.** A file the app generates and you store somewhere else. It carries full entropy, so a keyfile makes offline guessing infeasible regardless of password strength. The app generates it rather than letting you nominate an existing file, because a file chosen from a photo library or a documents folder is one re-save away from being permanently lost.

**Backup key.** A code the app generates and shows once. It opens the vault on its own. Write it down and store it away from the PC.

**Security key.** A FIDO2 authenticator. The secret never leaves the device and cannot be copied off it. Several can be enrolled, each opening the vault on its own, which is how a spare kept elsewhere works. Give each one a name so they can be told apart. On Windows this goes through the platform's WebAuthn support; where that is unavailable the option is hidden with the reason.

## Backup keys

A backup key must exist before a keyfile or a security key can be enrolled, and cannot be removed while either is enrolled. Files are lost and devices are reset. Without a second way in, losing one would leave the vault permanently unreadable.

Issuing a backup key replaces any previous one. A key you believe you have revoked stops working, rather than remaining valid to anyone who saw it.

The backup key is its own slot, not a printed copy of the vault key. That is why it can be replaced at all: reissuing rewrites one header entry instead of re-encrypting every record.

## Key derivation

The vault password is stretched with Argon2id before it wraps anything. New vaults use 256 MiB of memory, three passes and four lanes. Existing vaults keep the parameters recorded in their own header and continue to open unchanged.

Factors that already carry full entropy, such as a keyfile or a backup key, are not stretched. There is nothing to guess, so there is nothing for a memory-hard function to slow down. A slot combining a password with a keyfile runs the memory-hard step once, for the password, and folds the keyfile in afterwards.

Verified backups derive at 512 MiB and four passes, roughly ten times the cost of the live vault. A backup is opened rarely and is the copy most likely to leave the machine, so it can afford a cost that would be irritating on every unlock. Restoring a backup returns it to the live cost; a restored vault does not keep paying the backup rate.

Parameters are read from the file being opened, within fixed bounds. A vault or backup declaring parameters outside those bounds is rejected rather than honoured.

## What cannot be recovered

TcNo Account Switcher cannot reset a Steam Guard password, reconstruct a keyfile, or reissue a backup key you no longer have. It cannot ask Steam to recreate the stored secrets.

If every enrolled factor for a vault is lost, that vault is unreadable. Keep Steam's own account-recovery information separately, so losing the vault does not also mean losing the Steam accounts inside it.

The app refuses to remove the last remaining way into a vault, and refuses to leave the backup key as the only one - a code on paper is not something you can open the app with. Deleting the data is a separate, explicit action.

Changing the password re-derives every way in that uses it. If one of those needs a keyfile or a security key you cannot present at the time, the change is refused rather than half-applied: otherwise the old password would keep opening the vault for whoever holds that factor.
