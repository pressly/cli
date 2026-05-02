package usage

import (
	"bytes"
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
		Name:        "greet",
		Description: "print a greeting",
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
	var stdout bytes.Buffer
	err := cli.ParseAndRun(context.Background(), root, []string{"--help"}, &cli.RunOptions{
		Stdout: &stdout,
	})
	require.NoError(t, err)
	require.Equal(t, output, strings.TrimRight(stdout.String(), "\n"))
	require.Contains(t, output, "print a greeting")
	require.Contains(t, output, "Usage:")
	require.Contains(t, output, "greet [flags]")
	require.Contains(t, output, "-v, --verbose")
	require.Contains(t, output, "--format string")
}

func TestCommandHelpUsesResolvedCommand(t *testing.T) {
	t.Parallel()

	child := &cli.Command{
		Name:        "child",
		Description: "run the child command",
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
	var stdout bytes.Buffer
	err := cli.ParseAndRun(context.Background(), root, []string{"child", "--help"}, &cli.RunOptions{
		Stdout: &stdout,
	})
	require.NoError(t, err)
	require.Equal(t, output, strings.TrimRight(stdout.String(), "\n"))
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
		Name:        "greet",
		Description: "print a greeting",
		Help: func(c *cli.Command) string {
			doc := New(c)
			doc = append(doc, Lines("Examples:", "greet margo"))
			return doc.String()
		},
		Exec: func(ctx context.Context, s *cli.State) error { return nil },
	}
	var stdout bytes.Buffer
	err := cli.ParseAndRun(context.Background(), root, []string{"--help"}, &cli.RunOptions{
		Stdout: &stdout,
	})
	require.NoError(t, err)

	output := stdout.String()
	require.Equal(t, strings.TrimRight(output, "\n"), Help(root))
	require.Contains(t, output, "print a greeting")
	require.Contains(t, output, "Examples:")
	require.Contains(t, output, "greet margo")
}

func TestCommandHelpUsesCustomHook(t *testing.T) {
	t.Parallel()

	root := &cli.Command{
		Name: "greet",
		Help: func(c *cli.Command) string {
			require.Equal(t, "greet", c.Name)
			return "custom help\n"
		},
		Exec: func(ctx context.Context, s *cli.State) error { return nil },
	}
	require.NoError(t, cli.Parse(root, nil))

	require.Equal(t, "custom help", Help(root))
}

func TestCommandHelpUsesResolvedCustomHook(t *testing.T) {
	t.Parallel()

	child := &cli.Command{
		Name: "child",
		Help: func(c *cli.Command) string {
			require.Equal(t, "child", c.Name)
			return "child help"
		},
		Exec: func(ctx context.Context, s *cli.State) error { return nil },
	}
	root := &cli.Command{
		Name:        "root",
		SubCommands: []*cli.Command{child},
		Exec:        func(ctx context.Context, s *cli.State) error { return nil },
	}
	require.NoError(t, cli.Parse(root, []string{"child"}))

	require.Equal(t, "child help", Help(root))
}
