$ErrorActionPreference = 'Stop'
# go/7z/makensis log to stderr; PS 7+ treats that as a terminating NativeCommandError.
if (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue) {
  $PSNativeCommandUseErrorActionPreference = $false
}

if (-not $env:SIGNPATH_API_TOKEN) {
  throw @'
SIGNPATH_API_TOKEN is not set in AppVeyor environment variables.
Use a SignPath CI user API token with submitter access to the test-signing policy.
'@
}

$root = $env:APPVEYOR_BUILD_FOLDER
$exePath = Join-Path $root 'bin\TcNo-Acc-Switcher.exe'
$apiBase = "https://app.signpath.io/api/v1/$($env:SIGNPATH_ORGANIZATION_ID)"
$authHeaders = @{ Authorization = "Bearer $($env:SIGNPATH_API_TOKEN)" }
$expectedBuildUrl = "https://ci.appveyor.com/project/$($env:APPVEYOR_ACCOUNT_NAME)/$($env:APPVEYOR_PROJECT_SLUG)/builds/$($env:APPVEYOR_BUILD_ID)/job/$($env:APPVEYOR_JOB_ID)"

function Get-AppVeyorWebhookArtifacts {
  $jobId = $env:APPVEYOR_JOB_ID
  $artifactUrl = "https://ci.appveyor.com/api/buildjobs/$jobId/artifacts"
  try {
    $items = @(Invoke-RestMethod -Method Get -Uri $artifactUrl)
  } catch {
    Write-Host "Artifact API lookup failed ($_); using known executable artifact metadata."
    $items = @()
  }

  if ($items.Count -eq 0) {
    $relativePath = 'bin\TcNo-Acc-Switcher.exe'
    $items = @([PSCustomObject]@{
      fileName = $relativePath
      name     = 'executable'
      type     = 'File'
      sizeInBytes = (Get-Item (Join-Path $root $relativePath)).Length
    })
  }

  return @($items | ForEach-Object {
    $fileName = $_.fileName
    $encodedPath = $fileName -replace '\\', '/'
    @{
      fileName = $fileName
      name     = $_.name
      type     = $_.type
      size     = $_.sizeInBytes
      url      = "https://ci.appveyor.com/api/buildjobs/$jobId/artifacts/$encodedPath"
    }
  })
}

function Test-SignPathApiAccess {
  Write-Host "SignPath preflight: org=$($env:SIGNPATH_ORGANIZATION_ID) project=$($env:SIGNPATH_PROJECT_SLUG) policy=$($env:SIGNPATH_POLICY_SLUG)"
  Write-Host "AppVeyor preflight: account=$($env:APPVEYOR_ACCOUNT_NAME) project=$($env:APPVEYOR_PROJECT_SLUG) build=$($env:APPVEYOR_BUILD_ID) repo=$($env:APPVEYOR_REPO_NAME)"
  Write-Host "SIGNPATH_API_TOKEN length=$($env:SIGNPATH_API_TOKEN.Length)"

  $policyUrl = "$apiBase/Projects/$($env:SIGNPATH_PROJECT_SLUG)/SigningPolicies/$($env:SIGNPATH_POLICY_SLUG)"
  try {
    $policy = Invoke-RestMethod -Method Get -Uri $policyUrl -Headers $authHeaders
    Write-Host "SignPath API token OK (read signing policy: $($policy.signingPolicySlug))."
  } catch {
    $statusCode = $null
    if ($_.Exception.Response) { $statusCode = [int]$_.Exception.Response.StatusCode }
    throw @"
SignPath API token check failed ($statusCode) for GET $policyUrl.
The CI user token is missing, wrong, or lacks access to this project/policy.
Create a SignPath CI user, add it as submitter on $($env:SIGNPATH_POLICY_SLUG), and encrypt only the raw token in appveyor.yml.
"@
  }
}

