package cli

import (
	"cmp"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/pressly/cli/usage"
)

// Help returns the help document for root's resolved command.
//
// Call Help after Parse when you want to render help yourself, or inside a Command.Help hook when
// composing the default help document. ParseAndRun calls it automatically for --help and UsageErrorf
// errors.
func Help(root *Command) usage.Help {
	if root == nil {
		return nil
	}

	// Get terminal command from state
	terminalCmd := root.terminal()

	var help usage.Help

	if terminalCmd.ShortHelp != "" {
		help = append(help, usage.Text(terminalCmd.ShortHelp))
	}

	flags := collectHelpFlags(root, terminalCmd)

	var usageLine string
	if terminalCmd.Usage != "" {
		usageLine = terminalCmd.Usage
	} else {
		usageLine = terminalCmd.Name
		if root.state != nil && len(root.state.path) > 0 {
			usageLine = getCommandPath(root.state.path)
		}
		if len(flags) > 0 {
			usageLine += " [flags]"
		}
		if len(terminalCmd.SubCommands) > 0 {
			usageLine += " <command>"
		}
	}
	help = append(help, usage.Lines("Usage:", usageLine))

	if len(terminalCmd.SubCommands) > 0 {
		sortedCommands := slices.Clone(terminalCmd.SubCommands)
		slices.SortFunc(sortedCommands, func(a, b *Command) int {
			return cmp.Compare(a.Name, b.Name)
		})

		subcommands := make([]usage.Command, 0, len(sortedCommands))
		for _, sub := range sortedCommands {
			subcommands = append(subcommands, usage.Command{
				Name:    sub.Name,
				Summary: sub.ShortHelp,
			})
		}
		help = append(help, usage.Commands("Available Commands:", subcommands))
	}

	if len(flags) > 0 {
		slices.SortFunc(flags, func(a, b flagInfo) int {
			return cmp.Compare(a.name, b.name)
		})

		hasLocal := false
		hasInherited := false
		for _, f := range flags {
			if f.inherited {
				hasInherited = true
			} else {
				hasLocal = true
			}
		}

		if hasLocal {
			help = append(help, usage.Flags("Flags:", usageFlags(flags, false)))
		}

		if hasInherited {
			help = append(help, usage.Flags("Inherited Flags:", usageFlags(flags, true)))
		}
	}

	if len(terminalCmd.SubCommands) > 0 {
		cmdName := terminalCmd.Name
		if root.state != nil && len(root.state.path) > 0 {
			cmdName = getCommandPath(root.state.path)
		}
		help = append(help, usage.Text(
			fmt.Sprintf("Use \"%s [command] --help\" for more information about a command.", cmdName),
		))
	}

	if terminalCmd.Help != nil {
		help = terminalCmd.Help(terminalCmd, help)
	}

	return help
}

func collectHelpFlags(root, terminalCmd *Command) []flagInfo {
	var flags []flagInfo
	if root.state != nil && len(root.state.path) > 0 {
		terminalIdx := len(root.state.path) - 1
		for i, cmd := range root.state.path {
			if cmd.Flags == nil {
				continue
			}
			isInherited := i < terminalIdx
			metaMap := flagConfigMap(cmd.FlagConfigs)
			cmd.Flags.VisitAll(func(f *flag.Flag) {
				// Skip local flags from ancestor commands — they don't appear in child help.
				if isInherited {
					if m, ok := metaMap[f.Name]; ok && m.Local {
						return
					}
				}
				fi := flagInfo{
					name:      "--" + f.Name,
					usage:     f.Usage,
					defval:    f.DefValue,
					typeName:  flagTypeName(f),
					inherited: isInherited,
				}
				if m, ok := metaMap[f.Name]; ok {
					fi.required = m.Required
					fi.short = m.Short
				}
				flags = append(flags, fi)
			})
		}
	} else if terminalCmd.Flags != nil {
		// Pre-parse fallback: show the command's own flags even without state.
		metaMap := flagConfigMap(terminalCmd.FlagConfigs)
		terminalCmd.Flags.VisitAll(func(f *flag.Flag) {
			fi := flagInfo{
				name:     "--" + f.Name,
				usage:    f.Usage,
				defval:   f.DefValue,
				typeName: flagTypeName(f),
			}
			if m, ok := metaMap[f.Name]; ok {
				fi.required = m.Required
				fi.short = m.Short
			}
			flags = append(flags, fi)
		})
	}
	return flags
}

func usageFlags(flags []flagInfo, inherited bool) []usage.Flag {
	out := make([]usage.Flag, 0, len(flags))
	for _, f := range flags {
		if f.inherited != inherited {
			continue
		}
		defval := ""
		if !f.required && !isZeroDefault(f.defval, f.typeName) {
			defval = f.defval
		}
		out = append(out, usage.Flag{
			Name:        strings.TrimPrefix(f.name, "--"),
			Short:       f.short,
			Placeholder: f.typeName,
			Usage:       f.usage,
			Default:     defval,
			Required:    f.required,
		})
	}
	return out
}

// flagConfigMap builds a lookup map from flag name to its FlagConfig.
func flagConfigMap(options []FlagConfig) map[string]FlagConfig {
	m := make(map[string]FlagConfig, len(options))
	for _, fm := range options {
		m[fm.Name] = fm
	}
	return m
}

type flagInfo struct {
	name      string
	short     string
	usage     string
	defval    string
	typeName  string
	inherited bool
	required  bool
}

// flagTypeName returns a short type name for a flag's value. Bool flags return "" since their type
// is obvious from usage. This mirrors the approach used by Go's flag.PrintDefaults.
func flagTypeName(f *flag.Flag) string {
	// Use the type name from the Value interface, which returns the type as a string.
	typeName := fmt.Sprintf("%T", f.Value)
	// The flag package uses unexported types like *flag.boolValue, *flag.stringValue, etc. Extract
	// just the base name and strip the "Value" suffix.
	if i := strings.LastIndex(typeName, "."); i >= 0 {
		typeName = typeName[i+1:]
	}
	typeName = strings.TrimPrefix(typeName, "*")
	typeName = strings.TrimSuffix(typeName, "Value")

	// Don't show type for bools — their usage is self-evident.
	if typeName == "bool" {
		return ""
	}
	return typeName
}

// isZeroDefault returns true if the default value is the zero value for its type and should be
// suppressed in help output to reduce noise.
func isZeroDefault(defval, typeName string) bool {
	switch {
	case defval == "":
		return true
	case defval == "false" && typeName == "":
		// Bool flags (typeName is "" for bools).
		return true
	case defval == "0" && (typeName == "int" || typeName == "int64" || typeName == "uint" || typeName == "uint64"):
		return true
	case defval == "0" && typeName == "float64":
		return true
	}
	return false
}
