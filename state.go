package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// State carries the parsed invocation context into [Command.Exec]. Use Args for positional
// arguments, Stdin/Stdout/Stderr for I/O, Cmd for the selected command, and [GetFlag] to read flag
// values.
type State struct {
	// Args holds the positional arguments left after command resolution and flag parsing. Anything
	// after a "--" delimiter is included verbatim, even if it would otherwise look like a flag.
	Args []string

	// Stdin, Stdout, and Stderr are the streams command code should use in place of the package-
	// level os.Stdin, os.Stdout, and os.Stderr. Tests can swap them via [RunOptions].
	Stdin          io.Reader
	Stdout, Stderr io.Writer

	// Cmd is the resolved (terminal) command. Call Cmd.Path() for the chain from the root down,
	// useful for command-aware error messages or breadcrumbs.
	Cmd *Command

	// path is the command hierarchy from the root command to the current command. The root command
	// is the first element in the path, and the terminal command is the last element.
	path []*Command
}

// GetFlag returns the parsed value of a flag, type-checked against T. Call it from inside
// [Command.Exec] with the same Go type that was used when the flag was defined.
//
// Lookup walks from the selected command up through inherited parent flags, so a flag defined on
// the root command is reachable from any subcommand. An unknown flag name, or one read with the
// wrong type, is treated as a programming error: GetFlag panics, and [Run] recovers and returns
// the error to the caller.
//
//	verbose := cli.GetFlag[bool](s, "verbose")
//	count   := cli.GetFlag[int](s, "count")
//	path    := cli.GetFlag[string](s, "path")
func GetFlag[T any](s *State, name string) T {
	if s == nil {
		panic(&internalError{err: errors.New("state is nil")})
	}
	// Try to find the flag in each command's flag set, starting from the current command
	for i := len(s.path) - 1; i >= 0; i-- {
		cmd := s.path[i]
		if cmd.Flags == nil {
			continue
		}

		if f := cmd.Flags.Lookup(name); f != nil {
			if getter, ok := f.Value.(flag.Getter); ok {
				value := getter.Get()
				if v, ok := value.(T); ok {
					return v
				}
				err := fmt.Errorf("type mismatch for flag %q in command %q: registered %T, requested %T",
					formatFlagName(name),
					getCommandPath(s.path),
					value,
					*new(T),
				)
				// Flag exists but type doesn't match - this is an internal error
				panic(&internalError{err: err})
			}
		}
	}

	// If flag not found anywhere in hierarchy, panic with helpful message
	err := fmt.Errorf("flag %q not found in command %q flag set",
		formatFlagName(name),
		getCommandPath(s.path),
	)
	panic(&internalError{err: err})
}

// internalError is a marker type for errors that originate from the cli package itself. These are
// programming errors (e.g., flag type mismatches) that should be caught during development.
type internalError struct {
	err error
}

func (e *internalError) Error() string {
	return e.err.Error()
}

func (e *internalError) Unwrap() error {
	return e.err
}
