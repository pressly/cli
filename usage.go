package cli

import (
	"bytes"
	"cmp"
	"flag"
	"fmt"
	"slices"
	"strings"
	"text/tabwriter"
)

// Help returns help text for root's resolved command.
//
// Call Help after Parse when you want to render help yourself. ParseAndRun calls it automatically
// for --help, and Run calls it automatically for UsageErrorf errors.
func Help(root *Command) string {
	if root == nil {
		return ""
	}

	terminalCmd := root.terminal()
	if terminalCmd.Help != nil {
		return strings.TrimRight(terminalCmd.Help(terminalCmd), "\n")
	}

	return defaultHelp(root)
}

func defaultHelp(root *Command) string {
	terminalCmd := root.terminal()

	var blocks []string

	if terminalCmd.ShortHelp != "" {
		blocks = append(blocks, terminalCmd.ShortHelp)
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
	blocks = append(blocks, renderLines("Usage:", usageLine))

	if len(terminalCmd.SubCommands) > 0 {
		sortedCommands := slices.Clone(terminalCmd.SubCommands)
		slices.SortFunc(sortedCommands, func(a, b *Command) int {
			return cmp.Compare(a.Name, b.Name)
		})

		subcommands := make([]helpItem, 0, len(sortedCommands))
		for _, sub := range sortedCommands {
			subcommands = append(subcommands, helpItem{
				Name:    sub.Name,
				Summary: sub.ShortHelp,
			})
		}
		blocks = append(blocks, renderItems("Available Commands:", subcommands))
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
			blocks = append(blocks, renderFlags("Flags:", usageFlags(flags, false)))
		}

		if hasInherited {
			blocks = append(blocks, renderFlags("Inherited Flags:", usageFlags(flags, true)))
		}
	}

	if len(terminalCmd.SubCommands) > 0 {
		cmdName := terminalCmd.Name
		if root.state != nil && len(root.state.path) > 0 {
			cmdName = getCommandPath(root.state.path)
		}
		blocks = append(blocks,
			fmt.Sprintf("Use \"%s [command] --help\" for more information about a command.", cmdName),
		)
	}

	return strings.Join(blocks, "\n\n")
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

func usageFlags(flags []flagInfo, inherited bool) []helpFlag {
	out := make([]helpFlag, 0, len(flags))
	for _, f := range flags {
		if f.inherited != inherited {
			continue
		}
		defval := ""
		if !f.required && !isZeroDefault(f.defval, f.typeName) {
			defval = f.defval
		}
		out = append(out, helpFlag{
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

func renderLines(heading string, lines ...string) string {
	var b strings.Builder
	b.WriteString(heading)
	for _, line := range lines {
		b.WriteString("\n  ")
		b.WriteString(line)
	}
	return b.String()
}

func renderFlags(heading string, flags []helpFlag) string {
	hasShort := false
	for _, f := range flags {
		if f.Short != "" {
			hasShort = true
			break
		}
	}
	items := make([]helpItem, 0, len(flags))
	for _, f := range flags {
		items = append(items, helpItem{
			Name:    f.spec(hasShort),
			Summary: f.description(),
		})
	}
	return renderItems(heading, items)
}

func renderItems(heading string, items []helpItem) string {
	var out strings.Builder
	out.WriteString(heading)
	out.WriteByte('\n')

	var rows bytes.Buffer
	tw := tabwriter.NewWriter(&rows, 0, 0, 4, ' ', 0)
	for _, item := range items {
		if item.Summary == "" {
			_, _ = fmt.Fprintf(tw, "  %s\n", item.Name)
			continue
		}
		_, _ = fmt.Fprintf(tw, "  %s\t%s\n", item.Name, item.Summary)
	}
	_ = tw.Flush()
	out.Write(rows.Bytes())

	return strings.TrimRight(out.String(), "\n")
}

type helpItem struct {
	Name    string
	Summary string
}

type helpFlag struct {
	Name        string
	Short       string
	Placeholder string
	Usage       string
	Default     string
	Required    bool
}

func (f helpFlag) spec(padShort bool) string {
	var name string
	if f.Short != "" {
		name = "-" + f.Short + ", --" + f.Name
	} else if padShort {
		name = "    --" + f.Name
	} else {
		name = "--" + f.Name
	}
	if f.Placeholder == "" {
		return name
	}
	return name + " " + f.Placeholder
}

func (f helpFlag) description() string {
	description := f.Usage
	if f.Required {
		return description + " (required)"
	}
	if f.Default != "" {
		return fmt.Sprintf("%s (default: %s)", description, f.Default)
	}
	return description
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
