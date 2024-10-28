REM Resume where the last build script was
cd C:\projects\tcno-acc-switcher-qih5m\TcNo-Acc-Switcher-Client\bin\x64\Release\net8.0-windows7.0\

echo Copying files back from signing process (Thanks SignPath!)
COPY /B /Y "C:\projects\tcno-acc-switcher-qih5m\signed_artifacts\_Wrapper.exe" "C:\projects\tcno-acc-switcher-qih5m\TcNo-Acc-Switcher-Client\bin\Wrapper\_Wrapper.exe"
COPY /B /Y "C:\projects\tcno-acc-switcher-qih5m\signed_artifacts\_First_Run_Installer.exe" "_First_Run_Installer.exe"
COPY /B /Y "C:\projects\tcno-acc-switcher-qih5m\signed_artifacts\runas.exe" "runas.exe"
COPY /B /Y "C:\projects\tcno-acc-switcher-qih5m\signed_artifacts\runas.dll" "runas.dll"
COPY /B /Y "C:\projects\tcno-acc-switcher-qih5m\signed_artifacts\TcNo-Acc-Switcher.exe" "TcNo-Acc-Switcher.exe"
COPY /B /Y "C:\projects\tcno-acc-switcher-qih5m\signed_artifacts\TcNo-Acc-Switcher.dll" "TcNo-Acc-Switcher.dll"
COPY /B /Y "C:\projects\tcno-acc-switcher-qih5m\signed_artifacts\TcNo-Acc-Switcher-Server.exe" "TcNo-Acc-Switcher-Server.exe"
COPY /B /Y "C:\projects\tcno-acc-switcher-qih5m\signed_artifacts\TcNo-Acc-Switcher-Server.dll" "TcNo-Acc-Switcher-Server.dll"
COPY /B /Y "C:\projects\tcno-acc-switcher-qih5m\signed_artifacts\TcNo-Acc-Switcher-Tray.exe" "TcNo-Acc-Switcher-Tray.exe"
COPY /B /Y "C:\projects\tcno-acc-switcher-qih5m\signed_artifacts\TcNo-Acc-Switcher-Tray.dll" "TcNo-Acc-Switcher-Tray.dll"
COPY /B /Y "C:\projects\tcno-acc-switcher-qih5m\signed_artifacts\TcNo-Acc-Switcher-Globals.dll" "TcNo-Acc-Switcher-Globals.dll"
COPY /B /Y "C:\projects\tcno-acc-switcher-qih5m\signed_artifacts\TcNo-Acc-Switcher-Updater.exe" "TcNo-Acc-Switcher-Updater.exe"
COPY /B /Y "C:\projects\tcno-acc-switcher-qih5m\signed_artifacts\TcNo-Acc-Switcher-Updater.dll" "TcNo-Acc-Switcher-Updater.dll"


ECHO -----------------------------------
ECHO "Moving main .exes to make space for wrapper"
ECHO -----------------------------------
REN "TcNo-Acc-Switcher.exe" "TcNo-Acc-Switcher_main.exe"
REN "TcNo-Acc-Switcher-Server.exe" "TcNo-Acc-Switcher-Server_main.exe"
REN "TcNo-Acc-Switcher-Tray.exe" "TcNo-Acc-Switcher-Tray_main.exe"
move /Y "TcNo-Acc-Switcher-Updater.exe" "updater\TcNo-Acc-Switcher-Updater_main.exe"

copy /B /Y "C:\projects\tcno-acc-switcher-qih5m\TcNo-Acc-Switcher-Client\bin\Wrapper\_Wrapper.exe" "TcNo-Acc-Switcher.exe"
copy /B /Y "C:\projects\tcno-acc-switcher-qih5m\TcNo-Acc-Switcher-Client\bin\Wrapper\_Wrapper.exe" "TcNo-Acc-Switcher-Server.exe"
copy /B /Y "C:\projects\tcno-acc-switcher-qih5m\TcNo-Acc-Switcher-Client\bin\Wrapper\_Wrapper.exe" "TcNo-Acc-Switcher-Tray.exe"
copy /B /Y "C:\projects\tcno-acc-switcher-qih5m\TcNo-Acc-Switcher-Client\bin\Wrapper\_Wrapper.exe" "updater\TcNo-Acc-Switcher-Updater.exe"
copy /B /Y "_First_Run_Installer.exe" "updater\_First_Run_Installer.exe"

ECHO -----------------------------------
ECHO Copy in Server runtimes that are missing for some reason...
ECHO -----------------------------------
xcopy C:\projects\tcno-acc-switcher-qih5m\TcNo-Acc-Switcher-Server\bin\x64\Release\net8.0\runtimes\win\lib\net8.0 runtimes\win\lib\net8.0 /E /H /C /I /Y

