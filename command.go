package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/pressly/cli/pkg/suggest"
)

// Command describes a single command in the CLI.
//
// Pass a Command to [ParseAndRun] (or [Parse] and [Run]) to drive a program, or list one inside
// another command's [Command.SubCommands] to add a subcommand. Most commands set Name, a one-line
// Summary, Flags, and Exec; Description and SubCommands are added as the program grows.
type Command struct {
	// Name is the word users type to select this command. It must start with a letter and may
	// contain letters, digits, dashes, or underscores. For the root command it also identifies the
	// program in generated help.
	Name string

	// Usage replaces the auto-generated usage line shown at the top of help. Set it to convey the
	// expected positional arguments; the generated form covers only the command path plus a
	// trailing [flags] when the command has flags.
	//
	// Example: "todo list <view> [flags]"
	Usage string

	// Summary is the one-line description shown next to this command in a parent's command listing.
	// It is also shown at the top of this command's own help when Description is empty.
	//
	// Most commands only need Summary; reach for Description when one line is not enough.
	Summary string

	// Description is the longer help text shown at the top of this command's own help. Use it for
	// paragraphs that explain behavior, defaults, or important context.
	//
	// When Summary is empty, the first line of Description is used in command listings.
	Description string

	// Help replaces the built-in help text for this command. Leave it nil to use the generated help.
	//
	// The function receives the resolved command and returns the full help string printed for
	// --help and for [UsageErrorf] errors. Each command in a tree may set its own Help; only the
	// selected command's Help is invoked.
	Help func(*Command) string

	// Flags is the standard library [flag.FlagSet] that defines this command's flags. Construct it
	// with [flag.NewFlagSet], or use [FlagsFunc] for a compact inline form.
	//
	// Flags defined here are inherited by SubCommands unless marked Local in FlagConfigs. Read
	// parsed values inside Exec with [GetFlag].
	Flags *flag.FlagSet

	// FlagConfigs layers cli-specific behavior on top of flags already defined in Flags: short
	// aliases ([FlagConfig.Short]), required flags ([FlagConfig.Required]), and opting out of
	// inheritance ([FlagConfig.Local]).
	//
	// Each entry must reference a flag registered in Flags by Name; otherwise [Parse] returns an
	// error.
	FlagConfigs []FlagConfig

	// SubCommands are commands selected after this command's Name.
	//
	// When a command has SubCommands, the first non-flag argument must match one of them; an
	// unknown name produces an "unknown command" error with suggestions. Commands without
	// SubCommands receive any non-flag arguments as positionals in [State.Args].
	SubCommands []*Command

	// Exec is the function invoked once parsing selects this command. It receives the parsed
	// [State], which carries positional arguments, I/O streams, and access to flag values via
	// [GetFlag].
	//
	// Return [UsageErrorf] for bad arguments or flag combinations so [Run] prints command help to
	// stderr; return a normal error for operational failures, which [Run] returns without printing
	// help.
	Exec func(ctx context.Context, s *State) error

	state *State
}

// Path returns the chain of resolved commands from the root down to this command, inclusive. It is
// available after [Parse] succeeds and is most often called from inside Exec as s.Cmd.Path() to
// build command-aware error messages or breadcrumbs.
//
// Path returns nil if called before parsing.
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

// FlagConfig attaches cli-specific behavior to a single flag already defined in a [Command.Flags]
// FlagSet. It is the entry type used in [Command.FlagConfigs].
type FlagConfig struct {
	// Name is the long flag name as registered in the command's FlagSet.
	Name string

	// Short is a one-letter alias for the flag, such as "v" so users can type -v in place of
	// --verbose. Both forms are listed in help output.
	Short string

	// Required, when true, makes [Parse] fail unless the user provides the flag explicitly. The
	// flag's default value alone is not enough.
	Required bool

	// Local, when true, keeps the flag on this command and prevents it from being inherited by
	// subcommands. By default, parent flags are inherited.
	Local bool
}

// FlagsFunc constructs a [flag.FlagSet] inline for a [Command.Flags] field, sparing callers from
// declaring and assigning the FlagSet separately. The returned FlagSet uses [flag.ContinueOnError]
// so parsing errors are returned rather than fatal.
//
//	Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
//	    f.Bool("verbose", false, "enable verbose output")
//	    f.String("output", "", "output file")
//	    f.Int("count", 0, "number of items")
//	}),
func FlagsFunc(fn func(f *flag.FlagSet)) *flag.FlagSet {
	fset := flag.NewFlagSet("", flag.ContinueOnError)
	fn(fset)
	return fset
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
