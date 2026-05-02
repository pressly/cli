// Package cli provides a lightweight library for building command-line applications using Go's
// standard library flag package. It extends flag functionality to support flags anywhere in command
// arguments.
//
// Key features:
//   - Nested subcommands for organizing complex CLIs
//   - Flexible flag parsing, allowing flags anywhere in arguments
//   - Parent-to-child flag inheritance
//   - Type-safe flag access
//   - Automatic help text generation
//   - Command suggestions for misspelled inputs
//
// Quick example:
//
//	root := &cli.Command{
//	    Name:        "echo",
//	    Usage:       "echo [flags] <text>...",
//	    Description: "prints the provided text",
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
//
// The package intentionally maintains a minimal API surface to serve as a building block for CLI
// applications while leveraging the standard library's flag package. This approach enables
// developers to build maintainable command-line tools quickly while focusing on application logic
// rather than framework complexity.
package cli
