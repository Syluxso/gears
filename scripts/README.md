# Gears CLI Scripts

Development and testing scripts for the gears CLI tool.

## Build Scripts

Build and install the gears CLI to your system.

### Git Bash / Linux / macOS

```bash
./scripts/build.sh
```

### PowerShell / Windows

```powershell
.\scripts\build.ps1
```

**What it does:**

1. Builds `gears` (or `gears.exe` on Windows)
2. Installs to `$GOPATH/bin` (typically `~/go/bin` or `%USERPROFILE%\go\bin`)
3. Shows installed location and version

## Test Scripts

Test all gears CLI features in a temporary directory, then optionally clean up.

### Git Bash / Linux / macOS

```bash
./scripts/test-feature.sh
```

### PowerShell / Windows

```powershell
.\scripts\test-feature.ps1
```

**What it does:**

1. Creates test directory at workspace root: `test-feature/`
2. Runs these commands:
   - `gears init` - Initialize .gears structure
   - `gears session` - Create session file
   - `gears story new "test feature"` - Create story
   - `gears adr new "test pattern"` - Create ADR
   - `gears story list` - List stories
   - `gears adr list` - List ADRs
3. Shows created directory structure
4. Prompts to delete test directory (default: keep)

## Quick Development Workflow

```bash
# After making code changes:
./scripts/build.sh

# Test the changes:
./scripts/test-feature.sh
```

Or in PowerShell:

```powershell
.\scripts\build.ps1
.\scripts\test-feature.ps1
```

## Script Location

These scripts live in `/projects/gears/scripts/` to keep them with the CLI project source code.
