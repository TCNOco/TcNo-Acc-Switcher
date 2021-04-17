
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
  <img alt="JetBrains Support - Open Source License" src="/other/img/DownloadLatest.png" height=70"></a></p>
  
**A Super fast account switcher for Steam** [Now with GUI]
**Saves NO passwords** or any user information*. It works purely off changing a file and 2 registry keys.
*Wastes no time closing, switching and restarting Steam.*<br />
**NOTE:** Not created for cheating purposes. All it does is change accounts. Use it as you see fit, accepting responsibility.

# Indev version
THIS IS THE IN DEV VERSION OF THE ACCOUNT SWITCHER.
THIS IS A WORK IN PROGRESS AND HAS NO COMPLETION DATE.
### News:
- [13 Feb 2021] Moved from .NET 3.1 + Blazor + Electron to .NET 5 + Blazor + WebView2

### Current progress:
Not much. it has a semi-functional list of users from Steam's `loginusers.vdf` and a good looking right-click menu. No options orsettings have been added to the program (Still trying to understand how this would work with Blazor.

<hr>

### Visit the [Wiki](https://github.com/TcNobo/TcNo-Acc-Switcher/wiki) for more info and steps for your first launch

### How does it work?
1. It lists your accounts based on the names in "C:\Program Files (x86)\Steam\config\loginusers.vdf"
2. After picking one, it edits that file so that the one you chose is the latest, and makes sure Remember Password is set to true.
3. It edits "HKEY_CURRENT_USER\Software\Valve\Steam\AutoLoginUser" to your selected username, and also sets the RememberPassword DWORD to True.

- It ends any processes that start with "Steam", and then restarts Steam.exe once the switch is made. You don't need to do anything but use the arrow keys and press Enter.

**Options available**: Start Steam as Administrator, Change Steam install folder, Hide VAC Status for each account and Show Steam ID. 

## Required runtimes (Download and install):
- Microsoft .NET Framework 4.8 Runtime: [Web Installer](https://dotnet.microsoft.com/download/dotnet-framework/thank-you/net48-web-installer), [Offline Installer](https://dotnet.microsoft.com/download/dotnet-framework/thank-you/net48-offline-installer), [Other languages](https://dotnet.microsoft.com/download/dotnet-framework/net48)

**Running the program:**
After downloading your .zip from the [GitHub Releases](https://github.com/TcNobo/TcNo-Acc-Switcher/releases) page, extract everything to a folder of your choice and run `TcNo Account Switcher.exe`


> If [HardenTools](https://github.com/securitywithoutborders/hardentools) was used, ensure that cmd.exe (Command Prompt) access is still allowed; if access is denied, TcNo Account Switcher will encounter a fatal unhandled exception (crash).

### Screenshots:
[imgur library](https://imgur.com/prhdlks)
<p><a href="https://imgur.com/a/iIlPtrW">
  <img alt="Main window screenshot" src="https://i.imgur.com/prhdlks.png" height=420">
  <img alt="Other windows (Combined screenshot)" src="https://i.imgur.com/7wti1KR.png" width=773">
</a></p>

### Known issues
- Issues caused by .NET Core (TcNo Account Switcher 2.0) are solved. No more issues clearing your `%temp%`

# Major updates
## 3.0 Update [Beta]: Support for more platforms!
Origin and more are coming to the TcNo Account Switcher! Easily handle multiple accounts with a few clicks!
With an all-new tray, you can switch accounts directly from the tray as well!
Tons more features to come.

#### Disclaimer
All trademarks and materials are property of their respective owners and their licensors.<br>
This project is not affiliated with Steam or Valve Corporation.<br>
I am not responsible for the contents of external links.<br>
USE THIS SOFTWARE AT YOUR OWN RISK. I AM NOT RESPONSIBLE FOR ANY DAMAGES IF YOU CHOOSE TO USE THIS SOFTWARE. COMPONENTS ARE PROVIDED ON AN "AS IS" AND "AS AVAILABLE" BASIS, WITHOUT ANY WARRANTIES OF ANY KIND TO THE FULLEST EXTENT PERMITTED BY LAW, AND I EXPRESSLY DISCLAIM ANY AND ALL WARRANTIES, WHETHER EXPRESS OR IMPLIED.

<p align="center"><a target="_blank" align="center" href="https://www.jetbrains.com/?from=TcNo-Account-Switcher">
  <img alt="JetBrains Support - Open Source License" src="/other/img/JetBrains_Banner.png" height=70"></a></p>
