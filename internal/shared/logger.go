package shared

import (
	"fmt"
	"os"
)

var Verbose bool

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	boldRed     = "\033[1;31m"
	dimGray     = "\033[2;90m"
)

func Info(format string, args ...interface{}) {
	prefix := colorCyan + "[INFO]" + colorReset
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", prefix, msg)
}

func Debug(format string, args ...interface{}) {
	if !Verbose {
		return
	}
	prefix := dimGray + "[DEBUG]" + colorReset
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", prefix, msg)
}

func Warn(format string, args ...interface{}) {
	prefix := colorYellow + "[WARN]" + colorReset
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", prefix, msg)
}

func Error(format string, args ...interface{}) {
	prefix := boldRed + "[ERROR]" + colorReset
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", prefix, msg)
}

func Fatal(format string, args ...interface{}) {
	prefix := boldRed + "[FATAL]" + colorReset
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", prefix, msg)
	os.Exit(1)
}

func Success(format string, args ...interface{}) {
	prefix := colorGreen + "[OK]" + colorReset
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", prefix, msg)
}
