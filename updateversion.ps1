# Define the paths to the NSIS script file and the Installer RC file

$dateVersion = Read-Host "Enter the date version number (format: YYYY-MM-DD_VV)"
$nsisVersion = $dateVersion -replace "-", "." -replace "_", "."
$rcVersion = $dateVersion -replace "-", "," -replace "_", ","

# -------------------------------
# Update the NSIS script file
# -------------------------------

$nsisFilePath = "other\NSIS\nsis-build-x64.nsi"
$nsisFileContent = Get-Content $nsisFilePath
$nsisReplacement = '!define VERSION "' + $nsisVersion + '"'
$nsisFileContent = $nsisFileContent -replace '^!define VERSION\s+".*"$', $nsisReplacement
$nsisFileContent | Set-Content $nsisFilePath

Write-Host "UPDATED NSIS"

# -------------------------------
# Update the Installer RC file
# -------------------------------

$rcFilePath = "Installer\Installer.rc"
$rcFileContent = Get-Content $rcFilePath

$rcProductVersionReplacement = ' PRODUCTVERSION ' + $rcVersion
$rcFileVersionReplacement = ' FILEVERSION ' + $rcVersion

$rcValueProductVersionReplacement = '            VALUE "ProductVersion", "' + $nsisVersion + '"'
$rcValueFileVersionReplacement = '            VALUE "FileVersion", "' + $nsisVersion + '"'

$rcFileContent = $rcFileContent -replace '^\s*PRODUCTVERSION\s+\d+,\d+,\d+,\d+', $rcProductVersionReplacement
$rcFileContent = $rcFileContent -replace '^\s*FILEVERSION\s+\d+,\d+,\d+,\d+', $rcFileVersionReplacement
$rcFileContent = $rcFileContent -replace '^\s*VALUE "ProductVersion",\s*"\d+\.\d+\.\d+\.\d+"', $rcValueProductVersionReplacement
$rcFileContent = $rcFileContent -replace '^\s*VALUE "FileVersion",\s*"\d+\.\d+\.\d+\.\d+"', $rcValueFileVersionReplacement

$rcFileContent | Set-Content $rcFilePath

Write-Host "UPDATED Installer.rc"

# -------------------------------
# Update the Wrapper RC file
# -------------------------------

$rcFilePath = "_Updater_Wrapper\_Wrapper.rc"
$rcFileContent = Get-Content $rcFilePath

$rcProductVersionReplacement = ' PRODUCTVERSION ' + $rcVersion
$rcFileVersionReplacement = ' FILEVERSION ' + $rcVersion

$rcValueProductVersionReplacement = '            VALUE "ProductVersion", "' + $nsisVersion + '"'
$rcValueFileVersionReplacement = '            VALUE "FileVersion", "' + $nsisVersion + '"'

$rcFileContent = $rcFileContent -replace '^\s*PRODUCTVERSION\s+\d+,\d+,\d+,\d+', $rcProductVersionReplacement
$rcFileContent = $rcFileContent -replace '^\s*FILEVERSION\s+\d+,\d+,\d+,\d+', $rcFileVersionReplacement
$rcFileContent = $rcFileContent -replace '^\s*VALUE "ProductVersion",\s*"\d+\.\d+\.\d+\.\d+"', $rcValueProductVersionReplacement
$rcFileContent = $rcFileContent -replace '^\s*VALUE "FileVersion",\s*"\d+\.\d+\.\d+\.\d+"', $rcValueFileVersionReplacement

$rcFileContent | Set-Content $rcFilePath

Write-Host "UPDATED Wrapper.rc"

# -------------------------------
# Update the TcNo-Acc-Switcher-Client.csproj file
# -------------------------------

$csprojFilePath = "TcNo-Acc-Switcher-Client\TcNo-Acc-Switcher-Client.csproj"
$csprojFileContent = Get-Content $csprojFilePath
$csprojVersionReplacement = '<Version>' + $nsisVersion + '</Version>'
$csprojFileContent = $csprojFileContent -replace '<Version>.*</Version>', $csprojVersionReplacement
$csprojVersionReplacement = '<AssemblyVersion>' + $nsisVersion + '</AssemblyVersion>'
$csprojFileContent = $csprojFileContent -replace '<AssemblyVersion>.*</AssemblyVersion>', $csprojVersionReplacement

$csprojFileContent | Set-Content $csprojFilePath

Write-Host "UPDATED TcNo-Acc-Switcher-Client.csproj"

# -------------------------------
# Update the TcNo-Acc-Switcher-Server.csproj file
# -------------------------------

$csprojFilePath = "TcNo-Acc-Switcher-Server\TcNo-Acc-Switcher-Server.csproj"

$csprojFileContent = Get-Content $csprojFilePath
$csprojVersionReplacement = '<Version>' + $nsisVersion + '</Version>'
$csprojFileContent = $csprojFileContent -replace '<Version>.*</Version>', $csprojVersionReplacement
$csprojFileContent | Set-Content $csprojFilePath

Write-Host "UPDATED TcNo-Acc-Switcher-Server.csproj"

# -------------------------------
# Update the Globals.cs file
# -------------------------------

$globalsFilePath = "TcNo-Acc-Switcher-Globals\Globals.cs"
$globalsFileContent = Get-Content $globalsFilePath
$globalsVersionReplacement = 'public static readonly string Version = "' + $dateVersion + '";'
$globalsFileContent = $globalsFileContent -replace 'public static readonly string Version\s*=\s*".*";', $globalsVersionReplacement
$globalsFileContent | Set-Content $globalsFilePath

Write-Host "UPDATED Globals.cs"