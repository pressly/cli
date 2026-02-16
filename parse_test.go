package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testState is a helper struct to hold the commands for testing
//
//	root --verbose --version
//	├── add --dry-run
//	└── nested --force
//	   └── sub --echo
//	└── hello --mandatory-flag=false --another-mandatory-flag some-value
type testState struct {
	add                *Command
	nested, sub, hello *Command
	root               *Command
}

func newTestState() testState {
	exec := func(ctx context.Context, s *State) error { return errors.New("not implemented") }
	add := &Command{
		Name: "add",
		Flags: FlagsFunc(func(fset *flag.FlagSet) {
			fset.Bool("dry-run", false, "enable dry-run mode")
		}),
		Exec: exec,
	}
	sub := &Command{
		Name: "sub",
		Flags: FlagsFunc(func(fset *flag.FlagSet) {
			fset.String("echo", "", "echo the message")
		}),
		FlagsMetadata: []FlagMetadata{
			{Name: "echo", Required: false}, // not required
		},
		Exec: exec,
	}
	hello := &Command{
		Name: "hello",
		Flags: FlagsFunc(func(fset *flag.FlagSet) {
			fset.Bool("mandatory-flag", false, "mandatory flag")
			fset.String("another-mandatory-flag", "", "another mandatory flag")
		}),
		FlagsMetadata: []FlagMetadata{
			{Name: "mandatory-flag", Required: true},
			{Name: "another-mandatory-flag", Required: true},
		},
		Exec: exec,
	}
	nested := &Command{
		Name: "nested",
		Flags: FlagsFunc(func(fset *flag.FlagSet) {
			fset.Bool("force", false, "force the operation")
		}),
		SubCommands: []*Command{sub, hello},
		Exec:        exec,
	}
	root := &Command{
		Name: "todo",
		Flags: FlagsFunc(func(fset *flag.FlagSet) {
			fset.Bool("verbose", false, "enable verbose mode")
			fset.Bool("version", false, "show version")
		}),
		SubCommands: []*Command{add, nested},
		Exec:        exec,
	}
	return testState{
		add:    add,
		nested: nested,
		sub:    sub,
		root:   root,
		hello:  hello,
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("error on parse with no exec", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "foo",
			Exec: func(ctx context.Context, s *State) error { return nil },
			SubCommands: []*Command{
				{
					Name: "bar",
					Exec: func(ctx context.Context, s *State) error { return nil },
					SubCommands: []*Command{
						{
							Name: "baz",
						},
					},
				},
			},
		}
		err := Parse(cmd, []string{"bar", "baz"})
		require.Error(t, err)
		assert.ErrorContains(t, err, `command "foo bar baz": no exec function defined`)
	})
	t.Run("parsing errors", func(t *testing.T) {
		t.Parallel()

		err := Parse(nil, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "command is nil")

		err = Parse(&Command{}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "root command has no name")
	})
	t.Run("subcommand nil flags", func(t *testing.T) {
		t.Parallel()

		err := Parse(&Command{
			Name: "root",
			SubCommands: []*Command{{
				Name: "sub",
				Exec: func(ctx context.Context, s *State) error { return nil },
			}},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}, []string{"sub"})
		require.NoError(t, err)
	})
	t.Run("default flag usage", func(t *testing.T) {
		t.Parallel()

		by := bytes.NewBuffer(nil)
		root := &Command{
			Name:  "root",
			Usage: "root [flags]",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.SetOutput(by)
			}),
		}
		err := Parse(root, []string{"--help"})
		require.Error(t, err)
		require.ErrorIs(t, err, flag.ErrHelp)
		require.Empty(t, by.String())
	})
	t.Run("no flags", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"add", "item1"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		require.Equal(t, s.add, cmd)
		require.False(t, GetFlag[bool](s.root.state, "dry-run"))
	})
	t.Run("unknown flag", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"add", "--unknown", "item1"})
		require.Error(t, err)
		require.Contains(t, err.Error(), `command "todo add": flag provided but not defined: -unknown`)
	})
	t.Run("with subcommand flags", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"add", "--dry-run", "item1"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		assert.Equal(t, s.add, cmd)
		assert.True(t, GetFlag[bool](s.root.state, "dry-run"))
	})
	t.Run("help flag", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"--help"})
		require.Error(t, err)
		require.ErrorIs(t, err, flag.ErrHelp)
	})
	t.Run("help flag with subcommand", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"add", "--help"})
		require.Error(t, err)
		require.ErrorIs(t, err, flag.ErrHelp)
	})
	t.Run("help flag with subcommand at s.root", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"--help", "add"})
		require.Error(t, err)
		require.ErrorIs(t, err, flag.ErrHelp)
	})
	t.Run("help flag with subcommand and other flags", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"add", "--help", "--dry-run"})
		require.Error(t, err)
		require.ErrorIs(t, err, flag.ErrHelp)
	})
	t.Run("unknown subcommand", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"unknown"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown command")
	})
	t.Run("flags at multiple levels", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"add", "--dry-run", "item1", "--verbose"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		assert.Equal(t, s.add, cmd)
		assert.True(t, GetFlag[bool](s.root.state, "dry-run"))
		assert.True(t, GetFlag[bool](s.root.state, "verbose"))
	})
	t.Run("nested subcommand and root flag", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"--verbose", "nested", "sub", "--echo", "hello"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		assert.Equal(t, s.sub, cmd)
		assert.Equal(t, "hello", GetFlag[string](s.root.state, "echo"))
		assert.True(t, GetFlag[bool](s.root.state, "verbose"))
	})
	t.Run("nested subcommand with mixed flags", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"nested", "sub", "--echo", "hello", "--verbose"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		assert.Equal(t, s.sub, cmd)
		assert.Equal(t, "hello", GetFlag[string](s.root.state, "echo"))
		assert.True(t, GetFlag[bool](s.root.state, "verbose"))
	})
	t.Run("end of options delimiter", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"--verbose", "--", "nested", "sub", "--echo", "hello"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		assert.Equal(t, s.root, cmd)
		assert.Equal(t, []string{"nested", "sub", "--echo", "hello"}, s.root.state.Args)
		assert.True(t, GetFlag[bool](s.root.state, "verbose"))
	})
	t.Run("flags and args", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"add", "item1", "--dry-run", "item2"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		assert.Equal(t, s.add, cmd)
		assert.True(t, GetFlag[bool](s.root.state, "dry-run"))
		assert.Equal(t, []string{"item1", "item2"}, s.root.state.Args)
	})
	t.Run("nested subcommand with flags and args", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"nested", "sub", "--echo", "hello", "world"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		assert.Equal(t, s.sub, cmd)
		assert.Equal(t, "hello", GetFlag[string](s.root.state, "echo"))
		assert.Equal(t, []string{"world"}, s.root.state.Args)
	})
	t.Run("subcommand flags not available in parent", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"--dry-run"})
		require.Error(t, err)
		require.ErrorContains(t, err, "flag provided but not defined")
	})
	t.Run("parent flags inherited in subcommand", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"nested", "sub", "--force"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		assert.Equal(t, s.sub, cmd)
		assert.True(t, GetFlag[bool](s.root.state, "force"))
	})
	t.Run("unrelated subcommand flags not inherited in other subcommands", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"nested", "sub", "--dry-run"})
		require.Error(t, err)
		require.ErrorContains(t, err, "flag provided but not defined")
	})
	t.Run("empty name in subcommand", func(t *testing.T) {
		t.Parallel()
		s := newTestState()
		s.sub.Name = ""

		err := Parse(s.root, nil)
		require.Error(t, err)
		require.ErrorContains(t, err, `subcommand in path [todo, nested] has no name`)
	})
	t.Run("required flag", func(t *testing.T) {
		t.Parallel()
		{
			s := newTestState()
			err := Parse(s.root, []string{"nested", "hello"})
			require.Error(t, err)
			require.ErrorContains(t, err, `command "todo nested hello": required flags "-mandatory-flag, -another-mandatory-flag" not set`)
		}
		{
			// Correct type - true
			s := newTestState()
			err := Parse(s.root, []string{"nested", "hello", "--mandatory-flag=true", "--another-mandatory-flag", "some-value"})
			require.NoError(t, err)
			cmd := getCommand(t, s.root)

			assert.Equal(t, s.hello, cmd)
			require.True(t, GetFlag[bool](s.root.state, "mandatory-flag"))
		}
		{
			// Correct type - false
			s := newTestState()
			err := Parse(s.root, []string{"nested", "hello", "--mandatory-flag=false", "--another-mandatory-flag=some-value"})
			require.NoError(t, err)
			cmd := s.root.terminal()
			assert.Equal(t, s.hello, cmd)
			require.False(t, GetFlag[bool](s.root.state, "mandatory-flag"))
		}
		{
			// Incorrect type
			s := newTestState()
			err := Parse(s.root, []string{"nested", "hello", "--mandatory-flag=not-a-bool"})
			require.Error(t, err)
			require.ErrorContains(t, err, `command "todo nested hello": invalid boolean value "not-a-bool" for -mandatory-flag: parse error`)
		}
	})
	t.Run("unknown required flag set by cli author", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			FlagsMetadata: []FlagMetadata{
				{Name: "some-other-flag", Required: true},
			},
		}
		err := Parse(cmd, nil)
		require.Error(t, err)
		require.ErrorContains(t, err, `flag metadata references unknown flag "some-other-flag"`)
	})
	t.Run("space in command name", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			SubCommands: []*Command{
				{Name: "sub command"},
			},
		}
		err := Parse(cmd, nil)
		require.Error(t, err)
		require.ErrorContains(t, err, `failed to parse: command ["root", "sub command"]: name must start with a letter and contain only letters, numbers, dashes (-) or underscores (_)`)
	})
	t.Run("dash in command name", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Exec: func(ctx context.Context, s *State) error { return nil },
			SubCommands: []*Command{
				{Name: "sub-command"},
			},
		}
		err := Parse(cmd, nil)
		require.NoError(t, err)
	})
	t.Run("underscore in command name", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Exec: func(ctx context.Context, s *State) error { return nil },
			SubCommands: []*Command{
				{Name: "sub_command", Exec: func(ctx context.Context, s *State) error { return nil }},
			},
		}
		err := Parse(cmd, []string{"sub_command"})
		require.NoError(t, err)
	})
	t.Run("command name starting with number", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			SubCommands: []*Command{
				{Name: "1command"},
			},
		}
		err := Parse(cmd, nil)
		require.Error(t, err)
		require.ErrorContains(t, err, `name must start with a letter`)
	})
	t.Run("command name with special characters", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			SubCommands: []*Command{
				{Name: "sub@command"},
			},
		}
		err := Parse(cmd, nil)
		require.Error(t, err)
		require.ErrorContains(t, err, `name must start with a letter and contain only letters, numbers, dashes (-) or underscores (_)`)
	})
	t.Run("very long command name", func(t *testing.T) {
		t.Parallel()
		longName := "very-long-command-name-that-exceeds-normal-expectations-and-continues-for-a-while-to-test-edge-cases"
		cmd := &Command{
			Name: "root",
			Exec: func(ctx context.Context, s *State) error { return nil },
			SubCommands: []*Command{
				{Name: longName, Exec: func(ctx context.Context, s *State) error { return nil }},
			},
		}
		err := Parse(cmd, []string{longName})
		require.NoError(t, err)
	})
	t.Run("empty args list", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{})
		require.NoError(t, err)
		require.Len(t, cmd.state.Args, 0)
	})
	t.Run("args with whitespace only", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{"   ", "\t", ""})
		require.NoError(t, err)
		require.Equal(t, []string{"   ", "\t", ""}, cmd.state.Args)
	})
	t.Run("flag with empty value", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.String("config", "", "config file")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{"--config="})
		require.NoError(t, err)
		require.Equal(t, "", GetFlag[string](cmd.state, "config"))
	})
	t.Run("boolean flag with explicit false", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.Bool("verbose", true, "verbose mode")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{"--verbose=false"})
		require.NoError(t, err)
		require.False(t, GetFlag[bool](cmd.state, "verbose"))
	})
	t.Run("deeply nested command hierarchy", func(t *testing.T) {
		t.Parallel()
		level5 := &Command{
			Name: "level5",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		level4 := &Command{
			Name:        "level4",
			SubCommands: []*Command{level5},
		}
		level3 := &Command{
			Name:        "level3",
			SubCommands: []*Command{level4},
		}
		level2 := &Command{
			Name:        "level2",
			SubCommands: []*Command{level3},
		}
		level1 := &Command{
			Name:        "level1",
			SubCommands: []*Command{level2},
		}
		root := &Command{
			Name:        "root",
			SubCommands: []*Command{level1},
		}
		err := Parse(root, []string{"level1", "level2", "level3", "level4", "level5"})
		require.NoError(t, err)
		terminal := root.terminal()
		require.Equal(t, level5, terminal)
	})
	t.Run("many subcommands", func(t *testing.T) {
		t.Parallel()
		var subcommands []*Command
		for i := 0; i < 25; i++ {
			subcommands = append(subcommands, &Command{
				Name: "cmd" + string(rune('a'+i%26)),
				Exec: func(ctx context.Context, s *State) error { return nil },
			})
		}
		root := &Command{
			Name:        "root",
			SubCommands: subcommands,
		}
		err := Parse(root, []string{"cmda"})
		require.NoError(t, err)
		terminal := root.terminal()
		require.Equal(t, "cmda", terminal.Name)
	})
	t.Run("duplicate subcommand names", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			SubCommands: []*Command{
				{Name: "duplicate", Exec: func(ctx context.Context, s *State) error { return nil }},
				{Name: "duplicate", Exec: func(ctx context.Context, s *State) error { return nil }},
			},
		}
		// This library may not check for duplicate names, so just verify it works
		err := Parse(cmd, []string{"duplicate"})
		require.NoError(t, err)
		// Just ensure it doesn't crash and can parse the first match
	})
	t.Run("flag metadata for non-existent flag", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.String("existing", "", "existing flag")
			}),
			FlagsMetadata: []FlagMetadata{
				{Name: "existing", Required: true},
				{Name: "nonexistent", Required: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{"--existing=value"})
		require.Error(t, err)
		require.ErrorContains(t, err, `flag metadata references unknown flag "nonexistent"`)
	})
	t.Run("args with special characters", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		specialArgs := []string{"file with spaces.txt", "file@symbol.txt", "file\"quote.txt", "file'apostrophe.txt"}
		err := Parse(cmd, specialArgs)
		require.NoError(t, err)
		require.Equal(t, specialArgs, cmd.state.Args)
	})
	t.Run("very long argument list", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		var longArgList []string
		for i := 0; i < 100; i++ {
			longArgList = append(longArgList, "arg"+string(rune('0'+i%10)))
		}
		err := Parse(cmd, longArgList)
		require.NoError(t, err)
		require.Equal(t, longArgList, cmd.state.Args)
	})
	t.Run("positional arg matching command name", func(t *testing.T) {
		t.Parallel()

		add := &Command{
			Name: "add",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		root := &Command{
			Name:        "mycli",
			SubCommands: []*Command{add},
		}
		err := Parse(root, []string{"add", "add"})
		require.NoError(t, err)
		assert.Equal(t, add, getCommand(t, root))
		// The second "add" is a positional arg, not a command traversal.
		assert.Equal(t, []string{"add"}, root.state.Args)
	})
	t.Run("ancestor flag value not treated as command", func(t *testing.T) {
		t.Parallel()

		child := &Command{
			Name: "child",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		parent := &Command{
			Name:        "parent",
			SubCommands: []*Command{child},
		}
		root := &Command{
			Name: "mycli",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.String("output", "", "output file")
			}),
			SubCommands: []*Command{parent},
		}

		// Root flag --output used between parent and child: the value "foo" should be skipped
		// during command resolution, not treated as an unknown command.
		err := Parse(root, []string{"parent", "--output", "foo", "child"})
		require.NoError(t, err, "ancestor flag value should not be treated as unknown command")
		assert.Equal(t, child, getCommand(t, root))
		assert.Equal(t, "foo", GetFlag[string](root.state, "output"))
	})
	t.Run("required flag set to default value", func(t *testing.T) {
		t.Parallel()

		root := &Command{
			Name: "mycli",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.String("port", "8080", "port number")
			}),
			FlagsMetadata: []FlagMetadata{
				{Name: "port", Required: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		// Explicitly passing the default value should satisfy the required check.
		err := Parse(root, []string{"--port", "8080"})
		require.NoError(t, err, "explicitly setting required flag to its default value should not fail")
		assert.Equal(t, "8080", GetFlag[string](root.state, "port"))
	})
	t.Run("required bool flag prefix match not too broad", func(t *testing.T) {
		t.Parallel()

		root := &Command{
			Name: "mycli",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("force", false, "force operation")
				f.Bool("force-all", false, "force all")
			}),
			FlagsMetadata: []FlagMetadata{
				{Name: "force", Required: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		// --force-all should NOT satisfy the required --force flag.
		err := Parse(root, []string{"--force-all"})
		require.Error(t, err, "--force-all should not satisfy required --force")
		assert.Contains(t, err.Error(), "required flag")
	})
	t.Run("mixed flags and args in various orders", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.String("flag1", "", "first flag")
				fset.String("flag2", "", "second flag")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{"arg1", "--flag1=val1", "arg2", "--flag2", "val2", "arg3"})
		require.NoError(t, err)
		require.Equal(t, "val1", GetFlag[string](cmd.state, "flag1"))
		require.Equal(t, "val2", GetFlag[string](cmd.state, "flag2"))
		require.Equal(t, []string{"arg1", "arg2", "arg3"}, cmd.state.Args)
	})
}

