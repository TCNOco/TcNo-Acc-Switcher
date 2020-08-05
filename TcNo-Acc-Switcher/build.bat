
@echo off
@setlocal enableDelayedExpansion

@set sourcedir=%1
@set packagesetup=%2
@set electronbuildersetup=%3
@set buildplatforms=%4



if %sourcedir%==help (
    GOTO help
) else (
    GOTO no_help
)

:no_help
for /F "tokens=* usebackq" %%A in (%packagesetup%) do call set "packagejson=%%packagejson%% %%A"
@echo %packagejson% >"%sourcedir:~1,-1%\package.json"
for /F "tokens=* usebackq" %%A in (%electronbuildersetup%) do call set "electronbuilderjson=%%electronbuilderjson%% %%A"
@echo %electronbuilderjson% > "%sourcedir:~1,-1%\electron-builder.json"
@echo "Config File Written, Starting build"
@echo %buildplatforms:~1,-1%

call electron-builder build --config ./electron-builder.json --project %sourcedir% %buildplatforms:~1,-1%
GOTO done

:help
@echo  ------------------------
@echo  ------------------------
@echo The current batch script requires the following parameters in order:
@echo  ------------------------
@echo  ------------------------
@echo ./build.bat builddirpath  packagesetuppath electronbuildersetuppath electronbuilderclicommands
@echo  ------------------------
@echo  ------------------------
@echo builddir = Path to build directory within double quotes.
@echo name = name of your appliation 
@echo version = version of your application
@echo packagesetuppath = custom package.json to overwrite the existing one, if you want to use the existing one, just use that file path.
@echo electronbuildersetuppath = custom electron-builder configuration to overwrite default values.
@echo electronbuilderclicommands = to set extra cli commands used by electron-builder
@echo  ------------------------
@echo  ------------------------
@echo  EXAMPLE:
@echo  ------------------------
@echo  ------------------------
@echo ./build.bat "path/to/build/dir" "/path/to/package.json" "/path/to/electron-builder.json" "--windows --linux"
@echo  ------------------------
@echo  ------------------------
@echo  WARNING:
@echo  ------------------------
@echo  ------------------------
@echo  When you have custom dependencies within package.json, make sure when using a custom package.json file to make the match up with the generated package.json file!
@echo  ------------------------
@echo  ------------------------
GOTO done

:done
@echo done
