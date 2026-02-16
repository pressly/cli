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

	argsToParse, remainingArgs := splitAtDelimiter(args)

	current, err := resolveCommandPath(root, argsToParse)
	if err != nil {
		return err
	}
	current.Flags.Usage = func() { /* suppress default usage */ }

	// Check for help flags after resolving the correct command
	for _, arg := range argsToParse {
		if arg == "-h" || arg == "--h" || arg == "-help" || arg == "--help" {
			// Combine flags first so the help message includes all inherited flags
			combineFlags(root.state.path)
			return ErrHelp
		}
	}

	combinedFlags := combineFlags(root.state.path)

	// Let ParseToEnd handle the flag parsing
	if err := xflag.ParseToEnd(combinedFlags, argsToParse); err != nil {
		return fmt.Errorf("command %q: %w", getCommandPath(root.state.path), err)
	}

	if err := checkRequiredFlags(root.state.path, combinedFlags); err != nil {
		return err
	}

	root.state.Args = collectArgs(root.state.path, combinedFlags.Args(), remainingArgs)

	if current.Exec == nil {
		return fmt.Errorf("command %q: no exec function defined", getCommandPath(root.state.path))
	}
	return nil
}

// splitAtDelimiter splits args at the first "--" delimiter. Returns the args before the delimiter
// and any args after it.
func splitAtDelimiter(args []string) (argsToParse, remaining []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

// resolveCommandPath walks argsToParse to resolve the subcommand chain, building root.state.path
// and initializing flag sets along the way. Returns the terminal (deepest) command.
func resolveCommandPath(root *Command, argsToParse []string) (*Command, error) {
	current := root
	if current.Flags == nil {
		current.Flags = flag.NewFlagSet(root.Name, flag.ContinueOnError)
	}

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

			// Check if this flag expects a value across all commands in the chain (not just the
			// current command), since flags from ancestor commands are inherited and can appear
			// anywhere. Also check short flag aliases from FlagsMetadata.
			name := strings.TrimLeft(arg, "-")
			skipValue := false
			for _, cmd := range root.state.path {
				// First try direct lookup.
				f := cmd.Flags.Lookup(name)
				// If not found, check if it's a short alias.
				if f == nil {
					for _, fm := range cmd.FlagsMetadata {
						if fm.Short == name {
							f = cmd.Flags.Lookup(fm.Name)
							break
						}
					}
				}
				if f != nil {
					if _, isBool := f.Value.(interface{ IsBoolFlag() bool }); !isBool {
						skipValue = true
					}
					break
				}
			}
			if skipValue {
				// Skip both flag and its value
				i += 2
				continue
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
				i++
				continue
			}
			return nil, current.formatUnknownCommandError(arg)
		}
		break
	}
	return current, nil
}

// combineFlags merges flags from the command path into a single FlagSet. Flags are added in reverse
// order (deepest command first) so that child flags take precedence over parent flags. Short flag
// aliases from FlagsMetadata are also registered, sharing the same Value as their long counterpart.
func combineFlags(path []*Command) *flag.FlagSet {
	combined := flag.NewFlagSet(path[0].Name, flag.ContinueOnError)
	combined.SetOutput(io.Discard)
	for i := len(path) - 1; i >= 0; i-- {
		cmd := path[i]
		if cmd.Flags == nil {
			continue
		}
		shortMap := shortFlagMap(cmd.FlagsMetadata)
		cmd.Flags.VisitAll(func(f *flag.Flag) {
			if combined.Lookup(f.Name) == nil {
				combined.Var(f.Value, f.Name, f.Usage)
			}
			// Register the short alias pointing to the same Value.
			if short, ok := shortMap[f.Name]; ok {
				if combined.Lookup(short) == nil {
					combined.Var(f.Value, short, f.Usage)
				}
			}
		})
	}
	return combined
}

// shortFlagMap builds a map from long flag name to short alias from FlagsMetadata.
func shortFlagMap(metadata []FlagMetadata) map[string]string {
	m := make(map[string]string, len(metadata))
	for _, fm := range metadata {
		if fm.Short != "" {
			m[fm.Name] = fm.Short
		}
	}
	return m
}

// checkRequiredFlags verifies that all flags marked as required in FlagsMetadata were explicitly
// set during parsing.
func checkRequiredFlags(path []*Command, combined *flag.FlagSet) error {
	// Build a set of flags that were explicitly set during parsing. Visit (unlike VisitAll) only
	// iterates over flags that were actually provided by the user, regardless of their value.
	setFlags := make(map[string]struct{})
	combined.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = struct{}{}
	})

	var missingFlags []string
	for _, cmd := range path {
		for _, flagMetadata := range cmd.FlagsMetadata {
			if !flagMetadata.Required {
				continue
			}
			if combined.Lookup(flagMetadata.Name) == nil {
				return fmt.Errorf("command %q: internal error: required flag %s not found in flag set", getCommandPath(path), formatFlagName(flagMetadata.Name))
			}
			if _, ok := setFlags[flagMetadata.Name]; !ok {
				missingFlags = append(missingFlags, formatFlagName(flagMetadata.Name))
			}
		}
	}
	if len(missingFlags) > 0 {
		msg := "required flag"
		if len(missingFlags) > 1 {
			msg += "s"
		}
		return fmt.Errorf("command %q: %s %q not set", getCommandPath(path), msg, strings.Join(missingFlags, ", "))
	}
	return nil
}

// collectArgs strips resolved command names from the parsed positional args and appends any args
// that appeared after the "--" delimiter.
func collectArgs(path []*Command, parsed, remaining []string) []string {
	// Skip past command names in remaining args. Only strip the exact command names that were
	// resolved during traversal (path[1:], since root never appears in user args), in order and
	// only once each.
	startIdx := 0
	chainIdx := 1 // Skip root
	for startIdx < len(parsed) && chainIdx < len(path) {
		if strings.EqualFold(parsed[startIdx], path[chainIdx].Name) {
			startIdx++
			chainIdx++
		} else {
			break
		}
	}

	var finalArgs []string
	if startIdx < len(parsed) {
		finalArgs = append(finalArgs, parsed[startIdx:]...)
	}
	if len(remaining) > 0 {
		finalArgs = append(finalArgs, remaining...)
	}
	return finalArgs
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

	if err := validateFlagsMetadata(root); err != nil {
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

// validateFlagsMetadata checks that each FlagMetadata entry refers to a flag that exists in the
// command's FlagSet, that Short aliases are single ASCII letters, and that no two entries share the
// same Short alias.
func validateFlagsMetadata(cmd *Command) error {
	if len(cmd.FlagsMetadata) == 0 {
		return nil
	}
	seenShorts := make(map[string]string) // short -> flag name
	for _, fm := range cmd.FlagsMetadata {
		if cmd.Flags == nil || cmd.Flags.Lookup(fm.Name) == nil {
			return fmt.Errorf("flag metadata references unknown flag %q", fm.Name)
		}
		if fm.Short == "" {
			continue
		}
		if len(fm.Short) != 1 || fm.Short[0] < 'a' || fm.Short[0] > 'z' {
			if fm.Short[0] < 'A' || fm.Short[0] > 'Z' {
				return fmt.Errorf("flag %q: short alias must be a single ASCII letter, got %q", fm.Name, fm.Short)
			}
		}
		if other, ok := seenShorts[fm.Short]; ok {
			return fmt.Errorf("duplicate short flag %q: used by both %q and %q", fm.Short, other, fm.Name)
		}
		seenShorts[fm.Short] = fm.Name
	}
	return nil
}
