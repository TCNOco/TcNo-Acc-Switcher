# Sets APP_VERSION / NSIS_VERSION from DATEVERSION and patches build assets.
$ErrorActionPreference = 'Stop'

Write-Host '[version] parsing tag...'
$tag = $env:DATEVERSION -replace '^v', ''
$versionNumbers = [regex]::Matches($tag, '\d+') | ForEach-Object { [int]$_.Value }
if ($versionNumbers.Count -eq 0) {
    throw "Tag '$tag' has no numeric version for NSIS (use e.g. v4.0.10 or Test-4)."
}
while ($versionNumbers.Count -lt 4) { $versionNumbers += 0 }
if ($versionNumbers.Count -gt 4) { $versionNumbers = $versionNumbers[0..3] }
$nsisVersion = $versionNumbers -join '.'
$env:APP_VERSION = $tag
$env:NSIS_VERSION = $nsisVersion
Write-Host "[version] Tag=$env:DATEVERSION Display=$tag NSIS=$nsisVersion"

Write-Host '[version] patching config.yml...'
$configPath = Join-Path $env:APPVEYOR_BUILD_FOLDER 'build\config.yml'
$config = [System.IO.File]::ReadAllText($configPath)
$config = $config -replace '(?m)^(\s+)version:\s*"[^"]*"', "`${1}version: `"$tag`""
[System.IO.File]::WriteAllText($configPath, $config, [System.Text.UTF8Encoding]::new($false))

Write-Host '[version] patching info.json...'
$infoPath = Join-Path $env:APPVEYOR_BUILD_FOLDER 'build\windows\info.json'
$info = [System.IO.File]::ReadAllText($infoPath)
$info = $info -replace '"file_version":\s*"[^"]*"', "`"file_version`": `"$nsisVersion`""
$info = $info -replace '"ProductVersion":\s*"[^"]*"', "`"ProductVersion`": `"$nsisVersion`""
[System.IO.File]::WriteAllText($infoPath, $info, [System.Text.UTF8Encoding]::new($false))

Write-Host '[version] done.'
