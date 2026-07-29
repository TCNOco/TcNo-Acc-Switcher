// Package legacyinstall finds and removes the files the C# release (4.x and
// earlier) left next to the executable when the Go build is installed over the
// top of it. The NSIS installer keeps the same install directory and the same
// main exe name, so an upgrade overwrites TcNo-Acc-Switcher.exe and leaves
// roughly 250 MB of .NET assemblies, PDBs and asset folders orphaned.
package legacyinstall

import "strings"

// legacyFiles are the top-level file names shipped by the last C# release
// (2025-11-20_03). Removal is driven off this list and nothing else: a rule
// like "delete everything the Go build didn't install" would take a portable
// data folder, a user's own files, or a sibling install with it.
//
// TcNo-Acc-Switcher.exe is deliberately absent. The C# launcher had the same
// name as the Go binary and was overwritten in place, so the file sitting there
// now is the running app. [keepNames] repeats that guard.
var legacyFiles = []string{
	"Additional Licenses.txt",
	"CefSharp.Wpf.dll",
	"Disclaimer.txt",
	"DiscordRPC.dll",
	"ExCSS.dll",
	"GameStats.json",
	"Gameloop.Vdf.JsonConverter.dll",
	"Gameloop.Vdf.dll",
	"HtmlAgilityPack.dll",
	"IconLib.dll",
	"Magick.NET-Q8-AnyCPU.dll",
	"Magick.NET.Core.dll",
	"Microsoft.Extensions.DependencyInjection.Abstractions.dll",
	"Microsoft.Extensions.Localization.Abstractions.dll",
	"Microsoft.Extensions.Localization.dll",
	"Microsoft.Extensions.Logging.Abstractions.dll",
	"Microsoft.Extensions.Options.dll",
	"Microsoft.Extensions.Primitives.dll",
	"Microsoft.IO.RecyclableMemoryStream.dll",
	"Microsoft.Web.WebView2.Core.dll",
	"Microsoft.Web.WebView2.Core.xml",
	"Microsoft.Web.WebView2.WinForms.dll",
	"Microsoft.Web.WebView2.WinForms.xml",
	"Microsoft.Web.WebView2.Wpf.dll",
	"Microsoft.Web.WebView2.Wpf.xml",
	"Newtonsoft.Json.dll",
	"Platforms.json",
	"Privacy Policy.txt",
	"SevenZipExtractor.dll",
	"SevenZipSharp.dll",
	"SharpScss.dll",
	"ShimSkiaSharp.dll",
	"SkiaSharp.dll",
	"SteamKit2.dll",
	"Svg.Custom.dll",
	"Svg.Model.dll",
	"Svg.Skia.dll",
	"System.IO.Hashing.dll",
	"System.Management.dll",
	"System.ServiceProcess.ServiceController.dll",
	"TcNo-Acc-Switcher-Globals.dll",
	"TcNo-Acc-Switcher-Globals.pdb",
	"TcNo-Acc-Switcher-Server.deps.json",
	"TcNo-Acc-Switcher-Server.dll",
	"TcNo-Acc-Switcher-Server.exe",
	"TcNo-Acc-Switcher-Server.pdb",
	"TcNo-Acc-Switcher-Server.runtimeconfig.json",
	"TcNo-Acc-Switcher-Server.staticwebassets.endpoints.json",
	"TcNo-Acc-Switcher-Server.staticwebassets.runtime.json",
	"TcNo-Acc-Switcher-Server_main.exe",
	"TcNo-Acc-Switcher-Tray.deps.json",
	"TcNo-Acc-Switcher-Tray.dll",
	"TcNo-Acc-Switcher-Tray.exe",
	"TcNo-Acc-Switcher-Tray.pdb",
	"TcNo-Acc-Switcher-Tray.runtimeconfig.json",
	"TcNo-Acc-Switcher-Tray_main.exe",
	"TcNo-Acc-Switcher-Updater.dll",
	"TcNo-Acc-Switcher.deps.json",
	"TcNo-Acc-Switcher.dll",
	"TcNo-Acc-Switcher.pdb",
	"TcNo-Acc-Switcher.runtimeconfig.json",
	"TcNo-Acc-Switcher_main.exe",
	"VCDiff.dll",
	"YamlDotNet.dll",
	"ZstdSharp.dll",
	"_First_Run_Installer.exe",
	"appsettings.Development.json",
	"appsettings.json",
	"protobuf-net.Core.dll",
	"protobuf-net.dll",
	"runas.dll",
	"runas.exe",
	"runas.runtimeconfig.json",
	"securifybv.PropertyStore.dll",
	"securifybv.ShellLink.dll",
	"temp.dll",              // written by the C# updater, not in the release archive
	"UpdateFinalizeLog.txt", // same
}

