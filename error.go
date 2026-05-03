package cli

import "fmt"

type usageError struct {
	err error
}

// UsageErrorf builds an error that signals invalid command-line usage. Return it from
// [Command.Exec] when the command was selected successfully but the arguments or flag combination
// are wrong:
//
//	if len(s.Args) == 0 {
//	    return cli.UsageErrorf("must supply a name")
//	}
//
// When [Run] sees a UsageErrorf error, it prints the resolved command's help to stderr and returns
// the underlying formatted error to the caller. Return a normal error if you do not want help
// printed.
func UsageErrorf(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...)}
}

func (e *usageError) Error() string {
	return e.err.Error()
}

func (e *usageError) Unwrap() error {
	return e.err
}
