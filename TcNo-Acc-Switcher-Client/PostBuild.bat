REM Move is currently only for build, as moving the files seems to prevent the program from running properly...

REM Move updater files in Debug folder:
echo %CD%
cd bin\x64\Debug\net5.0-windows\
mkdir updater
mkdir updater\x64
mkdir updater\x86
mkdir updater\ref
copy /B /Y "VCDiff.dll" "updater\VCDiff.dll"
move /Y "TcNo-Acc-Switcher-Updater.runtimeconfig.json" "updater\TcNo-Acc-Switcher-Updater.runtimeconfig.json"
move /Y "TcNo-Acc-Switcher-Updater.runtimeconfig.dev.json" "updater\TcNo-Acc-Switcher-Updater.runtimeconfig.dev.json"
move /Y "TcNo-Acc-Switcher-Updater.pdb" "updater\TcNo-Acc-Switcher-Updater.pdb"
move /Y "TcNo-Acc-Switcher-Updater.exe" "updater\TcNo-Acc-Switcher-Updater.exe"
copy /B /Y "TcNo-Acc-Switcher-Updater.dll" "updater\TcNo-Acc-Switcher-Updater.dll"
move /Y "TcNo-Acc-Switcher-Updater.deps.json" "updater\TcNo-Acc-Switcher-Updater.deps.json"
copy /B /Y "SevenZipExtractor.dll" "updater\SevenZipExtractor.dll"
move /Y "x86\7z.dll" "updater\x86\7z.dll"
move /Y "x64\7z.dll" "updater\x64\7z.dll"
copy /B /Y "ref\TcNo-Acc-Switcher-Updater.dll" "updater\ref\TcNo-Acc-Switcher-Updater.dll"
copy /B /Y "Newtonsoft.Json.dll" "updater\Newtonsoft.Json.dll"
copy /B /Y "TcNo-Acc-Switcher-Globals.deps.json" "updater\TcNo-Acc-Switcher-Globals.deps.json"
copy /B /Y "TcNo-Acc-Switcher-Globals.dll" "updater\TcNo-Acc-Switcher-Globals.dll"
copy /B /Y "TcNo-Acc-Switcher-Globals.pdb" "updater\TcNo-Acc-Switcher-Globals.pdb"
copy /B /Y "TcNo-Acc-Switcher-Globals.pdb" "updater\TcNo-Acc-Switcher-Globals.pdb"
copy /B /Y "ref\TcNo-Acc-Switcher-Globals.dll" "updater\ref\TcNo-Acc-Switcher-Globals.dll"
RMDIR /Q/S "runtimes\linux-musl-x64"
RMDIR /Q/S "runtimes\linux-x64"
RMDIR /Q/S "runtimes\osx"
RMDIR /Q/S "runtimes\osx-x64"
RMDIR /Q/S "runtimes\unix"
RMDIR /Q/S "runtimes\win-arm64"
RMDIR /Q x64
RMDIR /Q x86

REM Move updater files in Release folder:
cd ..\..\Release\net5.0-windows\
mkdir updater
mkdir updater\x64
mkdir updater\x86
mkdir updater\ref
copy /B /Y "VCDiff.dll" "updater\VCDiff.dll"
move /Y "TcNo-Acc-Switcher-Updater.runtimeconfig.json" "updater\TcNo-Acc-Switcher-Updater.runtimeconfig.json"
move /Y "TcNo-Acc-Switcher-Updater.runtimeconfig.dev.json" "updater\TcNo-Acc-Switcher-Updater.runtimeconfig.dev.json"
move /Y "TcNo-Acc-Switcher-Updater.pdb" "updater\TcNo-Acc-Switcher-Updater.pdb"
move /Y "TcNo-Acc-Switcher-Updater.exe" "updater\TcNo-Acc-Switcher-Updater.exe"
copy /B /Y "TcNo-Acc-Switcher-Updater.dll" "updater\TcNo-Acc-Switcher-Updater.dll"
move /Y "TcNo-Acc-Switcher-Updater.deps.json" "updater\TcNo-Acc-Switcher-Updater.deps.json"
copy /B /Y "SevenZipExtractor.dll" "updater\SevenZipExtractor.dll"
move /Y "x86\7z.dll" "updater\x86\7z.dll"
move /Y "x64\7z.dll" "updater\x64\7z.dll"
copy /B /Y "ref\TcNo-Acc-Switcher-Updater.dll" "updater\ref\TcNo-Acc-Switcher-Updater.dll"
copy /B /Y "Newtonsoft.Json.dll" "updater\Newtonsoft.Json.dll"
copy /B /Y "TcNo-Acc-Switcher-Globals.deps.json" "updater\TcNo-Acc-Switcher-Globals.deps.json"
copy /B /Y "TcNo-Acc-Switcher-Globals.dll" "updater\TcNo-Acc-Switcher-Globals.dll"
copy /B /Y "TcNo-Acc-Switcher-Globals.pdb" "updater\TcNo-Acc-Switcher-Globals.pdb"
copy /B /Y "TcNo-Acc-Switcher-Globals.pdb" "updater\TcNo-Acc-Switcher-Globals.pdb"
copy /B /Y "ref\TcNo-Acc-Switcher-Globals.dll" "updater\ref\TcNo-Acc-Switcher-Globals.dll"
RMDIR /Q/S "runtimes\linux-musl-x64"
RMDIR /Q/S "runtimes\linux-x64"
RMDIR /Q/S "runtimes\osx"
RMDIR /Q/S "runtimes\osx-x64"
RMDIR /Q/S "runtimes\unix"
RMDIR /Q/S "runtimes\win-arm64"
RMDIR /Q x64
RMDIR /Q x86