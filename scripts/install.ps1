# occb installer script for PowerShell
# Usage: iwr -useb https://raw.githubusercontent.com/nilparra-dev/opencode-go-cc/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = "nilparra-dev/opencode-go-cc"
$Binary = "occb"

# Detect architecture
$Arch = if ([System.Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { $Arch = "arm64" }

$Platform = "windows-$Arch"
$BinaryName = "$Binary.exe"

# Get latest release
$LatestUrl = "https://api.github.com/repos/$Repo/releases/latest"
Write-Host "Fetching latest release info..."

$Release = Invoke-RestMethod -Uri $LatestUrl
$Asset = $Release.assets | Where-Object { $_.name -like "*$Platform*" }

if (-not $Asset) {
    Write-Error "Could not find release binary for platform: $Platform"
    exit 1
}

# Download
$TempDir = New-TemporaryFile | ForEach-Object { Remove-Item $_; New-Item -ItemType Directory -Path ($_ + "_dir") }
$DownloadPath = Join-Path $TempDir $BinaryName

Write-Host "Downloading $Binary for $Platform..."
Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $DownloadPath

# Install
$InstallDir = "$env:LOCALAPPDATA\Microsoft\WindowsApps"
if (-not (Test-Path $InstallDir)) {
    $InstallDir = "$env:USERPROFILE\bin"
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$InstallPath = Join-Path $InstallDir $BinaryName
Write-Host "Installing to $InstallPath..."
Move-Item -Path $DownloadPath -Destination $InstallPath -Force

# Clean up
Remove-Item -Recurse -Force $TempDir

Write-Host ""
Write-Host "$Binary installed successfully!" -ForegroundColor Green
Write-Host "Run '$Binary init' to get started."
