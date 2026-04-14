# Gears Installation Test Script
# Tests CLI installation, PATH configuration, and basic functionality

param(
    [switch]$SkipCleanup,
    [switch]$Verbose
)

$ErrorActionPreference = "Continue"
$TestDir = Join-Path $env:TEMP "gears-test-$(Get-Date -Format 'yyyyMMdd-HHmmss')"
$PassCount = 0
$FailCount = 0
$WarnCount = 0

function Write-TestHeader {
    param([string]$Message)
    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host " $Message" -ForegroundColor Cyan
    Write-Host "========================================`n" -ForegroundColor Cyan
}

function Write-TestResult {
    param(
        [string]$Test,
        [string]$Status,
        [string]$Message = ""
    )
    
    $icon = switch ($Status) {
        "PASS" { "✓"; $script:PassCount++; "Green" }
        "FAIL" { "✗"; $script:FailCount++; "Red" }
        "WARN" { "⚠"; $script:WarnCount++; "Yellow" }
        default { "•"; "Gray" }
    }
    
    Write-Host "[$icon[0]] " -ForegroundColor $icon[1] -NoNewline
    Write-Host "$Test" -NoNewline
    if ($Message) {
        Write-Host " - $Message" -ForegroundColor Gray
    } else {
        Write-Host ""
    }
}

function Test-GearsCommand {
    param([string]$Command)
    try {
        $result = Invoke-Expression $Command 2>&1
        return @{
            Success = $LASTEXITCODE -eq 0
            Output = $result
        }
    } catch {
        return @{
            Success = $false
            Output = $_.Exception.Message
        }
    }
}

# =============================================================================
# TEST 1: Binary Existence and Accessibility
# =============================================================================
Write-TestHeader "Test 1: Binary Check"

$gearsCommand = Get-Command gears -ErrorAction SilentlyContinue

if ($gearsCommand) {
    Write-TestResult "gears command found" "PASS" $gearsCommand.Source
    
    # Check if it's in the expected location
    $expectedPath = Join-Path $env:USERPROFILE "go\bin\gears.exe"
    if ($gearsCommand.Source -eq $expectedPath) {
        Write-TestResult "Binary in expected location" "PASS" "C:\Users\<username>\go\bin\gears.exe"
    } else {
        Write-TestResult "Binary in non-standard location" "WARN" $gearsCommand.Source
    }
} else {
    Write-TestResult "gears command not found" "FAIL" "Run: go install github.com/Syluxso/gears@latest"
    Write-Host "`nDiagnostics:" -ForegroundColor Yellow
    Write-Host "  GOPATH: $env:GOPATH"
    Write-Host "  Expected binary: $(Join-Path $env:USERPROFILE 'go\bin\gears.exe')"
    Write-Host "  Binary exists: $(Test-Path (Join-Path $env:USERPROFILE 'go\bin\gears.exe'))"
}

# =============================================================================
# TEST 2: PATH Configuration
# =============================================================================
Write-TestHeader "Test 2: PATH Configuration"

$goBinPath = Join-Path $env:USERPROFILE "go\bin"
$pathContainsGoBin = $env:PATH -split ';' | Where-Object { $_ -like "*go\bin*" }

if ($pathContainsGoBin) {
    Write-TestResult "go\bin in PATH" "PASS" $pathContainsGoBin[0]
} else {
    Write-TestResult "go\bin not in PATH" "FAIL" "Add to PATH: $goBinPath"
    Write-Host "`nFix with:" -ForegroundColor Yellow
    Write-Host "  [Environment]::SetEnvironmentVariable('Path', `$env:Path + ';$goBinPath', 'User')" -ForegroundColor Gray
}

# =============================================================================
# TEST 3: Basic Commands
# =============================================================================
Write-TestHeader "Test 3: Basic Commands"

# Test: gears version
$versionTest = Test-GearsCommand "gears version"
if ($versionTest.Success) {
    Write-TestResult "gears version" "PASS" ($versionTest.Output -join ' ').Trim()
} else {
    Write-TestResult "gears version" "FAIL" $versionTest.Output
}

# Test: gears help
$helpTest = Test-GearsCommand "gears help"
if ($helpTest.Success) {
    Write-TestResult "gears help" "PASS" "Help output accessible"
} else {
    Write-TestResult "gears help" "FAIL" $helpTest.Output
}

# Test: gears --version flag
$vFlagTest = Test-GearsCommand "gears --version"
if ($vFlagTest.Success) {
    Write-TestResult "gears --version flag" "PASS"
} else {
    Write-TestResult "gears --version flag" "WARN" "Version flag may not be implemented"
}

