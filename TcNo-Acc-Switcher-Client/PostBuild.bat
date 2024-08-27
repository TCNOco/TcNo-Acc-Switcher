REM Move is currently only for build, as moving the files seems to prevent the program from running properly...
setlocal
set SkipSign=true
set SkipCEF=false
set SkipInstaller=false

REM SkipSign true while no certificate. SignPath can help, but requires GitHub build action, like AppVeyor. See AppVeyor branch.
REM SkipFEC and SkipInstaller should be false for full normal build.

echo -----------------------------------
set ConfigurationName=%1
set ProjectDir=%2
echo Running Postbuild!
echo Configuration: "%ConfigurationName%"
echo Project Directory: "%ProjectDir%"
echo -----------------------------------
if /I "%ConfigurationName%" == "Release" (
	GOTO :release
) else (
	GOTO :debug
)


:release
echo -----------------------------------
echo BUILDING RELEASE
echo -----------------------------------

REM Get current directory:
echo -----------------------------------
echo Current directory: %cd%
set origDir=%cd%
echo Current time: %time%
echo -----------------------------------

REM SET VARIABLES
if "%SkipSign%"=="true" goto ST
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
if "%SkipInstaller%"=="true" goto NS
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
echo Original DIR: %origDir%

IF EXIST bin\x64\Release\net8.0-windows7.0\updater GOTO end
cd %origDir%\bin\x64\Release\net8.0-windows7.0\
ECHO -----------------------------------
ECHO Moving files for x64 Release in Visual Studio
ECHO %CD%
ECHO -----------------------------------
mkdir updater
mkdir updater\x64
mkdir updater\x86
mkdir updater\ref
copy /B /Y "..\..\..\Installer\_First_Run_Installer.exe" "_First_Run_Installer.exe"
copy /B /Y "..\..\..\runas\x64\Release\net8.0\runas.exe" "runas.exe"
copy /B /Y "..\..\..\runas\x64\Release\net8.0\runas.dll" "runas.dll"
copy /B /Y "..\..\..\runas\x64\Release\net8.0\runas.runtimeconfig.json" "runas.runtimeconfig.json"

REM Signing
REM Currently disabled as I do not have the funds to renew my certificate.
if "%SkipSign%"=="true" goto POSTSIGN
	ECHO -----------------------------------
	ECHO Signing binaries
	ECHO -----------------------------------
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
:POSTSIGN

ECHO -----------------------------------
ECHO "Moving main .exes to make space for wrapper"
ECHO -----------------------------------
REN "TcNo-Acc-Switcher.exe" "TcNo-Acc-Switcher_main.exe"
REN "TcNo-Acc-Switcher-Server.exe" "TcNo-Acc-Switcher-Server_main.exe"
REN "TcNo-Acc-Switcher-Tray.exe" "TcNo-Acc-Switcher-Tray_main.exe"
move /Y "TcNo-Acc-Switcher-Updater.exe" "updater\TcNo-Acc-Switcher-Updater_main.exe"

copy /B /Y "..\..\..\Wrapper\_Wrapper.exe" "TcNo-Acc-Switcher.exe"
copy /B /Y "..\..\..\Wrapper\_Wrapper.exe" "TcNo-Acc-Switcher-Server.exe"
copy /B /Y "..\..\..\Wrapper\_Wrapper.exe" "TcNo-Acc-Switcher-Tray.exe"
copy /B /Y "..\..\..\Wrapper\_Wrapper.exe" "updater\TcNo-Acc-Switcher-Updater.exe"
copy /B /Y "_First_Run_Installer.exe" "updater\_First_Run_Installer.exe"

ECHO -----------------------------------
ECHO Copy in Server runtimes that are missing for some reason...
ECHO -----------------------------------
xcopy ..\..\..\..\..\TcNo-Acc-Switcher-Server\bin\x64\Release\net8.0\runtimes\win\lib\net8.0 runtimes\win\lib\net8.0 /E /H /C /I /Y

