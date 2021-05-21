


<p align="center">
  <a href="https://tcno.co/">
    <img src="/other/img/Banner.png"></a>
</p>
<p align="center">
  <img alt="GitHub All Releases" src="https://img.shields.io/github/downloads/TcNobo/TcNo-Acc-Switcher/total?logo=GitHub&style=flat-square">
  <a href="https://tcno.co/">
    <img alt="Website" src="/other/img/web.svg" height=20"></a>
  <a href="https://s.tcno.co/AccSwitcherDiscord">
    <img alt="Discord server" src="https://img.shields.io/discord/217649733915770880?label=Discord&logo=discord&style=flat-square"></a>
  <a href="https://twitter.com/TcNobo">
    <img alt="Twitter" src="https://img.shields.io/twitter/follow/TcNobo?label=Follow%20%40TcNobo&logo=Twitter&style=flat-square"></a>
  <img alt="GitHub last commit" src="https://img.shields.io/github/last-commit/TcNobo/TcNo-Acc-Switcher?logo=GitHub&style=flat-square">
  <img alt="GitHub repo size" src="https://img.shields.io/github/repo-size/TcNobo/TcNo-Acc-Switcher?logo=GitHub&style=flat-square">
</p>
                                                                                                                                  
<p align="center"><a target="_blank" href="https://github.com/TcNobo/TcNo-Acc-Switcher/releases/latest">
  <img alt="Download latest" src="/other/img/DownloadLatest.png" height=70"></a><a target="_blank" href="https://tcno.co/Projects/AccSwitcher/api/latest">
  <img alt="Download Beta" src="/other/img/DownloadBeta.png" height=70"></a></p>
  
**A Super fast account switcher for Steam (and more soon)** [New version now in Beta]
**Saves NO passwords** or any user information*. Steam switcher works purely off changing a file and 2 registry keys.
*Wastes no time closing, switching and restarting Steam and other platforms.*<br />
**NOTE:** Not created for cheating purposes. All it does is change accounts. Use it as you see fit, accepting responsibility.

