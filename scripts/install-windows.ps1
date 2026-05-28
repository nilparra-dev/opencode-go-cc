# Install occb on Windows
# Run this script to install occb to a permanent location

$ErrorActionPreference = "Stop"

# Configuration
$InstallDir = "$env:LOCALAPPDATA\Programs\occb"
$RepoDir = Split-Path -Parent $PSScriptRoot

Write-Host "=== occb Windows Installer ===" -ForegroundColor Cyan
Write-Host ""

# Check if Go is installed
try {
    $goVersion = go version 2>$null
    if (-not $goVersion) {
        Write-Error "Go is not installed or not in PATH. Please install Go first: https://go.dev/dl/"
        exit 1
    }
    Write-Host "Found: $goVersion" -ForegroundColor Green
} catch {
    Write-Error "Go is not installed. Please install Go first: https://go.dev/dl/"
    exit 1
}

# Build the binary
Write-Host "`nBuilding occb..." -ForegroundColor Yellow
Set-Location $RepoDir
go build -ldflags "-X main.version=0.1.0" -o bin\occb.exe .\cmd\occb

if (-not (Test-Path .\bin\occb.exe)) {
    Write-Error "Build failed! Binary not found."
    exit 1
}
Write-Host "Build successful!" -ForegroundColor Green

# Create install directory
Write-Host "`nCreating install directory: $InstallDir" -ForegroundColor Yellow
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

# Copy binary
Write-Host "Installing occb.exe..." -ForegroundColor Yellow
Copy-Item .\bin\occb.exe -Destination $InstallDir -Force

# Add to PATH if not already there
Write-Host "`nChecking PATH..." -ForegroundColor Yellow
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$InstallDir*") {
    Write-Host "Adding $InstallDir to your PATH..." -ForegroundColor Yellow
    $newPath = $currentPath + ";" + $InstallDir
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "PATH updated! You may need to restart your terminal." -ForegroundColor Green
} else {
    Write-Host "Already in PATH." -ForegroundColor Green
}

# Update current session PATH
$env:Path = $InstallDir + ";" + $env:Path

# Test installation
Write-Host "`nTesting installation..." -ForegroundColor Yellow
try {
    $version = occb --version
    Write-Host "Success! $version" -ForegroundColor Green
} catch {
    Write-Host "Warning: Could not verify installation. Try restarting your terminal." -ForegroundColor Yellow
}

Write-Host "`n=== Installation Complete ===" -ForegroundColor Cyan
Write-Host "occb is now installed at: $InstallDir\occb.exe" -ForegroundColor White
Write-Host ""
Write-Host "Quick start:" -ForegroundColor Cyan
Write-Host "  occb init      # Create config file"
Write-Host "  occb on        # Start proxy and enable OpenCode mode"
Write-Host "  claude         # Launch Claude Code with OpenCode"
Write-Host "  occb off       # Return to Anthropic mode"
Write-Host ""
Write-Host "Note: If 'occb' is not recognized, close and reopen your terminal." -ForegroundColor Yellow