function Invoke-SignPathAppVeyorIntegration {
  param(
    [array]$Artifacts = $null
  )

  $integrationUrl = "https://app.signpath.io/API/v1/$($env:SIGNPATH_ORGANIZATION_ID)/Integrations/AppVeyor?ProjectSlug=$($env:SIGNPATH_PROJECT_SLUG)&SigningPolicySlug=$($env:SIGNPATH_POLICY_SLUG)"
  if (-not $Artifacts) {
    $Artifacts = @(Get-AppVeyorWebhookArtifacts)
  }
  $payload = @{
    accountName = $env:APPVEYOR_ACCOUNT_NAME
    projectId   = [int]$env:APPVEYOR_PROJECT_ID
    projectName = $env:APPVEYOR_PROJECT_NAME
    projectSlug = $env:APPVEYOR_PROJECT_SLUG
    buildId     = [int]$env:APPVEYOR_BUILD_ID
    buildNumber = $env:APPVEYOR_BUILD_NUMBER
    buildVersion = $env:APPVEYOR_BUILD_VERSION
    buildJobId  = $env:APPVEYOR_JOB_ID
    jobId       = $env:APPVEYOR_JOB_ID
    repositoryName = $env:APPVEYOR_REPO_NAME
    branch      = $env:APPVEYOR_REPO_BRANCH
    commitId    = $env:APPVEYOR_REPO_COMMIT
    commitAuthor = $env:APPVEYOR_REPO_COMMIT_AUTHOR
    commitAuthorEmail = $env:APPVEYOR_REPO_COMMIT_AUTHOR_EMAIL
    commitDate  = $env:APPVEYOR_REPO_COMMIT_TIMESTAMP
    commitMessage = $env:APPVEYOR_REPO_COMMIT_MESSAGE
    artifacts   = @($Artifacts)
  }

  Write-Host "Triggering SignPath AppVeyor origin verification..."
  try {
    $response = Invoke-WebRequest -Method Post -Uri $integrationUrl -Headers $authHeaders -Body ($payload | ConvertTo-Json -Depth 6) -ContentType 'application/json'
    Write-Host "SignPath integration accepted ($($response.StatusCode))."
    if ($response.Headers.Location) {
      Write-Host "Signing request location: $($response.Headers.Location)"
    }
  } catch {
    $statusCode = $null
    $details = $_.Exception.Message
    if ($_.Exception.Response) {
      $statusCode = [int]$_.Exception.Response.StatusCode
      try {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $details = $reader.ReadToEnd()
        $reader.Close()
      } catch {
        # Keep default message when response body is unavailable.
      }
    }
    throw @"
SignPath AppVeyor integration failed ($statusCode).
$details

Checklist:
- SIGNPATH_API_TOKEN is a SignPath CI user token with submitter access to $($env:SIGNPATH_POLICY_SLUG)
- SignPath project links the AppVeyor trusted build system
- AppVeyor account API v1+v2 are enabled and that bearer token is saved in SignPath
"@
  }
}

function Get-SignPathSigningRequest {
  param([string]$SigningRequestId)
  return Invoke-RestMethod -Method Get -Uri "$apiBase/SigningRequests/$SigningRequestId" -Headers $authHeaders
}

function Get-SignPathSigningRequestList {
  $preApiBase = "https://app.signpath.io/api/v1-pre/$($env:SIGNPATH_ORGANIZATION_ID)"
  $listUrls = @(
    "$apiBase/SigningRequests?projectSlug=$($env:SIGNPATH_PROJECT_SLUG)&pageSize=25&sortOrder=Descending",
    "$apiBase/SigningRequests?projectSlug=$($env:SIGNPATH_PROJECT_SLUG)",
    "$preApiBase/SigningRequests?projectSlug=$($env:SIGNPATH_PROJECT_SLUG)&pageSize=25",
    "$preApiBase/SigningRequests"
  )
  foreach ($listUrl in $listUrls) {
    try {
      $response = Invoke-RestMethod -Method Get -Uri $listUrl -Headers $authHeaders
    } catch {
      Write-Host "SignPath list API failed for $listUrl : $($_.Exception.Message)"
      continue
    }
    $requests = @($response)
    if ($response.PSObject.Properties.Name -contains 'items') { $requests = @($response.items) }
    if ($response.PSObject.Properties.Name -contains 'signingRequests') { $requests = @($response.signingRequests) }
    if ($requests.Count -gt 0) {
      Write-Host "SignPath list API returned $($requests.Count) request(s) from $listUrl"
      return $requests
    }
  }
  return @()
}

function Get-SignPathRequestArtifactName {
  param($Request)
  foreach ($key in @('unsignedArtifactFileName', 'artifactFileName', 'fileName', 'unsignedArtifactName')) {
    if ($Request.PSObject.Properties.Name -contains $key -and $Request.$key) {
      return [string]$Request.$key
    }
  }
  return $null
}

