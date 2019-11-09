
# TCNO-Acc-Switcher
**A Super fast account switcher for Steam**
**Saves NO passwords** or any user information. It works purely off changing a file and 2 registry keys.
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
