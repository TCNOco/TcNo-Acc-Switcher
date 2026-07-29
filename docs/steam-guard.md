# Steam Guard

TcNo Account Switcher can store Steam mobile-authenticator data, generate login codes, approve or reject confirmations, and approve Steam login QR challenges. This places phone-equivalent credentials on the PC. Read the backup and security sections before adding an authenticator.

## Back up the complete folder

Keep a verified backup of the complete Steam Guard folder. The app displays its full path during import and setup and in Steam Guard settings. Record that path. A copied `.maFile` or one vault file is not a complete backup.

Use **Create Verified Backup** while the app is running. It copies one consistent vault generation and verifies the copy before reporting success. For a manual copy, lock Steam Guard or close TcNo Account Switcher first. Copying the live folder while a record or encryption setting changes can combine files from different generations.

Store a backup away from the PC. A disk failure, Windows reinstall, or stolen computer can remove both the working vault and a backup kept beside it. Keep Steam recovery codes and Steam's own account-recovery information separately.

A backup remains encrypted. It opens with whatever factors the vault had enrolled when the backup was made, and with nothing else. Losing every enrolled factor makes the copy unreadable. If saved-account encryption supplies an outer layer, losing the app password also makes that copy unreadable. TcNo Account Switcher cannot reset a password, reconstruct a keyfile, reissue a backup key, or ask Steam to recreate the stored secrets.

A backup derives its key more slowly than the live vault, so creating and restoring one takes noticeably longer than an ordinary unlock. Restoring returns the vault to the live cost.

## Password and encryption layers

Every Steam Guard vault has its own password and encryption layer. It is independent of the optional app-wide password.

| App configuration | Steam Guard storage |
|---|---|
| No app password | Encrypted with the Steam Guard password |
| App password set; saved-account encryption off | Encrypted with the Steam Guard password; the app password is an access gate, not a second storage layer |
| App password set; saved-account encryption on | Encrypted once with the Steam Guard password and again with the app encryption layer |

The two password prompts are managed separately. Setting or changing one does not silently replace the other.

A Steam Guard password must be at least five characters, with no other requirements. A vault can also have a keyfile or a backup key enrolled alongside or instead of its password. See [Steam Guard passwords and factors](steam-guard-passwords-and-factors.md) for what each option protects and what cannot be recovered.

With **Remember Steam Guard password for session** off, a successful unlock lasts five minutes and then locks again. Selecting the session option in settings or the unlock dialog keeps the vault unlocked until the app exits, the user locks it, or the protected session is revoked. The remembered key stays in process memory; it is not written as a plaintext password.

## Add or import an authenticator

Open a Steam account's context menu, then choose **2-Factor** or **Steam Guard**. You can add a new authenticator through Steam or import an existing SDA maFile.

Import accepts:

- A plaintext Steam Desktop Authenticator `.maFile`.
- SDA's legacy encrypted maFile format when its matching `manifest.json` entry and legacy password are available.
- A dropped `.maFile`; the SteamID inside the file selects the matching Steam account.

The source file is parsed with bounded size and schema checks, copied into the encrypted vault, and is not needed for normal use after import. Do not delete the source until **Create Verified Backup** has produced and verified a separate backup.

An unfinished authenticator enrollment is stored as encrypted pending state. It is not treated as an active maFile. Finish it from the same account or close the dialog and resume later. Do not delete pending state unless Steam has safely removed the remote authenticator. Record the revocation code when the app reveals it; each protected dialog capability can reveal it once, and setup requires you to type it back before continuing. If the app closes before acknowledgment, unlock the vault and reveal it again.

## Login QR capture

The login dialog offers three capture methods:

- **Steam window** scans visible Steam login windows on the PC.
- **Drop screenshot** decodes an image supplied by the user.
- **Select region** captures the rectangle drawn on screen after the selection overlay hides.

Image pixels are decoded in memory and cleared after the attempt. They are not added to the Steam Guard vault or its backups. A screenshot saved by another program remains that program's responsibility; delete it when it is no longer needed.

Only Steam login QR URLs are accepted. A screenshot with multiple different Steam login codes is rejected as ambiguous. Automatic window capture does not cover minimized windows, QR codes rendered outside a visible Steam login window, or images that are too small, blurred, partly hidden, or expired. Use a dropped screenshot or region selection in those cases.

A Steam login QR approves the device or browser that displayed it. Capture QR codes only from a login you started and can see. Do not approve a code sent through chat, email, or a support request.

## Codes, sessions, and confirmations

Click the current two-factor code to copy it. The clipboard entry is cleared when that code interval ends, with a maximum lifetime of 31 seconds. The countdown follows Steam's 30-second code interval and synchronized Steam time.

**View Confirmations** opens a separate window. It refreshes while open, normally every 10 seconds, and provides a manual refresh control. Steam can reject requests, expire a session, or rate-limit repeated requests. **Login Again** first tries to refresh the stored Steam session; if that cannot succeed, use the password or QR login flow.

This is an unofficial integration with Steam's mobile endpoints. TcNo Account Switcher is not affiliated with Valve, and those endpoints can change without notice. A failed refresh is not proof that a trade or market action disappeared. Check the official Steam mobile app or Steam account history before retrying a sensitive action.

## Export a maFile

Export requires an unlocked Steam Guard vault. The exported maFile is plaintext and contains the authenticator secrets needed to generate codes and confirmations. Access and refresh tokens are excluded from the default export.

The exporter does not overwrite an existing file. Choose a new destination, protect the result, and remove unneeded plaintext copies. Export is for migration or recovery, not routine backup; **Create Verified Backup** preserves the complete encrypted folder and its metadata.

## Security boundary

A phone keeps the Steam password session and the mobile authenticator on separate devices. Enabling Steam Guard in TcNo Account Switcher reduces that separation: malware or a person controlling the PC can target the Steam client, remembered sessions, authenticator vault, screen capture, and clipboard from one machine.

The separate vault password, short unlock timeout, encrypted backups, bounded imports, and token-free default export reduce exposure. They do not make a compromised PC equivalent to an uncompromised phone. Keep the operating system updated, lock the PC when unattended, and do not leave the vault unlocked for the session on shared machines.
