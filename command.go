package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/pressly/cli/pkg/suggest"
)

// Command represents a CLI command or subcommand within the application's command hierarchy.
type Command struct {
	// Name is always a single word representing the command's name. It is used to identify the
	// command in the command hierarchy and in help text.
	Name string

	// Usage provides the command's full usage pattern.
	//
	// Example: "cli todo list [flags]"
	Usage string

	// ShortHelp is a brief description of the command's purpose. It is displayed in the help text
	// when the command is shown.
	ShortHelp string

	// UsageFunc is an optional function that can be used to generate a custom usage string for the
	// command. It receives the current command and should return a string with the full usage
	// pattern.
	UsageFunc func(*Command) string

	// Flags holds the command-specific flag definitions. Each command maintains its own flag set
	// for parsing arguments.
	Flags *flag.FlagSet
	// FlagsMetadata is an optional list of flag information to extend the FlagSet with additional
	// metadata. This is useful for tracking required flags.
	FlagsMetadata []FlagMetadata

	// SubCommands is a list of nested commands that exist under this command.
	SubCommands []*Command

	// Exec defines the command's execution logic. It receives the current application [State] and
	// returns an error if execution fails. This function is called when [Run] is invoked on the
	// command.
	Exec func(ctx context.Context, s *State) error

	state *State
}

// Path returns the command chain from root to current command. It can only be called after the root
// command has been parsed and the command hierarchy has been established.
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

// FlagMetadata holds additional metadata for a flag, such as whether it is required.
type FlagMetadata struct {
	// Name is the flag's name. Must match the flag name in the flag set.
	Name string

	// Required indicates whether the flag is required.
	Required bool
}

// FlagsFunc is a helper function that creates a new [flag.FlagSet] and applies the given function
// to it. Intended for use in command definitions to simplify flag setup. Example usage:
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