function Test-SignPathSigningRequestMatch {
  param(
    $Request,
    [string]$BuildUrl
  )
  $request = $Request
  $requestId = $request.id
  if (-not $requestId) { $requestId = $request.signingRequestId }

  if (-not $request.origin -and $requestId) {
    try {
      $request = Get-SignPathSigningRequest -SigningRequestId $requestId
    } catch {
      Write-Host "Could not load signing request $requestId for matching: $_"
      return $false
    }
  }

  if ($request.projectSlug -and $request.projectSlug -ne $env:SIGNPATH_PROJECT_SLUG) { return $false }
  if ($request.signingPolicySlug -and $request.signingPolicySlug -ne $env:SIGNPATH_POLICY_SLUG) { return $false }

  $originUrl = $request.origin.buildData.url
  if ($originUrl) {
    if ($originUrl -eq $BuildUrl) { return $true }
    if ($originUrl -like "*$($env:APPVEYOR_BUILD_ID)*") { return $true }
    if ($originUrl -like "*$($env:APPVEYOR_JOB_ID)*") { return $true }
  }

  $commitId = $request.origin.repositoryData.commitId
  if ($commitId -and $env:APPVEYOR_REPO_COMMIT) {
    $shortCommit = $env:APPVEYOR_REPO_COMMIT.Substring(0, [Math]::Min(7, $env:APPVEYOR_REPO_COMMIT.Length))
    if ($commitId -eq $env:APPVEYOR_REPO_COMMIT) { return $true }
    if ($commitId.StartsWith($shortCommit) -or $env:APPVEYOR_REPO_COMMIT.StartsWith($commitId)) { return $true }
  }

  foreach ($tag in @($env:APPVEYOR_REPO_TAG_NAME, $env:DATEVERSION)) {
    if ($tag -and $request.description -and $request.description -like "*$tag*") {
      return $true
    }
  }

  return $false
}

function Find-SignPathSigningRequestId {
  param(
    [string]$BuildUrl,
    [string[]]$ExcludeRequestIds = @(),
    [string]$ArtifactName = $null,
    [string]$ArtifactNameContains = $null,
    [switch]$RequireOrigin
  )
  foreach ($request in (Get-SignPathSigningRequestList)) {
    $requestId = $request.id
    if (-not $requestId) { $requestId = $request.signingRequestId }
    if (-not $requestId) { continue }
    if ($ExcludeRequestIds -contains $requestId) { continue }

    $artifactLabel = Get-SignPathRequestArtifactName -Request $request
    if (-not $artifactLabel -and $requestId) {
      try {
        $artifactLabel = Get-SignPathRequestArtifactName -Request (Get-SignPathSigningRequest -SigningRequestId $requestId)
      } catch {
        # Keep matching on build metadata when artifact name is unavailable in list API.
      }
    }
    if ($ArtifactName -and $artifactLabel -and $artifactLabel -notlike "*$ArtifactName") { continue }
    if ($ArtifactNameContains -and $artifactLabel -and $artifactLabel -notlike "*$ArtifactNameContains*") { continue }

    if (-not (Test-SignPathSigningRequestMatch -Request $request -BuildUrl $BuildUrl)) { continue }

    $originRequest = $request
    if (-not $originRequest.origin -and $requestId) {
      try { $originRequest = Get-SignPathSigningRequest -SigningRequestId $requestId } catch { }
    }
    if ($RequireOrigin -and -not $originRequest.origin) { continue }

    if ($artifactLabel) {
      Write-Host "Matched SignPath signing request $requestId (artifact: $artifactLabel)"
    } else {
      Write-Host "Matched SignPath signing request $requestId"
    }
    return $requestId
  }
  return $null
}

