param(
  [Parameter(Mandatory = $true)]
  [ValidateSet('PrepareVersion', 'PackageInstaller', 'VerifyReleaseArtifacts')]
  [string]$Step,

  [string]$SignedExePath,
  [string]$SignedInstallerPath
)

$ErrorActionPreference = 'Stop'
if (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue) {
  $PSNativeCommandUseErrorActionPreference = $false
}

$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path

function Add-CiEnv {
  param(
    [Parameter(Mandatory = $true)][string]$Name,
    [Parameter(Mandatory = $true)][string]$Value
  )
  Set-Item -Path "Env:$Name" -Value $Value
  if ($env:GITHUB_ENV) {
    "$Name=$Value" | Out-File -FilePath $env:GITHUB_ENV -Append -Encoding utf8
  }
}

function Get-ReleaseTag {
  if ($env:GITHUB_REF_NAME) {
    return $env:GITHUB_REF_NAME
  }
  if ($env:GITHUB_REF -match '^refs/tags/(.+)$') {
    return $Matches[1]
  }
  throw 'GITHUB_REF_NAME is not set; release workflow must run from a tag.'
}

function Get-NsisVersion {
  param([Parameter(Mandatory = $true)][string]$DisplayVersion)

  $nums = @()
  foreach ($m in [regex]::Matches($DisplayVersion, '\d+')) {
    $nums += [int]$m.Value
  }
  if ($nums.Count -eq 0) {
    throw "Tag '$DisplayVersion' has no numeric version for NSIS."
  }
  while ($nums.Count -lt 4) {
    $nums += 0
  }
  if ($nums.Count -gt 4) {
    $nums = $nums[0..3]
  }
  return ($nums -join '.')
}

function Set-ReleaseVersion {
  $tag = Get-ReleaseTag
  $displayVersion = $tag -replace '^v', ''
  $nsisVersion = Get-NsisVersion -DisplayVersion $displayVersion

  Add-CiEnv -Name DATEVERSION -Value $tag
  Add-CiEnv -Name APP_VERSION -Value $displayVersion
  Add-CiEnv -Name NSIS_VERSION -Value $nsisVersion

  $configPath = Join-Path $root 'build\config.yml'
  $config = [System.IO.File]::ReadAllText($configPath)
  $config = $config -replace '(?m)^(\s+)version:\s*"[^"]*"', "`${1}version: `"$displayVersion`""
  [System.IO.File]::WriteAllText($configPath, $config, [System.Text.UTF8Encoding]::new($false))

  $infoPath = Join-Path $root 'build\windows\info.json'
  $info = [System.IO.File]::ReadAllText($infoPath)
  $info = $info -replace '"file_version":\s*"[^"]*"', "`"file_version`": `"$nsisVersion`""
  $info = $info -replace '"ProductVersion":\s*"[^"]*"', "`"ProductVersion`": `"$nsisVersion`""
  [System.IO.File]::WriteAllText($infoPath, $info, [System.Text.UTF8Encoding]::new($false))

  Write-Host "Tag: $tag"
  Write-Host "Display version: $displayVersion"
  Write-Host "NSIS VIProductVersion: $nsisVersion"
}

function Write-UpdaterSignature {
  param([Parameter(Mandatory = $true)][string]$ExePath)

  if (-not $env:UPDATER_KEY) {
    throw 'UPDATER_KEY is not set. Add the updater private key as a GitHub Actions secret.'
  }

  $keyPath = Join-Path ([System.IO.Path]::GetTempPath()) 'tcno-updater-key'
  $sigPath = Join-Path $root 'bin\TcNo-Acc-Switcher.exe.sig'
  $keyMaterial = $env:UPDATER_KEY.Trim().Trim('"').Trim("'")

  try {
    if ($keyMaterial -match 'BEGIN OPENSSH PRIVATE KEY') {
      [System.IO.File]::WriteAllText($keyPath, $keyMaterial, [System.Text.UTF8Encoding]::new($false))
    } else {
      $normalized = ($keyMaterial -replace '\s', '')
      [System.IO.File]::WriteAllBytes($keyPath, [System.Convert]::FromBase64String($normalized))
    }

    $sigOutput = & go run (Join-Path $root 'other\sign-release\main.go') $keyPath $ExePath
    if ($LASTEXITCODE -ne 0) {
      throw "Ed25519 signing failed with exit code $LASTEXITCODE."
    }
    if (-not $sigOutput) {
      throw 'Ed25519 signing failed: no signature output.'
    }

    $sigOutput | Out-File -Encoding ascii $sigPath
    Write-Host "Ed25519 signature created at $sigPath."
  } finally {
    Remove-Item -Force $keyPath -ErrorAction SilentlyContinue
  }
}

