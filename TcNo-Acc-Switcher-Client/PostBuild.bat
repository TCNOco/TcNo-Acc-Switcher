REM Move is currently only for build, as moving the files seems to prevent the program from running properly...

REM Get current directory:
echo Current directory: %cd%
set origDir=%cd%

REM SET VARIABLES
REM If SIGNTOOL environment variable is not set then try setting it to a known location
if "%SIGNTOOL%"=="" set SIGNTOOL=%ProgramFiles(x86)%\Windows Kits\10\bin\10.0.22000.0\x64\signtool.exe
REM Check to see if the signtool utility is missing
if exist "%SIGNTOOL%" goto ST
    REM Give error that SIGNTOOL environment variable needs to be set
    echo "Must set environment variable SIGNTOOL to full path for signtool.exe code signing utility"
    echo Location is of the form "C:\Program Files (x86)\Windows Kits\10\bin\10.0.22000.0\x64\bin\signtool.exe"
    exit -1
:ST

REM Set NSIS path for building the installer
if "%NSIS%"=="" set NSIS=%ProgramFiles(x86)%\NSIS\makensis.exe
if exist "%NSIS%" goto NS
    REM Give error that NSIS environment variable needs to be set
    echo "Must set environment variable NSIS to full path for makensis.exe"
    echo Location is of the form "C:\Program Files (x86)\NSIS\makensis.exe"
    IF NOT EXIST A:\AccountSwitcherConfig\sign.txt exit -1
:NS

REM Set 7-Zip path for compressing built files
if "%zip%"=="" set zip=C:\Program Files\7-Zip\7z.exe
if exist "%zip%" goto ZJ
    REM Give error that NSIS environment variable needs to be set
    echo "Must set environment variable 7-Zip to full path for 7z.exe"
    echo Location is of the form "C:\Program Files\7-Zip\7z.exe"
    exit -1
:ZJ
echo %origDir%

IF EXIST bin\x64\Release\net6.0-windows\updater GOTO end
cd %origDir%\bin\x64\Release\net6.0-windows\
ECHO -----------------------------------
ECHO Moving files for x64 Release in Visual Studio
ECHO -----------------------------------
mkdir updater
mkdir updater\x64
mkdir updater\x86
mkdir updater\ref
copy /B /Y "..\..\..\Installer\_First_Run_Installer.exe" "_First_Run_Installer.exe"
copy /B /Y "..\..\..\runas\x64\Release\net6.0\runas.exe" "runas.exe"
copy /B /Y "..\..\..\runas\x64\Release\net6.0\runas.dll" "runas.dll"
copy /B /Y "..\..\..\runas\x64\Release\net6.0\runas.runtimeconfig.json" "runas.runtimeconfig.json"

