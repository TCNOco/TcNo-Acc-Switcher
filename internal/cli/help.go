package cli

// HelpText returns CLI usage for console / --help.
func HelpText() string {
	return `TcNo Account Switcher — CLI

Swap (Steam):        +s:<steamId64>[:<personaState>]
                     tcno://s:<steamId64>[:<personaState>]
Swap (other):        +<platformShort>:<uniqueId>
                     (platformShort is the first Identifiers entry in Platforms.json)

Extra argv for the platform exe (e.g. Steam) after swap/launch from CLI or shortcuts:
                     +s:<id> -dev -x
                     (any token that is not a TcNo flag, +swap, logout, or a GUI page name)

Swap & launch (desktop shortcuts / game tiles):
                     +s:<steamId64> --run-appid=<appId>
                     (Steam only; launches steam://rungameid/<appId> after swap)
                     +s:<id> --run-shortcut=<urlEncodedFile.lnk>
                     +<platformShort>:<uniqueId> --run-shortcut=<urlEncodedFile.lnk>
                     (launches cached .lnk/.url after swap; resolves Desktop / Start Menu if cache missing)

Open GUI to page:    <PlatformName>      e.g. Steam
                     --page=<PlatformName>   (same; useful after restart-as-admin)

Logout:              logout[:<PlatformName>[:<accountId>]]

List:                --list-platforms
                     --list-accounts
                     --list-accounts=<PlatformNameOrAlias>
                     --json        JSON instead of plain text (use with list flags only)

Other:               -h, --help    Show this help
                     -v, --verbose Debug logging (same as --log-level=debug)
                     --log-level=  Logging: debug, info, warn, error (app + Wails; default info)
                     -tray, --tray Start with the main window hidden (tray / background)

Second instance forwards arguments to the running GUI via a named pipe (Windows).
`
}
