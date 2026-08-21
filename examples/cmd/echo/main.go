package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pressly/cli"
)

func main() {
	root := &cli.Command{
		Name:  "echo",
		Usage: "echo [flags] <text>...",
		Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
			f.Bool("capitalize", false, "capitalize the input")
		}),
		Exec: func(ctx context.Context, s *cli.State) error {
			text := strings.Join(s.Args, " ")
			// GetFlag uses generic methods, available in Go 1.27 or later.
			if s.GetFlag[bool]("capitalize") {
				text = strings.ToUpper(text)
			}
			fmt.Fprintln(s.Stdout, text)
			return nil
		},
	}
	if err := cli.ParseAndRun(context.Background(), root, os.Args[1:], nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
