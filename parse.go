package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/pressly/cli/xflag"
)

// Parse traverses the command hierarchy and parses arguments. It returns an error if parsing fails
// at any point.
//
// This function is the main entry point for parsing command-line arguments and should be called
// with the root command and the arguments to parse, typically os.Args[1:]. Once parsing is
// complete, the root command is ready to be executed with the [Run] function.
func Parse(root *Command, args []string) error {
	if root == nil {
		return fmt.Errorf("failed to parse: root command is nil")
	}
	if err := validateCommands(root, nil); err != nil {
		return fmt.Errorf("failed to parse: %w", err)
	}

	// Initialize or update root state
	if root.state == nil {
		root.state = &State{
			path: []*Command{root},
		}
	} else {
		// Reset command path but preserve other state
		root.state.path = []*Command{root}
	}
	// First split args at the -- delimiter if present
	var argsToParse []string
	var remainingArgs []string
	for i, arg := range args {
		if arg == "--" {
			argsToParse = args[:i]
			remainingArgs = args[i+1:]
			break
		}
	}
	if argsToParse == nil {
		argsToParse = args
	}

	current := root
	if current.Flags == nil {
		current.Flags = flag.NewFlagSet(root.Name, flag.ContinueOnError)
	}
	var commandChain []*Command
	commandChain = append(commandChain, root)

	// Create combined flags with all parent flags
	combinedFlags := flag.NewFlagSet(root.Name, flag.ContinueOnError)
	combinedFlags.SetOutput(io.Discard)

	// First pass: process commands and build the flag set
	i := 0
	for i < len(argsToParse) {
		arg := argsToParse[i]

		// Skip flags and their values
		if strings.HasPrefix(arg, "-") {
			// For formats like -flag=x or --flag=x
			if strings.Contains(arg, "=") {
				i++
				continue
			}

			// Check if this flag expects a value
			name := strings.TrimLeft(arg, "-")
			if f := current.Flags.Lookup(name); f != nil {
				if _, isBool := f.Value.(interface{ IsBoolFlag() bool }); !isBool {
					// Skip both flag and its value
					i += 2
					continue
				}
			}
			i++
			continue
		}

		// Try to traverse to subcommand
		if len(current.SubCommands) > 0 {
			if sub := current.findSubCommand(arg); sub != nil {
				root.state.path = append(slices.Clone(root.state.path), sub)
				if sub.Flags == nil {
					sub.Flags = flag.NewFlagSet(sub.Name, flag.ContinueOnError)
				}
				current = sub
				commandChain = append(commandChain, sub)
				i++
				continue
			}
			return current.formatUnknownCommandError(arg)
		}
		break
	}
	current.Flags.Usage = func() { /* suppress default usage */ }

	// Add the help check here, after we've found the correct command
	hasHelp := false
	for _, arg := range argsToParse {
		if arg == "-h" || arg == "--h" || arg == "-help" || arg == "--help" {
			hasHelp = true
			break
		}
	}

	// Add flags in reverse order for proper precedence
	for i := len(commandChain) - 1; i >= 0; i-- {
		cmd := commandChain[i]
		if cmd.Flags != nil {
			cmd.Flags.VisitAll(func(f *flag.Flag) {
				if combinedFlags.Lookup(f.Name) == nil {
					combinedFlags.Var(f.Value, f.Name, f.Usage)
				}
			})
		}
	}
	// Make sure to return help only after combining all flags, this way we get the full list of
	// flags in the help message!
	if hasHelp {
		return flag.ErrHelp
	}

	// Let ParseToEnd handle the flag parsing
	if err := xflag.ParseToEnd(combinedFlags, argsToParse); err != nil {
		return fmt.Errorf("command %q: %w", getCommandPath(root.state.path), err)
	}

	// Check required flags
	var missingFlags []string
	for _, cmd := range commandChain {
		if len(cmd.FlagsMetadata) > 0 {
			for _, flagMetadata := range cmd.FlagsMetadata {
				if !flagMetadata.Required {
					continue
				}
				flag := combinedFlags.Lookup(flagMetadata.Name)
				if flag == nil {
					return fmt.Errorf("command %q: internal error: required flag %s not found in flag set", getCommandPath(root.state.path), formatFlagName(flagMetadata.Name))
				}
				if _, isBool := flag.Value.(interface{ IsBoolFlag() bool }); isBool {
					isSet := false
					for _, arg := range argsToParse {
						if strings.HasPrefix(arg, "-"+flagMetadata.Name) || strings.HasPrefix(arg, "--"+flagMetadata.Name) {
							isSet = true
							break
						}
					}
					if !isSet {
						missingFlags = append(missingFlags, formatFlagName(flagMetadata.Name))
					}
				} else if flag.Value.String() == flag.DefValue {
					missingFlags = append(missingFlags, formatFlagName(flagMetadata.Name))
				}
			}
		}
	}
	if len(missingFlags) > 0 {
		msg := "required flag"
		if len(missingFlags) > 1 {
			msg += "s"
		}
		return fmt.Errorf("command %q: %s %q not set", getCommandPath(root.state.path), msg, strings.Join(missingFlags, ", "))
	}

	// Skip past command names in remaining args
	parsed := combinedFlags.Args()
	startIdx := 0
	for _, arg := range parsed {
		isCommand := false
		for _, cmd := range commandChain {
			if arg == cmd.Name {
				startIdx++
				isCommand = true
				break
			}
		}
		if !isCommand {
			break
		}
	}

	// Combine remaining parsed args and everything after delimiter
	var finalArgs []string
	if startIdx < len(parsed) {
		finalArgs = append(finalArgs, parsed[startIdx:]...)
	}
	if len(remainingArgs) > 0 {
		finalArgs = append(finalArgs, remainingArgs...)
	}
	root.state.Args = finalArgs

	if current.Exec == nil {
		return fmt.Errorf("command %q: no exec function defined", getCommandPath(root.state.path))
	}
	return nil
}

var validNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

func validateName(root *Command) error {
	if !validNameRegex.MatchString(root.Name) {
		return fmt.Errorf("name must start with a letter and contain only letters, numbers, dashes (-) or underscores (_)")
	}
	return nil
}

func validateCommands(root *Command, path []string) error {
	if root.Name == "" {
		if len(path) == 0 {
			return errors.New("root command has no name")
		}
		return fmt.Errorf("subcommand in path [%s] has no name", strings.Join(path, ", "))
	}

	currentPath := append(path, root.Name)
	if err := validateName(root); err != nil {
		quoted := make([]string, len(currentPath))
		for i, p := range currentPath {
			quoted[i] = strconv.Quote(p)
		}
		return fmt.Errorf("command [%s]: %w", strings.Join(quoted, ", "), err)
	}

	for _, sub := range root.SubCommands {
		if err := validateCommands(sub, currentPath); err != nil {
			return err
		}
	}
	return nil
}
