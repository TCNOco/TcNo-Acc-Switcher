# Installer for the NSIS 7z plugin.
# More info on this plugin: https://nsis.sourceforge.io/Nsis7z_plug-in
# This is used in the AppVeyor build process.
# Results in a much smaller compressed install file. Worth the time and effort!

$downloadUrl = "https://nsis.sourceforge.io/mediawiki/images/6/69/Nsis7z_19.00.7z"
$destinationFolder = "C:\Program Files (x86)\NSIS"
$tempDownloadPath = "$env:TEMP\Nsis7z_19.00.7z"
$extractPath = "$env:TEMP\NsisExtracted"

Invoke-WebRequest -Uri $downloadUrl -OutFile $tempDownloadPath

# Create a temporary extraction folder
if (-Not (Test-Path -Path $extractPath)) {
    New-Item -ItemType Directory -Path $extractPath | Out-Null
}

# Extract the 7z file (requires 7-Zip installed and in PATH)
& 7z x "$tempDownloadPath" -o"$extractPath" -y

# List of folders to extract and merge
$foldersToMerge = @("Contrib", "Examples", "Plugins")

# Merge the extracted folders into the destination
foreach ($folder in $foldersToMerge) {
    $sourcePath = Join-Path -Path $extractPath -ChildPath $folder
    $destinationPath = Join-Path -Path $destinationFolder -ChildPath $folder

    if (Test-Path -Path $sourcePath) {
        Copy-Item -Path $sourcePath\* -Destination $destinationPath -Recurse -Force
    }
}

# Clean up temporary files
Remove-Item -Path $tempDownloadPath -Force
Remove-Item -Path $extractPath -Recurse -Force

Write-Host "Download, extraction, and merge complete."
