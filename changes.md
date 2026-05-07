## Changes from the C# version:

### Application

- Decrease in RAM usage from ~`76-400MB` + to `17-40MB` with better GC.
- Single executable for `tray`, `server` and `client`. HTML content embedded, but still overwritable if you want to mod the program. Much smaller footprint.
- Removed CEF fallback - Uses native OS WebView. WebView2 on Windows, see [Wails](https://v3.wails.io/guides/build/windows/).

### Platforms

- `Steam` switcher is much faster now. Collects profile images and Mini profiles for hover interactions!
- `Discord`, `PTB` and `Canary` now get username and profile image from local files - Super easy to add & switch new accounts. Improved exiting, so no more manual steps to get these accounts imported!
- Added built-in solution to get account images and usernames for multiple platforms:

| Platform | Username |   Image  |
|----------|----------|----------|
| Discord | ✅ | ✅ |
| Discord PTB | ✅ | ✅ |
| Discord Canary | ✅ | ✅ |
| EA Desktop | ✅ | ✅ |
| Rockstar | ✅ | ✅ |
| GeForce Now | ✅ | - |
| Ubisoft | - | ✅ |

- Fixed and improved switchers for almost everything. `Oculus` is now `Meta Horizon Link`. `Delta Force: Hawk Ops` is now `Delta Force`. 
- Removed switchers for Genshin Impact & Honkai Star Rail - These seem improbably to create a working switcher for. Seems to get last UID/Login from server using your computers HWID.

### Accounts

- Added `Tags` for accounts! You can now add tags to accounts, and filter accounts by tags.
- Accounts can now have custom images. Right-Click and change image, or drag an image from your OS into the program, and drop onto an account to change it on any platform.

### Game Shortcuts

- Drag and drop game/app shortcuts into the program to add them to the switcher's hotbar for any platform.

### Quality of Life improvements

- Type anywhere to search. Search platforms (and enable them) from the Home page, or search and quickly switch accounts from any platform.
- Forward mouse/keyboard button works as well as just back.
- Right-Click Context Menu does not shake and jump around as violently as before. Much more usable.
- Added `Open Game Data` option for Steam's context menu.

### Platforms.json

- leveldb support for grabbing JSON key/values and more.
- Removed now unused `Extras.UsernameModalExtraButtons`, `Extras.UsernameModalCopyText` and `Extras.UsernameModalHintText` keys & handlers
- Added `ClosingMethod: Electron` for Discord and other Electron-based programs. These programs do NOT save login if force-quit/killed, nor do they properly always exit when the window is closed (Some, like Discord, minimize to the tray). This fixes it.
- Added globl support for registry keys, such as `REG:HKCU\\SOFTWARE\\program\\key:LastLoginString_*`
- `ExeLocationDefault` can now be an array for multiple install paths. Usually launcher, Steam and other platforms possible defaults.

### Code

- No more jQuery to handle frontend. Svelte used almost entirely. No more bootstrap either.