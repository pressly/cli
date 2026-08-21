// Package cli builds command-line programs on top of the standard library [flag] package. It adds
// nested subcommands and lets users place flags anywhere in command arguments.
//
// Features:
//   - Nested subcommands via [Command.SubCommands]
//   - Flags placed anywhere on the command line
//   - Parent flags inherited by child commands
//   - Type-safe flag access via [State.GetFlag]
//   - Generated help, replaceable per command via [Command.Help]
//   - "Did you mean" suggestions for misspelled subcommands
//
// Quick example:
//
//	root := &cli.Command{
//	    Name:        "echo",
//	    Usage:       "echo [flags] <text>...",
//	    Summary:     "Print text",
//	    Description: "echo prints the provided text.",
//	    Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
//	        f.Bool("c", false, "capitalize the input")
//	    }),
//	    Exec: func(ctx context.Context, s *cli.State) error {
//	        output := strings.Join(s.Args, " ")
//	        if s.GetFlag[bool]("c") {
//	            output = strings.ToUpper(output)
//	        }
//	        fmt.Fprintln(s.Stdout, output)
//	        return nil
//	    },
//	}
//	if err := cli.ParseAndRun(ctx, root, os.Args[1:], nil); err != nil {
//	    fmt.Fprintf(os.Stderr, "error: %v\n", err)
//	    os.Exit(1)
//	}
//
// The API is small on purpose. cli uses the standard library flag package instead of replacing it,
// so most of what you write is your program.
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

	"github.com/pressly/cli/internal/helpdoc"
	"github.com/pressly/cli/pkg/suggest"
	"github.com/pressly/cli/xflag"
)

// Command describes a single command in the CLI.
//
// Pass a Command to [ParseAndRun] (or [Parse] and [Run]) to run a program. To add a subcommand,
// list it in another command's [Command.SubCommands].
type Command struct {
	// Name is the word users type to pick this command. It must start with a letter and can contain
	// letters, digits, dashes, or underscores. For the root command it is also the program name
	// shown in help.
	Name string

	// Usage replaces the usage line shown at the top of help. Set it to show the expected
	// arguments. The default usage line shows only the command path, plus "[flags]" when the
	// command has flags.
	//
	// A common convention is to write required values as "<name>", optional values as "[name]", and
	// repeated values with "...".
	//
	//	Example: "todo list <view> [flags]"
	//	Example: "todo add <text> [flags]"
	//	Example: "todo remove <id>"
	//	Example: "echo [flags] <text>..."
	//	Example: "serve [flags] [addr]"
	Usage string

	// Summary is the one-line description shown next to this command in its parent's command list.
	// It is also shown at the top of this command's own help when [Command.Description] is empty.
	//
	// Most commands only need Summary. Use [Command.Description] when one line is not enough.
	Summary string

	// Description is the longer help text shown at the top of this command's own help. Use it to
	// explain behavior, defaults, or anything else worth knowing.
	//
	// When [Command.Summary] is empty, the first line of Description is used in command lists
	// instead.
	Description string

	// Help replaces the built-in help text for this command. Leave it nil to use the default help.
	//
	// The function is given the command and returns the full help string. Help is used for --help
	// and for [UsageErrorf] errors. Each command can set its own Help, and only the selected
	// command's Help is called.
	Help func(*Command) string

	// Flags holds this command's flags as a standard library [flag.FlagSet]. Build it with
	// [flag.NewFlagSet], or use [FlagsFunc] to define flags inline.
	//
	// Subcommands inherit these flags unless they are marked [FlagConfig.Local] in
	// [Command.FlagConfigs]. Read flag values inside [Command.Exec] with [State.GetFlag].
	Flags *flag.FlagSet

	// FlagConfigs adds extra behavior to flags already defined in [Command.Flags]. See [FlagConfig]
	// for the available options.
	//
	// Each entry must point to a flag defined in [Command.Flags]. Otherwise [Parse] returns an
	// error.
	FlagConfigs []FlagConfig

	// SubCommands are the commands users can pick after this command's name.
	//
	// When a command has SubCommands, the first non-flag argument must match one of them. An
	// unknown name returns an "unknown command" error with suggestions. Commands without
	// SubCommands pass any non-flag arguments through to [State.Args]. Leave [Command.Exec] nil on
	// a command that only groups subcommands; selecting it without a child returns a usage error.
	SubCommands []*Command

	// Exec is the function that runs when this command is picked. It is given a [State] holding the
	// parsed inputs the command needs.
	//
	// Return [UsageErrorf] for bad arguments or flag combinations so [Run] prints the command's
	// help to stderr. Return a normal error for everything else; [Run] returns it without printing
	// help.
	Exec func(ctx context.Context, s *State) error

	state *State
}