ECHO -----------------------------------
ECHO Copying runtime files to updater
ECHO -----------------------------------
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


ECHO -----------------------------------
ECHO Removing unused files
ECHO -----------------------------------
RMDIR /Q/S "runtimes\linux-arm64"
RMDIR /Q/S "runtimes\linux-musl-x64"
RMDIR /Q/S "runtimes\linux-x64"
RMDIR /Q/S "runtimes\osx"
RMDIR /Q/S "runtimes\osx-x64"
RMDIR /Q/S "runtimes\unix"
RMDIR /Q/S "runtimes\win-arm64"
RMDIR /Q/S "runtimes\win-x86"

ECHO -----------------------------------
ECHO Moving wwwroot and main program folder.
ECHO -----------------------------------
REN "wwwroot" "originalwwwroot"
cd ..\
RMDIR /Q/S %origDir%\bin\x64\Release\TcNo-Acc-Switcher
REN "net8.0-windows7.0" "TcNo-Acc-Switcher"

ECHO -----------------------------------
ECHO Copy out updater for update creation
ECHO -----------------------------------
xcopy TcNo-Acc-Switcher\updater updater /E /H /C /I /Y

ECHO -----------------------------------
ECHO "Move win-x64 runtimes for CEF download & smaller main download"
ECHO -----------------------------------
mkdir CEF
move TcNo-Acc-Switcher\runtimes\win-x64\native\libcef.dll CEF\libcef.dll
move TcNo-Acc-Switcher\runtimes\win-x64\native\icudtl.dat CEF\icudtl.dat
move TcNo-Acc-Switcher\runtimes\win-x64\native\resources.pak CEF\resources.pak
move TcNo-Acc-Switcher\runtimes\win-x64\native\libGLESv2.dll CEF\libGLESv2.dll
move TcNo-Acc-Switcher\runtimes\win-x64\native\d3dcompiler_47.dll CEF\d3dcompiler_47.dll
move TcNo-Acc-Switcher\runtimes\win-x64\native\vk_swiftshader.dll CEF\vk_swiftshader.dll
move TcNo-Acc-Switcher\runtimes\win-x64\native\chrome_elf.dll CEF\chrome_elf.dll
move TcNo-Acc-Switcher\runtimes\win-x64\native\CefSharp.BrowserSubprocess.Core.dll CEF\CefSharp.BrowserSubprocess.Core.dll

ECHO -----------------------------------
ECHO Creating CEF placeholders
ECHO -----------------------------------
break > TcNo-Acc-Switcher\runtimes\win-x64\native\libcef.dll
break > TcNo-Acc-Switcher\runtimes\win-x64\native\icudtl.dat
break > TcNo-Acc-Switcher\runtimes\win-x64\native\resources.pak
break > TcNo-Acc-Switcher\runtimes\win-x64\native\libGLESv2.dll
break > TcNo-Acc-Switcher\runtimes\win-x64\native\d3dcompiler_47.dll
break > TcNo-Acc-Switcher\runtimes\win-x64\native\vk_swiftshader.dll
break > TcNo-Acc-Switcher\runtimes\win-x64\native\chrome_elf.dll
break > TcNo-Acc-Switcher\runtimes\win-x64\native\CefSharp.BrowserSubprocess.Core.dll

REM Verify signatures
if "%SkipSign%"=="true" goto POSTVERIFY
	ECHO Verifying signatures of binaries
	powershell -ExecutionPolicy Unrestricted ../../../VerifySignatures.ps1
:POSTVERIFY

ECHO -----------------------------------
ECHO Compressing CEF and program files
ECHO -----------------------------------
if "%SkipCEF%"=="true" goto COMPRESSEDCEF
	echo Creating .7z CEF archive
	"%zip%" a -t7z -mmt24 -mx9  "CEF.7z" ".\CEF\*"
