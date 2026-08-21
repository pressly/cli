// Package cli builds command-line programs on top of the standard library [flag] package. It adds
// nested subcommands, flags anywhere, inherited flags, generated help, and type-safe flag access.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/pressly/cli/internal/helpdoc"
	"github.com/pressly/cli/pkg/suggest"
	"github.com/pressly/cli/xflag"
)

// Command describes a command in a CLI.
type Command struct {
	// Name identifies the command. It must start with a letter and contain only letters, digits,
	// dashes, or underscores.
	Name string

	// Usage overrides the generated usage line. Angle brackets usually mark required arguments,
	// square brackets optional arguments, and an ellipsis repeated arguments.
	//
	//	Usage: "echo [flags] <text>..."
	Usage string

	// Summary is the one-line description used in command lists and, when Description is empty, in
	// the command's help.
	Summary string

	// Description is the command's longer help text. Its first line is used in command lists when
	// Summary is empty.
	Description string

	// Help overrides the generated help for this command.
	Help func(*Command) string

	// Flags holds this command's [flag.FlagSet]. Subcommands inherit these flags unless they are
	// marked [FlagConfig.Local].
	Flags *flag.FlagSet

	// FlagConfigs adds behavior to flags already defined in Flags.
	FlagConfigs []FlagConfig

	// SubCommands are the commands available below this command. A command that only groups
	// subcommands may leave Exec nil.
	SubCommands []*Command

	// Exec runs the selected command. Return [UsageErrorf] for invalid arguments or flag
	// combinations so [Run] prints the command's help.
	Exec func(ctx context.Context, s *State) error

	state *State
}

// Path returns the parsed command path from root to this command, or nil before [Parse].
func (c *Command) Path() []*Command {
	if c.state == nil {
		return nil
	}
	return c.state.path
}

// FlagConfig adds behavior to a flag already defined in [Command.Flags].
type FlagConfig struct {
	// Name is the flag's registered name.
	Name string

	// Short is a one-letter alias, such as "v" for --verbose.
	Short string

	// Required makes [Parse] fail unless the user explicitly sets the flag.
	Required bool

	// Local prevents subcommands from inheriting the flag.
	Local bool
}

// FlagName ties a flag name to the type returned by [State.GetFlag].
type FlagName[T any] string

// State contains the parsed inputs passed to [Command.Exec].
type State struct {
	// Args holds positional arguments. Anything after "--" is included as-is.
	Args []string

	// Stdin, Stdout, and Stderr are the command's streams.
	Stdin          io.Reader
	Stdout, Stderr io.Writer

	// Cmd is the selected command.
	Cmd *Command

	path []*Command
}

