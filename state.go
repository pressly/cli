package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// State is the value passed to [Command.Exec]. It holds the parsed inputs the command needs to run.
type State struct {
	// Args holds the positional arguments left after the command name and flags are parsed.
	// Anything after "--" is included as-is, even if it looks like a flag.
	Args []string

	// Stdin, Stdout, and Stderr are the streams to use in your command code instead of os.Stdin,
	// os.Stdout, and os.Stderr. Tests can swap them via [RunOptions].
	Stdin          io.Reader
	Stdout, Stderr io.Writer

	// Cmd is the command that was picked. Call Cmd.Path() to get the full list of commands from the
	// root down, useful for error messages that include the command path.
	Cmd *Command

	// path is the command hierarchy from the root command to the current command. The root command
	// is the first element in the path, and the terminal command is the last element.
	path []*Command
}

// GetFlag returns the value of a flag as type T. Call it from inside [Command.Exec] with the same
// Go type that was used when the flag was defined.
//
// GetFlag looks for the flag on the picked command first, then in its parent commands. A flag
// defined on the root command can be read from any subcommand. An unknown flag name or a wrong type
// is a programming error: GetFlag panics, and [Run] catches the panic and returns the error.
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
