$accountSwitcherVersion = "2024-08-27_00"

# Define the URL for the GitHub API to get the latest release
$apiUrl = "https://api.github.com/repos/TCNOco/TcNo-Acc-Switcher/releases/latest"

# Define the local paths
$downloadPath = "LAST-TcNo-Acc-Switcher_and_CEF.7z"
$extractPath = "OldVersion"

# Define 7z executable path (update this if 7z is installed in a different location)
$sevenZipPath = "C:\Program Files\7-Zip\7z.exe"

# Function to download file
# These are my open-source powershell scripts: https://tc.ht/
# (This points to): https://github.com/TCNOco/TcNo-TCHT/PowerShell/

Invoke-Expression (Invoke-RestMethod Import-RemoteFunction.tc.ht)
Import-RemoteFunction("Get-GeneralFuncs.tc.ht")
Import-FunctionIfNotExists -Command Get-Aria2File -ScriptUri "File-DownloadMethods.tc.ht"


# Function to extract file
function ExtractFile {
    param (
        [string]$filePath,
        [string]$outputPath
    )
    try {
        & $sevenZipPath x "$filePath" -o"$outputPath" -y
        Write-Output "Extracted file to $outputPath"
    } catch {
        Write-Error "Failed to extract file $filePath. $_"
    }
}

# -----------------------------------
# Download and extract the last build (including CEF)
# -----------------------------------

# Fetch the latest release data
$response = Invoke-RestMethod -Uri $apiUrl -Headers @{ "User-Agent" = "PowerShell" }

# Find the asset with "and_CEF" in its name
$asset = $response.assets | Where-Object { $_.name -like "*and_CEF*" } | Select-Object -First 1

if ($null -ne $asset) {
    # Download the file
    Get-Aria2File -Url $asset.browser_download_url -OutputPath $downloadPath

    # Create the extraction folder if it doesn't exist
    if (-not (Test-Path $extractPath)) {
        New-Item -ItemType Directory -Path $extractPath
    }

    # Extract the file
    ExtractFile -filePath $downloadPath -outputPath $extractPath
} else {
    Write-Error "No asset with 'and_CEF' found in the latest release."
}

# -----------------------------------
# Run the updater to create updates based on the differences
# -----------------------------------
$updaterPath = "updater\TcNo-Acc-Switcher-Updater.exe"
$updaterArgs = "createupdate"

if (Test-Path $updaterPath) {
    Start-Process -FilePath $updaterPath -ArgumentList $updaterArgs -Wait
} else {
    Write-Error "Updater not found at $updaterPath"
}

# Use 7z to compress the contents of UpdateOutput
$compressPath = "UpdateOutput"
$compressOutput = "$accountSwitcherVersion.7z"

if (Test-Path $compressPath) {
    & $sevenZipPath a -r $compressOutput "./$compressPath/*" -mx9
} else {
    Write-Error "UpdateOutput folder not found at $compressPath"
}
