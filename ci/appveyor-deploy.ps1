$ErrorActionPreference = 'Stop'

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
  $integrationUrl = "https://app.signpath.io/API/v1/$($env:SIGNPATH_ORGANIZATION_ID)/Integrations/AppVeyor?ProjectSlug=$($env:SIGNPATH_PROJECT_SLUG)&SigningPolicySlug=$($env:SIGNPATH_POLICY_SLUG)"
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
    artifacts   = @(Get-AppVeyorWebhookArtifacts)
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
  param([string]$BuildUrl)
  foreach ($request in (Get-SignPathSigningRequestList)) {
    $requestId = $request.id
    if (-not $requestId) { $requestId = $request.signingRequestId }
    if (-not $requestId) { continue }
    if (Test-SignPathSigningRequestMatch -Request $request -BuildUrl $BuildUrl) {
      Write-Host "Matched SignPath signing request $requestId"
      return $requestId
    }
  }
  return $null
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

Write-Host "Waiting for SignPath origin-verified signing request for $expectedBuildUrl..."
Write-Host "Commit: $($env:APPVEYOR_REPO_COMMIT)  Tag: $($env:APPVEYOR_REPO_TAG_NAME)"
$signingRequestId = $null
$deadline = (Get-Date).AddMinutes(30)
while (-not $signingRequestId -and (Get-Date) -lt $deadline) {
  $signingRequestId = Find-SignPathSigningRequestId -BuildUrl $expectedBuildUrl
  if ($signingRequestId) { break }
  Write-Host 'No matching SignPath signing request yet; retrying in 10s...'
  Start-Sleep -Seconds 10
}
if (-not $signingRequestId) {
  throw "Timed out waiting for SignPath signing request linked to AppVeyor build $($env:APPVEYOR_BUILD_ID)."
}
Write-Host "Found SignPath signing request $signingRequestId"

$status = Wait-SignPathSigningRequest -SigningRequestId $signingRequestId -Deadline $deadline
if ($status.status -ne 'Completed') {
  throw "SignPath signing failed with status $($status.status) / $($status.workflowStatus)."
}

$signingRequest = Get-SignPathSigningRequest -SigningRequestId $signingRequestId
if (-not $signingRequest.origin) {
  throw "SignPath signing request completed without origin metadata (origin verification missing)."
}
Write-Host "Origin verified: $($signingRequest.origin.repositoryData.url) @ $($signingRequest.origin.repositoryData.commitId)"

Write-Host "Downloading signed artifact..."
Invoke-WebRequest -Method Get -Uri "$apiBase/SigningRequests/$signingRequestId/SignedArtifact" -Headers $authHeaders -OutFile $exePath

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
  go run "$root\cmd\sign-release\main.go" $keyPath $exePath | Out-File -Encoding ascii $sigPath
  if ($LASTEXITCODE -ne 0) {
    Write-Host "Ed25519 signing failed."
    Remove-Item -Force $sigPath -ErrorAction SilentlyContinue
  } else {
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

Write-Host "Signing NSIS installer with SignPath (direct submit)..."
$signedInstallerPath = Join-Path $installer.DirectoryName ($installer.BaseName + '-signed' + $installer.Extension)
Submit-SigningRequest `
  -InputArtifactPath $installer.FullName `
  -ApiToken $env:SIGNPATH_API_TOKEN `
  -OrganizationId $env:SIGNPATH_ORGANIZATION_ID `
  -ProjectSlug $env:SIGNPATH_PROJECT_SLUG `
  -SigningPolicySlug $env:SIGNPATH_POLICY_SLUG `
  -ArtifactConfigurationSlug $env:SIGNPATH_INSTALLER_ARTIFACT_CONFIGURATION_SLUG `
  -OutputArtifactPath $signedInstallerPath `
  -Description "Installer $($env:APPVEYOR_REPO_TAG_NAME)" `
  -WaitForCompletion
Move-Item -Force $signedInstallerPath $installer.FullName
$installer = Get-Item $installer.FullName
Write-Host "NSIS installer signed."

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
