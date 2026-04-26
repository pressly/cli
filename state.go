package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// State is passed to Exec with the parsed invocation context.
//
// Use Args for remaining positional arguments, Stdin/Stdout/Stderr for command I/O, Cmd for the
// selected command, and [GetFlag] to read parsed flag values.
type State struct {
	// Args contains positional arguments left after command and flag parsing.
	Args []string

	// Stdin, Stdout, and Stderr are the streams command code should use instead of package-level
	// os.Stdin, os.Stdout, and os.Stderr.
	Stdin          io.Reader
	Stdout, Stderr io.Writer

	// Cmd is the command selected by parsing.
	Cmd *Command

	// path is the command hierarchy from the root command to the current command. The root command
	// is the first element in the path, and the terminal command is the last element.
	path []*Command
}

// GetFlag reads a parsed flag value from State.
//
// Call GetFlag from Exec with the same Go type used to define the flag. It checks the selected
// command first, then inherited parent flags. A missing flag or wrong type is treated as a
// programming error and returned from [Run].
//
//	verbose := GetFlag[bool](state, "verbose")
//	count := GetFlag[int](state, "count")
//	path := GetFlag[string](state, "path")
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
