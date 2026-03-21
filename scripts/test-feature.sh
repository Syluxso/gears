#!/bin/bash
# Test gears CLI features in a temporary directory

set -e

# Colors
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get workspace root (3 levels up from script location)
WORKSPACE_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
TEST_DIR="$WORKSPACE_ROOT/test-feature"

echo -e "${CYAN}Testing gears CLI features${NC}"
echo -e "${YELLOW}Test directory: $TEST_DIR${NC}"
echo ""

# Create test directory
echo -e "${CYAN}Creating test directory...${NC}"
mkdir -p "$TEST_DIR"
cd "$TEST_DIR"

# Test gears init
echo -e "${CYAN}Testing: gears init${NC}"
gears init
echo ""

# Test gears session
echo -e "${CYAN}Testing: gears session${NC}"
gears session
echo ""

# Test gears story new
echo -e "${CYAN}Testing: gears story new 'test feature'${NC}"
gears story new "test feature"
echo ""

# Test gears adr new
echo -e "${CYAN}Testing: gears adr new 'test pattern'${NC}"
gears adr new "test pattern"
echo ""

# Test gears story list
echo -e "${CYAN}Testing: gears story list${NC}"
gears story list
echo ""

# Test gears adr list
echo -e "${CYAN}Testing: gears adr list${NC}"
gears adr list
echo ""

# Show created structure
echo -e "${CYAN}Created .gears structure:${NC}"
tree -L 2 .gears 2>/dev/null || find .gears -maxdepth 2 -print 2>/dev/null || ls -R .gears
echo ""

# Cleanup prompt
echo -e "${YELLOW}Test complete!${NC}"
read -p "Delete test directory? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    cd "$WORKSPACE_ROOT"
    rm -rf "$TEST_DIR"
    echo -e "${GREEN}✓ Test directory deleted${NC}"
else
    echo -e "${YELLOW}Test directory preserved: $TEST_DIR${NC}"
fi
