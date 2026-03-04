package ui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Prompt prints a question with an optional default and returns user input.
func Prompt(scanner *bufio.Scanner, question, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", question, defaultVal)
	} else {
		fmt.Printf("%s: ", question)
	}
	if scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			return text
		}
	}
	return defaultVal
}

// Confirm asks a y/n question, defaulting to yes.
func Confirm(scanner *bufio.Scanner, question string) bool {
	fmt.Printf("%s [Y/n]: ", question)
	if scanner.Scan() {
		text := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return text == "" || text == "y" || text == "yes"
	}
	return false
}

// IsTerminal reports whether stdin is connected to a terminal.
func IsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// Choose presents numbered options and returns the selected one.
func Choose(scanner *bufio.Scanner, question string, options []string) string {
	fmt.Printf("%s:\n", question)
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}
	fmt.Printf("Choose [1]: ")
	if scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if n, err := strconv.Atoi(text); err == nil && n >= 1 && n <= len(options) {
			return options[n-1]
		}
	}
	return options[0]
}
