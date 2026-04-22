package cli

// HelpText returns CLI usage for console / --help.
func HelpText() string {
	return `TcNo Account Switcher — CLI

Swap (Steam):        +s:<steamId64>[:<personaState>]
                     tcno://s:<steamId64>[:<personaState>]
Swap (other):        +<platformShort>:<uniqueId>
                     (platformShort is the first Identifiers entry in Platforms.json)

Open GUI to page:    <PlatformName>      e.g. Steam

Logout:              logout[:<PlatformName>[:<accountId>]]

Other:               -h, --help    Show this help
                     -v, --verbose Verbose logging (reserved)

Second instance forwards arguments to the running GUI via a named pipe (Windows).
`
}
