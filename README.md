

<p align="center">
    <a href="">
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
                                                                                                                                         
**A Super fast account switcher for Steam**
**Saves NO passwords** or any user information*. It works purely off changing a file and 2 registry keys.
*Wastes no time closing, switching and restarting Steam.*

### How does it work?
1. It lists your accounts based on the names in "C:\Program Files (x86)\Steam\config\loginusers.vdf"
2. After picking one, it edits that file so that the one you chose is the latest, and makes sure Remember Password is set to true.
3. It edits "HKEY_CURRENT_USER\Software\Valve\Steam\AutoLoginUser" to your selected username, and also sets the RememberPassword DWORD to True.

- It ends any processes that start with "Steam", and then restarts Steam.exe once the switch is made. You don't need to do anything but use the arrow keys and press Enter.

**To: Run Steam as Admin**: Create a file named *"admin.txt"* or *"admin"* in the same directory as the .exe

**To: Choose a custom Steam install folder**: Create a file named *"SteamLocation.txt"* in the same directory as the .exe. Inside put only a folder path, *ie:* `C:\Steam\`

### Required libraries for building:
- Curl (release uses 7.67.0) - [Download](https://curl.haxx.se/download.html), Build guide: https://youtu.be/q_mXVZ6VJs4

### PLEASE NOTE: A GUI version is in the works.
Users will have an update notification as usual, and it will be pushed here via GitHub Releases.

*The program checks for updates each run. If found, it notifies user on next run. When the program checks for updates, it adds to a counter saying "Add 1 to the counter for my Province/State in country", purely for interest's sake. IP addresses and identifyable information is not collected.
