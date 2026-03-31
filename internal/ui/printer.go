package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

var (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

var noColor bool

func init() {
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		noColor = true
	}
}

// SetNoColor disables color output.
func SetNoColor(v bool) {
	noColor = v
}

func color(c, s string) string {
	if noColor {
		return s
	}
	return c + s + colorReset
}

// Bold returns text in bold.
func Bold(s string) string { return color(colorBold, s) }

// Dim returns text in dim.
func Dim(s string) string { return color(colorDim, s) }

// Green returns text in green.
func Green(s string) string { return color(colorGreen, s) }

// Red returns text in red.
func Red(s string) string { return color(colorRed, s) }

// Yellow returns text in yellow.
func Yellow(s string) string { return color(colorYellow, s) }

// Cyan returns text in cyan.
func Cyan(s string) string { return color(colorCyan, s) }

// Success prints a green success message.
func Success(format string, args ...interface{}) {
	fmt.Println(Green(fmt.Sprintf(format, args...)))
}

// Error prints a red error message to stderr.
func Error(format string, args ...interface{}) {
	fmt.Fprintln(os.Stderr, Red(fmt.Sprintf("Error: "+format, args...)))
}

// Warn prints a yellow warning message.
func Warn(format string, args ...interface{}) {
	fmt.Println(Yellow(fmt.Sprintf("Warning: "+format, args...)))
}

// Info prints a cyan info message.
func Info(format string, args ...interface{}) {
	fmt.Println(Cyan(fmt.Sprintf(format, args...)))
}

// Confirm prompts the user for a yes/no confirmation. Returns true if user confirms.
func Confirm(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
