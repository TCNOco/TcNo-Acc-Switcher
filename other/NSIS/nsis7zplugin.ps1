# Installer for the NSIS 7z plugin.
# More info: https://nsis.sourceforge.io/Nsis7z_plug-in
# Bundled in-repo (Nsis7z_19.00.7z) because nsis.sourceforge.io blocks CI with Cloudflare.

$ErrorActionPreference = 'Stop'

$nsis7zPlugin = 'C:\Program Files (x86)\NSIS\Plugins\x86-unicode\nsis7z.dll'
if (Test-Path $nsis7zPlugin) {
    Write-Host "NSIS 7z plugin already present; skipping install."
    return
}

$destinationFolder = 'C:\Program Files (x86)\NSIS'
$bundledArchive = Join-Path $PSScriptRoot 'Nsis7z_19.00.7z'
if (-not (Test-Path $bundledArchive)) {
    throw "Bundled NSIS 7z plugin not found: $bundledArchive"
}

$tempDownloadPath = Join-Path $env:TEMP 'Nsis7z_19.00.7z'
$extractPath = Join-Path $env:TEMP 'NsisExtracted'

Write-Host "Installing NSIS 7z plugin from bundled archive..."
Copy-Item -Path $bundledArchive -Destination $tempDownloadPath -Force

if (-not (Test-Path $extractPath)) {
    New-Item -ItemType Directory -Path $extractPath | Out-Null
}

& 7z x "$tempDownloadPath" -o"$extractPath" -y
if ($LASTEXITCODE -ne 0) {
    throw "7z failed to extract NSIS 7z plugin (exit $LASTEXITCODE)."
}

$foldersToMerge = @('Contrib', 'Examples', 'Plugins')
foreach ($folder in $foldersToMerge) {
    $sourcePath = Join-Path -Path $extractPath -ChildPath $folder
    $destinationPath = Join-Path -Path $destinationFolder -ChildPath $folder
    if (Test-Path -Path $sourcePath) {
        Copy-Item -Path "$sourcePath\*" -Destination $destinationPath -Recurse -Force
    }
}

Remove-Item -Path $tempDownloadPath -Force
Remove-Item -Path $extractPath -Recurse -Force

if (-not (Test-Path $nsis7zPlugin)) {
    throw "NSIS 7z plugin install failed: $nsis7zPlugin not found after extract."
}

Write-Host "NSIS 7z plugin installed."
