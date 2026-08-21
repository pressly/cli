package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
)

func splitAtDelimiter(args []string) (argsToParse, remaining []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func resolveCommandPath(root *Command, argsToParse []string) (*Command, error) {
	current := root
	if current.Flags == nil {
		current.Flags = flag.NewFlagSet(root.Name, flag.ContinueOnError)
	}

	i := 0
	for i < len(argsToParse) {
		arg := argsToParse[i]

		if strings.HasPrefix(arg, "-") {
			if strings.Contains(arg, "=") {
				i++
				continue
			}

			// A parent flag may appear before a subcommand, so inspect the full path before
			// deciding whether the next argument is its value.
			name := strings.TrimLeft(arg, "-")
			skipValue := false
			for _, cmd := range root.state.path {
				localFlags := localFlagSet(cmd.FlagConfigs)
				if localFlags[name] {
					continue
				}
				f := cmd.Flags.Lookup(name)
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
				i += 2
				continue
			}
			i++
			continue
		}

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

// combineFlags adds child flags first so they take precedence over inherited flags.
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
			if short, ok := shortMap[f.Name]; ok {
				if combined.Lookup(short) == nil {
					combined.Var(f.Value, short, f.Usage)
				}
			}
		})
	}
	return combined
}

func localFlagSet(configs []FlagConfig) map[string]bool {
	m := make(map[string]bool, len(configs))
	for _, flagConfig := range configs {
		if flagConfig.Local {
			m[flagConfig.Name] = true
		}
	}
	return m
}

func shortFlagMap(configs []FlagConfig) map[string]string {
	m := make(map[string]string, len(configs))
	for _, flagConfig := range configs {
		if flagConfig.Short != "" {
			m[flagConfig.Name] = flagConfig.Short
		}
	}
	return m
}

func checkRequiredFlags(path []*Command, combined *flag.FlagSet) error {
	// Visit reports flags explicitly set by the user, including explicit zero values.
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

// collectArgs removes the resolved command path and restores arguments after "--".
func collectArgs(path []*Command, parsed, remaining []string) []string {
	startIdx := 0
	chainIdx := 1 // The root name is not part of args.
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
