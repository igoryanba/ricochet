<#
.SYNOPSIS
    Installs Ricochet CLI on Windows.

.DESCRIPTION
    This script checks for the Ricochet binary, ensures it's in the system PATH,
    and validates the installation.

.EXAMPLE
    .\scripts\install-windows.ps1
#>

$ErrorActionPreference = "Stop"

Write-Host "🚀 Ricochet CLI Installer for Windows" -ForegroundColor Cyan

# 1. Define Paths
$InstallDir = "$env:LOCALAPPDATA\Ricochet\bin"
$BinaryName = "ricochet.exe"
$SourceBinary = ".\ricochet.exe" # Assumes running from built artifacts or release folder

# 2. Check source binary
if (-not (Test-Path $SourceBinary)) {
    Write-Warning "Source binary '$SourceBinary' not found in current directory."
    Write-Host "Attempting to build..."
    if (Get-Command "go" -ErrorAction SilentlyContinue) {
        go build -o ricochet.exe ./cmd/ricochet
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Build failed."
        }
    } else {
        Write-Error "Go not found. Please provide a compiled 'ricochet.exe' or install Go."
    }
}

# 3. Create Install Directory
if (-not (Test-Path $InstallDir)) {
    Write-Host "Creating installation directory: $InstallDir"
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

# 4. Copy Binary
Write-Host "Copying binary to $InstallDir..."
Copy-Item -Path $SourceBinary -Destination "$InstallDir\$BinaryName" -Force

# 5. Add to PATH
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host "Adding $InstallDir to User PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path += ";$InstallDir"
    Write-Host "✅ Added to PATH. You may need to restart your terminal." -ForegroundColor Green
} else {
    Write-Host "✅ Already in PATH." -ForegroundColor Green
}

# 6. Verify
Write-Host "Verifying installation..."
try {
    & "$InstallDir\$BinaryName" --version
    Write-Host "🎉 Ricochet successfully installed!" -ForegroundColor Green
} catch {
    Write-Warning "Could not verify version. Check permissions."
}
