// Package cli builds command-line programs on top of the standard library [flag] package. It adds
// nested subcommands and lets users place flags anywhere in command arguments.
//
// Features:
//   - Nested subcommands via [Command.SubCommands]
//   - Flags placed anywhere on the command line
//   - Parent flags inherited by child commands
//   - Type-safe flag access via [GetFlag]
//   - Generated help, replaceable per command via [Command.Help]
//   - "Did you mean" suggestions for misspelled subcommands
//
// Quick example:
//
//	root := &cli.Command{
//	    Name:        "echo",
//	    Usage:       "echo [flags] <text>...",
//	    Summary:     "Print text",
//	    Description: "echo prints the provided text.",
//	    Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
//	        f.Bool("c", false, "capitalize the input")
//	    }),
//	    Exec: func(ctx context.Context, s *cli.State) error {
//	        output := strings.Join(s.Args, " ")
//	        if cli.GetFlag[bool](s, "c") {
//	            output = strings.ToUpper(output)
//	        }
//	        fmt.Fprintln(s.Stdout, output)
//	        return nil
//	    },
//	}
//	if err := cli.ParseAndRun(ctx, root, os.Args[1:], nil); err != nil {
//	    fmt.Fprintf(os.Stderr, "error: %v\n", err)
//	    os.Exit(1)
//	}
//
// The API stays deliberately small. cli builds on the standard library's flag package instead of
// replacing it, so most of what you write is your program rather than the scaffolding around it.
package cli
