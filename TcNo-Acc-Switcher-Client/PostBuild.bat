REM Move is currently only for build, as moving the files seems to prevent the program from running properly...

REM Get current directory:
echo Current directory: %cd%
set origDir=%cd%


REM Move updater files in Debug folder (for Visual Studio):
IF not exist bin\x64\Debug\net5.0-windows\ GOTO vsRel
IF EXIST bin\x64\Debug\net5.0-windows\updater GOTO vsRel
cd %origDir%\bin\x64\Debug\net5.0-windows\
ECHO -----------------------------------
ECHO Moving files for x64 Debug in Visual Studio
ECHO -----------------------------------
mkdir updater
mkdir updater\x64
mkdir updater\x86
mkdir updater\ref
copy /B /Y "VCDiff.dll" "updater\VCDiff.dll"
copy /B /Y "YamlDotNet.dll" "updater\YamlDotNet.dll"
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
RMDIR /Q/S "runtimes\linux-musl-x64"
RMDIR /Q/S "runtimes\linux-x64"
RMDIR /Q/S "runtimes\osx"
RMDIR /Q/S "runtimes\osx-x64"
RMDIR /Q/S "runtimes\unix"
RMDIR /Q/S "runtimes\win-arm64"
RMDIR /Q x64
RMDIR /Q x86
copy /B /Y "..\..\..\Installer\_First_Run_Installer.exe" "_First_Run_Installer.exe"
REN "wwwroot" "originalwwwroot"
cd %origDir%
GOTO end

REM Move updater files in Release folder (for Visual Studio):
:vsRel
IF NOT EXIST bin\x64\Release\net5.0-windows\ GOTO ghDebug
IF EXIST bin\x64\Release\net5.0-windows\updater GOTO end
cd %origDir%\bin\x64\Release\net5.0-windows\
ECHO -----------------------------------
ECHO Moving files for x64 Release in Visual Studio
ECHO -----------------------------------
mkdir updater
mkdir updater\x64
mkdir updater\x86
mkdir updater\ref
copy /B /Y "VCDiff.dll" "updater\VCDiff.dll"
copy /B /Y "YamlDotNet.dll" "updater\YamlDotNet.dll"
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
RMDIR /Q/S "runtimes\linux-musl-x64"
RMDIR /Q/S "runtimes\linux-x64"
RMDIR /Q/S "runtimes\osx"
RMDIR /Q/S "runtimes\osx-x64"
RMDIR /Q/S "runtimes\unix"
RMDIR /Q/S "runtimes\win-arm64"
RMDIR /Q x64
RMDIR /Q x86
copy /B /Y "..\..\..\Installer\_First_Run_Installer.exe" "_First_Run_Installer.exe"
REN "wwwroot" "originalwwwroot"
cd %origDir%
GOTO end



REM Move updater files in Debug folder (for GitHub Actions):
:ghDebug
IF NOT EXIST bin\Debug\net5.0-windows\ GOTO ghRel
IF EXIST bin\Debug\net5.0-windows\updater GOTO ghRel
cd %origDir%
ECHO -----------------------------------
ECHO Moving files for x64 Debug in GitHub
ECHO -----------------------------------
mkdir bin\Debug\net5.0-windows\updater
mkdir bin\Debug\net5.0-windows\updater\x64
mkdir bin\Debug\net5.0-windows\updater\x86
mkdir bin\Debug\net5.0-windows\updater\ref
copy /B /Y "bin\Debug\net5.0-windows\VCDiff.dll" "bin\Debug\net5.0-windows\updater\VCDiff.dll"
copy /B /Y "bin\Debug\net5.0-windows\YamlDotNet.dll" "bin\Debug\net5.0-windows\updater\YamlDotNet.dll"
move /Y "bin\Debug\net5.0-windows\TcNo-Acc-Switcher-Updater.runtimeconfig.json" "bin\Debug\net5.0-windows\updater\TcNo-Acc-Switcher-Updater.runtimeconfig.json"
move /Y "bin\Debug\net5.0-windows\TcNo-Acc-Switcher-Updater.runtimeconfig.dev.json" "bin\Debug\net5.0-windows\updater\TcNo-Acc-Switcher-Updater.runtimeconfig.dev.json"
move /Y "bin\Debug\net5.0-windows\TcNo-Acc-Switcher-Updater.pdb" "bin\Debug\net5.0-windows\updater\TcNo-Acc-Switcher-Updater.pdb"
move /Y "bin\Debug\net5.0-windows\TcNo-Acc-Switcher-Updater.exe" "bin\Debug\net5.0-windows\updater\TcNo-Acc-Switcher-Updater.exe"
copy /B /Y "bin\Debug\net5.0-windows\TcNo-Acc-Switcher-Updater.dll" "bin\Debug\net5.0-windows\updater\TcNo-Acc-Switcher-Updater.dll"
move /Y "bin\Debug\net5.0-windows\TcNo-Acc-Switcher-Updater.deps.json" "bin\Debug\net5.0-windows\updater\TcNo-Acc-Switcher-Updater.deps.json"
copy /B /Y "bin\Debug\net5.0-windows\SevenZipExtractor.dll" "bin\Debug\net5.0-windows\updater\SevenZipExtractor.dll"
move /Y "bin\Debug\net5.0-windows\x86\7z.dll" "bin\Debug\net5.0-windows\updater\x86\7z.dll"
move /Y "bin\Debug\net5.0-windows\x64\7z.dll" "bin\Debug\net5.0-windows\updater\x64\7z.dll"
copy /B /Y "bin\Debug\net5.0-windows\ref\TcNo-Acc-Switcher-Updater.dll" "bin\Debug\net5.0-windows\updater\ref\TcNo-Acc-Switcher-Updater.dll"
copy /B /Y "bin\Debug\net5.0-windows\Newtonsoft.Json.dll" "bin\Debug\net5.0-windows\updater\Newtonsoft.Json.dll"
RMDIR /Q/S "bin\Debug\net5.0-windows\runtimes\linux-musl-x64"
RMDIR /Q/S "bin\Debug\net5.0-windows\runtimes\linux-x64"
RMDIR /Q/S "bin\Debug\net5.0-windows\runtimes\osx"
RMDIR /Q/S "bin\Debug\net5.0-windows\runtimes\osx-x64"
RMDIR /Q/S "bin\Debug\net5.0-windows\runtimes\unix"
RMDIR /Q/S "bin\Debug\net5.0-windows\runtimes\win-arm64"
RMDIR /Q "bin\Release\net5.0-windows\x64"
RMDIR /Q "bin\Release\net5.0-windows\x86"
copy /B /Y "..\..\Installer\_First_Run_Installer.exe" "_First_Run_Installer.exe"
REN "wwwroot" "originalwwwroot"
cd %origDir%
GOTO end

