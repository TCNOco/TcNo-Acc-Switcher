REM Move is currently only for build, as moving the files seems to prevent the program from running properly...
setlocal
set SkipSign=true
set SkipCEF=false
set SkipInstaller=false

REM SkipSign true while no certificate. SignPath can help, but requires GitHub build action, like AppVeyor. See AppVeyor branch.
REM SkipFEC and SkipInstaller should be false for full normal build.

echo -----------------------------------
set ConfigurationName=%1
set ProjectDirClient=%2

REM Navigate up one folder
for %%I in ("%ProjectDirClient%") do set ProjectDir=%%~dpI
set SolutionDir=%ProjectDir%..
pushd "%SolutionDir%" >nul
set SolutionDir=%CD%\
popd >nul

echo Running Postbuild!
echo Configuration: "%ConfigurationName%"
echo (Client) Project Directory: "%ProjectDirClient%"
echo Solution Directory: "%SolutionDir%"
REM Project Directory should be: TcNo-Acc-Switcher-Client\
cd %SolutionDir%\TcNo-Acc-Switcher-Client
echo -----------------------------------
if /I "%ConfigurationName%" == "Release" (
	GOTO :release
) else (
	GOTO :debug
)

:debug
echo -----------------------------------
echo BUILDING DEBUG
echo -----------------------------------
mkdir updater
mkdir updater\x64
mkdir updater\x86
mkdir updater\ref
echo %cd%
copy /B /Y "%SolutionDir%\TcNo-Acc-Switcher-Client\bin\Installer\_First_Run_Installer.exe" "%SolutionDir%\TcNo-Acc-Switcher-Client\bin\x64\Debug\net8.0-windows7.0\_First_Run_Installer.exe"
copy /B /Y "%SolutionDir%\TcNo-Acc-Switcher-Client\bin\runas\x64\Release\net8.0\runas.exe" "%SolutionDir%\TcNo-Acc-Switcher-Client\bin\x64\Debug\net8.0-windows7.0\runas.exe"
copy /B /Y "%SolutionDir%\TcNo-Acc-Switcher-Client\bin\runas\x64\Release\net8.0\runas.dll" "%SolutionDir%\TcNo-Acc-Switcher-Client\bin\x64\Debug\net8.0-windows7.0\runas.dll"
copy /B /Y "%SolutionDir%\TcNo-Acc-Switcher-Client\bin\runas\x64\Release\net8.0\runas.runtimeconfig.json" "%SolutionDir%\TcNo-Acc-Switcher-Client\bin\x64\Debug\net8.0-windows7.0\runas.runtimeconfig.json"
endlocal
echo -----------------------------------
echo DONE BUILDING DEBUG
echo -----------------------------------
goto :eof


:release
echo -----------------------------------
echo BUILDING RELEASE
echo -----------------------------------

REM Get current directory:
echo -----------------------------------
echo Current directory: %cd%
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

IF EXIST bin\x64\Release\net8.0-windows7.0\updater GOTO end
cd %SolutionDir%\TcNo-Acc-Switcher-Client\bin\x64\Release\net8.0-windows7.0\
ECHO -----------------------------------
ECHO Moving files for x64 Release in Visual Studio
ECHO CD into build artifact directory
ECHO %CD%
ECHO -----------------------------------
mkdir updater
mkdir updater\x64
mkdir updater\x86
mkdir updater\ref
copy /B /Y "%SolutionDir%\TcNo-Acc-Switcher-Client\bin\Installer\_First_Run_Installer.exe" "_First_Run_Installer.exe"
copy /B /Y "%SolutionDir%\TcNo-Acc-Switcher-Client\bin\runas\x64\Release\net8.0\runas.exe" "runas.exe"
copy /B /Y "%SolutionDir%\TcNo-Acc-Switcher-Client\bin\runas\x64\Release\net8.0\runas.dll" "runas.dll"
copy /B /Y "%SolutionDir%\TcNo-Acc-Switcher-Client\bin\runas\x64\Release\net8.0\runas.runtimeconfig.json" "runas.runtimeconfig.json"

REM Move files to a new directory for signing
mkdir "%SolutionDir%\to_sign"
COPY /B /Y "%SolutionDir%\TcNo-Acc-Switcher-Client\bin\Wrapper\_Wrapper.exe" "%SolutionDir%\to_sign\_Wrapper.exe"
COPY /B /Y "_First_Run_Installer.exe" "%SolutionDir%\to_sign\_First_Run_Installer.exe"
COPY /B /Y "runas.exe" "%SolutionDir%\to_sign\runas.exe"
COPY /B /Y "runas.dll" "%SolutionDir%\to_sign\runas.dll"
COPY /B /Y "TcNo-Acc-Switcher.exe" "%SolutionDir%\to_sign\TcNo-Acc-Switcher.exe"
COPY /B /Y "TcNo-Acc-Switcher.dll" "%SolutionDir%\to_sign\TcNo-Acc-Switcher.dll"
COPY /B /Y "TcNo-Acc-Switcher-Server.exe" "%SolutionDir%\to_sign\TcNo-Acc-Switcher-Server.exe"
COPY /B /Y "TcNo-Acc-Switcher-Server.dll" "%SolutionDir%\to_sign\TcNo-Acc-Switcher-Server.dll"
COPY /B /Y "TcNo-Acc-Switcher-Tray.exe" "%SolutionDir%\to_sign\TcNo-Acc-Switcher-Tray.exe"
COPY /B /Y "TcNo-Acc-Switcher-Tray.dll" "%SolutionDir%\to_sign\TcNo-Acc-Switcher-Tray.dll"
COPY /B /Y "TcNo-Acc-Switcher-Globals.dll" "%SolutionDir%\to_sign\TcNo-Acc-Switcher-Globals.dll"
COPY /B /Y "TcNo-Acc-Switcher-Updater.exe" "%SolutionDir%\to_sign\TcNo-Acc-Switcher-Updater.exe"
COPY /B /Y "TcNo-Acc-Switcher-Updater.dll" "%SolutionDir%\to_sign\TcNo-Acc-Switcher-Updater.dll"

REM Signing takes place on AppVeyor now.
REM Thn PostBuildFinalize.bat is run by the script.
REM %SolutionDir$ is C:\projects\tcno-acc-switcher-qih5m\