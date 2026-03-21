#!/bin/bash
# Build and install gears CLI

set -e  # Exit on error

echo "Building gears CLI..."
cd "$(dirname "$0")/.."

# Build
go build -o gears

# Install to GOPATH/bin
echo "Installing to GOPATH/bin..."
go install

echo ""
echo "✓ Build and install complete!"
echo ""
echo "Installed location: $(which gears)"
echo "Version: $(gears --version)"
