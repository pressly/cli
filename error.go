package cli

import "fmt"

// UsageError marks an invalid command invocation.
//
// You normally create one with [UsageErrorf] rather than constructing this type directly.
type UsageError struct {
	err error
}

// UsageErrorf returns an error for invalid command-line usage.
//
// Return UsageErrorf from Exec when the command was selected successfully but the remaining args or
// flag combination are invalid. [Run] prints Help(s.Cmd) to stderr, then returns the formatted error
// without the UsageError wrapper.
func UsageErrorf(format string, args ...any) error {
	return &UsageError{err: fmt.Errorf(format, args...)}
}

// Error returns the formatted usage error message.
func (e *UsageError) Error() string {
	return e.err.Error()
}

// Unwrap exposes the formatted error for errors.Is and errors.As.
func (e *UsageError) Unwrap() error {
	return e.err
}
