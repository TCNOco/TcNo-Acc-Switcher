# TCNO-Acc-Switcher
 A Super fast account switcher for Steam
Saves NO passwords or any user information. It works purely off changing a file and 2 registry keys.
Wastes no time closing, switching and restarting Steam.

How does it work?
1. It lists your accounts based on the names in "C:\Program Files (x86)\Steam\config\loginusers.vdf"
2. After picking one, it edits that file so that the one you chose is the latest, and makes sure Remember Password is set to true.
3. It edits "HKEY_CURRENT_USER\Software\Valve\Steam\AutoLoginUser" to your selected username, and also sets the RememberPassword DWORD to True.

It ends any processes that start with "Steam", and then restarts Steam.exe once the switch is made.