REM Signing
IF EXIST A:\AccountSwitcherConfig\sign.txt (
	ECHO Signing binaries
	echo %time%
	(
		start call ../../../../sign.bat "..\..\..\Wrapper\_Wrapper.exe"
		start call ../../../../sign.bat "_First_Run_Installer.exe"
		start call ../../../../sign.bat "runas.exe"
		start call ../../../../sign.bat "runas.dll"
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
) ELSE ECHO ----- SKIPPING SIGN -----


REN "TcNo-Acc-Switcher.exe" "TcNo-Acc-Switcher_main.exe"
REN "TcNo-Acc-Switcher-Server.exe" "TcNo-Acc-Switcher-Server_main.exe"
REN "TcNo-Acc-Switcher-Tray.exe" "TcNo-Acc-Switcher-Tray_main.exe"
move /Y "TcNo-Acc-Switcher-Updater.exe" "updater\TcNo-Acc-Switcher-Updater_main.exe"

copy /B /Y "..\..\..\Wrapper\_Wrapper.exe" "TcNo-Acc-Switcher.exe"
copy /B /Y "..\..\..\Wrapper\_Wrapper.exe" "TcNo-Acc-Switcher-Server.exe"
copy /B /Y "..\..\..\Wrapper\_Wrapper.exe" "TcNo-Acc-Switcher-Tray.exe"
copy /B /Y "..\..\..\Wrapper\_Wrapper.exe" "updater\TcNo-Acc-Switcher-Updater.exe"
copy /B /Y "_First_Run_Installer.exe" "updater\_First_Run_Installer.exe"

REM Copy in Server runtimes that are missing for some reason...
xcopy ..\..\..\..\..\TcNo-Acc-Switcher-Server\bin\Release\net6.0\runtimes\win\lib\net6.0 runtimes\win\lib\net6.0 /E /H /C /I /Y

echo %time%

copy /B /Y "VCDiff.dll" "updater\VCDiff.dll"
copy /B /Y "YamlDotNet.dll" "updater\YamlDotNet.dll"
move /Y "TcNo-Acc-Switcher-Updater.runtimeconfig.json" "updater\TcNo-Acc-Switcher-Updater.runtimeconfig.json"
move /Y "TcNo-Acc-Switcher-Updater.runtimeconfig.dev.json" "updater\TcNo-Acc-Switcher-Updater.runtimeconfig.dev.json"
move /Y "TcNo-Acc-Switcher-Updater.pdb" "updater\TcNo-Acc-Switcher-Updater.pdb"
copy /B /Y "TcNo-Acc-Switcher-Updater.dll" "updater\TcNo-Acc-Switcher-Updater.dll"
move /Y "TcNo-Acc-Switcher-Updater.deps.json" "updater\TcNo-Acc-Switcher-Updater.deps.json"
copy /B /Y "SevenZipExtractor.dll" "updater\SevenZipExtractor.dll"
copy /Y "x86\7z.dll" "updater\x86\7z.dll"
copy /Y "x64\7z.dll" "updater\x64\7z.dll"
copy /B /Y "ref\TcNo-Acc-Switcher-Updater.dll" "updater\ref\TcNo-Acc-Switcher-Updater.dll"
copy /B /Y "Microsoft.IO.RecyclableMemoryStream.dll" "updater\Microsoft.IO.RecyclableMemoryStream.dll"
copy /B /Y "Newtonsoft.Json.dll" "updater\Newtonsoft.Json.dll"
RMDIR /Q/S "runtimes\linux-arm64"
RMDIR /Q/S "runtimes\linux-musl-x64"
RMDIR /Q/S "runtimes\linux-x64"
RMDIR /Q/S "runtimes\osx"
RMDIR /Q/S "runtimes\osx-x64"
RMDIR /Q/S "runtimes\unix"
RMDIR /Q/S "runtimes\win-arm64"
RMDIR /Q/S "runtimes\win-x86"
REN "wwwroot" "originalwwwroot"
cd ..\
RMDIR /Q/S %origDir%\bin\x64\Release\TcNo-Acc-Switcher
REN "net6.0-windows" "TcNo-Acc-Switcher"

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
break > TcNo-Acc-Switcher\runtimes\win-x64\native\chrome_elf.dll
break > TcNo-Acc-Switcher\runtimes\win-x64\native\CefSharp.BrowserSubprocess.Core.dll

REM Verify signatures
IF EXIST A:\AccountSwitcherConfig\sign.txt (
	ECHO Verifying signatures of binaries
	powershell -ExecutionPolicy Unrestricted ../../../VerifySignatures.ps1
) ELSE ECHO ----- SKIPPING SIGNATURE VERIFICATION -----

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
IF EXIST A:\AccountSwitcherConfig\sign.txt (
	"%SIGNTOOL%" sign /tr http://timestamp.sectigo.com?td=sha256 /td SHA256 /fd SHA256 /a "TcNo Account Switcher - Installer.exe"
)

ECHO Moving CEF files BACK (for update)

copy /b/v/y CEF\libcef.dll TcNo-Acc-Switcher\runtimes\win-x64\native\libcef.dll
copy /b/v/y CEF\icudtl.dat TcNo-Acc-Switcher\runtimes\win-x64\native\icudtl.dat
copy /b/v/y CEF\resources.pak TcNo-Acc-Switcher\runtimes\win-x64\native\resources.pak
copy /b/v/y CEF\libGLESv2.dll TcNo-Acc-Switcher\runtimes\win-x64\native\libGLESv2.dll
copy /b/v/y CEF\d3dcompiler_47.dll TcNo-Acc-Switcher\runtimes\win-x64\native\d3dcompiler_47.dll
copy /b/v/y CEF\vk_swiftshader.dll TcNo-Acc-Switcher\runtimes\win-x64\native\vk_swiftshader.dll
copy /b/v/y CEF\chrome_elf.dll TcNo-Acc-Switcher\runtimes\win-x64\native\chrome_elf.dll
copy /b/v/y CEF\CefSharp.BrowserSubprocess.Core.dll TcNo-Acc-Switcher\runtimes\win-x64\native\CefSharp.BrowserSubprocess.Core.dll

REM create archive including CEF
echo Creating .7z archive
"%zip%" a -t7z -mmt24 -mx9  "TcNo-Acc-Switcher_and_CEF.7z" ".\TcNo-Acc-Switcher\*"

REM Verifying file sign state

cd %origDir%
goto :eof

:end