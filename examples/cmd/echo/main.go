package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pressly/cli"
)

func main() {
	root := &cli.Command{
		Name:      "echo",
		Usage:     "echo [flags] <text>...",
		ShortHelp: "echo is a simple command that prints the provided text",
		Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
			// Add a flag to capitalize the input
			f.Bool("c", false, "capitalize the input")
		}),
		FlagsMetadata: []cli.FlagMetadata{
			{Name: "c", Required: true},
		},
		Exec: func(ctx context.Context, s *cli.State) error {
			if len(s.Args) == 0 {
				return errors.New("must provide text to echo, see --help")
			}
			output := strings.Join(s.Args, " ")
			// If -c flag is set, capitalize the output
			if cli.GetFlag[bool](s, "c") {
				output = strings.ToUpper(output)
			}
			fmt.Fprintln(s.Stdout, output)
			return nil
		},
	}
	if err := cli.Parse(root, os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintf(os.Stdout, "%s\n", cli.DefaultUsage(root))
			return
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := cli.Run(context.Background(), root, nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
