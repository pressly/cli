package cli

import "fmt"

type usageError struct {
	err error
}

// UsageErrorf returns an error that means the command was used incorrectly. Return it from
// [Command.Exec] when the command itself was right but the arguments or flag combination are wrong:
//
//	if len(s.Args) == 0 {
//	    return cli.UsageErrorf("must supply a name")
//	}
//
// When [Run] sees a UsageErrorf error, it prints the command's help to stderr and returns the error
// message you passed in. Return a normal error if you do not want help printed.
func UsageErrorf(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...)}
}

func (e *usageError) Error() string {
	return e.err.Error()
}

func (e *usageError) Unwrap() error {
	return e.err
}
