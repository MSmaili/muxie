package logger

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

var (
	output io.Writer = os.Stderr

	successColor = color.New(color.FgGreen)
	errorColor   = color.New(color.FgRed)
	infoColor    = color.New(color.FgCyan)
	warningColor = color.New(color.FgYellow)
)

func Success(format string, args ...any) {
	successColor.Fprintf(output, format+"\n", args...)
}

func Error(format string, args ...any) {
	errorColor.Fprintf(output, format+"\n", args...)
}

func Info(format string, args ...any) {
	infoColor.Fprintf(output, format+"\n", args...)
}

func Warning(format string, args ...any) {
	warningColor.Fprintf(output, format+"\n", args...)
}

func Plain(format string, args ...any) {
	fmt.Fprintf(output, format+"\n", args...)
}
