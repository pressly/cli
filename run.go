package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
)

// RunOptions replaces the standard streams used by [Run] and [ParseAndRun]. Pass nil for normal
// programs to use os.Stdin, os.Stdout, and os.Stderr.
//
// Use RunOptions in tests, or anywhere you need to capture output or supply your own input.
type RunOptions struct {
	// Stdin, Stdout, and Stderr replace os.Stdin, os.Stdout, and os.Stderr when set. A nil field
	// falls back to its os equivalent.
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

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

// Run runs the command picked by a previous call to [Parse]. Use Run only when you call [Parse]
// separately. For the common case, use [ParseAndRun].
//
// If [Command.Exec] returns an error created by [UsageErrorf], Run prints the command's help to
// stderr and returns the error you passed to [UsageErrorf]. Other errors are returned as-is. A nil
// ctx defaults to [context.Background].
func Run(ctx context.Context, root *Command, options *RunOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if root == nil {
		return errors.New("root command is nil")
	}
	if root.state == nil || len(root.state.path) == 0 {
		return errors.New("command not parsed")
	}
	cmd := root.terminal()
	if cmd == nil {
		// This should never happen, but if it does, it's likely a bug in the Parse function.
		return errors.New("no terminal command found")
	}

	options = checkAndSetRunOptions(options)
	updateState(root.state, options)

	return run(ctx, cmd, root.state)
}

// ParseAndRun parses args, picks the right command, and runs its [Command.Exec]. This is the normal
// way to start a CLI program:
//
//	if err := cli.ParseAndRun(ctx, root, os.Args[1:], nil); err != nil {
//	    fmt.Fprintf(os.Stderr, "error: %v\n", err)
//	    os.Exit(1)
//	}
//
// When the user passes -h or --help, ParseAndRun prints the picked command's help to stdout and
// returns nil. Use [Parse] and [Run] separately when you need to do work between parsing and
// running, such as setting up resources based on parsed flags.
func ParseAndRun(ctx context.Context, root *Command, args []string, options *RunOptions) error {
	if err := Parse(root, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			options = checkAndSetRunOptions(options)
			_, _ = fmt.Fprintln(options.Stdout, help(root))
			return nil
		}
		return err
	}
	return Run(ctx, root, options)
}

func run(ctx context.Context, cmd *Command, state *State) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			switch err := r.(type) {
			case error:
				// If error is from cli package (e.g., flag type mismatch), don't add location info
				var intErr *internalError
				if errors.As(err, &intErr) {
					retErr = err
				} else {
					retErr = fmt.Errorf("panic: %v\n\n%s", err, location(4))
				}
			default:
				retErr = fmt.Errorf("panic: %v", r)
			}
		}
	}()
	err := cmd.Exec(ctx, state)
	var usageErr *usageError
	if errors.As(err, &usageErr) {
		_, _ = fmt.Fprintf(state.Stderr, "%s\n\n", help(state.Cmd))
		return usageErr.Unwrap()
	}
	return err
}

func updateState(s *State, opt *RunOptions) {
	if s.Stdin == nil {
		s.Stdin = opt.Stdin
	}
	if s.Stdout == nil {
		s.Stdout = opt.Stdout
	}
	if s.Stderr == nil {
		s.Stderr = opt.Stderr
	}
}

func checkAndSetRunOptions(opt *RunOptions) *RunOptions {
	if opt == nil {
		opt = &RunOptions{}
	}
	if opt.Stdin == nil {
		opt.Stdin = os.Stdin
	}
	if opt.Stdout == nil {
		opt.Stdout = os.Stdout
	}
	if opt.Stderr == nil {
		opt.Stderr = os.Stderr
	}
	return opt
}

var (
	once         sync.Once
	goModuleName string
)

func getGoModuleName() string {
	once.Do(func() {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Path != "" {
			goModuleName = info.Main.Path
		}
	})
	return goModuleName
}

func location(skip int) string {
	var pcs [1]uintptr
	// Need to add 2 to skip to account for this function and runtime.Callers
	n := runtime.Callers(skip+2, pcs[:])
	if n == 0 {
		return "unknown:0"
	}

	frame, _ := runtime.CallersFrames(pcs[:n]).Next()

	// Trim the module name from function and file paths for cleaner output. Function names use the
	// module path directly (e.g., "github.com/pressly/cli.Run").
	fn := strings.TrimPrefix(frame.Function, getGoModuleName()+"/")
	// File paths from runtime are absolute (e.g., "/Users/.../cli/run.go"). We want a relative path
	// for cleaner output. Try to find the module's import path in the filesystem path (works with
	// GOPATH-style layouts), otherwise fall back to just the base filename.
	file := frame.File
	mod := getGoModuleName()
	if mod != "" {
		if idx := strings.Index(file, mod+"/"); idx != -1 {
			file = file[idx+len(mod)+1:]
		} else {
			file = file[strings.LastIndex(file, "/")+1:]
		}
	} else {
		file = file[strings.LastIndex(file, "/")+1:]
	}

	return fn + " " + file + ":" + strconv.Itoa(frame.Line)
}
