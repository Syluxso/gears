# Test gears CLI features in a temporary directory

$ErrorActionPreference = "Stop"

# Get workspace root (3 levels up from script location)
$WorkspaceRoot = (Get-Item (Join-Path $PSScriptRoot "..\..\..")).FullName
$TestDir = Join-Path $WorkspaceRoot "test-feature"

Write-Host "Testing gears CLI features" -ForegroundColor Cyan
Write-Host "Test directory: $TestDir" -ForegroundColor Yellow
Write-Host ""

# Create test directory
Write-Host "Creating test directory..." -ForegroundColor Cyan
if (Test-Path $TestDir) {
    Remove-Item -Path $TestDir -Recurse -Force
}
New-Item -Path $TestDir -ItemType Directory | Out-Null
Set-Location $TestDir

# Test gears init
Write-Host "Testing: gears init" -ForegroundColor Cyan
gears init
Write-Host ""

# Test gears session
Write-Host "Testing: gears session" -ForegroundColor Cyan
gears session
Write-Host ""

# Test gears story new
Write-Host "Testing: gears story new 'test feature'" -ForegroundColor Cyan
gears story new "test feature"
Write-Host ""

# Test gears adr new
Write-Host "Testing: gears adr new 'test pattern'" -ForegroundColor Cyan
gears adr new "test pattern"
Write-Host ""

# Test gears story list
Write-Host "Testing: gears story list" -ForegroundColor Cyan
gears story list
Write-Host ""

# Test gears adr list
Write-Host "Testing: gears adr list" -ForegroundColor Cyan
gears adr list
Write-Host ""

# Show created structure
Write-Host "Created .gears structure:" -ForegroundColor Cyan
Get-ChildItem -Path .gears -Recurse -Depth 2 | Select-Object FullName
Write-Host ""

# Cleanup prompt
Write-Host "Test complete!" -ForegroundColor Yellow
$response = Read-Host "Delete test directory? (y/N)"
if ($response -eq "y" -or $response -eq "Y") {
    Set-Location $WorkspaceRoot
    Remove-Item -Path $TestDir -Recurse -Force
    Write-Host "✓ Test directory deleted" -ForegroundColor Green
} else {
    Write-Host "Test directory preserved: $TestDir" -ForegroundColor Yellow
}