// RunOptions replaces the standard streams used by [Run] and [ParseAndRun].
type RunOptions struct {
	// Nil fields default to the corresponding os stream.
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

// FlagsFunc builds a [flag.FlagSet] inline using [flag.ContinueOnError].
//
//	Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
//	    f.Bool("verbose", false, "enable verbose output")
//	}),
func FlagsFunc(fn func(f *flag.FlagSet)) (fset *flag.FlagSet) {
	fset = flag.NewFlagSet("", flag.ContinueOnError)
	fset.SetOutput(io.Discard)
	defer func() {
		if r := recover(); r != nil {
			flagSetupErrors.Store(fset, flagSetupPanicError(r))
		}
	}()
	fn(fset)
	return fset
}

// GetFlag returns a flag value as T, searching the selected command before its parents. Unknown
// names and type mismatches are programming errors: GetFlag panics, and [Run] returns the error.
//
//	verbose := s.GetFlag[bool]("verbose")
//	const count FlagName[int] = "count"
//	n := s.GetFlag(count)
func (s *State) GetFlag[T any](name FlagName[T]) T {
	if s == nil {
		panic(&internalError{err: errors.New("state is nil")})
	}
	flagName := string(name)
	for _, cmd := range slices.Backward(s.path) {
		if cmd.Flags == nil {
			continue
		}

		if f := cmd.Flags.Lookup(flagName); f != nil {
			if getter, ok := f.Value.(flag.Getter); ok {
				value := getter.Get()
				if v, ok := value.(T); ok {
					return v
				}
				err := fmt.Errorf("type mismatch for flag %q in command %q: registered %T, requested %T",
					formatFlagName(flagName),
					getCommandPath(s.path),
					value,
					*new(T),
				)
				panic(&internalError{err: err})
			}
		}
	}

	err := fmt.Errorf("flag %q not found in command %q flag set",
		formatFlagName(flagName),
		getCommandPath(s.path),
	)
	panic(&internalError{err: err})
}

// Parse selects a command and parses its flags without running it. It returns [flag.ErrHelp] for -h
// or --help; [ParseAndRun] handles that case automatically.
func Parse(root *Command, args []string) error {
	if root == nil {
		return errors.New("root command is nil")
	}
	if err := validateCommands(root, nil); err != nil {
		return err
	}

	// Initialize or update root state. Clear command pointers across the tree first so stale
	// subcommands from a previous parse do not retain the newly resolved path.
	state := root.state
	clearCommandState(root)
	if state == nil {
		state = &State{}
	}
	root.state = state
	root.state.Args = nil
	root.state.Cmd = nil
	root.state.path = []*Command{root}

	argsToParse, remainingArgs := splitAtDelimiter(args)

	current, err := resolveCommandPath(root, argsToParse)
	if err != nil {
		return err
	}
	root.state.Cmd = current
	current.Flags.Usage = func() { /* suppress default usage */ }

	for _, arg := range argsToParse {
		if arg == "-h" || arg == "--h" || arg == "-help" || arg == "--help" {
			return flag.ErrHelp
		}
	}

	combinedFlags := combineFlags(root.state.path)

	if err := xflag.ParseToEnd(combinedFlags, argsToParse); err != nil {
		return fmt.Errorf("command %q: %w", getCommandPath(root.state.path), err)
	}

	root.state.Args = collectArgs(root.state.path, combinedFlags.Args(), remainingArgs)

	if current.Exec == nil && len(current.SubCommands) > 0 {
		return UsageErrorf("subcommand required")
	}

	if err := checkRequiredFlags(root.state.path, combinedFlags); err != nil {
		return err
	}

	if current.Exec == nil {
		return fmt.Errorf("command %q: no exec function defined", getCommandPath(root.state.path))
	}
	return nil
}

// Run executes the command selected by [Parse]. Usage errors print help to stderr; other errors are
// returned as-is. A nil ctx uses [context.Background].
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
		return errors.New("no terminal command found")
	}

	options = checkAndSetRunOptions(options)
	updateState(root.state, options)

	return run(ctx, cmd, root.state)
}

// ParseAndRun parses args and runs the selected command. It prints help and returns nil for -h or
// --help.
//
//	if err := cli.ParseAndRun(ctx, root, os.Args[1:], nil); err != nil {
//	    fmt.Fprintf(os.Stderr, "error: %v\n", err)
//	    os.Exit(1)
//	}
func ParseAndRun(ctx context.Context, root *Command, args []string, options *RunOptions) error {
	if err := Parse(root, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			options = checkAndSetRunOptions(options)
			_, _ = fmt.Fprintln(options.Stdout, help(root))
			return nil
		}
		if usageErr, ok := errors.AsType[*usageError](err); ok {
			options = checkAndSetRunOptions(options)
			_, _ = fmt.Fprintf(options.Stderr, "%s\n\n", help(root))
			return usageErr.Unwrap()
		}
		return err
	}
	return Run(ctx, root, options)
}

// UsageErrorf returns an error for invalid command arguments or flag combinations. [Run] prints the
// command's help before returning the underlying error.
//
//	if len(s.Args) == 0 {
//	    return cli.UsageErrorf("must supply a name")
//	}
func UsageErrorf(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...)}
}

type usageError struct {
	err error
}

func (e *usageError) Error() string {
	return e.err.Error()
}

func (e *usageError) Unwrap() error {
	return e.err
}

