package app

import (
	"io"
	"os"
	"strings"
)

type terminalStyle struct {
	color bool
}

func newTerminalStyle(writer io.Writer) terminalStyle {
	return terminalStyle{color: colorEnabled(writer)}
}

func colorEnabled(writer io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("CLICOLOR") == "0" || strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	if force := os.Getenv("CLICOLOR_FORCE"); force != "" && force != "0" {
		return true
	}
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func (s terminalStyle) wrap(code, value string) string {
	if !s.color || value == "" {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

func (s terminalStyle) bold(value string) string   { return s.wrap("1", value) }
func (s terminalStyle) dim(value string) string    { return s.wrap("2", value) }
func (s terminalStyle) red(value string) string    { return s.wrap("31", value) }
func (s terminalStyle) green(value string) string  { return s.wrap("32", value) }
func (s terminalStyle) yellow(value string) string { return s.wrap("33", value) }
func (s terminalStyle) cyan(value string) string   { return s.wrap("36", value) }
