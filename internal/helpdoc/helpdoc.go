// Package helpdoc builds the default command help document.
package helpdoc

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/pressly/cli/pkg/textutil"
)

const defaultTerminalWidth = 80

// Document is an ordered list of help blocks.
type Document []Block

// Block is one section in a help document.
type Block struct {
	heading string
	lines   []string
	indent  bool
	items   []Item
}

// Item is one row in a list block.
type Item struct {
	Name    string
	Summary string
}

// Command is the command metadata needed to build help.
type Command struct {
	Name        string
	Usage       string
	Summary     string
	Description string
	Flags       *flag.FlagSet
	FlagConfigs []FlagConfig
	Subcommands []Command
}

// FlagConfig adds help-specific behavior to a flag in a FlagSet.
type FlagConfig struct {
	Name     string
	Short    string
	Required bool
	Local    bool
}

// New returns the default help document for the parsed command path.
func New(path []Command) Document {
	if len(path) == 0 {
		return nil
	}

	cmd := path[len(path)-1]
	flags := collectFlags(path)

	var doc Document
	if text := commandHelpText(cmd); text != "" {
		doc = append(doc, Text(text))
	}

	usageLine := cmd.Usage
	if usageLine == "" {
		usageLine = commandPath(path)
		if len(flags) > 0 {
			usageLine += " [flags]"
		}
		if len(cmd.Subcommands) > 0 {
			usageLine += " <command>"
		}
	}
	doc = append(doc, Lines("Usage:", usageLine))

	if len(cmd.Subcommands) > 0 {
		subcommands := slices.Clone(cmd.Subcommands)
		slices.SortFunc(subcommands, func(a, b Command) int {
			return strings.Compare(a.Name, b.Name)
		})

		items := make([]Item, 0, len(subcommands))
		for _, sub := range subcommands {
			items = append(items, Item{
				Name:    sub.Name,
				Summary: commandListSummary(sub),
			})
		}
		doc = append(doc, List("Available Commands:", items...))
	}

	if len(flags) > 0 {
		slices.SortFunc(flags, func(a, b flagInfo) int {
			return strings.Compare(a.name, b.name)
		})

		localFlags, inheritedFlags := splitFlags(flags)
		if len(localFlags) > 0 {
			doc = append(doc, flagsBlock("Flags:", localFlags))
		}
		if len(inheritedFlags) > 0 {
			doc = append(doc, flagsBlock("Inherited Flags:", inheritedFlags))
		}
	}

	if len(cmd.Subcommands) > 0 {
		doc = append(doc, Text(
			fmt.Sprintf("Use \"%s [command] --help\" for more information about a command.", commandPath(path)),
		))
	}

	return doc
}

// Text returns an untitled paragraph block.
func Text(lines ...string) Block {
	return Block{lines: lines}
}

// Lines returns a titled block of indented lines.
func Lines(heading string, lines ...string) Block {
	return Block{heading: heading, lines: lines, indent: true}
}

// List returns a titled list of name/summary pairs.
func List(heading string, items ...Item) Block {
	return Block{heading: heading, items: items}
}

func flagsBlock(heading string, flags []flagInfo) Block {
	hasShort := false
	for _, f := range flags {
		if f.short != "" {
			hasShort = true
			break
		}
	}

	items := make([]Item, 0, len(flags))
	for _, f := range flags {
		items = append(items, Item{
			Name:    flagSpec(f.name, f.short, f.placeholder, hasShort),
			Summary: flagDescription(f.usage, f.defaultValue, f.required),
		})
	}
	return List(heading, items...)
}

func commandPath(path []Command) string {
	names := make([]string, 0, len(path))
	for _, cmd := range path {
		names = append(names, cmd.Name)
	}
	return strings.Join(names, " ")
}

// String renders the help document as a string.
func (d Document) String() string {
	var b strings.Builder
	_, _ = d.WriteTo(&b)
	return strings.TrimRight(b.String(), "\n")
}

// WriteTo writes the help document to w.
func (d Document) WriteTo(w io.Writer) (n int64, err error) {
	cw := &countWriter{w: w}
	for i, block := range d {
		if i > 0 {
			if _, err := fmt.Fprintln(cw); err != nil {
				return cw.n, err
			}
		}
		if _, err := block.writeTo(cw); err != nil {
			return cw.n, err
		}
	}
	return cw.n, nil
}

func (b Block) writeTo(w io.Writer) (n int64, err error) {
	cw := &countWriter{w: w}
	if b.heading != "" {
		if _, err := fmt.Fprintln(cw, b.heading); err != nil {
			return cw.n, err
		}
	}
	if len(b.items) > 0 {
		if _, err := writeItems(cw, b.items); err != nil {
			return cw.n, err
		}
	}
	for _, line := range b.lines {
		if b.indent {
			line = "  " + line
		}
		if _, err := fmt.Fprintln(cw, line); err != nil {
			return cw.n, err
		}
	}
	return cw.n, nil
}