REM Move updater files in Release folder (for GitHub Actions):
:ghRel
IF NOT EXIST bin\Release\net5.0-windows\ GOTO end
IF EXIST bin\Release\net5.0-windows\updater GOTO end
cd %origDir%
ECHO -----------------------------------
ECHO Moving files for x64 Release in GitHub
ECHO -----------------------------------
mkdir bin\Release\net5.0-windows\updater
mkdir bin\Release\net5.0-windows\updater\x64
mkdir bin\Release\net5.0-windows\updater\x86
mkdir bin\Release\net5.0-windows\updater\ref
copy /B /Y "bin\Release\net5.0-windows\VCDiff.dll" "bin\Release\net5.0-windows\updater\VCDiff.dll"
copy /B /Y "bin\Debug\net5.0-windows\YamlDotNet.dll" "bin\Debug\net5.0-windows\updater\YamlDotNet.dll"
move /Y "bin\Release\net5.0-windows\TcNo-Acc-Switcher-Updater.runtimeconfig.json" "bin\Release\net5.0-windows\updater\TcNo-Acc-Switcher-Updater.runtimeconfig.json"
move /Y "bin\Release\net5.0-windows\TcNo-Acc-Switcher-Updater.runtimeconfig.dev.json" "bin\Release\net5.0-windows\updater\TcNo-Acc-Switcher-Updater.runtimeconfig.dev.json"
move /Y "bin\Release\net5.0-windows\TcNo-Acc-Switcher-Updater.pdb" "bin\Release\net5.0-windows\updater\TcNo-Acc-Switcher-Updater.pdb"
move /Y "bin\Release\net5.0-windows\TcNo-Acc-Switcher-Updater.exe" "bin\Release\net5.0-windows\updater\TcNo-Acc-Switcher-Updater.exe"
copy /B /Y "bin\Release\net5.0-windows\TcNo-Acc-Switcher-Updater.dll" "bin\Release\net5.0-windows\updater\TcNo-Acc-Switcher-Updater.dll"
move /Y "bin\Release\net5.0-windows\TcNo-Acc-Switcher-Updater.deps.json" "bin\Release\net5.0-windows\updater\TcNo-Acc-Switcher-Updater.deps.json"
copy /B /Y "bin\Release\net5.0-windows\SevenZipExtractor.dll" "bin\Release\net5.0-windows\updater\SevenZipExtractor.dll"
move /Y "bin\Release\net5.0-windows\x86\7z.dll" "bin\Release\net5.0-windows\updater\x86\7z.dll"
move /Y "bin\Release\net5.0-windows\x64\7z.dll" "bin\Release\net5.0-windows\updater\x64\7z.dll"
copy /B /Y "bin\Release\net5.0-windows\ref\TcNo-Acc-Switcher-Updater.dll" "bin\Release\net5.0-windows\updater\ref\TcNo-Acc-Switcher-Updater.dll"
copy /B /Y "bin\Release\net5.0-windows\Newtonsoft.Json.dll" "bin\Release\net5.0-windows\updater\Newtonsoft.Json.dll"
RMDIR /Q/S "bin\Release\net5.0-windows\runtimes\linux-musl-x64"
RMDIR /Q/S "bin\Release\net5.0-windows\runtimes\linux-x64"
RMDIR /Q/S "bin\Release\net5.0-windows\runtimes\osx"
RMDIR /Q/S "bin\Release\net5.0-windows\runtimes\osx-x64"
RMDIR /Q/S "bin\Release\net5.0-windows\runtimes\unix"
RMDIR /Q/S "bin\Release\net5.0-windows\runtimes\win-arm64"
RMDIR /Q "bin\Release\net5.0-windows\x64"
RMDIR /Q "bin\Release\net5.0-windows\x86"
copy /B /Y "..\..\Installer\_First_Run_Installer.exe" "_First_Run_Installer.exe"
REN "wwwroot" "originalwwwroot"
cd %origDir%
GOTO end

:end