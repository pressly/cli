package cli

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageGeneration(t *testing.T) {
	t.Parallel()

	t.Run("default usage with no flags", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "simple",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		require.NotEmpty(t, output)
		require.Contains(t, output, "simple")
		require.Contains(t, output, "Usage:")
	})

	t.Run("usage with flags", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "withflags",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.Bool("verbose", false, "enable verbose mode")
				fset.String("config", "", "config file path")
				fset.Int("count", 1, "number of items")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		require.Contains(t, output, "withflags")
		require.Contains(t, output, "-verbose")
		require.Contains(t, output, "-config")
		require.Contains(t, output, "-count")
		require.Contains(t, output, "enable verbose mode")
		require.Contains(t, output, "config file path")
		require.Contains(t, output, "number of items")
	})

	t.Run("usage with subcommands", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "parent",
			SubCommands: []*Command{
				{Name: "child1", ShortHelp: "first child command", Exec: func(ctx context.Context, s *State) error { return nil }},
				{Name: "child2", ShortHelp: "second child command", Exec: func(ctx context.Context, s *State) error { return nil }},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		require.Contains(t, output, "parent")
		require.Contains(t, output, "child1")
		require.Contains(t, output, "child2")
		require.Contains(t, output, "first child command")
		require.Contains(t, output, "second child command")
		require.Contains(t, output, "Available Commands:")
	})

	t.Run("usage with flags and subcommands", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name:      "complex",
			ShortHelp: "complex command with flags and subcommands",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.Bool("global", false, "global flag")
			}),
			SubCommands: []*Command{
				{
					Name:      "sub",
					ShortHelp: "subcommand with its own flags",
					Flags: FlagsFunc(func(fset *flag.FlagSet) {
						fset.String("local", "", "local flag")
					}),
					Exec: func(ctx context.Context, s *State) error { return nil },
				},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		require.Contains(t, output, "complex")
		require.Contains(t, output, "complex command with flags and subcommands")
		require.Contains(t, output, "-global")
		require.Contains(t, output, "global flag")
		require.Contains(t, output, "sub")
		require.Contains(t, output, "subcommand with its own flags")
	})

	t.Run("usage with very long descriptions", func(t *testing.T) {
		t.Parallel()

		longDesc := "This is a very long description that should be wrapped properly when displayed in the usage output to ensure readability and proper formatting"
		cmd := &Command{
			Name:      "longdesc",
			ShortHelp: longDesc,
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.String("long-flag", "", longDesc)
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		require.Contains(t, output, "longdesc")
		require.Contains(t, output, "very long description")
		require.Contains(t, output, "-long-flag")
	})

	t.Run("usage with no subcommands but global flags", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "globalonly",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.Bool("debug", false, "enable debug mode")
				fset.String("output", "", "output file")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		require.Contains(t, output, "globalonly")
		require.Contains(t, output, "-debug")
		require.Contains(t, output, "-output")
		require.Contains(t, output, "enable debug mode")
		require.Contains(t, output, "output file")
	})

	t.Run("usage with many subcommands", func(t *testing.T) {
		t.Parallel()

		var subcommands []*Command
		for i := 0; i < 10; i++ {
			subcommands = append(subcommands, &Command{
				Name:      "cmd" + string(rune('0'+i)),
				ShortHelp: "command number " + string(rune('0'+i)),
				Exec:      func(ctx context.Context, s *State) error { return nil },
			})
		}

		cmd := &Command{
			Name:        "manychildren",
			SubCommands: subcommands,
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		require.Contains(t, output, "manychildren")
		for i := 0; i < 10; i++ {
			require.Contains(t, output, "cmd"+string(rune('0'+i)))
			require.Contains(t, output, "command number "+string(rune('0'+i)))
		}
	})

	t.Run("usage with empty command structure", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "empty",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		require.Contains(t, output, "empty")
		require.NotEmpty(t, output)
	})

	t.Run("usage with nested command hierarchy", func(t *testing.T) {
		t.Parallel()

		child := &Command{
			Name:      "child",
			ShortHelp: "nested child command",
			Exec:      func(ctx context.Context, s *State) error { return nil },
		}
		parent := &Command{
			Name:        "parent",
			ShortHelp:   "parent command",
			SubCommands: []*Command{child},
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}
		root := &Command{
			Name:        "root",
			ShortHelp:   "root command",
			SubCommands: []*Command{parent},
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(root, []string{})
		require.NoError(t, err)

		output := DefaultUsage(root)
		require.Contains(t, output, "root")
		require.Contains(t, output, "root command")
		require.Contains(t, output, "parent")
		require.Contains(t, output, "parent command")
		// Child should not appear in root's usage
		require.NotContains(t, output, "child")
		require.NotContains(t, output, "nested child command")
	})

	t.Run("usage with mixed flag types", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "mixed",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.Bool("bool-flag", false, "boolean flag")
				fset.String("string-flag", "default", "string flag")
				fset.Int("int-flag", 0, "integer flag")
				fset.Float64("float-flag", 0.0, "float flag")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		require.Contains(t, output, "-bool-flag")
		require.Contains(t, output, "-string-flag")
		require.Contains(t, output, "-int-flag")
		require.Contains(t, output, "-float-flag")

		require.Contains(t, output, "boolean flag")
		require.Contains(t, output, "string flag")
		require.Contains(t, output, "integer flag")
		require.Contains(t, output, "float flag")
	})

	t.Run("usage before parsing shows flags", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "unparsed",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.Bool("debug", false, "enable debug mode")
				fset.String("config", "", "config file path")
			}),
			FlagOptions: []FlagOption{
				{Name: "config", Required: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		// Usage should work even before parsing and show flags
		output := DefaultUsage(cmd)
		require.NotEmpty(t, output)
		require.Contains(t, output, "Flags:")
		require.Contains(t, output, "-debug")
		require.Contains(t, output, "-config string")
		require.Contains(t, output, "(required)")
	})

	t.Run("usage with custom usage string", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name:  "custom",
			Usage: "custom [options] <file>",
			Exec:  func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		require.Contains(t, output, "custom [options] <file>")
	})

	t.Run("usage with inherited and local flags", func(t *testing.T) {
		t.Parallel()

		child := &Command{
			Name: "child",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.String("local", "", "local flag")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		parent := &Command{
			Name: "parent",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.Bool("global", false, "global flag")
			}),
			SubCommands: []*Command{child},
		}

		err := Parse(parent, []string{"child"})
		require.NoError(t, err)

		output := DefaultUsage(parent)
		require.Contains(t, output, "-local")
		require.Contains(t, output, "-global")
		require.Contains(t, output, "local flag")
		require.Contains(t, output, "global flag")
	})
}