ECHO -----------------------------------
ECHO Copying runtime files to updater
ECHO -----------------------------------
copy /B /Y "VCDiff.dll" "updater\VCDiff.dll"
copy /B /Y "YamlDotNet.dll" "updater\YamlDotNet.dll"
move /Y "TcNo-Acc-Switcher-Updater.runtimeconfig.json" "updater\TcNo-Acc-Switcher-Updater.runtimeconfig.json"
move /Y "TcNo-Acc-Switcher-Updater.pdb" "updater\TcNo-Acc-Switcher-Updater.pdb"
copy /B /Y "TcNo-Acc-Switcher-Updater.dll" "updater\TcNo-Acc-Switcher-Updater.dll"
move /Y "TcNo-Acc-Switcher-Updater.deps.json" "updater\TcNo-Acc-Switcher-Updater.deps.json"
copy /B /Y "SevenZipExtractor.dll" "updater\SevenZipExtractor.dll"
copy /Y "x86\7z.dll" "updater\x86\7z.dll"
copy /Y "x64\7z.dll" "updater\x64\7z.dll"
copy /B /Y "Microsoft.IO.RecyclableMemoryStream.dll" "updater\Microsoft.IO.RecyclableMemoryStream.dll"
copy /B /Y "Newtonsoft.Json.dll" "updater\Newtonsoft.Json.dll"


ECHO -----------------------------------
ECHO Removing unused files
ECHO -----------------------------------
if exist "runtimes\linux-arm64" RMDIR /Q/S "runtimes\linux-arm64"
if exist "runtimes\linux-musl-x64" RMDIR /Q/S "runtimes\linux-musl-x64"
if exist "runtimes\linux-x64" RMDIR /Q/S "runtimes\linux-x64"
if exist "runtimes\osx" RMDIR /Q/S "runtimes\osx"
if exist "runtimes\osx-x64" RMDIR /Q/S "runtimes\osx-x64"
if exist "runtimes\unix" RMDIR /Q/S "runtimes\unix"
if exist "runtimes\win-arm64" RMDIR /Q/S "runtimes\win-arm64"
if exist "runtimes\win-x86" RMDIR /Q/S "runtimes\win-x86"

ECHO -----------------------------------
ECHO Moving wwwroot and main program folder.
ECHO -----------------------------------
REN "wwwroot" "originalwwwroot"
cd ..\
ECHO -----------------------------------
ECHO Changed Directory to Build Dir (bin\x64\Release\)
ECHO %CD%
ECHO -----------------------------------
if exist "C:\projects\tcno-acc-switcher-qih5m\TcNo-Acc-Switcher-Client\bin\x64\Release\TcNo-Acc-Switcher\" (
    RMDIR /Q/S "C:\projects\tcno-acc-switcher-qih5m\TcNo-Acc-Switcher-Client\bin\x64\Release\TcNo-Acc-Switcher"
)
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

ECHO -----------------------------------
ECHO Compressing CEF and program files
ECHO -----------------------------------
@REM if "%SkipCEF%"=="true" goto COMPRESSEDCEF
@REM 	echo Creating .7z CEF archive
@REM 	"%zip%" a -t7z -mmt24 -mx9  "CEF.7z" ".\CEF\*"
@REM :COMPRESSEDCEF
echo Creating .7z archive
"%zip%" a -t7z -mmt24 -mx9  "TcNo-Acc-Switcher.7z" ".\TcNo-Acc-Switcher\*"
echo Done!

if "%SkipInstaller%"=="true" goto NSISSTEP
	ECHO -----------------------------------
	ECHO Creating installer
	ECHO -----------------------------------
	"%NSIS%" "C:\projects\tcno-acc-switcher-qih5m\other\NSIS\nsis-build-x64.nsi"
	echo Done. Moving...
	move /Y "C:\projects\tcno-acc-switcher-qih5m\other\NSIS\TcNo Account Switcher - Installer.exe" "TcNo Account Switcher - Installer.exe"
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


ECHO -----------------------------------
ECHO Preparing for update diff creation
ECHO -----------------------------------
mkdir OldVersion
call powershell -File "C:\projects\tcno-acc-switcher-qih5m\TcNo-Acc-Switcher-Client\PostBuildUpdate.ps1" -SolutionDir "C:\projects\tcno-acc-switcher-qih5m"



endlocal
echo -----------------------------------
echo DONE BUILDING RELEASE
echo -----------------------------------

if "%SkipSign%"=="true" (
    ECHO WARNING! Skipped Signing!
)

if "%SkipCEF%"=="true" (
    ECHO WARNING! Skipped CEF!
)

if "%SkipInstaller%"=="true" (
    ECHO WARNING! Skipped creating Installer!
)
goto :eof