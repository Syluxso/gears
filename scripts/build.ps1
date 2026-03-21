# Build and install gears CLI

$ErrorActionPreference = "Stop"

Write-Host "Building gears CLI..." -ForegroundColor Cyan
Set-Location (Join-Path $PSScriptRoot "..")

# Build
go build -o gears.exe

# Install to GOPATH/bin
Write-Host "Installing to GOPATH/bin..." -ForegroundColor Cyan
go install

Write-Host ""
Write-Host "Build and install complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Installed to: $env:USERPROFILE\go\bin\gears.exe"