func TestWriteFlagSection(t *testing.T) {
	t.Parallel()

	t.Run("non-zero defaults shown and type hints", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "test",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.Bool("verbose", false, "enable verbose output")
				fset.String("config", "/etc/config", "configuration file path")
				fset.Int("workers", 4, "number of worker threads")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		require.Contains(t, output, "Flags:")
		require.Contains(t, output, "-verbose")
		require.Contains(t, output, "-config string")
		require.Contains(t, output, "-workers int")
		require.Contains(t, output, "enable verbose output")
		require.Contains(t, output, "configuration file path")
		require.Contains(t, output, "number of worker threads")

		// Non-zero defaults are shown
		require.Contains(t, output, "(default: /etc/config)")
		require.Contains(t, output, "(default: 4)")
	})

	t.Run("zero-value defaults suppressed", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "test",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.Bool("verbose", false, "enable verbose output")
				fset.String("output", "", "output file")
				fset.Int("count", 0, "number of items")
				fset.Float64("rate", 0.0, "rate limit")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		// Zero-value defaults should not appear
		require.NotContains(t, output, "(default: false)")
		require.NotContains(t, output, "(default: 0)")
		require.NotContains(t, output, "(default: )")
		// But non-bool flags should still have type hints
		require.Contains(t, output, "-output string")
		require.Contains(t, output, "-count int")
		require.Contains(t, output, "-rate float64")
		// Bool flags should NOT have a type hint
		require.NotContains(t, output, "-verbose bool")
	})

	t.Run("required flags marked", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "test",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.String("file", "", "path to file")
				fset.String("output", "stdout", "output destination")
			}),
			FlagOptions: []FlagOption{
				{Name: "file", Required: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{"-file", "test.txt"})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		require.Contains(t, output, "(required)")
		// Required flag should not also show a default
		require.NotContains(t, output, "(default: )")
		// Non-required flag with non-zero default should show default
		require.Contains(t, output, "(default: stdout)")
	})

	t.Run("short flags displayed", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "test",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.Bool("verbose", false, "enable verbose output")
				fset.String("output", "", "output file")
				fset.String("config", "", "config file path")
			}),
			FlagOptions: []FlagOption{
				{Name: "verbose", Short: "v"},
				{Name: "output", Short: "o"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		// Flags with short aliases show both forms
		require.Contains(t, output, "-v, --verbose")
		require.Contains(t, output, "-o, --output string")
		// Flags without short aliases are padded to align with double-dash
		require.Contains(t, output, "     --config string")
	})

	t.Run("no short flags means no padding", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "test",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.Bool("verbose", false, "enable verbose output")
				fset.String("config", "", "config file path")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		// Without any short flags, no extra padding should be added
		require.Contains(t, output, "  --verbose")
		require.Contains(t, output, "  --config string")
		require.NotContains(t, output, "     --verbose")
		require.NotContains(t, output, "     --config")
	})

	t.Run("no flags section when no flags", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "noflag",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := DefaultUsage(cmd)
		require.NotContains(t, output, "Flags:")
		require.NotContains(t, output, "Inherited Flags:")
	})
}
