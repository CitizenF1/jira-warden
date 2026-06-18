param(
    [string]$InstallDir = "$env:USERPROFILE\bin"
)

$ErrorActionPreference = "Stop"

if (!(Test-Path "bin/jirawarden.exe")) {
    & "$PSScriptRoot\build.ps1"
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item "bin/jirawarden.exe" "$InstallDir\jirawarden.exe" -Force

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
    Write-Host "Added $InstallDir to user PATH. Restart terminal to use jirawarden."
}

Write-Host "Installed $InstallDir\jirawarden.exe"
