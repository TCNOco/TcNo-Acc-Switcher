# PRIVACY POLICY

This privacy policy only applies to the TcNo Account Switcher. Should you wish to see https://tcno.co's Privacy Policy, please see: https://hub.tcno.co/privacy/.

To see this formatted correctly, see: https://github.com/TCNOco/TcNo-Acc-Switcher/blob/master/PRIVACY.md

Please note: This project is licensed under the [GNU General Public License v3.0](https://github.com/TCNOco/TcNo-Acc-Switcher/blob/master/LICENSE).

Please also note the [Disclaimer](https://github.com/TCNOco/TcNo-Acc-Switcher/blob/master/DISCLAIMER.md) - Also at the end of this file.

This program will not transfer any information to other networked systems unless specifically requested by the user or the person installing or operating it.

This project works entirely by moving files and registry keys around that store user information for specific programs, game platforms and more. This project does not send those files, nor information from them in any way over internet. This data remains entirely on your computer, usually within `%AppData%\TcNo Account Switcher` unless otherwise specified in settings.

However, this project does communicate over the internet in the following ways:
- During installation, on first run (and subsequent runs) the program checks for required runtimes like .NET, downloads and executes those installers. Information like your IP address will be handled by Microsoft.com as defined in their policies.
- Each launch in these scenarios:
  - If the user has `Statistics collection` enabled AND `Anonymous Statistics Sharing` enabled their statistics will be submitted. See below.
  - If the user has `Check for updates` enabled: When the program is launched it checks for an update. This web request is logged through normal means by ISPs and the usual - but also used by our server in the following way:
    - Your IP address is used to get your geographical region ONLY accurate to your province or state.
    - This is added to a global total for that region completely anonymously. It is not stored in any other way than is normal for websites such as anonymized in rolling access logs.
    - The current version you are using is also submitted with this request, which also adds to a global total. This is again, anonymous.
  - When the `Game Stats` function is used. See below.
  - When the user checks the [CrowdIn](https://crowdin.com/project/tcno-account-switcher) information in-app the usernames of persons who helped translate this app is via [CrowdIn](https://crowdin.com/project/tcno-account-switcher) are displayed publicly in the TcNo Account Switcher, and possibly on the website.
  - When the user opens the Steam account switcher in the TcNo Account Switcher each profile is checked on Steam via `https://steamcommunity.com/profiles/{SteamId}?xml=1` for the VAC status of the account, as well as the profile image for that account. This can be disabled at any time through the TcNo Account Switcher > Steam > Settings > `Collect profile images and VAC ban status from Steam`.

Every web request from updates, to data collecting, submission and more can be turned off through `Offline Mode`.

`Offline Mode` and `Anonymous Statistics Sharing` can both be disabled before first launch through the installer.

The TcNo Account Switcher functions through a Client (web client such as CEF or WebView2) and a Server back-end (so you can use it in a browser or elsewhere). This will not be turned off through `Offline Mode` as it is a core feature requirement. Through an incorrect firewall configuration this could be reached from other computers on your network, or elsewhere.

## Logs
There are a few types of logs that this program creates.

### Crash Logs
Crash logs contain useful information for debugging, and improving the program. These crash logs contain: 
  - The crash information including information like a stack trace.

### Log files
- The log file. This log file contains every action in the program that run. Each function used logs the time it was accessed, actions like moving files and registry items, and more. There is no personally identifiable information included here where possible. For example, this file includes information like:
  - What platform was being switched when before a crash, but not usernames of specific accounts.
  - What files were deleted (or failed to be deleted) by the program
  - While not intended, things like usernames can enter this file through being the cause of a crash (malformed text), included in filenames by platforms (such as a file named `userid.ext`) and so on

For this reason crash logs and log files are not automatically submitted.

When creating issues on GitHub, or contacting me through other places you may include these files AFTER you have double-checked for personal information. Redact as much as you see fit. These files will not be kept for longer than needed. If I don't control where they are hosted, for example Discord, it is up to you to remove them from those hosting platforms.

## Statistics
This is an optional feature. The user can have `Collect Statistics` enabled to view their own usage habits, but the user must also have `Share stats anonymously` enabled as well for submission to our servers. Change your choice in Settings > Statistics & Sharing at any time. Your statistics can also be viewed with the button next to this option.

These statistics include:
 - Operating system
 - First and last launch date and time.
 - A randomly generated UUID for storage, and updating this on our server. This is random, and not personally identifiable without you telling us what yours is.
 - Number of times launched,  crashed,  accounts switched (total), games launched  (total), unique days with accounts switched (total)
 - The last time (if ever) your statistics were submitted
 - Platform specific information like:
  - Your most used platform overall.
  - For each platform the number of:
    - Accounts you have saved for this platform
    - Account switches you've made on this platform
    - Unique days you've used this platform in the TcNo Account Switcher
    - Game shortcuts you have saved in this app (as well as the number on your TcNo Account Switcher hotbar).
    - Games launched through the TcNo Account Switcher.
  - The number of times you've visited each platform in the TcNo Account Switcher and for how long (total).

This information can not be used to identify you in any way.

This is used to improve the application overall (which platforms are used the most), and will add to the global total (Currently a page does not exist for this).

## Game Stats
This is an opt-in feature on an account basis. When a user opens a platform in the TcNo Account Switcher compatible with the [GameStats.json](https://github.com/TCNOco/TcNo-Acc-Switcher/blob/master/TcNo-Acc-Switcher-Server/GameStats.json), Right-clicks an account > Manage > Manage game stats > Enabled a platform and enters a username or account ID and clicks `Submit` this information will be used in accordance to the [GameStats.json](https://github.com/TCNOco/TcNo-Acc-Switcher/blob/master/TcNo-Acc-Switcher-Server/GameStats.json) in URLs visited.

These URLs are then checked on the user's behalf for information such as account rank in a game.

The information submitted to these sites is presented to, and often must be set manually by the user in the TcNo Account Switcher before it can be used with this feature. The TcNo Account Switcher does not submit this account information to anywhere else on the internet.

Your information will then be handled in accordance of those third-parties privacy policies, terms and more.

Information collected such as your rank and more will then be displayed based on the rest of the file, usually under your account's username in the TcNo Account Switcher.

# DISCLAIMER

All trademarks and materials are property of their respective owners and their licensors. This project is not affiliated
with Battle.net or Blizzard Entertainment Inc, Epic Games Inc or the Epic Games Launcher, Origin or Electronic Arts Inc,
League of Legends or Legends of Runeterra or Valorant or Riot Games Inc, Steam or Valve Corporation, Ubisoft Connect or
Ubisoft Entertainment, or any other companies or groups that this software may have reference to. This project should
not be considered "Official" or related to platforms mentioned in any way. All it does is let you move your files around
on your computer.

I am not responsible for the contents of external links.
For the rest of the disclaimer, refer to the License (GNU General Public License v3.0) file:
<https://github.com/TcNobo/TcNo-Acc-Switcher/blob/master/LICENSE> - See sections like 15, 16 and 17, as well as GitHub's
'simplification' at the top of the above website.