func flagSpec(name, short, placeholder string, padShort bool) string {
	var spec string
	if short != "" {
		spec = "-" + short + ", --" + name
	} else if padShort {
		spec = "    --" + name
	} else {
		spec = "--" + name
	}
	if placeholder == "" {
		return spec
	}
	return spec + " " + placeholder
}

func flagDescription(usage, defaultValue string, required bool) string {
	if required {
		return usage + " (required)"
	}
	if defaultValue != "" {
		return fmt.Sprintf("%s (default: %s)", usage, defaultValue)
	}
	return usage
}

func flagTypeName(f *flag.Flag) string {
	typeName := fmt.Sprintf("%T", f.Value)
	if i := strings.LastIndex(typeName, "."); i >= 0 {
		typeName = typeName[i+1:]
	}
	typeName = strings.TrimPrefix(typeName, "*")
	typeName = strings.TrimSuffix(typeName, "Value")
	if typeName == "bool" {
		return ""
	}
	return typeName
}

func commandHelpText(cmd Command) string {
	if cmd.Description != "" {
		return cmd.Description
	}
	return cmd.Summary
}

func commandListSummary(cmd Command) string {
	if cmd.Summary != "" {
		return cmd.Summary
	}
	return firstLine(cmd.Description)
}

func firstLine(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func collectFlags(path []Command) []flagInfo {
	var flags []flagInfo
	terminalIdx := len(path) - 1
	for i, cmd := range path {
		if cmd.Flags == nil {
			continue
		}
		inherited := i < terminalIdx
		meta := flagConfigMap(cmd.FlagConfigs)
		cmd.Flags.VisitAll(func(f *flag.Flag) {
			cfg := meta[f.Name]
			if inherited && cfg.Local {
				return
			}
			info := flagInfo{
				name:         f.Name,
				short:        cfg.Short,
				usage:        f.Usage,
				defaultValue: f.DefValue,
				placeholder:  flagTypeName(f),
				required:     cfg.Required,
				inherited:    inherited,
			}
			info.defaultValue = flagDefault(info.defaultValue, info.placeholder, info.required)
			flags = append(flags, info)
		})
	}
	return flags
}

func splitFlags(flags []flagInfo) (local, inherited []flagInfo) {
	for _, f := range flags {
		if f.inherited {
			inherited = append(inherited, f)
		} else {
			local = append(local, f)
		}
	}
	return local, inherited
}

func flagConfigMap(configs []FlagConfig) map[string]FlagConfig {
	m := make(map[string]FlagConfig, len(configs))
	for _, cfg := range configs {
		m[cfg.Name] = cfg
	}
	return m
}

func flagDefault(defval, typeName string, required bool) string {
	if required || isZeroDefault(defval, typeName) {
		return ""
	}
	return defval
}

func isZeroDefault(defval, typeName string) bool {
	switch {
	case defval == "":
		return true
	case defval == "false" && typeName == "":
		return true
	case defval == "0" && (typeName == "int" || typeName == "int64" || typeName == "uint" || typeName == "uint64"):
		return true
	case defval == "0" && typeName == "float64":
		return true
	}
	return false
}

type flagInfo struct {
	name         string
	short        string
	placeholder  string
	usage        string
	defaultValue string
	required     bool
	inherited    bool
}

func writeItems(w io.Writer, items []Item) (int64, error) {
	cw := &countWriter{w: w}
	maxNameLen := 0
	for _, item := range items {
		if len(item.Name) > maxNameLen {
			maxNameLen = len(item.Name)
		}
	}

	summaryIndent := maxNameLen + 6
	wrapWidth := max(defaultTerminalWidth-summaryIndent, 20)

	for _, item := range items {
		if item.Summary == "" {
			if _, err := fmt.Fprintf(cw, "  %s\n", item.Name); err != nil {
				return cw.n, err
			}
			continue
		}

		lines := textutil.Wrap(item.Summary, wrapWidth)
		if len(lines) == 0 {
			if _, err := fmt.Fprintf(cw, "  %s\n", item.Name); err != nil {
				return cw.n, err
			}
			continue
		}

		padding := strings.Repeat(" ", maxNameLen-len(item.Name)+4)
		if _, err := fmt.Fprintf(cw, "  %s%s%s\n", item.Name, padding, lines[0]); err != nil {
			return cw.n, err
		}

		indent := strings.Repeat(" ", summaryIndent)
		for _, line := range lines[1:] {
			if _, err := fmt.Fprintf(cw, "%s%s\n", indent, line); err != nil {
				return cw.n, err
			}
		}
	}
	return cw.n, nil
}

type countWriter struct {
	w io.Writer
	n int64
}

func (w *countWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}
