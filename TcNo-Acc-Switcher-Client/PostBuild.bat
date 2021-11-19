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
RMDIR /Q/S "runtimes\win-x86"
RMDIR /Q x64
RMDIR /Q x86
copy /B /Y "..\..\..\Installer\_First_Run_Installer.exe" "_First_Run_Installer.exe"
REN "wwwroot" "originalwwwroot"
cd %origDir%
GOTO end

REM Move updater files in Release folder (for Visual Studio):
:vsRel
REM SET VARIABLES
REM If SIGNTOOL environment variable is not set then try setting it to a known location
if "%SIGNTOOL%"=="" set SIGNTOOL=%ProgramFiles(x86)%\Windows Kits\10\bin\10.0.19041.0\x64\signtool.exe
REM Check to see if the signtool utility is missing
if exist "%SIGNTOOL%" goto ST
    REM Give error that SIGNTOOL environment variable needs to be set
    echo "Must set environment variable SIGNTOOL to full path for signtool.exe code signing utility"
    echo Location is of the form "C:\Program Files (x86)\Windows Kits\10\bin\10.0.19041.0\x64\bin\signtool.exe"
    exit -1
:ST

REM Set NSIS path for building the installer
if "%NSIS%"=="" set NSIS=%ProgramFiles(x86)%\NSIS\makensis.exe
if exist "%NSIS%" goto NS
    REM Give error that NSIS environment variable needs to be set
    echo "Must set environment variable NSIS to full path for makensis.exe"
    echo Location is of the form "C:\Program Files (x86)\NSIS\makensis.exe"
    exit -1
:NS


REM Set 7-Zip path for compressing built files
if "%zip%"=="" set zip=C:\Program Files\7-Zip\7z.exe
if exist "%zip%" goto ZJ
    REM Give error that NSIS environment variable needs to be set
    echo "Must set environment variable 7-Zip to full path for 7z.exe"
    echo Location is of the form "C:\Program Files\7-Zip\7z.exe"
    exit -1
:ZJ

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
copy /B /Y "..\..\..\Installer\_First_Run_Installer.exe" "_First_Run_Installer.exe"
REM Signing
ECHO Signing binaries
echo %time%

REM GOTO :skipsign

(
    start call ../../../../sign.bat "_First_Run_Installer.exe"
    start call ../../../../sign.bat "TcNo-Acc-Switcher.exe"
    start call ../../../../sign.bat "TcNo-Acc-Switcher.dll"
    start call ../../../../sign.bat "TcNo-Acc-Switcher-Server.exe"
    start call ../../../../sign.bat "TcNo-Acc-Switcher-Server.dll"
    start call ../../../../sign.bat "TcNo-Acc-Switcher-Tray.exe"
    start call ../../../../sign.bat "TcNo-Acc-Switcher-Tray.dll"
    start call ../../../../sign.bat "TcNo-Acc-Switcher-Globals.dll"
    start call ../../../../sign.bat "TcNo-Acc-Switcher-Updater.exe"
    start call ../../../../sign.bat "TcNo-Acc-Switcher-Updater.dll"
) | set /P "="

echo %time%

:skipsign
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
RMDIR /Q/S "runtimes\win-x86"
RMDIR /Q x64
RMDIR /Q x86
REN "wwwroot" "originalwwwroot"
cd ..\
RMDIR /Q/S %origDir%\bin\x64\Release\TcNo-Acc-Switcher
REN "net5.0-windows" "TcNo-Acc-Switcher"

REM Copy out updater for update creation
xcopy TcNo-Acc-Switcher\updater updater /E /H /C /I /Y

REM Move win-x64 runtimes for CEF download & smaller main download
ECHO Moving CEF files
mkdir CEF
move TcNo-Acc-Switcher\runtimes\win-x64\native\libcef.dll CEF\libcef.dll
move TcNo-Acc-Switcher\runtimes\win-x64\native\icudtl.dat CEF\icudtl.dat
move TcNo-Acc-Switcher\runtimes\win-x64\native\resources.pak CEF\resources.pak
move TcNo-Acc-Switcher\runtimes\win-x64\native\libGLESv2.dll CEF\libGLESv2.dll
move TcNo-Acc-Switcher\runtimes\win-x64\native\d3dcompiler_47.dll CEF\d3dcompiler_47.dll
move TcNo-Acc-Switcher\runtimes\win-x64\native\vk_swiftshader.dll CEF\vk_swiftshader.dll
move TcNo-Acc-Switcher\runtimes\win-x64\native\CefSharp.dll CEF\CefSharp.dll
move TcNo-Acc-Switcher\runtimes\win-x64\native\chrome_elf.dll CEF\chrome_elf.dll
move TcNo-Acc-Switcher\runtimes\win-x64\native\CefSharp.BrowserSubprocess.Core.dll CEF\CefSharp.BrowserSubprocess.Core.dll

REM Create placeholders
ECHO Creating CEF placeholders
break > TcNo-Acc-Switcher\runtimes\win-x64\native\libcef.dll
break > TcNo-Acc-Switcher\runtimes\win-x64\native\icudtl.dat
break > TcNo-Acc-Switcher\runtimes\win-x64\native\resources.pak
break > TcNo-Acc-Switcher\runtimes\win-x64\native\libGLESv2.dll
break > TcNo-Acc-Switcher\runtimes\win-x64\native\d3dcompiler_47.dll
break > TcNo-Acc-Switcher\runtimes\win-x64\native\vk_swiftshader.dll
break > TcNo-Acc-Switcher\runtimes\win-x64\native\CefSharp.dll
break > TcNo-Acc-Switcher\runtimes\win-x64\native\chrome_elf.dll
break > TcNo-Acc-Switcher\runtimes\win-x64\native\CefSharp.BrowserSubprocess.Core.dll

REM Compress files
echo Creating .7z CEF archive
"%zip%" a -t7z -mmt24 -mx9  "CEF.7z" ".\CEF\*"
echo Creating .7z archive
"%zip%" a -t7z -mmt24 -mx9  "TcNo-Acc-Switcher.7z" ".\TcNo-Acc-Switcher\*"
echo Done!

REM Create installer
echo Creating installer
"%NSIS%" "%origDir%\..\other\NSIS\nsis-build-x64.nsi"
echo Done. Moving...
move /Y "..\..\..\..\other\NSIS\TcNo Account Switcher - Installer.exe" "TcNo Account Switcher - Installer.exe"
"%SIGNTOOL%" sign /tr http://timestamp.sectigo.com?td=sha256 /td SHA256 /fd SHA256 /a "TcNo Account Switcher - Installer.exe"

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
RMDIR /Q/S "bin\Debug\net5.0-windows\runtimes\win-x86"
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
RMDIR /Q/S "bin\Release\net5.0-windows\runtimes\win-x86"
RMDIR /Q "bin\Release\net5.0-windows\x64"
RMDIR /Q "bin\Release\net5.0-windows\x86"
copy /B /Y "..\..\Installer\_First_Run_Installer.exe" "_First_Run_Installer.exe"
REN "wwwroot" "originalwwwroot"
cd %origDir%
GOTO end

:end