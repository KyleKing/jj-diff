#!/usr/bin/env bash
# Interactive testing script for jj-diff
# Creates various test scenarios and allows testing different workflows

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Cleanup function
cleanup() {
    if [ -n "$TEST_DIR" ] && [ -d "$TEST_DIR" ]; then
        echo -e "${YELLOW}Cleaning up test directory: $TEST_DIR${NC}"
        rm -rf "$TEST_DIR"
    fi
}

trap cleanup EXIT

# Find jj-diff binary
find_jj_diff() {
    local locations=(
        "../jj-diff"
        "../../jj-diff"
        "/Users/kyleking/Developer/local-code/jj-staging/jj-diff/jj-diff"
        "./jj-diff"
    )

    for loc in "${locations[@]}"; do
        if [ -x "$loc" ]; then
            echo "$loc"
            return 0
        fi
    done

    return 1
}

# Create test scenario 1: Simple changes
scenario_simple() {
    echo -e "${BLUE}Creating scenario: Simple Changes${NC}"

    cat > main.go << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
EOF

    jj commit -m "Initial version"

    cat > main.go << 'EOF'
package main

import (
    "fmt"
    "log"
)

func main() {
    log.Println("Starting application")
    fmt.Println("Hello, World!")
    fmt.Println("Goodbye, World!")
}
EOF

    echo -e "${GREEN}✓ Simple changes scenario ready${NC}"
    echo -e "  - Modified: main.go (added imports and log statements)"
}

# Create test scenario 2: Multiple files
scenario_multiple_files() {
    echo -e "${BLUE}Creating scenario: Multiple Files${NC}"

    mkdir -p src/utils

    cat > src/main.go << 'EOF'
package main

func main() {
    println("main")
}
EOF

    cat > src/utils/helper.go << 'EOF'
package utils

func Helper() string {
    return "helper"
}
EOF

    jj commit -m "Add multiple files"

    cat > src/main.go << 'EOF'
package main

import "myapp/utils"

func main() {
    println("main")
    println(utils.Helper())
}
EOF

    cat > src/utils/helper.go << 'EOF'
package utils

import "fmt"

func Helper() string {
    return fmt.Sprintf("helper v2")
}

func NewHelper() string {
    return "new helper"
}
EOF

    cat > README.md << 'EOF'
# My App

New readme file
EOF

    echo -e "${GREEN}✓ Multiple files scenario ready${NC}"
    echo -e "  - Modified: src/main.go, src/utils/helper.go"
    echo -e "  - New: README.md"
}

# Create test scenario 3: Large diff
scenario_large_diff() {
    echo -e "${BLUE}Creating scenario: Large Diff${NC}"

    cat > data.txt << 'EOF'
EOF

    for i in {1..100}; do
        echo "Line $i: original content" >> data.txt
    done

    jj commit -m "Add data file"

    # Modify various lines throughout
    {
        for i in {1..100}; do
            if [ $((i % 10)) -eq 0 ]; then
                echo "Line $i: MODIFIED CONTENT"
            else
                echo "Line $i: original content"
            fi
        done
        echo "Line 101: NEW LINE"
        echo "Line 102: NEW LINE"
    } > data.txt

    echo -e "${GREEN}✓ Large diff scenario ready${NC}"
    echo -e "  - Modified: data.txt (100+ lines with multiple hunks)"
}

# Create test scenario 4: Move changes workflow
scenario_move_workflow() {
    echo -e "${BLUE}Creating scenario: Move Changes Workflow${NC}"

    cat > feature.go << 'EOF'
package main

func Feature() {
    // Feature implementation
}
EOF

    jj commit -m "WIP: feature work"

    cat > feature.go << 'EOF'
package main

import "fmt"

func Feature() {
    fmt.Println("Debug: entering feature")
    // Feature implementation
    businessLogic()
    fmt.Println("Debug: exiting feature")
}

func businessLogic() {
    // Core logic here
}
EOF

    # Create a destination revision
    jj new -m "Clean feature implementation"
    jj new @- -m "Debug statements"

    echo -e "${GREEN}✓ Move changes workflow scenario ready${NC}"
    echo -e "  - Working copy has mixed changes (business logic + debug)"
    echo -e "  - Two destinations: 'Clean feature' and 'Debug statements'"
    echo -e "  - Use interactive mode to separate concerns"
}