// Path returns the list of commands from the root down to this command. It is usually called inside
// [Command.Exec] as s.Cmd.Path() to build error messages that include the full command path.
//
// Path returns nil if called before [Parse].
func (c *Command) Path() []*Command {
	if c.state == nil {
		return nil
	}
	return c.state.path
}

// FlagConfig adds extra behavior to a single flag already defined in [Command.Flags]. It is used as
// an entry in [Command.FlagConfigs].
type FlagConfig struct {
	// Name is the long flag name as registered in the command's [flag.FlagSet].
	Name string

	// Short is a one-letter alias for the flag, such as "v" so users can type -v instead of
	// --verbose. Both forms are shown in help.
	Short string

	// Required, when true, makes [Parse] fail unless the user sets the flag. The default value is
	// not enough; the user must pass it.
	Required bool

	// Local, when true, keeps the flag on this command only and stops it from being inherited by
	// subcommands. Parent flags are inherited by default.
	Local bool
}

// FlagName associates a flag's canonical name with its Go type. Pass a FlagName to [State.GetFlag]
// to infer the returned type instead of specifying it at each lookup.
//
//	const verbose FlagName[bool] = "verbose"
//
// Define the flag with its string name as usual:
//
//	f.Bool(string(verbose), false, "enable verbose output")
type FlagName[T any] string

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

// FlagsFunc creates a [flag.FlagSet] inline so you don't have to make one and assign it separately.
// The returned FlagSet uses [flag.ContinueOnError], so parsing errors are returned instead of being
// fatal.
//
//	Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
//	    f.Bool("verbose", false, "enable verbose output")
//	    f.String("output", "", "output file")
//	    f.Int("count", 0, "number of items")
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

// GetFlag returns the value of a flag as type T. Call it from inside [Command.Exec] with the same
// Go type that was used when the flag was defined.
//
// GetFlag looks for the flag on the picked command first, then in its parent commands. A flag
// defined on the root command can be read from any subcommand. An unknown flag name or a wrong type
// is a programming error: GetFlag panics, and [Run] catches the panic and returns the error.
//
//	verbose := s.GetFlag[bool]("verbose")
//	count   := s.GetFlag[int]("count")
//	path    := s.GetFlag[string]("path")
//
// Use [FlagName] to define a reusable name and infer the returned type:
//
//	const verbose FlagName[bool] = "verbose"
//	enabled := s.GetFlag(verbose)
func (s *State) GetFlag[T any](name FlagName[T]) T {
	if s == nil {
		panic(&internalError{err: errors.New("state is nil")})
	}
	flagName := string(name)
	// Try to find the flag in each command's flag set, starting from the current command
	for i := len(s.path) - 1; i >= 0; i-- {
		cmd := s.path[i]
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
				// Flag exists but type doesn't match - this is an internal error
				panic(&internalError{err: err})
			}
		}
	}

	// If flag not found anywhere in hierarchy, panic with helpful message
	err := fmt.Errorf("flag %q not found in command %q flag set",
		formatFlagName(flagName),
		getCommandPath(s.path),
	)
	panic(&internalError{err: err})
}

// Parse picks the right command and parses its flags from args, but does not run [Command.Exec].
// Use Parse with [Run] when you need to do work between parsing and running. For the common case,
// call [ParseAndRun].
//
// Parse returns [flag.ErrHelp] when the user passes -h or --help. You have to print the help
// yourself when this happens. [ParseAndRun] does it for you.
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

	// Check for help flags after resolving the correct command
	for _, arg := range argsToParse {
		if arg == "-h" || arg == "--h" || arg == "-help" || arg == "--help" {
			return flag.ErrHelp
		}
	}

	combinedFlags := combineFlags(root.state.path)

	// Let ParseToEnd handle the flag parsing
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
		var usageErr *usageError
		if errors.As(err, &usageErr) {
			options = checkAndSetRunOptions(options)
			_, _ = fmt.Fprintf(options.Stderr, "%s\n\n", help(root))
			return usageErr.Unwrap()
		}
		return err
	}
	return Run(ctx, root, options)
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
	// Get the last command in the path - this is our terminal command
	return c.state.path[len(c.state.path)-1]
}

// findSubCommand searches for a subcommand by name and returns it if found. Returns nil if no
// subcommand with the given name exists.
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
