#!/usr/bin/env bash
# Script to create a temporary jj repository and test jj-diff operations
# This allows testing write operations outside the main repository

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Cleanup function
cleanup() {
    if [ -n "$TEST_DIR" ] && [ -d "$TEST_DIR" ]; then
        echo -e "${YELLOW}Cleaning up test directory: $TEST_DIR${NC}"
        rm -rf "$TEST_DIR"
    fi
}

# Set trap to cleanup on exit
trap cleanup EXIT

# Create temporary directory
TEST_DIR=$(mktemp -d -t jj-diff-test-XXXXXX)
echo -e "${BLUE}Created test directory: $TEST_DIR${NC}"

# Change to test directory
cd "$TEST_DIR"

# Initialize jj repository
echo -e "${BLUE}Initializing jj repository...${NC}"
jj init --git

# Create initial file and commit
echo -e "${BLUE}Creating initial commit...${NC}"
cat > file1.txt << 'EOF'
line 1
line 2
line 3
line 4
line 5
EOF

cat > file2.go << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Hello")
    fmt.Println("World")
}
EOF

jj commit -m "Initial commit"

# Create changes in working copy
echo -e "${BLUE}Creating changes in working copy...${NC}"
cat > file1.txt << 'EOF'
line 1 modified
line 2
line 3 modified
line 4
line 5
line 6 added
EOF

cat > file2.go << 'EOF'
package main

import (
    "fmt"
    "log"
)

func main() {
    log.Println("Starting")
    fmt.Println("Hello")
    fmt.Println("World")
    log.Println("Done")
}
EOF

cat > file3.txt << 'EOF'
new file
with content
EOF

# Show current state
echo -e "\n${GREEN}=== Repository State ===${NC}"
jj log -r 'all()' --summary

echo -e "\n${GREEN}=== Working Copy Changes ===${NC}"
jj diff

echo -e "\n${GREEN}=== Test Repository Ready ===${NC}"
echo -e "Directory: ${BLUE}$TEST_DIR${NC}"
echo -e "\nYou can now:"
echo -e "  1. cd $TEST_DIR"
echo -e "  2. Run jj-diff from the parent directory"
echo -e "  3. Test MoveChanges and other operations"
echo -e "\nAvailable test scenarios:"
echo -e "  - Modified lines: file1.txt (lines 1, 3, 6)"
echo -e "  - Multiple hunks: file2.go (import + log statements)"
echo -e "  - New file: file3.txt"
echo -e "\n${YELLOW}Press Enter to run jj-diff in browse mode, or Ctrl-C to exit${NC}"
read -r

# Get the jj-diff binary path (assumes it's in the parent directory structure)
JJ_DIFF_BIN=""
if [ -x "../jj-diff" ]; then
    JJ_DIFF_BIN="../jj-diff"
elif [ -x "../../jj-diff" ]; then
    JJ_DIFF_BIN="../../jj-diff"
elif [ -x "/Users/kyleking/Developer/local-code/jj-staging/jj-diff/jj-diff" ]; then
    JJ_DIFF_BIN="/Users/kyleking/Developer/local-code/jj-staging/jj-diff/jj-diff"
else
    echo -e "${RED}Error: jj-diff binary not found. Please build it first with 'go build'${NC}"
    echo -e "${YELLOW}Test directory will remain at: $TEST_DIR${NC}"
    trap - EXIT  # Disable cleanup
    exit 1
fi

echo -e "${BLUE}Running jj-diff from: $JJ_DIFF_BIN${NC}"
"$JJ_DIFF_BIN" --browse

echo -e "\n${GREEN}Test complete!${NC}"
echo -e "${YELLOW}Test directory will be cleaned up on exit${NC}"
