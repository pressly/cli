package cli

import (
	"cmp"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/pressly/cli/pkg/textutil"
)

// defaultTerminalWidth is the assumed terminal width for wrapping help text.
const defaultTerminalWidth = 80

// DefaultUsage returns the default usage string for the command hierarchy. It is used when the
// command does not provide a custom usage function. The usage string includes the command's short
// help, usage pattern, available subcommands, and flags.
func DefaultUsage(root *Command) string {
	if root == nil {
		return ""
	}

	// Get terminal command from state
	terminalCmd := root.terminal()

	var b strings.Builder

	if terminalCmd.ShortHelp != "" {
		b.WriteString(terminalCmd.ShortHelp)
		b.WriteString("\n\n")
	}

	b.WriteString("Usage:\n")
	if terminalCmd.Usage != "" {
		b.WriteString("  " + terminalCmd.Usage + "\n")
	} else {
		usage := terminalCmd.Name
		if root.state != nil && len(root.state.path) > 0 {
			usage = getCommandPath(root.state.path)
		}
		if terminalCmd.Flags != nil {
			usage += " [flags]"
		}
		if len(terminalCmd.SubCommands) > 0 {
			usage += " <command>"
		}
		b.WriteString("  " + usage + "\n")
	}
	b.WriteString("\n")

	if len(terminalCmd.SubCommands) > 0 {
		b.WriteString("Available Commands:\n")
		sortedCommands := slices.Clone(terminalCmd.SubCommands)
		slices.SortFunc(sortedCommands, func(a, b *Command) int {
			return cmp.Compare(a.Name, b.Name)
		})

		maxNameLen := 0
		for _, sub := range sortedCommands {
			if len(sub.Name) > maxNameLen {
				maxNameLen = len(sub.Name)
			}
		}

		nameWidth := maxNameLen + 4
		wrapWidth := defaultTerminalWidth - nameWidth

		for _, sub := range sortedCommands {
			if sub.ShortHelp == "" {
				fmt.Fprintf(&b, "  %s\n", sub.Name)
				continue
			}

			lines := textutil.Wrap(sub.ShortHelp, wrapWidth)
			padding := strings.Repeat(" ", maxNameLen-len(sub.Name)+4)
			fmt.Fprintf(&b, "  %s%s%s\n", sub.Name, padding, lines[0])

			indentPadding := strings.Repeat(" ", nameWidth+2)
			for _, line := range lines[1:] {
				fmt.Fprintf(&b, "%s%s\n", indentPadding, line)
			}
		}
		b.WriteString("\n")
	}

	var flags []flagInfo
	if root.state != nil && len(root.state.path) > 0 {
		terminalIdx := len(root.state.path) - 1
		for i, cmd := range root.state.path {
			if cmd.Flags == nil {
				continue
			}
			isInherited := i < terminalIdx
			metaMap := flagOptionMap(cmd.FlagOptions)
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
		metaMap := flagOptionMap(terminalCmd.FlagOptions)
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

	if len(flags) > 0 {
		slices.SortFunc(flags, func(a, b flagInfo) int {
			return cmp.Compare(a.name, b.name)
		})

		hasAnyShort := false
		for _, f := range flags {
			if f.short != "" {
				hasAnyShort = true
				break
			}
		}

		maxFlagLen := 0
		for _, f := range flags {
			if n := len(f.displayName(hasAnyShort)); n > maxFlagLen {
				maxFlagLen = n
			}
		}

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
			b.WriteString("Flags:\n")
			writeFlagSection(&b, flags, maxFlagLen, false, hasAnyShort)
			b.WriteString("\n")
		}

		if hasInherited {
			b.WriteString("Inherited Flags:\n")
			writeFlagSection(&b, flags, maxFlagLen, true, hasAnyShort)
			b.WriteString("\n")
		}
	}

	if len(terminalCmd.SubCommands) > 0 {
		cmdName := terminalCmd.Name
		if root.state != nil && len(root.state.path) > 0 {
			cmdName = getCommandPath(root.state.path)
		}
		fmt.Fprintf(&b, "Use \"%s [command] --help\" for more information about a command.\n", cmdName)
	}

	return strings.TrimRight(b.String(), "\n")
}

// writeFlagSection handles the formatting of flag descriptions
func writeFlagSection(b *strings.Builder, flags []flagInfo, maxLen int, inherited, hasAnyShort bool) {
	nameWidth := maxLen + 4
	wrapWidth := defaultTerminalWidth - nameWidth

	for _, f := range flags {
		if f.inherited != inherited {
			continue
		}

		description := f.usage
		if f.required {
			description += " (required)"
		} else if !isZeroDefault(f.defval, f.typeName) {
			description += fmt.Sprintf(" (default: %s)", f.defval)
		}

		display := f.displayName(hasAnyShort)
		lines := textutil.Wrap(description, wrapWidth)
		padding := strings.Repeat(" ", maxLen-len(display)+4)
		fmt.Fprintf(b, "  %s%s%s\n", display, padding, lines[0])

		indentPadding := strings.Repeat(" ", nameWidth+2)
		for _, line := range lines[1:] {
			fmt.Fprintf(b, "%s%s\n", indentPadding, line)
		}
	}
}

// flagOptionMap builds a lookup map from flag name to its FlagOption.
func flagOptionMap(options []FlagOption) map[string]FlagOption {
	m := make(map[string]FlagOption, len(options))
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

// displayName returns the flag name with optional short alias and type hint. When hasAnyShort is
// true, flags without a short alias are padded to align with those that have one. Examples: "-v,
// --verbose", "-o, --output string", "    --config string", "--debug".
func (f flagInfo) displayName(hasAnyShort bool) string {
	var name string
	if f.short != "" {
		name = "-" + f.short + ", " + f.name
	} else if hasAnyShort {
		name = "    " + f.name
	} else {
		name = f.name
	}
	if f.typeName == "" {
		return name
	}
	return name + " " + f.typeName
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
