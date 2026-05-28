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

# Get latest release or fall back to Go source
$TempDir = New-TemporaryFile | ForEach-Object { Remove-Item $_; New-Item -ItemType Directory -Path ($_ + "_dir") }
$DownloadPath = Join-Path $TempDir $BinaryName
$UseGoFallback = $false

try {
    $LatestUrl = "https://api.github.com/repos/$Repo/releases/latest"
    Write-Host "Fetching latest release info..."
    $Release = Invoke-RestMethod -Uri $LatestUrl
    $Asset = $Release.assets | Where-Object { $_.name -eq "occb_$Platform.exe" }
    if (-not $Asset) {
        $UseGoFallback = $true
    }
} catch {
    $UseGoFallback = $true
}

if ($UseGoFallback) {
    Write-Host "No published release found for $Platform. Falling back to 'go install'." -ForegroundColor Yellow
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Error "Go is required for fallback installation but was not found in PATH."
        exit 1
    }

    $PreviousGobin = $env:GOBIN
    $env:GOBIN = $TempDir
    try {
        go install github.com/nilparra-dev/opencode-go-cc/cmd/occb@latest
    } finally {
        $env:GOBIN = $PreviousGobin
    }
} else {
    Write-Host "Downloading $Binary for $Platform..."
    Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $DownloadPath
}

# Install
$InstallDir = "$env:LOCALAPPDATA\Programs\occb"
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null

$InstallPath = Join-Path $InstallDir $BinaryName
Write-Host "Installing to $InstallPath..."
Move-Item -Path $DownloadPath -Destination $InstallPath -Force

# Add to PATH if needed
$CurrentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($CurrentPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$CurrentPath;$InstallDir", "User")
}

$env:Path = "$InstallDir;$env:Path"

# Clean up
Remove-Item -Recurse -Force $TempDir

Write-Host ""
Write-Host "$Binary installed successfully!" -ForegroundColor Green
Write-Host "Run '$Binary init' to get started."
Write-Host "Run '$Binary update' later to install the latest release."
