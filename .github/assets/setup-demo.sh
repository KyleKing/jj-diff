#!/usr/bin/env bash
set -euo pipefail

# Create initial files with content
cat > main.go << 'EOF'
package main

import (
	"fmt"
	"log"
)

func main() {
	fmt.Println("Hello, World!")
	processData()
}

func processData() {
	data := []string{"apple", "banana", "cherry"}
	for _, item := range data {
		fmt.Println(item)
	}
}

func calculateSum(nums []int) int {
	sum := 0
	for _, n := range nums {
		sum += n
	}
	return sum
}
EOF

mkdir -p utils
cat > utils/helpers.go << 'EOF'
package utils

import "strings"

func FormatName(name string) string {
	return strings.ToUpper(name)
}

func ValidateEmail(email string) bool {
	return strings.Contains(email, "@")
}
EOF

cat > README.md << 'EOF'
# Demo Project

This is a demo project for jj-diff.

## Features

- Simple Go application
- Utility functions
- Configuration management

## Usage

Run the application with:

```bash
go run main.go
```
EOF

# Create initial commit
jj describe -m "Initial commit with basic structure"

# Make working copy changes - add new features, modify existing code
cat > main.go << 'EOF'
package main

import (
	"fmt"
	"log"
	"os"
	"utils"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Printf("Hello, %s!\n", utils.FormatName(os.Args[1]))
	} else {
		fmt.Println("Hello, World!")
	}

	processData()

	numbers := []int{1, 2, 3, 4, 5}
	sum := calculateSum(numbers)
	fmt.Printf("Sum: %d\n", sum)

	average := calculateAverage(numbers)
	fmt.Printf("Average: %.2f\n", average)
}

func processData() {
	data := []string{"apple", "banana", "cherry", "date", "elderberry"}
	fmt.Println("Processing data:")
	for i, item := range data {
		fmt.Printf("%d: %s\n", i+1, item)
	}
}

func calculateSum(nums []int) int {
	sum := 0
	for _, n := range nums {
		sum += n
	}
	return sum
}

func calculateAverage(nums []int) float64 {
	if len(nums) == 0 {
		return 0
	}
	sum := calculateSum(nums)
	return float64(sum) / float64(len(nums))
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
EOF

cat > utils/helpers.go << 'EOF'
package utils

import (
	"regexp"
	"strings"
)

func FormatName(name string) string {
	return strings.ToTitle(name)
}

func ValidateEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func IsPalindrome(s string) bool {
	cleaned := strings.ToLower(strings.ReplaceAll(s, " ", ""))
	return cleaned == Reverse(cleaned)
}
EOF

cat > config.yaml << 'EOF'
app:
  name: demo-app
  version: 1.0.0
  debug: true

server:
  host: localhost
  port: 8080
  timeout: 30s

database:
  driver: postgres
  host: localhost
  port: 5432
  name: demo_db
  user: demo_user

logging:
  level: info
  format: json
  output: stdout
EOF

echo "Demo repository setup complete!"
