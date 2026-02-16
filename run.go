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

// RunOptions specifies options for running a command.
type RunOptions struct {
	// Stdin, Stdout, and Stderr are the standard input, output, and error streams for the command.
	// If any of these are nil, the command will use the default streams ([os.Stdin], [os.Stdout],
	// and [os.Stderr], respectively).
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

// Run executes the current command. It returns an error if the command has not been parsed or if
// the command has no execution function.
//
// The options parameter may be nil, in which case default values are used. See [RunOptions] for
// more details.
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

// ParseAndRun is a convenience function that combines [Parse] and [Run] into a single call. It
// parses the command hierarchy, handles help flags automatically (printing usage to stdout and
// returning nil), and then executes the resolved command.
//
// This is the recommended entry point for most CLI applications:
//
//	if err := cli.ParseAndRun(ctx, root, os.Args[1:], nil); err != nil {
//	    fmt.Fprintf(os.Stderr, "error: %v\n", err)
//	    os.Exit(1)
//	}
//
// For applications that need to perform work between parsing and execution (e.g., initializing
// resources based on parsed flags), use [Parse] and [Run] separately.
func ParseAndRun(ctx context.Context, root *Command, args []string, options *RunOptions) error {
	if err := Parse(root, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			options = checkAndSetRunOptions(options)
			fmt.Fprintln(options.Stdout, DefaultUsage(root))
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
	return cmd.Exec(ctx, state)
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
