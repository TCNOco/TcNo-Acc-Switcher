
mkdir updater
mkdir updater\x64
mkdir updater\x86
mkdir updater\ref
copy /B /Y "..\..\..\Installer\_First_Run_Installer.exe" "_First_Run_Installer.exe"
copy /B /Y "..\..\..\runas\x64\Release\net6.0\runas.exe" "runas.exe"
copy /B /Y "..\..\..\runas\x64\Release\net6.0\runas.dll" "runas.dll"
copy /B /Y "..\..\..\runas\x64\Release\net6.0\runas.runtimeconfig.json" "runas.runtimeconfig.json"