// preservedFiles are set aside as <name>.old rather than deleted. Both are
// catalogs frozen at the last C# release, so nothing imports them - the newer
// ones this build ships would only be shadowed. Users could edit them by hand
// though, so a copy is kept where they can find it and port their changes over.
//
// The installer does the same rename before the app first runs; this covers
// installs it never ran for - portable copies, and anyone who upgraded before
// that shipped.
var preservedFiles = []string{
	"Platforms.json",
	"GameStats.json",
}

func isPreserved(name string) bool {
	for _, p := range preservedFiles {
		if strings.EqualFold(p, name) {
			return true
		}
	}
	return false
}

// legacyDir is a top-level folder from the C# release. Signature names are
// entries that must exist inside before the folder is touched: the names alone
// are generic enough (themes, updater, Resources) that a folder of the same
// name from another source should be left alone.
type legacyDir struct {
	name string
	// signature entries, any one of which marks the folder as the C# build's.
	// Slash-separated paths are resolved relative to the folder.
	signature []string
	// anySubdirFile matches when any immediate subdirectory holds this file.
	anySubdirFile string
}

var legacyDirs = []legacyDir{
	{name: "Platforms", signature: []string{"Discord/Username.js", "Discord"}},
	{name: "Resources", signature: []string{"en-US.yml"}},
	{name: "inc", signature: []string{"TcNo-Acc-Switcher.runtimeconfig.json"}},
	{name: "originalwwwroot", signature: []string{"prog_icons", "css", "favicon.ico"}},
	{name: "runtimes", signature: []string{"win-x64", "win"}},
	{name: "themes", anySubdirFile: "style.scss"},
	{name: "updater", signature: []string{"TcNo-Acc-Switcher-Updater.exe", "TcNo-Acc-Switcher-Updater.dll"}},
	{name: "x64", signature: []string{"7z.dll"}},
	{name: "x86", signature: []string{"7z.dll"}},
}

// legacyMarkers are C#-only executables and assemblies. At least one must be
// present before anything is reported, so a directory that merely shares a name
// with a legacy folder is never touched.
var legacyMarkers = []string{
	"TcNo-Acc-Switcher-Server.exe",
	"TcNo-Acc-Switcher-Server.dll",
	"TcNo-Acc-Switcher-Globals.dll",
	"TcNo-Acc-Switcher-Tray.exe",
	"TcNo-Acc-Switcher.dll",
	"_First_Run_Installer.exe",
}

// keepNames never get deleted, whatever else says. The Go build installs the
// first three; MicrosoftEdgeWebview2Setup.exe is the bootstrapper the current
// installer drops in, and the last is a portable user data folder that can sit
// beside the exe (platform.UserDataDirName).
var keepNames = []string{
	"TcNo-Acc-Switcher.exe",
	"OPEN_SOURCE_LICENSES.txt",
	"Uninstall TcNo Account Switcher.exe",
	"MicrosoftEdgeWebview2Setup.exe",
	"TcNo Account Switcher",
}