function Receive-SignPathDirectSignedArtifact {
  param(
    [string]$InputPath,
    [string]$OutputPath,
    [string]$Description
  )

  Write-Host "Submitting $(Split-Path $InputPath -Leaf) for direct SignPath signing..."
  $signingRequestId = Submit-SigningRequest `
    -InputArtifactPath $InputPath `
    -ProjectSlug $env:SIGNPATH_PROJECT_SLUG `
    -SigningPolicySlug $env:SIGNPATH_POLICY_SLUG `
    -OrganizationId $env:SIGNPATH_ORGANIZATION_ID `
    -ApiToken $env:SIGNPATH_API_TOKEN `
    -Description $Description `
    -WaitForCompletion `
    -OutputArtifactPath $OutputPath `
    -Force
  Write-Host "Direct SignPath signing completed (request $signingRequestId)."
  return $signingRequestId
}

function Receive-SignPathOriginVerifiedArtifact {
  param(
    [string]$BuildUrl,
    [datetime]$Deadline,
    [string]$OutputPath,
    [string]$PhaseLabel,
    [string[]]$ExcludeRequestIds = @(),
    [string]$ArtifactName = $null,
    [string]$ArtifactNameContains = $null
  )

  Write-Host "Waiting for SignPath origin-verified $PhaseLabel..."
  $signingRequestId = $null
  while (-not $signingRequestId -and (Get-Date) -lt $Deadline) {
    $signingRequestId = Find-SignPathSigningRequestId `
      -BuildUrl $BuildUrl `
      -ExcludeRequestIds $ExcludeRequestIds `
      -ArtifactName $ArtifactName `
      -ArtifactNameContains $ArtifactNameContains `
      -RequireOrigin
    if ($signingRequestId) { break }
    Write-Host "No matching SignPath signing request for $PhaseLabel yet; retrying in 10s..."
    Start-Sleep -Seconds 10
  }
  if (-not $signingRequestId) {
    throw "Timed out waiting for SignPath signing request for $PhaseLabel on build $($env:APPVEYOR_BUILD_ID)."
  }

  $status = Wait-SignPathSigningRequest -SigningRequestId $signingRequestId -Deadline $Deadline
  if ($status.status -ne 'Completed') {
    throw "SignPath signing failed for $PhaseLabel with status $($status.status) / $($status.workflowStatus)."
  }

  $signingRequest = Get-SignPathSigningRequest -SigningRequestId $signingRequestId
  if (-not $signingRequest.origin) {
    throw "SignPath signing request for $PhaseLabel completed without origin metadata."
  }
  Write-Host "Origin verified ($PhaseLabel): $($signingRequest.origin.repositoryData.url) @ $($signingRequest.origin.repositoryData.commitId)"

  Write-Host "Downloading signed $PhaseLabel..."
  Invoke-WebRequest -Method Get -Uri "$apiBase/SigningRequests/$signingRequestId/SignedArtifact" -Headers $authHeaders -OutFile $OutputPath
  return $signingRequestId
}

function Wait-SignPathSigningRequest {
  param(
    [string]$SigningRequestId,
    [datetime]$Deadline
  )
  $status = $null
  while ((Get-Date) -lt $Deadline) {
    $status = Invoke-RestMethod -Method Get -Uri "$apiBase/SigningRequests/$SigningRequestId/Status" -Headers $authHeaders
    Write-Host "SignPath status: $($status.status) / $($status.workflowStatus)"
    if ($status.isFinalStatus) { return $status }
    Start-Sleep -Seconds 10
  }
  throw "Timed out waiting for SignPath signing request $SigningRequestId."
}

Test-SignPathApiAccess
if ($env:SIGNPATH_TRIGGER_VIA_WEBHOOK -eq 'true') {
  Write-Host 'SignPath origin verification was triggered by the AppVeyor deploy webhook.'
} else {
  Invoke-SignPathAppVeyorIntegration
}