func TestShortFlags(t *testing.T) {
	t.Parallel()

	t.Run("short flag sets value", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("verbose", false, "enable verbose output")
				f.String("output", "", "output file")
			}),
			FlagsMetadata: []FlagMetadata{
				{Name: "verbose", Short: "v"},
				{Name: "output", Short: "o"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{"-v", "-o", "file.txt"})
		require.NoError(t, err)
		require.True(t, GetFlag[bool](cmd.state, "verbose"))
		require.Equal(t, "file.txt", GetFlag[string](cmd.state, "output"))
	})

	t.Run("long flag still works with short alias defined", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("verbose", false, "enable verbose output")
			}),
			FlagsMetadata: []FlagMetadata{
				{Name: "verbose", Short: "v"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{"-verbose"})
		require.NoError(t, err)
		require.True(t, GetFlag[bool](cmd.state, "verbose"))
	})

	t.Run("short flag with subcommand", func(t *testing.T) {
		t.Parallel()
		child := &Command{
			Name: "child",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.String("name", "", "the name")
			}),
			FlagsMetadata: []FlagMetadata{
				{Name: "name", Short: "n"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		root := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("verbose", false, "verbose")
			}),
			FlagsMetadata: []FlagMetadata{
				{Name: "verbose", Short: "v"},
			},
			SubCommands: []*Command{child},
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(root, []string{"-v", "child", "-n", "hello"})
		require.NoError(t, err)
		require.True(t, GetFlag[bool](root.state, "verbose"))
		require.Equal(t, "hello", GetFlag[string](root.state, "name"))
	})

	t.Run("short and long flags are aliases sharing same value", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Int("count", 0, "number of items")
			}),
			FlagsMetadata: []FlagMetadata{
				{Name: "count", Short: "c"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		// Use short flag
		err := Parse(cmd, []string{"-c", "42"})
		require.NoError(t, err)
		// Both short and long name should return the same value
		require.Equal(t, 42, GetFlag[int](cmd.state, "count"))
	})

	t.Run("metadata references unknown flag", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("verbose", false, "enable verbose output")
			}),
			FlagsMetadata: []FlagMetadata{
				{Name: "vrbose", Short: "v"}, // typo in Name
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{})
		require.Error(t, err)
		require.Contains(t, err.Error(), `flag metadata references unknown flag "vrbose"`)
	})

	t.Run("short alias must be single ASCII letter", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("verbose", false, "enable verbose output")
			}),
			FlagsMetadata: []FlagMetadata{
				{Name: "verbose", Short: "vv"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "short alias must be a single ASCII letter")
	})

	t.Run("duplicate short alias", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("verbose", false, "enable verbose output")
				f.Bool("version", false, "show version")
			}),
			FlagsMetadata: []FlagMetadata{
				{Name: "verbose", Short: "v"},
				{Name: "version", Short: "v"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{})
		require.Error(t, err)
		require.Contains(t, err.Error(), `duplicate short flag "v"`)
	})
}

func getCommand(t *testing.T, c *Command) *Command {
	require.NotNil(t, c)
	require.NotNil(t, c.state)
	require.NotEmpty(t, c.state.path)
	terminal := c.terminal()
	require.NotNil(t, terminal)
	return terminal
}
