// Package usage builds and renders command help.
package usage

import (
	"io"
	"strings"

	"github.com/pressly/cli"
	"github.com/pressly/cli/internal/helpdoc"
)

// Document is an ordered list of blocks that can be rendered as command help.
type Document []Block

// Block is one section in a help document.
type Block struct {
	block helpdoc.Block
}

// Item is one row in a List block.
type Item struct {
	Name    string
	Summary string
}

// Text returns an untitled paragraph block.
func Text(lines ...string) Block {
	return Block{block: helpdoc.Text(lines...)}
}

// Lines returns a titled block of indented lines.
func Lines(heading string, lines ...string) Block {
	return Block{block: helpdoc.Lines(heading, lines...)}
}

// List returns a titled list of name and summary pairs.
func List(heading string, items ...Item) Block {
	return Block{block: helpdoc.List(heading, helpItems(items)...)}
}

// String renders the document.
func (d Document) String() string {
	return d.helpdoc().String()
}

// WriteTo writes the document to w.
func (d Document) WriteTo(w io.Writer) (int64, error) {
	return d.helpdoc().WriteTo(w)
}

// New returns the default help document for cmd. Use it inside a [cli.Command.Help] hook to avoid
// calling the hook recursively.
func New(cmd *cli.Command) Document {
	cmd = resolveCommand(cmd)
	if cmd == nil {
		return nil
	}
	return fromHelpDoc(helpdoc.New(helpPath(cmd)))
}

// Help returns custom help for cmd when configured, otherwise the default help from [New].
func Help(cmd *cli.Command) string {
	cmd = resolveCommand(cmd)
	if cmd == nil {
		return ""
	}
	if cmd.Help != nil {
		return strings.TrimRight(cmd.Help(cmd), "\n")
	}
	return New(cmd).String()
}

func resolveCommand(cmd *cli.Command) *cli.Command {
	if cmd == nil {
		return nil
	}
	if path := cmd.Path(); len(path) > 0 {
		return path[len(path)-1]
	}
	return cmd
}

func fromHelpDoc(doc helpdoc.Document) Document {
	out := make(Document, 0, len(doc))
	for _, block := range doc {
		out = append(out, Block{block: block})
	}
	return out
}

func (d Document) helpdoc() helpdoc.Document {
	out := make(helpdoc.Document, 0, len(d))
	for _, block := range d {
		out = append(out, block.block)
	}
	return out
}

func helpItems(items []Item) []helpdoc.Item {
	out := make([]helpdoc.Item, 0, len(items))
	for _, item := range items {
		out = append(out, helpdoc.Item{
			Name:    item.Name,
			Summary: item.Summary,
		})
	}
	return out
}

func helpPath(cmd *cli.Command) []helpdoc.Command {
	path := cmd.Path()
	if len(path) == 0 {
		path = []*cli.Command{cmd}
	}
	out := make([]helpdoc.Command, 0, len(path))
	for _, c := range path {
		out = append(out, helpCommand(c))
	}
	return out
}

func helpCommand(cmd *cli.Command) helpdoc.Command {
	return helpdoc.Command{
		Name:        cmd.Name,
		Usage:       cmd.Usage,
		Summary:     cmd.Summary,
		Description: cmd.Description,
		Flags:       cmd.Flags,
		FlagConfigs: helpFlagConfigs(cmd.FlagConfigs),
		Subcommands: helpSubcommands(cmd.SubCommands),
	}
}

func helpSubcommands(commands []*cli.Command) []helpdoc.Command {
	out := make([]helpdoc.Command, 0, len(commands))
	for _, cmd := range commands {
		out = append(out, helpdoc.Command{
			Name:        cmd.Name,
			Summary:     cmd.Summary,
			Description: cmd.Description,
		})
	}
	return out
}

func helpFlagConfigs(configs []cli.FlagConfig) []helpdoc.FlagConfig {
	out := make([]helpdoc.FlagConfig, 0, len(configs))
	for _, cfg := range configs {
		out = append(out, helpdoc.FlagConfig{
			Name:     cfg.Name,
			Short:    cfg.Short,
			Required: cfg.Required,
			Local:    cfg.Local,
		})
	}
	return out
}