func run(ctx context.Context, cmd *Command, state *State) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			switch err := r.(type) {
			case error:
				if _, ok := errors.AsType[*internalError](err); ok {
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
	if usageErr, ok := errors.AsType[*usageError](err); ok {
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

func help(root *Command) string {
	if root == nil {
		return ""
	}

	terminalCmd := root.terminal()
	if terminalCmd.Help != nil {
		return strings.TrimRight(terminalCmd.Help(terminalCmd), "\n")
	}

	return defaultHelp(root)
}

func defaultHelp(root *Command) string {
	return helpdoc.New(helpPath(root)).String()
}

func helpPath(root *Command) []helpdoc.Command {
	path := root.Path()
	if len(path) == 0 {
		path = []*Command{root.terminal()}
	}

	out := make([]helpdoc.Command, 0, len(path))
	for _, cmd := range path {
		out = append(out, helpCommand(cmd))
	}
	return out
}

func helpCommand(cmd *Command) helpdoc.Command {
	return helpdoc.Command{
		Name:        cmd.Name,
		Usage:       cmd.Usage,
		Summary:     cmd.Summary,
		Description: cmd.Description,
		Flags:       cmd.Flags,
		FlagConfigs: helpFlagConfigs(cmd.FlagConfigs),
		Subcommands: helpSubcommands(cmd.SubCommands),
	}
}

func helpSubcommands(commands []*Command) []helpdoc.Command {
	out := make([]helpdoc.Command, 0, len(commands))
	for _, cmd := range commands {
		out = append(out, helpdoc.Command{
			Name:        cmd.Name,
			Summary:     cmd.Summary,
			Description: cmd.Description,
		})
	}
	return out
}

func helpFlagConfigs(configs []FlagConfig) []helpdoc.FlagConfig {
	out := make([]helpdoc.FlagConfig, 0, len(configs))
	for _, cfg := range configs {
		out = append(out, helpdoc.FlagConfig{
			Name:     cfg.Name,
			Short:    cfg.Short,
			Required: cfg.Required,
			Local:    cfg.Local,
		})
	}
	return out
}

// internalError marks programmer errors that Run returns without adding a panic location.
type internalError struct {
	err error
}

func (e *internalError) Error() string {
	return e.err.Error()
}

func (e *internalError) Unwrap() error {
	return e.err
}

var flagSetupErrors sync.Map

func flagSetupError(fset *flag.FlagSet) error {
	if fset == nil {
		return nil
	}
	err, ok := flagSetupErrors.Load(fset)
	if !ok {
		return nil
	}
	return err.(error)
}

func flagSetupPanicError(r any) error {
	msg := fmt.Sprint(r)
	if name, ok := strings.CutPrefix(msg, "flag redefined: "); ok {
		return fmt.Errorf("flag %s is defined more than once", formatFlagName(name))
	}
	return fmt.Errorf("flag setup failed: %s", msg)
}

func (c *Command) terminal() *Command {
	if c.state == nil || len(c.state.path) == 0 {
		return c
	}
	return c.state.path[len(c.state.path)-1]
}

func (c *Command) findSubCommand(name string) *Command {
	for _, sub := range c.SubCommands {
		if strings.EqualFold(sub.Name, name) {
			return sub
		}
	}
	return nil
}

func (c *Command) formatUnknownCommandError(unknownCmd string) error {
	var known []string
	for _, sub := range c.SubCommands {
		known = append(known, sub.Name)
	}
	suggestions := suggest.FindSimilar(unknownCmd, known, 3)
	if len(suggestions) > 0 {
		return fmt.Errorf("unknown command %q. Did you mean one of these?\n\t%s",
			unknownCmd,
			strings.Join(suggestions, "\n\t"))
	}
	return fmt.Errorf("unknown command %q", unknownCmd)
}

func formatFlagName(name string) string {
	return "-" + name
}

func getCommandPath(commands []*Command) string {
	var commandPath []string
	for _, c := range commands {
		commandPath = append(commandPath, c.Name)
	}
	return strings.Join(commandPath, " ")
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
	// Skip location and runtime.Callers.
	n := runtime.Callers(skip+2, pcs[:])
	if n == 0 {
		return "unknown:0"
	}

	frame, _ := runtime.CallersFrames(pcs[:n]).Next()

	mod := getGoModuleName()
	fn := strings.TrimPrefix(frame.Function, mod+"/")
	file := filepath.Base(frame.File)
	if mod != "" {
		if _, relative, ok := strings.Cut(frame.File, mod+"/"); ok {
			file = relative
		}
	}

	return fn + " " + file + ":" + strconv.Itoa(frame.Line)
}