# =============================================================================
# TEST 4: Workspace Initialization (No Auth Required)
# =============================================================================
Write-TestHeader "Test 4: Workspace Initialization"

Write-Host "Creating test workspace: $TestDir`n" -ForegroundColor Gray
New-Item -ItemType Directory -Path $TestDir -Force | Out-Null
Push-Location $TestDir

try {
    # Test: gears init
    $initTest = Test-GearsCommand "gears init"
    if ($initTest.Success) {
        Write-TestResult "gears init" "PASS" "Workspace initialized"
        
        # Verify .gears directory structure
        $expectedDirs = @("sessions", "story", "adr", "memory", "context")
        foreach ($dir in $expectedDirs) {
            $dirPath = Join-Path ".gears" $dir
            if (Test-Path $dirPath) {
                Write-TestResult ".gears/$dir created" "PASS"
            } else {
                Write-TestResult ".gears/$dir created" "FAIL" "Directory missing"
            }
        }
        
        # Check config.json
        $configPath = ".gears\config.json"
        if (Test-Path $configPath) {
            Write-TestResult "config.json created" "PASS"
            
            # Validate config.json structure
            try {
                $config = Get-Content $configPath -Raw | ConvertFrom-Json
                
                if ($config.workspace_id) {
                    Write-TestResult "workspace_id generated" "PASS" $config.workspace_id.Substring(0, 8) + "..."
                } else {
                    Write-TestResult "workspace_id generated" "FAIL" "Missing in config"
                }
                
                if ($config.api_base_url) {
                    Write-TestResult "api_base_url configured" "PASS" $config.api_base_url
                    
                    # Verify it's pointing to production
                    if ($config.api_base_url -eq "https://mygears.dev/api/v1") {
                        Write-TestResult "Production API endpoint" "PASS" "Correct default URL"
                    } else {
                        Write-TestResult "Production API endpoint" "WARN" "Non-standard: $($config.api_base_url)"
                    }
                } else {
                    Write-TestResult "api_base_url configured" "FAIL" "Missing in config"
                }
                
            } catch {
                Write-TestResult "config.json valid JSON" "FAIL" $_.Exception.Message
            }
        } else {
            Write-TestResult "config.json created" "FAIL" "File not created"
        }
        
    } else {
        Write-TestResult "gears init" "FAIL" $initTest.Output
    }
    
    # Test: gears session (should work without auth)
    $sessionTest = Test-GearsCommand "gears session"
    if ($sessionTest.Success) {
        Write-TestResult "gears session" "PASS" "Session file created"
        
        $todaySession = ".gears\sessions\$(Get-Date -Format 'yyyy-MM-dd').md"
        if (Test-Path $todaySession) {
            Write-TestResult "Today's session file exists" "PASS" (Split-Path $todaySession -Leaf)
        } else {
            Write-TestResult "Today's session file exists" "FAIL" "File not created"
        }
    } else {
        Write-TestResult "gears session" "FAIL" $sessionTest.Output
    }
    
    # Test: Re-running init (should be idempotent)
    $reinitTest = Test-GearsCommand "gears init"
    if ($reinitTest.Success) {
        Write-TestResult "Re-running init (idempotent)" "PASS" "No errors on re-init"
    } else {
        Write-TestResult "Re-running init (idempotent)" "WARN" "Init not idempotent"
    }
    
} finally {
    Pop-Location
}

# =============================================================================
# TEST 5: API Connectivity (No Auth Required)
# =============================================================================
Write-TestHeader "Test 5: API Connectivity"

try {
    $apiUrl = "https://mygears.dev"
    $response = Invoke-WebRequest -Uri $apiUrl -Method Head -TimeoutSec 10 -ErrorAction Stop
    
    if ($response.StatusCode -eq 200) {
        Write-TestResult "mygears.dev reachable" "PASS" "HTTP $($response.StatusCode)"
    } else {
        Write-TestResult "mygears.dev reachable" "WARN" "HTTP $($response.StatusCode)"
    }
} catch {
    Write-TestResult "mygears.dev reachable" "FAIL" $_.Exception.Message
    Write-Host "`n  Check your internet connection or firewall settings" -ForegroundColor Yellow
}

# Test API endpoint specifically
try {
    $apiEndpoint = "https://mygears.dev/api/v1"
    $response = Invoke-WebRequest -Uri $apiEndpoint -Method Get -TimeoutSec 10 -ErrorAction Stop
    Write-TestResult "API endpoint accessible" "PASS" $apiEndpoint
} catch {
    if ($_.Exception.Response.StatusCode -eq 401 -or $_.Exception.Response.StatusCode -eq 404) {
        Write-TestResult "API endpoint accessible" "PASS" "Endpoint responds (auth not required for test)"
    } else {
        Write-TestResult "API endpoint accessible" "WARN" $_.Exception.Message
    }
}