:COMPRESSEDCEF
echo Creating .7z archive
"%zip%" a -t7z -mmt24 -mx9  "TcNo-Acc-Switcher.7z" ".\TcNo-Acc-Switcher\*"
echo Done!

if "%SkipInstaller%"=="true" goto NSISSTEP
	ECHO -----------------------------------
	ECHO Creating installer
	ECHO -----------------------------------
	"%NSIS%" "%origDir%\..\other\NSIS\nsis-build-x64.nsi"
	echo Done. Moving...
	move /Y "..\..\..\..\other\NSIS\TcNo Account Switcher - Installer.exe" "TcNo Account Switcher - Installer.exe"
	if "%SkipSign%"=="false" (
		"%SIGNTOOL%" sign /tr http://timestamp.sectigo.com?td=sha256 /td SHA256 /fd SHA256 /a "TcNo Account Switcher - Installer.exe"
	)
:NSISSTEP

ECHO -----------------------------------
ECHO Moving CEF files BACK (for update)
ECHO -----------------------------------
copy /b/v/y CEF\libcef.dll TcNo-Acc-Switcher\runtimes\win-x64\native\libcef.dll
copy /b/v/y CEF\icudtl.dat TcNo-Acc-Switcher\runtimes\win-x64\native\icudtl.dat
copy /b/v/y CEF\resources.pak TcNo-Acc-Switcher\runtimes\win-x64\native\resources.pak
copy /b/v/y CEF\libGLESv2.dll TcNo-Acc-Switcher\runtimes\win-x64\native\libGLESv2.dll
copy /b/v/y CEF\d3dcompiler_47.dll TcNo-Acc-Switcher\runtimes\win-x64\native\d3dcompiler_47.dll
copy /b/v/y CEF\vk_swiftshader.dll TcNo-Acc-Switcher\runtimes\win-x64\native\vk_swiftshader.dll
copy /b/v/y CEF\chrome_elf.dll TcNo-Acc-Switcher\runtimes\win-x64\native\chrome_elf.dll
copy /b/v/y CEF\CefSharp.BrowserSubprocess.Core.dll TcNo-Acc-Switcher\runtimes\win-x64\native\CefSharp.BrowserSubprocess.Core.dll

if "%SkipCEF%"=="true" goto COMPRESSEDCOMBINED
	ECHO -----------------------------------
	ECHO Creating .7z archive
	ECHO -----------------------------------
	"%zip%" a -t7z -mmt24 -mx9  "TcNo-Acc-Switcher_and_CEF.7z" ".\TcNo-Acc-Switcher\*"
:COMPRESSEDCOMBINED

cd %origDir%

if "%SkipSign%"=="true" (
    ECHO WARNING! Skipped Signing!
)

if "%SkipCEF%"=="true" (
    ECHO WARNING! Skipped CEF!
)

if "%SkipInstaller%"=="true" (
    ECHO WARNING! Skipped creating Installer!
)
endlocal
echo -----------------------------------
echo DONE BUILDING RELEASE
echo -----------------------------------
goto :eof




:debug
echo -----------------------------------
echo BUILDING DEBUG
echo -----------------------------------
mkdir updater
mkdir updater\x64
mkdir updater\x86
mkdir updater\ref
echo %cd%
copy /B /Y "bin\Installer\_First_Run_Installer.exe" "bin\x64\Debug\net8.0-windows7.0\_First_Run_Installer.exe"
copy /B /Y "bin\runas\x64\Release\net8.0\runas.exe" "bin\x64\Debug\net8.0-windows7.0\runas.exe"
copy /B /Y "bin\runas\x64\Release\net8.0\runas.dll" "bin\x64\Debug\net8.0-windows7.0\runas.dll"
copy /B /Y "bin\runas\x64\Release\net8.0\runas.runtimeconfig.json" "bin\x64\Debug\net8.0-windows7.0\runas.runtimeconfig.json"
endlocal
echo -----------------------------------
echo DONE BUILDING DEBUG
echo -----------------------------------
goto :eof