Write-Host "Commit: $($env:APPVEYOR_REPO_COMMIT)  Tag: $($env:APPVEYOR_REPO_TAG_NAME)"
$deadline = (Get-Date).AddMinutes(30)
$exeSigningRequestId = Receive-SignPathOriginVerifiedArtifact `
  -BuildUrl $expectedBuildUrl `
  -Deadline $deadline `
  -OutputPath $exePath `
  -PhaseLabel 'application EXE' `
  -ArtifactName 'TcNo-Acc-Switcher.exe'
Write-Host "EXE signed with AppVeyor origin verification (request $exeSigningRequestId)."

Write-Host "Signing with Ed25519..."
$keyPath = Join-Path $root 'updater-key'
$keyReady = $false
if ($env:UPDATER_KEY) {
  $keyMaterial = $env:UPDATER_KEY.Trim().Trim('"').Trim("'")
  try {
    if ($keyMaterial -match 'BEGIN OPENSSH PRIVATE KEY') {
      [System.IO.File]::WriteAllText($keyPath, $keyMaterial, [System.Text.UTF8Encoding]::new($false))
      $keyReady = $true
    } else {
      $normalized = ($keyMaterial -replace '\s', '')
      [System.IO.File]::WriteAllBytes($keyPath, [System.Convert]::FromBase64String($normalized))
      $keyReady = $true
    }
  } catch {
    Write-Host "Failed to decode UPDATER_KEY: $_"
  }
}
$sigPath = Join-Path $root 'bin\TcNo-Acc-Switcher.exe.sig'
if ($keyReady) {
  # Run via cmd so go's "downloading ..." stderr cannot terminate this script under ErrorAction Stop.
  $signCmd = "go run `"$root\cmd\sign-release\main.go`" `"$keyPath`" `"$exePath`" 2>nul"
  $sigOutput = cmd /c $signCmd
  if ($LASTEXITCODE -ne 0) {
    Write-Host "Ed25519 signing failed (exit $LASTEXITCODE)."
    Remove-Item -Force $sigPath -ErrorAction SilentlyContinue
  } elseif (-not $sigOutput) {
    Write-Host "Ed25519 signing failed (no signature output)."
    Remove-Item -Force $sigPath -ErrorAction SilentlyContinue
  } else {
    $sigOutput | Out-File -Encoding ascii $sigPath
    Write-Host "Ed25519 signature created."
  }
  Remove-Item -Force $keyPath -ErrorAction SilentlyContinue
} else {
  Write-Host "Ed25519 signing skipped (UPDATER_KEY not set or invalid)."
}

Write-Host "Creating 7z archive for NSIS installer..."
$stageDir = Join-Path $root 'bin\installer_stage'
New-Item -ItemType Directory -Force -Path $stageDir | Out-Null
Copy-Item -Force $exePath $stageDir
Copy-Item -Force (Join-Path $root 'build\windows\nsis\MicrosoftEdgeWebview2Setup.exe') $stageDir
& 7z a -mx9 (Join-Path $root 'bin\TcNo-Acc-Switcher.7z') "$stageDir\*"
Remove-Item -Recurse -Force $stageDir
Write-Host "7z archive created."

Write-Host "Generating SHA256SUMS..."
$shaOut = Join-Path $root 'bin\SHA256SUMS'
@(
  "$((Get-FileHash $exePath -Algorithm SHA256).Hash.ToLower())  TcNo-Acc-Switcher.exe",
  "$((Get-FileHash (Join-Path $root 'bin\TcNo-Acc-Switcher.7z') -Algorithm SHA256).Hash.ToLower())  TcNo-Acc-Switcher.7z"
) | Out-File -Encoding ascii $shaOut
Write-Host "SHA256SUMS generated."

Write-Host "Building NSIS installer..."
Push-Location (Join-Path $root 'build\windows\nsis')
makensis -DVERSION="$env:NSIS_VERSION" -DDISPLAY_VERSION="$env:APP_VERSION" project.nsi
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Pop-Location
Write-Host "NSIS installer built."

$installer = Get-ChildItem -Path (Join-Path $root 'build\windows\nsis') -Filter '*installer*.exe' | Select-Object -First 1
if (-not $installer) {
  throw 'NSIS installer not found after build.'
}

$installerSigningRequestId = Receive-SignPathDirectSignedArtifact `
  -InputPath $installer.FullName `
  -OutputPath $installer.FullName `
  -Description "NSIS installer $($env:APPVEYOR_REPO_TAG_NAME)"
$installer = Get-Item $installer.FullName
Write-Host "Installer signed via direct API, no origin (request $installerSigningRequestId)."

Write-Host "Publishing draft GitHub release $($env:APPVEYOR_REPO_TAG_NAME)..."
if (-not $env:github_release_token) {
  throw 'github_release_token is not configured in AppVeyor environment.'
}
$repo = $env:APPVEYOR_REPO_NAME
$tag = $env:APPVEYOR_REPO_TAG_NAME
$ghHeaders = @{
  Authorization = "token $($env:github_release_token)"
  'User-Agent' = 'AppVeyor'
  Accept = 'application/vnd.github+json'
}
$releaseBody = @{
  tag_name = $tag
  name = "Release $tag"
  body = "Release $tag"
  draft = $true
  prerelease = $false
} | ConvertTo-Json
try {
  $release = Invoke-RestMethod -Method Post -Uri "https://api.github.com/repos/$repo/releases" -Headers $ghHeaders -Body $releaseBody -ContentType 'application/json; charset=utf-8'
} catch {
  if ($_.Exception.Response.StatusCode.value__ -eq 422) {
    $existing = Invoke-RestMethod -Method Get -Uri "https://api.github.com/repos/$repo/releases/tags/$tag" -Headers $ghHeaders
    $release = $existing
  } else {
    throw
  }
}

$assets = @(
  $installer.FullName,
  $exePath,
  (Join-Path $root 'bin\TcNo-Acc-Switcher.7z'),
  $shaOut
)
if (Test-Path $sigPath) { $assets += $sigPath }
foreach ($assetPath in $assets) {
  $assetName = Split-Path $assetPath -Leaf
  $uploadUrl = ($release.upload_url -replace '\{\?name,label\}$', '') + "?name=$assetName"
  Write-Host "Uploading $assetName..."
  Invoke-RestMethod -Method Post -Uri $uploadUrl -Headers @{
    Authorization = "token $($env:github_release_token)"
    'User-Agent' = 'AppVeyor'
    Accept = 'application/vnd.github+json'
  } -InFile $assetPath -ContentType 'application/octet-stream' | Out-Null
}
Write-Host "GitHub draft release published."