# =============================================================================
# TEST 6: Environment Variable Override
# =============================================================================
Write-TestHeader "Test 6: Environment Variable Override"

$env:GEARS_API_URL = "http://localhost:8080/api/v1"
Push-Location $TestDir

try {
    # Re-run init to generate new config
    $overrideTest = Test-GearsCommand "gears init"
    
    if (Test-Path ".gears\config.json") {
        $config = Get-Content ".gears\config.json" -Raw | ConvertFrom-Json
        
        if ($config.api_base_url -eq "http://localhost:8080/api/v1") {
            Write-TestResult "GEARS_API_URL override works" "PASS" "Config reflects env var"
        } else {
            Write-TestResult "GEARS_API_URL override works" "FAIL" "Expected localhost, got $($config.api_base_url)"
        }
    }
} finally {
    Remove-Item env:GEARS_API_URL
    Pop-Location
}

# =============================================================================
# TEST 7: Common User Mistakes
# =============================================================================
Write-TestHeader "Test 7: Common User Mistakes"

# Mistake 1: Running init from project subdirectory instead of workspace root
$projectDir = Join-Path $TestDir "projects\myapp"
New-Item -ItemType Directory -Path $projectDir -Force | Out-Null
Push-Location $projectDir

try {
    $wrongDirTest = Test-GearsCommand "gears init"
    if ($wrongDirTest.Success) {
        if (Test-Path ".gears") {
            Write-TestResult "Init from project subdir creates .gears" "WARN" "Users might do this incorrectly"
            Write-Host "  Note: .gears should be at workspace root, not project root" -ForegroundColor Yellow
        }
    }
} finally {
    Pop-Location
}

# Mistake 2: Check if multiple .gears directories exist
$gearsCount = (Get-ChildItem -Path $TestDir -Directory -Recurse -Filter ".gears" -ErrorAction SilentlyContinue).Count
if ($gearsCount -gt 1) {
    Write-TestResult "Multiple .gears directories" "WARN" "Found $gearsCount - should only be one at workspace root"
} else {
    Write-TestResult "Single .gears directory" "PASS" "Correct workspace structure"
}

# =============================================================================
# Cleanup
# =============================================================================
if (-not $SkipCleanup) {
    Write-Host "`nCleaning up test workspace..." -ForegroundColor Gray
    Remove-Item -Path $TestDir -Recurse -Force -ErrorAction SilentlyContinue
}

# =============================================================================
# Summary
# =============================================================================
Write-TestHeader "Test Summary"

$total = $PassCount + $FailCount + $WarnCount
Write-Host "Total Tests: $total" -ForegroundColor Cyan
Write-Host "  ✓ Passed:  $PassCount" -ForegroundColor Green
if ($WarnCount -gt 0) {
    Write-Host "  ⚠ Warnings: $WarnCount" -ForegroundColor Yellow
}
if ($FailCount -gt 0) {
    Write-Host "  ✗ Failed:  $FailCount" -ForegroundColor Red
}

Write-Host ""

# Exit code
if ($FailCount -gt 0) {
    Write-Host "RESULT: Installation has issues that need to be fixed" -ForegroundColor Red
    Write-Host "`nNext steps:" -ForegroundColor Yellow
    Write-Host "  1. Review failed tests above"
    Write-Host "  2. Check installation docs: https://mygears.dev/docs/installation"
    Write-Host "  3. Verify Go is installed: go version"
    Write-Host "  4. Re-install: go install github.com/Syluxso/gears@latest"
    exit 1
} elseif ($WarnCount -gt 0) {
    Write-Host "RESULT: Installation works but has warnings" -ForegroundColor Yellow
    Write-Host "`nYour installation is functional but review warnings above for best practices." -ForegroundColor Gray
    exit 0
} else {
    Write-Host "RESULT: Installation is working correctly! ✓" -ForegroundColor Green
    Write-Host "`nNext steps:" -ForegroundColor Cyan
    Write-Host "  1. Navigate to your workspace root: cd /root"
    Write-Host "  2. Initialize: gears init"
    Write-Host "  3. Authenticate: gears auth"
    Write-Host "  4. Start working: gears session"
    Write-Host "`nDocumentation: https://mygears.dev/docs/getting-started" -ForegroundColor Gray
    exit 0
}
