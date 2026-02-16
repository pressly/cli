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

	if terminalCmd.UsageFunc != nil {
		return terminalCmd.UsageFunc(terminalCmd)
	}

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
		for i, cmd := range root.state.path {
			if cmd.Flags == nil {
				continue
			}
			isGlobal := i < len(root.state.path)-1
			requiredFlags := make(map[string]bool)
			for _, m := range cmd.FlagsMetadata {
				if m.Required {
					requiredFlags[m.Name] = true
				}
			}
			cmd.Flags.VisitAll(func(f *flag.Flag) {
				flags = append(flags, flagInfo{
					name:     "-" + f.Name,
					usage:    f.Usage,
					defval:   f.DefValue,
					typeName: flagTypeName(f),
					global:   isGlobal,
					required: requiredFlags[f.Name],
				})
			})
		}
	} else if terminalCmd.Flags != nil {
		// Pre-parse fallback: show the command's own flags even without state.
		requiredFlags := make(map[string]bool)
		for _, m := range terminalCmd.FlagsMetadata {
			if m.Required {
				requiredFlags[m.Name] = true
			}
		}
		terminalCmd.Flags.VisitAll(func(f *flag.Flag) {
			flags = append(flags, flagInfo{
				name:     "-" + f.Name,
				usage:    f.Usage,
				defval:   f.DefValue,
				typeName: flagTypeName(f),
				required: requiredFlags[f.Name],
			})
		})
	}

	if len(flags) > 0 {
		slices.SortFunc(flags, func(a, b flagInfo) int {
			return cmp.Compare(a.name, b.name)
		})

		maxFlagLen := 0
		for _, f := range flags {
			if n := len(f.displayName()); n > maxFlagLen {
				maxFlagLen = n
			}
		}

		hasLocal := false
		hasGlobal := false
		for _, f := range flags {
			if f.global {
				hasGlobal = true
			} else {
				hasLocal = true
			}
		}

		if hasLocal {
			b.WriteString("Flags:\n")
			writeFlagSection(&b, flags, maxFlagLen, false)
			b.WriteString("\n")
		}

		if hasGlobal {
			b.WriteString("Global Flags:\n")
			writeFlagSection(&b, flags, maxFlagLen, true)
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
func writeFlagSection(b *strings.Builder, flags []flagInfo, maxLen int, global bool) {
	nameWidth := maxLen + 4
	wrapWidth := defaultTerminalWidth - nameWidth

	for _, f := range flags {
		if f.global != global {
			continue
		}

		description := f.usage
		if f.required {
			description += " (required)"
		} else if !isZeroDefault(f.defval, f.typeName) {
			description += fmt.Sprintf(" (default: %s)", f.defval)
		}

		display := f.displayName()
		lines := textutil.Wrap(description, wrapWidth)
		padding := strings.Repeat(" ", maxLen-len(display)+4)
		fmt.Fprintf(b, "  %s%s%s\n", display, padding, lines[0])

		indentPadding := strings.Repeat(" ", nameWidth+2)
		for _, line := range lines[1:] {
			fmt.Fprintf(b, "%s%s\n", indentPadding, line)
		}
	}
}

type flagInfo struct {
	name     string
	usage    string
	defval   string
	typeName string
	global   bool
	required bool
}

// displayName returns the flag name with its type hint, e.g., "-config string" or "-verbose" (no
// type for bools).
func (f flagInfo) displayName() string {
	if f.typeName == "" {
		return f.name
	}
	return f.name + " " + f.typeName
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
