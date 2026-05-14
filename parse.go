package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/pressly/cli/xflag"
)

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
			// anywhere. Also check short flag aliases from FlagConfigs.
			name := strings.TrimLeft(arg, "-")
			skipValue := false
			for _, cmd := range root.state.path {
				localFlags := localFlagSet(cmd.FlagConfigs)
				// Skip local flags on ancestor commands (any command already in the path is an
				// ancestor of the not-yet-resolved terminal command).
				if localFlags[name] {
					continue
				}
				// First try direct lookup.
				f := cmd.Flags.Lookup(name)
				// If not found, check if it's a short alias.
				if f == nil {
					for _, flagConfig := range cmd.FlagConfigs {
						if flagConfig.Short == name {
							if localFlags[flagConfig.Name] {
								break
							}
							f = cmd.Flags.Lookup(flagConfig.Name)
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
				sub.state = root.state
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

func clearCommandState(cmd *Command) {
	if cmd == nil {
		return
	}
	cmd.state = nil
	for _, sub := range cmd.SubCommands {
		clearCommandState(sub)
	}
}

// combineFlags merges flags from the command path into a single FlagSet. Flags are added in reverse
// order (deepest command first) so that child flags take precedence over parent flags. Short flag
// aliases from FlagConfigs are also registered, sharing the same Value as their long counterpart.
func combineFlags(path []*Command) *flag.FlagSet {
	combined := flag.NewFlagSet(path[0].Name, flag.ContinueOnError)
	combined.SetOutput(io.Discard)
	terminalIdx := len(path) - 1
	for i := terminalIdx; i >= 0; i-- {
		cmd := path[i]
		if cmd.Flags == nil {
			continue
		}
		localFlags := localFlagSet(cmd.FlagConfigs)
		shortMap := shortFlagMap(cmd.FlagConfigs)
		isAncestor := i < terminalIdx
		cmd.Flags.VisitAll(func(f *flag.Flag) {
			// Skip local flags from ancestor commands — they are not inherited.
			if isAncestor && localFlags[f.Name] {
				return
			}
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

// localFlagSet builds a set of flag names that are marked as local in FlagConfigs.
func localFlagSet(configs []FlagConfig) map[string]bool {
	m := make(map[string]bool, len(configs))
	for _, flagConfig := range configs {
		if flagConfig.Local {
			m[flagConfig.Name] = true
		}
	}
	return m
}

// shortFlagMap builds a map from long flag name to short alias from FlagConfigs.
func shortFlagMap(configs []FlagConfig) map[string]string {
	m := make(map[string]string, len(configs))
	for _, flagConfig := range configs {
		if flagConfig.Short != "" {
			m[flagConfig.Name] = flagConfig.Short
		}
	}
	return m
}

// checkRequiredFlags verifies that all flags marked as required in FlagConfigs were explicitly set
// during parsing.
func checkRequiredFlags(path []*Command, combined *flag.FlagSet) error {
	// Build a set of flags that were explicitly set during parsing. Visit (unlike VisitAll) only
	// iterates over flags that were actually provided by the user, regardless of their value.
	setFlags := make(map[string]struct{})
	combined.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = struct{}{}
	})

	terminalIdx := len(path) - 1
	var missingFlags []string
	for i, cmd := range path {
		for _, flagConfig := range cmd.FlagConfigs {
			if !flagConfig.Required {
				continue
			}
			// Skip required-flag checks for local flags on ancestor commands.
			if flagConfig.Local && i < terminalIdx {
				continue
			}
			if combined.Lookup(flagConfig.Name) == nil {
				return fmt.Errorf("command %q: internal error: required flag %s not found in flag set", getCommandPath(path), formatFlagName(flagConfig.Name))
			}
			if _, ok := setFlags[flagConfig.Name]; !ok {
				missingFlags = append(missingFlags, formatFlagName(flagConfig.Name))
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

func validateName(name string) error {
	if !validNameRegex.MatchString(name) {
		return errors.New("invalid name: must start with a letter and contain only letters, numbers, dashes, or underscores")
	}
	return nil
}

func validateCommands(root *Command, path []string) error {
	if root.Name == "" {
		if len(path) == 0 {
			return errors.New("root command has no name")
		}
		return fmt.Errorf("command %q: subcommand has no name", strings.Join(path, " "))
	}

	currentPath := append(path, root.Name)
	if err := validateName(root.Name); err != nil {
		return commandDefinitionError(currentPath, err)
	}

	if err := flagSetupError(root.Flags); err != nil {
		return commandDefinitionError(currentPath, err)
	}

	if err := validateFlagConfigs(root); err != nil {
		return commandDefinitionError(currentPath, err)
	}

	for _, sub := range root.SubCommands {
		if err := validateCommands(sub, currentPath); err != nil {
			return err
		}
	}
	return nil
}

func commandDefinitionError(path []string, err error) error {
	return fmt.Errorf("command %q: %w", strings.Join(path, " "), err)
}

// validateFlagConfigs checks that each FlagConfig entry refers to a flag that exists in the
// command's FlagSet, that Short aliases are single ASCII letters, and that no two entries share the
// same Short alias.
func validateFlagConfigs(cmd *Command) error {
	if len(cmd.FlagConfigs) == 0 {
		return nil
	}
	seenShorts := make(map[string]string) // short -> flag name
	for _, flagConfig := range cmd.FlagConfigs {
		if flagConfig.Name == "" {
			return errors.New("flag config is missing a name")
		}
		if cmd.Flags == nil || cmd.Flags.Lookup(flagConfig.Name) == nil {
			return fmt.Errorf("flag %s is configured but not defined", formatFlagName(flagConfig.Name))
		}
		if flagConfig.Short == "" {
			continue
		}
		if !isShortAlias(flagConfig.Short) {
			return fmt.Errorf("flag %s has invalid short alias %q; short aliases must be one ASCII letter", formatFlagName(flagConfig.Name), flagConfig.Short)
		}
		if other, ok := seenShorts[flagConfig.Short]; ok {
			return fmt.Errorf("short flag %s is configured for both %s and %s", formatFlagName(flagConfig.Short), formatFlagName(other), formatFlagName(flagConfig.Name))
		}
		seenShorts[flagConfig.Short] = flagConfig.Name
	}
	return nil
}

func isShortAlias(alias string) bool {
	if len(alias) != 1 {
		return false
	}
	return alias[0] >= 'a' && alias[0] <= 'z' || alias[0] >= 'A' && alias[0] <= 'Z'
}
