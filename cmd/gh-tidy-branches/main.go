package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/teamleaderleo/gh-tidy-branches/internal/app"
)

var version = "0.1.0-dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version":
			fmt.Fprintln(os.Stdout, version)
			return
		case "doctor":
			var output bytes.Buffer
			if err := app.Run(ctx, args, os.Stdin, &output, os.Stderr); err != nil {
				fail(err)
			}
			text := strings.Replace(output.String(), "Version: "+app.Version, "Version: "+version, 1)
			fmt.Fprint(os.Stdout, text)
			return
		}
	}

	if err := app.Run(ctx, args, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gh tidy-branches:", err)
	os.Exit(1)
}