# STABLE VERSION:
New users should use the Download button above, or click [HERE](https://github.com/TcNobo/TcNo-Acc-Switcher/releases/latest). You'll also need the .NET Framework 4.8 Runtime, below.

### Required runtimes (Download and install):
- Microsoft .NET Framework 4.8 Runtime: [Web Installer](https://dotnet.microsoft.com/download/dotnet-framework/thank-you/net48-web-installer), [Offline Installer](https://dotnet.microsoft.com/download/dotnet-framework/thank-you/net48-offline-installer), [Other languages](https://dotnet.microsoft.com/download/dotnet-framework/net48)

**Running the program:**
After installing using the installer, or downloading your .zip (portable version) from the [GitHub Releases](https://github.com/TcNobo/TcNo-Acc-Switcher/releases) page, extract everything to a folder of your choice and run `TcNo Account Switcher.exe`


> If [HardenTools](https://github.com/securitywithoutborders/hardentools) was used, ensure that cmd.exe (Command Prompt) access is still allowed; if access is denied, TcNo Account Switcher will encounter a fatal unhandled exception (crash).
> 
# New beta version
The new Beta version is available for testing. Please report any and all bugs, as well as steps to recreate them into the Issues section, or the `#bug-report` channel under `TCNO ACCOUNT SWITCHER` section on the [Community Discord](https://s.tcno.co/AccSwitcherDiscord). Download the Beta in the Discord (This is to prevent confusion here)

<p><img alt="Youtube" src="/other/img/youtube.svg" height=18"> <b>Guides:</b> <a href="https://youtu.be/cvbo_VY05bo">BattleNet</a>, <a href="https://youtu.be/qRYra_fQt0I">Origin</a>, <a href="https://youtu.be/rLXGs1Yr3m8">Steam</a>, <a href="https://youtu.be/XKBkIQaJzOA">UPlay</a></p>

**New in this version:**
- **NEW: Battle.net account switcher** Thank's to [iR3turnZ](https://github.com/HoeblingerDaniel) :)
- **NEW: Epic Games account switcher** (Very early in development, but functional).
- **NEW: Origin account switcher** (Very early in development, but functional).
- **NEW: Ubisoft Connect account switcher** (Very early in development, but functional).
- **NEW:** Better UI, with animations. Fully user/community customisable theme system. 2 Themes built in (so far).
- **NEW:** Streamer mode to hide SteamIDs and more while Stream software is running (ie OBS, XSplit...)
- **NEW:** Easier ability to expand into other platforms (Yes, this is coming soon)
- **NEW:** WAY smaller updates, due to using a new Patch system. No more redownloading the entire app. Only a few MB at a time.
- **STEAM:** Log in as Invisible, Offline and more! Copy profile links, SteamID and create quick-switch desktop shortcuts!

### Required runtimes (Download and install):
- **WebView2 Runtime**:  Click [HERE](https://go.microsoft.com/fwlink/p/?LinkId=2124703) and install.
- **Microsoft .NET 5 Desktop Runtime AND: ASP.NET Core 5.0 Runtime:** Click [HERE](https://dotnet.microsoft.com/download/dotnet/5.0/runtime) and click `x64`, as well as `Download Hosting Bundle`. **See below:**
![Buttons to click for .NET Desktop & ASP.NET runtime](https://i.imgur.com/f4e14Mr.png)


# FAQ

### Visit the [Wiki](https://github.com/TcNobo/TcNo-Acc-Switcher/wiki) for more info and steps for your first launch.
(Steps for Beta are not available yet)

### How does the Steam switcher work?
1. It lists your accounts based on the names in "C:\Program Files (x86)\Steam\config\loginusers.vdf"
2. After picking one, it edits that file so that the one you chose is the latest, and makes sure Remember Password is set to true.
3. It edits "HKEY_CURRENT_USER\Software\Valve\Steam\AutoLoginUser" to your selected username, and also sets the RememberPassword DWORD to True.

- It ends any processes that start with "Steam", and then restarts Steam.exe once the switch is made. You don't need to do anything but use the arrow keys and press Enter.

**Options available**: Start Steam as Administrator, Change Steam install folder, Hide VAC Status for each account and Show Steam ID. 

### Screenshots:
[imgur library](https://imgur.com/prhdlks)
<p><a href="https://imgur.com/a/iIlPtrW">
  <img alt="Main window screenshot" src="https://i.imgur.com/prhdlks.png" height=420">
  <img alt="Other windows (Combined screenshot)" src="https://i.imgur.com/7wti1KR.png" width=773">
</a></p>

### Known issues
- Issues caused by .NET Core (TcNo Account Switcher 2.0) are solved. No more issues clearing your `%temp%`
(This is not an issue in the new Beta version, hence no fix)


#### Disclaimer
All trademarks and materials are property of their respective owners and their licensors.<br>
This project is not affiliated with Battle.net or Blizzard Entertainment Inc, Epic Games Inc or the Epic Games Launcher, Origin or Electronic Arts Inc, Steam or Valve Corporation, Ubisoft Connect or Ubisoft Entertainment, or any other companies or groups that this software may have reference to. This project should not be considered "Official" or related to platforms mentioned in any way, other than letting you move your files around on your computer.<br>
I am not responsible for the contents of external links.<br>
USE THIS SOFTWARE AT YOUR OWN RISK. I AM NOT RESPONSIBLE FOR ANY DAMAGES IF YOU CHOOSE TO USE THIS SOFTWARE. COMPONENTS ARE PROVIDED ON AN "AS IS" AND "AS AVAILABLE" BASIS, WITHOUT ANY WARRANTIES OF ANY KIND TO THE FULLEST EXTENT PERMITTED BY LAW, AND I EXPRESSLY DISCLAIM ANY AND ALL WARRANTIES, WHETHER EXPRESS OR IMPLIED.

Additional license information for included NuGet packages and other parts of code can be found in: [HERE](https://github.com/TcNobo/TcNo-Acc-Switcher/blob/master/TcNo-Acc-Switcher-Server/Additional%20Licenses.txt) `TcNo-Acc-Switcher-Server/Additional Licenses.txt`, and are copied to the build directory, as well as distributed with release versions of this software.

<p align="center"><a target="_blank" align="center" href="https://www.jetbrains.com/?from=TcNo-Account-Switcher">
  <img alt="JetBrains Support - Open Source License" src="/other/img/JetBrains_Banner.png" height=70"></a></p>
