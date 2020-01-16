<p align="center">
    <a href="https://tcno.co/">
	<img src="/docs/img/Banner.png"></a>
</p>
<p align="center">
    <a href="https://tcno.co/">
        <img alt="Website" src="/docs/img/web.svg" height=20"></a>
    <a href="https://discord.gg/wkJp38m">
        <img alt="Discord server" src="https://img.shields.io/discord/217649733915770880.svg?label=Discord&logo=discord&style=flat-square"></a>
    <a href="https://twitter.com/TcNobo">
        <img alt="Twitter" src="https://img.shields.io/twitter/follow/TcNobo.svg?label=Follow%20%40TcNobo&logo=Twitter&style=flat-square"></a>
    <img alt="Last commit" src="https://img.shields.io/github/last-commit/TcNobo/TcNo-Acc-Switcher.svg?label=Last%20commit&logo=GitHub&style=flat-square">
    <img alt="Repo size" src="https://img.shields.io/github/repo-size/TcNobo/TcNo-Acc-Switcher.svg?label=Repo%20size&logo=GitHub&style=flat-square">
</p>
                                                                                                                                         
**A Super fast account switcher for Steam** [Now with GUI]
**Saves NO passwords** or any user information*. It works purely off changing a file and 2 registry keys.
*Wastes no time closing, switching and restarting Steam.*

### Visit the [Wiki](https://github.com/TcNobo/TcNo-Acc-Switcher/wiki) for more info and steps for your first launch

### How does it work?
1. It lists your accounts based on the names in "C:\Program Files (x86)\Steam\config\loginusers.vdf"
2. After picking one, it edits that file so that the one you chose is the latest, and makes sure Remember Password is set to true.
3. It edits "HKEY_CURRENT_USER\Software\Valve\Steam\AutoLoginUser" to your selected username, and also sets the RememberPassword DWORD to True.

- It ends any processes that start with "Steam", and then restarts Steam.exe once the switch is made. You don't need to do anything but use the arrow keys and press Enter.

**Options available**: Start Steam as Administrator, Change Steam install folder, Hide VAC Status for each account and Show Steam ID. 

## 2.0 Update: GUI added. Auto update added.
Users can still download updates via the GitHub Releases page, but they will be prompted when an update is detected, and the program will update when you choose. Nothing is forced.

*When the program checks for updates, it adds to a counter saying "Add 1 to the counter for my Province/State in country", purely for interest's sake. IP addresses and identifiable information are not collected.
## Required runtimes (Download and install):
- Microsoft .NET Core 3.1 Runtime: [x86](https://dotnet.microsoft.com/download/dotnet-core/thank-you/runtime-3.1.1-windows-x86-installer) [x64](https://dotnet.microsoft.com/download/dotnet-core/thank-you/runtime-3.1.1-windows-x64-installer)
- Microsoft .NET Core 3.1 Desktop Runtime: [x86](https://dotnet.microsoft.com/download/dotnet-core/thank-you/runtime-desktop-3.1.1-windows-x86-installer) [x64](https://dotnet.microsoft.com/download/dotnet-core/thank-you/runtime-desktop-3.1.1-windows-x64-installer)

**Running the program:** After downloading your .zip from the [GitHub Releases](https://github.com/TcNobo/TcNo-Acc-Switcher/releases) page, extract everything to a folder of your choice and run `TcNo Account Switcher.exe`

### Screenshots:
[imgur library](https://imgur.com/prhdlks)
<p><a href="https://imgur.com/a/iIlPtrW">
  <img alt="Main window screenshot" src="https://i.imgur.com/prhdlks.png" height=420">
  <img alt="Other windows (Combined screenshot)" src="https://i.imgur.com/7wti1KR.png" width=773">
</a></p>

### Required libraries for building (Developers only):
- Curl (release uses 7.67.0) - [Download](https://curl.haxx.se/download.html), Build guide: https://youtu.be/q_mXVZ6VJs4

Downloaded from a different source? Verify hashes [HERE](https://tcno.co/Projects/AccSwitcher/Hashes.html)