# Main menu
show_menu() {
    echo -e "\n${CYAN}╔════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║    jj-diff Interactive Test Suite     ║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════╝${NC}\n"
    echo -e "Select a test scenario:\n"
    echo -e "  ${GREEN}1${NC} - Simple Changes (single file, few hunks)"
    echo -e "  ${GREEN}2${NC} - Multiple Files (nested directories, new files)"
    echo -e "  ${GREEN}3${NC} - Large Diff (100+ lines, many hunks)"
    echo -e "  ${GREEN}4${NC} - Move Changes Workflow (interactive mode testing)"
    echo -e "  ${GREEN}5${NC} - Custom (open shell in test repo)"
    echo -e "  ${RED}q${NC} - Quit\n"
}

# Main execution
main() {
    # Check for jj-diff binary
    if ! JJ_DIFF=$(find_jj_diff); then
        echo -e "${RED}Error: jj-diff binary not found${NC}"
        echo -e "${YELLOW}Please build it first: cd /Users/kyleking/Developer/local-code/jj-staging/jj-diff && go build${NC}"
        exit 1
    fi

    echo -e "${GREEN}Found jj-diff at: $JJ_DIFF${NC}"

    # Create temporary directory
    TEST_DIR=$(mktemp -d -t jj-diff-test-XXXXXX)
    echo -e "${BLUE}Test directory: $TEST_DIR${NC}\n"

    cd "$TEST_DIR"
    jj init --git

    while true; do
        show_menu
        read -r -p "$(echo -e ${CYAN}Enter choice: ${NC})" choice

        case $choice in
            1)
                scenario_simple
                ;;
            2)
                scenario_multiple_files
                ;;
            3)
                scenario_large_diff
                ;;
            4)
                scenario_move_workflow
                ;;
            5)
                echo -e "${BLUE}Opening shell in test repository${NC}"
                echo -e "${YELLOW}Test directory: $TEST_DIR${NC}"
                echo -e "${YELLOW}Run 'exit' to return to menu${NC}\n"
                bash
                continue
                ;;
            q|Q)
                echo -e "${GREEN}Exiting...${NC}"
                exit 0
                ;;
            *)
                echo -e "${RED}Invalid choice${NC}"
                continue
                ;;
        esac

        # Show current state
        echo -e "\n${CYAN}=== Current State ===${NC}"
        jj log -r 'all()' -T 'commit_id.short() ++ " " ++ description'

        echo -e "\n${CYAN}=== Files Changed ===${NC}"
        jj diff --stat || true

        # Ask what to do next
        echo -e "\n${YELLOW}What would you like to do?${NC}"
        echo -e "  ${GREEN}b${NC} - Run jj-diff in browse mode"
        echo -e "  ${GREEN}i${NC} - Run jj-diff in interactive mode"
        echo -e "  ${GREEN}d${NC} - Show full diff"
        echo -e "  ${GREEN}s${NC} - Open shell in test repo"
        echo -e "  ${GREEN}m${NC} - Return to main menu"
        echo -e "  ${RED}q${NC} - Quit"

        read -r -p "$(echo -e ${CYAN}Choice: ${NC})" action

        case $action in
            b|B)
                "$JJ_DIFF" --browse
                ;;
            i|I)
                "$JJ_DIFF" --interactive
                ;;
            d|D)
                jj diff
                read -r -p "$(echo -e ${YELLOW}Press Enter to continue...${NC})"
                ;;
            s|S)
                echo -e "${BLUE}Opening shell in test repository${NC}"
                echo -e "${YELLOW}Test directory: $TEST_DIR${NC}"
                echo -e "${YELLOW}Run 'exit' to return${NC}\n"
                bash
                ;;
            m|M)
                continue
                ;;
            q|Q)
                echo -e "${GREEN}Exiting...${NC}"
                exit 0
                ;;
        esac
    done
}

main "$@"
