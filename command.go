package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/pressly/cli/pkg/suggest"
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
	// [Command.FlagConfigs]. Read flag values inside [Command.Exec] with [GetFlag].
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

func (c *Command) terminal() *Command {
	if c.state == nil || len(c.state.path) == 0 {
		return c
	}
	// Get the last command in the path - this is our terminal command
	return c.state.path[len(c.state.path)-1]
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
