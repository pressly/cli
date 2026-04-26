package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/pressly/cli/pkg/suggest"
	"github.com/pressly/cli/usage"
)

// ErrHelp is returned by [Parse] when a help flag is present.
//
// [ParseAndRun] handles ErrHelp automatically by printing [Help] to stdout and returning nil.
// Callers that use [Parse] and [Run] separately can check errors.Is(err, ErrHelp) and render help
// themselves.
var ErrHelp = flag.ErrHelp

// Command defines one command in a CLI.
//
// A command can be the root command passed to [ParseAndRun], or a subcommand listed in
// [Command.SubCommands]. Most programs define Name, optional help fields, optional flags, and Exec.
type Command struct {
	// Name is the single word users type to select the command.
	Name string

	// Usage overrides the generated usage line when the command needs a custom synopsis.
	//
	// Example: "cli todo list [flags]"
	Usage string

	// ShortHelp describes the command in help output and parent command listings.
	ShortHelp string

	// Help customizes the command's help document.
	//
	// Leave Help nil for the built-in help. Set it when you want to append examples, reorder
	// sections, or replace the document entirely. The function receives the command being shown and
	// the built-in document.
	Help func(*Command, usage.Help) usage.Help

	// Flags defines the command's flags using the standard library flag package.
	Flags *flag.FlagSet

	// FlagConfigs adds cli-specific behavior to flags already defined in Flags.
	//
	// Use it for required flags, short aliases, and flags that should not be inherited by
	// subcommands.
	FlagConfigs []FlagConfig

	// SubCommands lists commands users can select after this command's name.
	SubCommands []*Command

	// Exec runs after parsing selects this command.
	//
	// Return [UsageErrorf] for invalid args or flag combinations so Run can print help. Return a
	// normal error for operational failures.
	Exec func(ctx context.Context, s *State) error

	state *State
}

// Path returns the parsed command chain from root to this command.
//
// Call Path after [Parse] when command logic needs to inspect where the selected command sits in
// the command tree.
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

// FlagConfig adds cli-specific behavior to a flag defined in a command's FlagSet.
type FlagConfig struct {
	// Name is the flag's long name as registered in the command's FlagSet.
	Name string

	// Short lets users type a one-letter alias, such as -v for --verbose.
	Short string

	// Required requires users to provide the flag explicitly.
	Required bool

	// Local keeps the flag on this command instead of inheriting it into subcommands.
	Local bool
}

// FlagsFunc creates a FlagSet inline for a command definition.
//
//	cmd.Flags = cli.FlagsFunc(func(f *flag.FlagSet) {
//	    f.Bool("verbose", false, "enable verbose output")
//	    f.String("output", "", "output file")
//	    f.Int("count", 0, "number of items")
//	})
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
