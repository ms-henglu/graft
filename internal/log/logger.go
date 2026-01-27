package log

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
)

var (
	isDebug bool
)

// Init sets the logging mode.
// If debug is true, enables timestamped, verbose output.
// If debug is false, uses standard clean output.
func Init(debug bool) {
	isDebug = debug
}

// Section prints a major step: [+] Message
func Section(msg string) {
	if isDebug {
		log("INFO", "[+] "+msg)
		return
	}
	color.Green("[+] %s\n", msg)
}

// Item prints a list item:     - Message
func Item(msg string) {
	if isDebug {
		log("INFO", "    - "+msg)
		return
	}
	fmt.Printf("    - %s\n", msg)
}

// Success prints a success message: ✨ Message
func Success(msg string) {
	if isDebug {
		log("INFO", "✨ "+msg)
		return
	}
	// Bold and Green
	c := color.New(color.FgGreen, color.Bold)
	_, _ = c.Printf("✨ %s\n", msg)
}

// Warn prints a warning message: [!] Message
func Warn(msg string) {
	if isDebug {
		log("WARN", "[!] "+msg)
		return
	}
	color.Yellow("[!] %s\n", msg)
}

// Error prints an error message: [✘] Message
func Error(msg string) {
	if isDebug {
		log("ERROR", "[✘] "+msg)
		return
	}
	color.Red("[✘] %s\n", msg)
}

// Hint prints a hint message: -> Message
func Hint(msg string) {
	if isDebug {
		log("INFO", "-> "+msg)
		return
	}
	color.Cyan("-> %s\n", msg)
}

// Debug prints a debug message only if debug mode is enabled.
func Debug(format string, v ...interface{}) {
	if !isDebug {
		return
	}
	log("DEBUG", fmt.Sprintf(format, v...))
}

// log prints a standardized log message with timestamp.
func log(level, msg string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", timestamp, level, msg)
}
