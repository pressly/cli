package usage

import (
	"context"
	"flag"
	"strings"
	"testing"

	"github.com/pressly/cli"
	"github.com/stretchr/testify/require"
)

func TestHelpString(t *testing.T) {
	t.Parallel()

	h := Document{
		Text("print a greeting"),
		Lines("Usage:", "greet [flags] <name>"),
		Flags("Flags:", []Flag{
			{Name: "verbose", Short: "v", Usage: "enable verbose output"},
			{Name: "output", Placeholder: "string", Usage: "output file", Required: true},
		}),
		Commands("Available Commands:", []Command{
			{Name: "hello", Summary: "print hello"},
		}),
	}

	output := h.String()
	require.Contains(t, output, "print a greeting")
	require.Contains(t, output, "Usage:")
	require.Contains(t, output, "greet [flags] <name>")
	require.Contains(t, output, "-v, --verbose")
	require.Contains(t, output, "--output string")
	require.Contains(t, output, "output file (required)")
	require.Contains(t, output, "Available Commands:")
	require.Contains(t, output, "hello")
	require.False(t, strings.HasSuffix(output, "\n"))
}

func TestBlockString(t *testing.T) {
	t.Parallel()

	output := Lines("Examples:", "greet margo").String()
	require.Equal(t, "Examples:\n  greet margo", output)
}

func TestListWithoutSummary(t *testing.T) {
	t.Parallel()

	output := List("Commands:", Item{Name: "serve"}).String()
	require.Equal(t, "Commands:\n  serve", output)
}

func TestCommandHelp(t *testing.T) {
	t.Parallel()

	root := &cli.Command{
		Name:      "greet",
		ShortHelp: "print a greeting",
		Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
			f.Bool("verbose", false, "enable verbose output")
			f.String("format", "plain", "output format")
		}),
		FlagConfigs: []cli.FlagConfig{
			{Name: "verbose", Short: "v"},
		},
		Exec: func(ctx context.Context, s *cli.State) error { return nil },
	}
	require.NoError(t, cli.Parse(root, nil))

	output := Help(root)
	require.Equal(t, cli.Help(root), output)
	require.Contains(t, output, "print a greeting")
	require.Contains(t, output, "Usage:")
	require.Contains(t, output, "greet [flags]")
	require.Contains(t, output, "-v, --verbose")
	require.Contains(t, output, "--format string")
}

func TestCommandHelpUsesResolvedCommand(t *testing.T) {
	t.Parallel()

	child := &cli.Command{
		Name:      "child",
		ShortHelp: "run the child command",
		Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
			f.String("file", "", "input file")
		}),
		Exec: func(ctx context.Context, s *cli.State) error { return nil },
	}
	root := &cli.Command{
		Name: "root",
		Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
			f.Bool("verbose", false, "enable verbose output")
		}),
		SubCommands: []*cli.Command{child},
		Exec:        func(ctx context.Context, s *cli.State) error { return nil },
	}
	require.NoError(t, cli.Parse(root, []string{"child"}))

	output := Help(root)
	require.Equal(t, cli.Help(root), output)
	require.Contains(t, output, "run the child command")
	require.Contains(t, output, "root child [flags]")
	require.Contains(t, output, "Flags:")
	require.Contains(t, output, "--file string")
	require.Contains(t, output, "Inherited Flags:")
	require.Contains(t, output, "--verbose")
}

func TestCommandDocumentComposition(t *testing.T) {
	t.Parallel()

	root := &cli.Command{
		Name:      "greet",
		ShortHelp: "print a greeting",
		Help: func(c *cli.Command) string {
			doc := New(c)
			doc = append(doc, Lines("Examples:", "greet margo"))
			return doc.String()
		},
		Exec: func(ctx context.Context, s *cli.State) error { return nil },
	}
	require.NoError(t, cli.Parse(root, nil))

	output := cli.Help(root)
	require.Contains(t, output, "print a greeting")
	require.Contains(t, output, "Examples:")
	require.Contains(t, output, "greet margo")
}
