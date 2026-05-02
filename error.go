package cli

import "fmt"

type usageError struct {
	err error
}

// UsageErrorf returns an error for invalid command-line usage.
//
// Return UsageErrorf from Exec when the command was selected successfully but the remaining args or
// flag combination are invalid. [Run] prints command help to stderr, then returns the formatted
// error without the usage wrapper. Return a normal error when you do not want help printed.
func UsageErrorf(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...)}
}

func (e *usageError) Error() string {
	return e.err.Error()
}

func (e *usageError) Unwrap() error {
	return e.err
}
