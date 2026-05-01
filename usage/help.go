// Package usage provides optional building blocks for command help documents.
//
// Use this package when cli's default help text is close to what you want, but you need to append
// examples, add sections, or render the same command metadata in a different layout.
package usage

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/pressly/cli"
)

// Document is an ordered list of blocks that can be rendered as command help.
type Document []Block

// Block is one section in a help document.
type Block struct {
	// Heading is rendered above the block when set, such as "Usage:" or "Examples:".
	Heading string

	lines  []string
	indent bool
	items  []Item
}

// Item is one row in a List block.
type Item struct {
	Name    string
	Summary string
}

// Text returns an untitled paragraph block.
//
// Use Text for descriptions, notes, or closing hints.
func Text(lines ...string) Block {
	return Block{lines: lines}
}

// Lines returns a titled block of indented lines.
//
// Use Lines for sections such as Usage or Examples where each line should stand on its own.
func Lines(heading string, lines ...string) Block {
	return Block{Heading: heading, lines: lines, indent: true}
}

// List returns a titled list of name/summary pairs.
//
// Use List for aligned sections such as commands, flags, or named examples.
func List(heading string, items ...Item) Block {
	return Block{Heading: heading, items: items}
}

// String renders the full help document as a string.
//
// Use String when returning help from cli.Command.Help or when comparing help text in tests.
func (d Document) String() string {
	var b strings.Builder
	_, _ = d.WriteTo(&b)
	return strings.TrimRight(b.String(), "\n")
}

// WriteTo writes the help document to w.
//
// Use WriteTo when streaming help directly to stdout, stderr, or another writer.
func (d Document) WriteTo(w io.Writer) (n int64, err error) {
	cw := &countWriter{w: w}
	for i, block := range d {
		if i > 0 {
			if _, err := fmt.Fprintln(cw); err != nil {
				return cw.n, err
			}
		}
		if _, err := block.WriteTo(cw); err != nil {
			return cw.n, err
		}
	}
	return cw.n, nil
}

// String renders the block as a string.
func (b Block) String() string {
	var s strings.Builder
	_, _ = b.WriteTo(&s)
	return strings.TrimRight(s.String(), "\n")
}

// WriteTo writes the block to w.
func (b Block) WriteTo(w io.Writer) (n int64, err error) {
	cw := &countWriter{w: w}
	if b.Heading != "" {
		if _, err := fmt.Fprintln(cw, b.Heading); err != nil {
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

func writeItems(w io.Writer, items []Item) (int64, error) {
	cw := &countWriter{w: w}
	var b bytes.Buffer
	tw := tabwriter.NewWriter(&b, 0, 0, 4, ' ', 0)
	for _, item := range items {
		if item.Summary == "" {
			if _, err := fmt.Fprintf(tw, "  %s\n", item.Name); err != nil {
				return cw.n, err
			}
			continue
		}
		if _, err := fmt.Fprintf(tw, "  %s\t%s\n", item.Name, item.Summary); err != nil {
			return cw.n, err
		}
	}
	if err := tw.Flush(); err != nil {
		return cw.n, err
	}
	if _, err := cw.Write(b.Bytes()); err != nil {
		return cw.n, err
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

// New returns the default help document for cmd.
func New(cmd *cli.Command) Document {
	if cmd == nil {
		return nil
	}
	if path := cmd.Path(); len(path) > 0 {
		cmd = path[len(path)-1]
	}

	var doc Document

	if cmd.ShortHelp != "" {
		doc = append(doc, Text(cmd.ShortHelp))
	}

	flags := collectHelpFlags(cmd)

	usageLine := cmd.Usage
	if usageLine == "" {
		usageLine = commandPath(cmd)
		if len(flags) > 0 {
			usageLine += " [flags]"
		}
		if len(cmd.SubCommands) > 0 {
			usageLine += " <command>"
		}
	}
	doc = append(doc, Lines("Usage:", usageLine))

	if len(cmd.SubCommands) > 0 {
		sortedCommands := slices.Clone(cmd.SubCommands)
		slices.SortFunc(sortedCommands, func(a, b *cli.Command) int {
			return strings.Compare(a.Name, b.Name)
		})

		subcommands := make([]Command, 0, len(sortedCommands))
		for _, sub := range sortedCommands {
			subcommands = append(subcommands, Command{
				Name:    sub.Name,
				Summary: sub.ShortHelp,
			})
		}
		doc = append(doc, Commands("Available Commands:", subcommands))
	}

	if len(flags) > 0 {
		slices.SortFunc(flags, func(a, b flagInfo) int {
			return strings.Compare(a.name, b.name)
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
			doc = append(doc, Flags("Flags:", usageFlags(flags, false)))
		}

		if hasInherited {
			doc = append(doc, Flags("Inherited Flags:", usageFlags(flags, true)))
		}
	}

	if len(cmd.SubCommands) > 0 {
		doc = append(doc, Text(
			fmt.Sprintf("Use \"%s [command] --help\" for more information about a command.", commandPath(cmd)),
		))
	}

	return doc
}

// Help returns the default help text for cmd.
func Help(cmd *cli.Command) string {
	return New(cmd).String()
}

func collectHelpFlags(cmd *cli.Command) []flagInfo {
	var flags []flagInfo
	path := cmd.Path()
	if len(path) == 0 {
		path = []*cli.Command{cmd}
	}

	terminalIdx := len(path) - 1
	for i, c := range path {
		if c.Flags == nil {
			continue
		}
		isInherited := i < terminalIdx
		metaMap := flagConfigMap(c.FlagConfigs)
		c.Flags.VisitAll(func(f *flag.Flag) {
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
	return flags
}

func commandPath(cmd *cli.Command) string {
	path := cmd.Path()
	if len(path) == 0 {
		return cmd.Name
	}
	var names []string
	for _, c := range path {
		names = append(names, c.Name)
	}
	return strings.Join(names, " ")
}

func usageFlags(flags []flagInfo, inherited bool) []Flag {
	out := make([]Flag, 0, len(flags))
	for _, f := range flags {
		if f.inherited != inherited {
			continue
		}
		defval := ""
		if !f.required && !isZeroDefault(f.defval, f.typeName) {
			defval = f.defval
		}
		out = append(out, Flag{
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

func flagConfigMap(options []cli.FlagConfig) map[string]cli.FlagConfig {
	m := make(map[string]cli.FlagConfig, len(options))
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
