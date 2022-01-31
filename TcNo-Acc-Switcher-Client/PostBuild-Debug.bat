
mkdir updater
mkdir updater\x64
mkdir updater\x86
mkdir updater\ref
echo %cd%
copy /B /Y "bin\Installer\_First_Run_Installer.exe" "bin\x64\Debug\net6.0-windows\_First_Run_Installer.exe"
copy /B /Y "bin\runas\x64\Release\net6.0\runas.exe" "bin\x64\Debug\net6.0-windows\runas.exe"
copy /B /Y "bin\runas\x64\Release\net6.0\runas.dll" "bin\x64\Debug\net6.0-windows\runas.dll"
copy /B /Y "bin\runas\x64\Release\net6.0\runas.runtimeconfig.json" "bin\x64\Debug\net6.0-windows\runas.runtimeconfig.json"