function New-InstallerPackage {
  param([Parameter(Mandatory = $true)][string]$SourceExePath)

  if (-not (Test-Path $SourceExePath)) {
    throw "Signed executable not found: $SourceExePath"
  }

  $binDir = Join-Path $root 'bin'
  New-Item -ItemType Directory -Force -Path $binDir | Out-Null
  $exePath = Join-Path $binDir 'TcNo-Acc-Switcher.exe'
  Copy-Item -Force $SourceExePath $exePath

  Write-UpdaterSignature -ExePath $exePath

  $nsisDir = Join-Path $root 'build\windows\nsis'
  $imgDest = Join-Path $nsisDir 'img'
  New-Item -ItemType Directory -Force -Path $imgDest | Out-Null
  Copy-Item -Force (Join-Path $root 'other\NSIS\img\*') $imgDest

  wails3 generate webview2bootstrapper -dir $nsisDir 2>&1 | ForEach-Object { Write-Host $_ }
  if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
  }

  $bootstrapper = Join-Path $nsisDir 'MicrosoftEdgeWebview2Setup.exe'
  if (-not (Test-Path $bootstrapper)) {
    throw "WebView2 bootstrapper was not created at $bootstrapper"
  }

  $archivePath = Join-Path $binDir 'TcNo-Acc-Switcher.7z'
  $stageDir = Join-Path ([System.IO.Path]::GetTempPath()) 'tcno-installer-stage'
  Remove-Item -Recurse -Force $stageDir -ErrorAction SilentlyContinue
  New-Item -ItemType Directory -Force -Path $stageDir | Out-Null
  Copy-Item -Force $exePath $stageDir
  Copy-Item -Force $bootstrapper $stageDir
  & 7z a -mx9 $archivePath "$stageDir\*"
  if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
  }
  Remove-Item -Recurse -Force $stageDir
  Write-Host "7z archive created at $archivePath."

  $shaOut = Join-Path $binDir 'SHA256SUMS'
  @(
    "$((Get-FileHash $exePath -Algorithm SHA256).Hash.ToLower())  TcNo-Acc-Switcher.exe",
    "$((Get-FileHash $archivePath -Algorithm SHA256).Hash.ToLower())  TcNo-Acc-Switcher.7z"
  ) | Out-File -Encoding ascii $shaOut
  Write-Host "SHA256SUMS generated at $shaOut."

  Push-Location $nsisDir
  try {
    makensis "-DVERSION=$env:NSIS_VERSION" "-DDISPLAY_VERSION=$env:APP_VERSION" project.nsi
    if ($LASTEXITCODE -ne 0) {
      exit $LASTEXITCODE
    }
  } finally {
    Pop-Location
  }

  $installer = Get-ChildItem -Path $nsisDir -Filter '*installer*.exe' |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First 1
  if (-not $installer) {
    throw 'NSIS installer not found after build.'
  }

  Add-CiEnv -Name UNSIGNED_INSTALLER_PATH -Value $installer.FullName
  Write-Host "Unsigned installer built at $($installer.FullName)."
}

function Test-ReleaseArtifacts {
  param([Parameter(Mandatory = $true)][string]$InstallerPath)

  $required = @(
    $InstallerPath,
    (Join-Path $root 'bin\TcNo-Acc-Switcher.exe'),
    (Join-Path $root 'bin\TcNo-Acc-Switcher.exe.sig'),
    (Join-Path $root 'bin\TcNo-Acc-Switcher.7z'),
    (Join-Path $root 'bin\SHA256SUMS')
  )

  foreach ($path in $required) {
    if (-not (Test-Path $path)) {
      throw "Release artifact missing: $path"
    }
    Write-Host "Release artifact ready: $path"
  }
}

switch ($Step) {
  'PrepareVersion' {
    Set-ReleaseVersion
  }
  'PackageInstaller' {
    New-InstallerPackage -SourceExePath $SignedExePath
  }
  'VerifyReleaseArtifacts' {
    Test-ReleaseArtifacts -InstallerPath $SignedInstallerPath
  }
}
