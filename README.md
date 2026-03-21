# Gears

Command-line installer and management tool for the Gears documentation framework.

## Installation

```bash
go install github.com/syluxso/gears@latest
```

Then run `gears --version` to verify it's installed.

## Development

### Build

```bash
go build -o gears
```

### Run

```bash
# Check version
./gears --version
./gears -v

# Get help
./gears --help
```

### Install locally

```bash
go install
```

Then `gears` will be available in your PATH (assuming `$GOPATH/bin` is in PATH).

## Commands

- `gears init` - Initialize a new .gears documentation structure
- `gears session` - Create or update daily session files
- `gears story new/list` - Manage feature stories
- `gears adr new/list` - Document architectural decisions from working code

## Status

🟢 Core features implemented - ready for use
