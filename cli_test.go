package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testState defines this command tree:
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
		FlagConfigs: []FlagConfig{
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
		FlagConfigs: []FlagConfig{
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
	t.Run("flags func setup panic returns parse error", func(t *testing.T) {
		t.Parallel()

		root := &Command{
			Name: "root",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.Bool("c", false, "capitalize the input")
				fset.Bool("c", false, "capitalize the input again")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(root, nil)
		require.Error(t, err)
		require.EqualError(t, err, `command "root": flag -c is defined more than once`)
	})
	t.Run("no flags", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"add", "item1"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		require.Equal(t, s.add, cmd)
		require.False(t, s.root.state.GetFlag[bool]("dry-run"))
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
		assert.True(t, s.root.state.GetFlag[bool]("dry-run"))
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
		assert.True(t, s.root.state.GetFlag[bool]("dry-run"))
		assert.True(t, s.root.state.GetFlag[bool]("verbose"))
	})
	t.Run("nested subcommand and root flag", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"--verbose", "nested", "sub", "--echo", "hello"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		assert.Equal(t, s.sub, cmd)
		assert.Equal(t, "hello", s.root.state.GetFlag[string]("echo"))
		assert.True(t, s.root.state.GetFlag[bool]("verbose"))
	})
	t.Run("nested subcommand with mixed flags", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"nested", "sub", "--echo", "hello", "--verbose"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		assert.Equal(t, s.sub, cmd)
		assert.Equal(t, "hello", s.root.state.GetFlag[string]("echo"))
		assert.True(t, s.root.state.GetFlag[bool]("verbose"))
	})
	t.Run("end of options delimiter", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"--verbose", "--", "nested", "sub", "--echo", "hello"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		assert.Equal(t, s.root, cmd)
		assert.Equal(t, []string{"nested", "sub", "--echo", "hello"}, s.root.state.Args)
		assert.True(t, s.root.state.GetFlag[bool]("verbose"))
	})
	t.Run("flags and args", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"add", "item1", "--dry-run", "item2"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		assert.Equal(t, s.add, cmd)
		assert.True(t, s.root.state.GetFlag[bool]("dry-run"))
		assert.Equal(t, []string{"item1", "item2"}, s.root.state.Args)
	})
	t.Run("nested subcommand with flags and args", func(t *testing.T) {
		t.Parallel()
		s := newTestState()

		err := Parse(s.root, []string{"nested", "sub", "--echo", "hello", "world"})
		require.NoError(t, err)
		cmd := getCommand(t, s.root)

		assert.Equal(t, s.sub, cmd)
		assert.Equal(t, "hello", s.root.state.GetFlag[string]("echo"))
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
		assert.True(t, s.root.state.GetFlag[bool]("force"))
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
		require.EqualError(t, err, `command "todo nested": subcommand has no name`)
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
			s := newTestState()
			err := Parse(s.root, []string{"nested", "hello", "--mandatory-flag=true", "--another-mandatory-flag", "some-value"})
			require.NoError(t, err)
			cmd := getCommand(t, s.root)

			assert.Equal(t, s.hello, cmd)
			require.True(t, s.root.state.GetFlag[bool]("mandatory-flag"))
		}
		{
			s := newTestState()
			err := Parse(s.root, []string{"nested", "hello", "--mandatory-flag=false", "--another-mandatory-flag=some-value"})
			require.NoError(t, err)
			cmd := s.root.terminal()
			assert.Equal(t, s.hello, cmd)
			require.False(t, s.root.state.GetFlag[bool]("mandatory-flag"))
		}
		{
			s := newTestState()
			err := Parse(s.root, []string{"nested", "hello", "--mandatory-flag=not-a-bool"})
			require.Error(t, err)
			require.ErrorContains(t, err, `command "todo nested hello": invalid boolean value "not-a-bool" for -mandatory-flag: parse error`)
		}
	})
	t.Run("group command missing subcommand before required flags", func(t *testing.T) {
		t.Parallel()

		restart := &Command{
			Name: "restart",
			Exec: func(ctx context.Context, s *State) error {
				return nil
			},
		}
		service := &Command{
			Name:  "service",
			Usage: "deploy service <command> [flags]",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.String("config", "", "path to config file")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "config", Required: true},
			},
			SubCommands: []*Command{restart},
		}
		root := &Command{
			Name:        "deploy",
			SubCommands: []*Command{service},
		}

		err := Parse(root, []string{"service"})
		require.Error(t, err)
		require.EqualError(t, err, "subcommand required")
		require.Equal(t, service, root.state.Cmd)

		var usageErr *usageError
		require.True(t, errors.As(err, &usageErr))
	})
	t.Run("group command required flags apply to selected child", func(t *testing.T) {
		t.Parallel()

		restart := &Command{
			Name: "restart",
			Exec: func(ctx context.Context, s *State) error {
				return nil
			},
		}
		service := &Command{
			Name: "service",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.String("config", "", "path to config file")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "config", Required: true},
			},
			SubCommands: []*Command{restart},
		}
		root := &Command{
			Name:        "deploy",
			SubCommands: []*Command{service},
		}

		err := Parse(root, []string{"service", "restart"})
		require.Error(t, err)
		require.EqualError(t, err, `command "deploy service restart": required flag "-config" not set`)
	})
	t.Run("runnable command with subcommands still checks required flags", func(t *testing.T) {
		t.Parallel()

		service := &Command{
			Name: "service",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.String("config", "", "path to config file")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "config", Required: true},
			},
			Exec: func(ctx context.Context, s *State) error {
				return nil
			},
			SubCommands: []*Command{
				{
					Name: "restart",
					Exec: func(ctx context.Context, s *State) error {
						return nil
					},
				},
			},
		}
		root := &Command{
			Name:        "deploy",
			SubCommands: []*Command{service},
		}

		err := Parse(root, []string{"service"})
		require.Error(t, err)
		require.EqualError(t, err, `command "deploy service": required flag "-config" not set`)
	})
	t.Run("unknown required flag set by cli author", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			FlagConfigs: []FlagConfig{
				{Name: "some-other-flag", Required: true},
			},
		}
		err := Parse(cmd, nil)
		require.Error(t, err)
		require.EqualError(t, err, `command "root": flag -some-other-flag is configured but not defined`)
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
		require.EqualError(t, err, `command "root sub command": invalid name: must start with a letter and contain only letters, numbers, dashes, or underscores`)
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
		require.EqualError(t, err, `command "root 1command": invalid name: must start with a letter and contain only letters, numbers, dashes, or underscores`)
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
		require.EqualError(t, err, `command "root sub@command": invalid name: must start with a letter and contain only letters, numbers, dashes, or underscores`)
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
		require.Equal(t, "", cmd.state.GetFlag[string]("config"))
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
		require.False(t, cmd.state.GetFlag[bool]("verbose"))
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
		for i := range 25 {
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
		err := Parse(cmd, []string{"duplicate"})
		require.NoError(t, err)
	})
	t.Run("flag config for non-existent flag", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.String("existing", "", "existing flag")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "existing", Required: true},
				{Name: "nonexistent", Required: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{"--existing=value"})
		require.Error(t, err)
		require.EqualError(t, err, `command "root": flag -nonexistent is configured but not defined`)
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
		for i := range 100 {
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
		assert.Equal(t, "foo", root.state.GetFlag[string]("output"))
	})
	t.Run("required flag set to default value", func(t *testing.T) {
		t.Parallel()

		root := &Command{
			Name: "mycli",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.String("port", "8080", "port number")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "port", Required: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(root, []string{"--port", "8080"})
		require.NoError(t, err, "explicitly setting required flag to its default value should not fail")
		assert.Equal(t, "8080", root.state.GetFlag[string]("port"))
	})
	t.Run("required bool flag prefix match not too broad", func(t *testing.T) {
		t.Parallel()

		root := &Command{
			Name: "mycli",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("force", false, "force operation")
				f.Bool("force-all", false, "force all")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "force", Required: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

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
		require.Equal(t, "val1", cmd.state.GetFlag[string]("flag1"))
		require.Equal(t, "val2", cmd.state.GetFlag[string]("flag2"))
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
			FlagConfigs: []FlagConfig{
				{Name: "verbose", Short: "v"},
				{Name: "output", Short: "o"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{"-v", "-o", "file.txt"})
		require.NoError(t, err)
		require.True(t, cmd.state.GetFlag[bool]("verbose"))
		require.Equal(t, "file.txt", cmd.state.GetFlag[string]("output"))
	})

	t.Run("long flag still works with short alias defined", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("verbose", false, "enable verbose output")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "verbose", Short: "v"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{"-verbose"})
		require.NoError(t, err)
		require.True(t, cmd.state.GetFlag[bool]("verbose"))
	})

	t.Run("short flag with subcommand", func(t *testing.T) {
		t.Parallel()
		child := &Command{
			Name: "child",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.String("name", "", "the name")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "name", Short: "n"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		root := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("verbose", false, "verbose")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "verbose", Short: "v"},
			},
			SubCommands: []*Command{child},
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(root, []string{"-v", "child", "-n", "hello"})
		require.NoError(t, err)
		require.True(t, root.state.GetFlag[bool]("verbose"))
		require.Equal(t, "hello", root.state.GetFlag[string]("name"))
	})

	t.Run("short and long flags are aliases sharing same value", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Int("count", 0, "number of items")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "count", Short: "c"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{"-c", "42"})
		require.NoError(t, err)
		require.Equal(t, 42, cmd.state.GetFlag[int]("count"))
	})

	t.Run("option references unknown flag", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("verbose", false, "enable verbose output")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "vrbose", Short: "v"}, // typo in Name
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{})
		require.Error(t, err)
		require.EqualError(t, err, `command "root": flag -vrbose is configured but not defined`)
	})

	t.Run("flag config name is required", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("verbose", false, "enable verbose output")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "", Required: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{})
		require.Error(t, err)
		require.EqualError(t, err, `command "root": flag config is missing a name`)
	})

	t.Run("short alias must be single ASCII letter", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("verbose", false, "enable verbose output")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "verbose", Short: "vv"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{})
		require.Error(t, err)
		require.EqualError(t, err, `command "root": flag -verbose has invalid short alias "vv"; short aliases must be one ASCII letter`)
	})

	t.Run("duplicate short alias", func(t *testing.T) {
		t.Parallel()
		cmd := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("verbose", false, "enable verbose output")
				f.Bool("version", false, "show version")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "verbose", Short: "v"},
				{Name: "version", Short: "v"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(cmd, []string{})
		require.Error(t, err)
		require.EqualError(t, err, `command "root": short flag -v is configured for both -verbose and -version`)
	})
}

func TestLocalFlags(t *testing.T) {
	t.Parallel()

	t.Run("local flag on parent not available to child", func(t *testing.T) {
		t.Parallel()
		child := &Command{
			Name: "child",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		root := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("version", false, "show version")
				f.Bool("verbose", false, "enable verbose output")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "version", Local: true},
			},
			SubCommands: []*Command{child},
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(root, []string{"child", "--version"})
		require.Error(t, err)
		require.ErrorContains(t, err, "flag provided but not defined")

		root2 := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("version", false, "show version")
				f.Bool("verbose", false, "enable verbose output")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "version", Local: true},
			},
			SubCommands: []*Command{{
				Name: "child",
				Exec: func(ctx context.Context, s *State) error { return nil },
			}},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err = Parse(root2, []string{"child", "--verbose"})
		require.NoError(t, err)
		assert.True(t, root2.state.GetFlag[bool]("verbose"))
	})

	t.Run("local flag works on defining command", func(t *testing.T) {
		t.Parallel()
		root := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("version", false, "show version")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "version", Local: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(root, []string{"--version"})
		require.NoError(t, err)
		assert.True(t, root.state.GetFlag[bool]("version"))
	})

	t.Run("local required flag only enforced on defining command", func(t *testing.T) {
		t.Parallel()
		child := &Command{
			Name: "child",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		root := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.String("token", "", "auth token")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "token", Required: true, Local: true},
			},
			SubCommands: []*Command{child},
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(root, []string{"child"})
		require.NoError(t, err)

		root2 := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.String("token", "", "auth token")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "token", Required: true, Local: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err = Parse(root2, []string{})
		require.Error(t, err)
		require.ErrorContains(t, err, "required flag")
	})

	t.Run("usage excludes local parent flags from inherited flags", func(t *testing.T) {
		t.Parallel()
		child := &Command{
			Name: "child",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("dry-run", false, "dry run mode")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		root := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("version", false, "show version")
				f.Bool("verbose", false, "enable verbose output")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "version", Local: true},
			},
			SubCommands: []*Command{child},
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(root, []string{"child", "--help"})
		require.ErrorIs(t, err, flag.ErrHelp)

		usage := help(root)
		assert.Contains(t, usage, "--verbose")
		assert.NotContains(t, usage, "--version")
		assert.Contains(t, usage, "--dry-run")
	})

	t.Run("local flag with short alias not inherited", func(t *testing.T) {
		t.Parallel()
		child := &Command{
			Name: "child",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		root := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("version", false, "show version")
			}),
			FlagConfigs: []FlagConfig{
				{Name: "version", Short: "V", Local: true},
			},
			SubCommands: []*Command{child},
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(root, []string{"child", "-V"})
		require.Error(t, err)
		require.ErrorContains(t, err, "flag provided but not defined")
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

func TestCommandPath(t *testing.T) {
	t.Parallel()

	t.Run("single command path", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "root",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, nil)
		require.NoError(t, err)

		path := cmd.Path()
		require.Len(t, path, 1)
		require.Equal(t, "root", path[0].Name)
	})

	t.Run("nested command path", func(t *testing.T) {
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
			Name:        "root",
			SubCommands: []*Command{parent},
		}

		err := Parse(root, []string{"parent", "child"})
		require.NoError(t, err)

		path := root.Path()
		require.Len(t, path, 3)
		require.Equal(t, "root", path[0].Name)
		require.Equal(t, "parent", path[1].Name)
		require.Equal(t, "child", path[2].Name)

		terminal := root.terminal()
		require.Equal(t, child, terminal)
	})

	t.Run("deeply nested command path", func(t *testing.T) {
		t.Parallel()

		level4 := &Command{
			Name: "level4",
			Exec: func(ctx context.Context, s *State) error { return nil },
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

		err := Parse(root, []string{"level1", "level2", "level3", "level4"})
		require.NoError(t, err)

		terminal := root.terminal()
		require.Equal(t, level4, terminal)

		path := root.Path()
		require.Len(t, path, 5)
		expected := []string{"root", "level1", "level2", "level3", "level4"}
		for i, cmd := range path {
			require.Equal(t, expected[i], cmd.Name)
		}
	})

	t.Run("path before parsing", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "unparsed",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		path := cmd.Path()
		require.Nil(t, path)
	})

	t.Run("path with command hierarchy not executed", func(t *testing.T) {
		t.Parallel()

		child := &Command{
			Name: "child",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		parent := &Command{
			Name:        "parent",
			SubCommands: []*Command{child},
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}
		root := &Command{
			Name:        "root",
			SubCommands: []*Command{parent},
		}

		err := Parse(root, []string{"parent"})
		require.NoError(t, err)

		terminal := root.terminal()
		require.Equal(t, parent, terminal)

		path := root.Path()
		require.Len(t, path, 2)
		require.Equal(t, "root", path[0].Name)
		require.Equal(t, "parent", path[1].Name)

		childPath := child.Path()
		require.Nil(t, childPath)
	})

	t.Run("multiple sibling commands path", func(t *testing.T) {
		t.Parallel()

		child1 := &Command{
			Name: "child1",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		child2 := &Command{
			Name: "child2",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		root := &Command{
			Name:        "root",
			SubCommands: []*Command{child1, child2},
		}

		err := Parse(root, []string{"child1"})
		require.NoError(t, err)

		terminal := root.terminal()
		require.Equal(t, child1, terminal)

		path := root.Path()
		require.Len(t, path, 2)
		require.Equal(t, "root", path[0].Name)
		require.Equal(t, "child1", path[1].Name)

		err = Parse(root, []string{"child2"})
		require.NoError(t, err)

		terminal = root.terminal()
		require.Equal(t, child2, terminal)

		path = root.Path()
		require.Len(t, path, 2)
		require.Equal(t, "root", path[0].Name)
		require.Equal(t, "child2", path[1].Name)
		require.Nil(t, child1.Path())
		require.Equal(t, path, child2.Path())
	})

	t.Run("command with complex names in path", func(t *testing.T) {
		t.Parallel()

		child := &Command{
			Name: "complex-child_name",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		parent := &Command{
			Name:        "parent-with-dashes",
			SubCommands: []*Command{child},
		}
		root := &Command{
			Name:        "root_with_underscores",
			SubCommands: []*Command{parent},
		}

		err := Parse(root, []string{"parent-with-dashes", "complex-child_name"})
		require.NoError(t, err)

		path := root.Path()
		require.Len(t, path, 3)
		expected := []string{"root_with_underscores", "parent-with-dashes", "complex-child_name"}
		for i, cmd := range path {
			require.Equal(t, expected[i], cmd.Name)
		}
	})

	t.Run("path consistency across multiple parses", func(t *testing.T) {
		t.Parallel()

		child := &Command{
			Name: "child",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		parent := &Command{
			Name:        "parent",
			SubCommands: []*Command{child},
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}
		root := &Command{
			Name:        "root",
			SubCommands: []*Command{parent},
		}

		err := Parse(root, []string{"parent"})
		require.NoError(t, err)

		path1 := root.Path()
		require.Len(t, path1, 2)
		require.Equal(t, "root", path1[0].Name)
		require.Equal(t, "parent", path1[1].Name)

		err = Parse(root, []string{"parent", "child"})
		require.NoError(t, err)

		path2 := root.Path()
		require.Len(t, path2, 3)
		require.Equal(t, "root", path2[0].Name)
		require.Equal(t, "parent", path2[1].Name)
		require.Equal(t, "child", path2[2].Name)
	})
}

func TestTerminalCommand(t *testing.T) {
	t.Parallel()

	t.Run("terminal command is root", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "root",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, nil)
		require.NoError(t, err)

		terminal := cmd.terminal()
		require.Equal(t, cmd, terminal)
	})

	t.Run("terminal command is nested", func(t *testing.T) {
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
			Name:        "root",
			SubCommands: []*Command{parent},
		}

		err := Parse(root, []string{"parent", "child"})
		require.NoError(t, err)

		terminal := root.terminal()
		require.Equal(t, child, terminal)
		require.NotEqual(t, parent, terminal)
		require.NotEqual(t, root, terminal)
	})

	t.Run("terminal command with multiple levels", func(t *testing.T) {
		t.Parallel()

		deepest := &Command{
			Name: "deepest",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		middle := &Command{
			Name:        "middle",
			SubCommands: []*Command{deepest},
		}
		root := &Command{
			Name:        "root",
			SubCommands: []*Command{middle},
		}

		err := Parse(root, []string{"middle", "deepest"})
		require.NoError(t, err)

		terminal := root.terminal()
		require.Equal(t, deepest, terminal)
	})

	t.Run("terminal command before parsing", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "unparsed",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		terminal := cmd.terminal()
		require.Equal(t, cmd, terminal)
	})

	t.Run("terminal with partial command path", func(t *testing.T) {
		t.Parallel()

		child := &Command{
			Name: "child",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		parent := &Command{
			Name:        "parent",
			SubCommands: []*Command{child},
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}
		root := &Command{
			Name:        "root",
			SubCommands: []*Command{parent},
		}

		err := Parse(root, []string{"parent"})
		require.NoError(t, err)

		terminal := root.terminal()
		require.Equal(t, parent, terminal)
		require.NotEqual(t, child, terminal)
	})
}

func TestUsageError(t *testing.T) {
	t.Parallel()

	err := UsageErrorf("missing %s", "name")
	require.EqualError(t, err, "missing name")

	var usageErr *usageError
	require.True(t, errors.As(err, &usageErr))
	require.EqualError(t, errors.Unwrap(err), "missing name")
}

func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("print version", func(t *testing.T) {
		t.Parallel()

		root := &Command{
			Name:  "printer",
			Usage: "printer [flags] [command]",
			SubCommands: []*Command{
				{
					Name:  "version",
					Usage: "show version",
					Exec: func(ctx context.Context, s *State) error {
						_, _ = s.Stdout.Write([]byte("1.0.0\n"))
						return nil
					},
				},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Parse(root, []string{"version"})
		require.NoError(t, err)

		output := bytes.NewBuffer(nil)
		require.NoError(t, err)
		err = Run(context.Background(), root, &RunOptions{Stdout: output})
		require.NoError(t, err)
		require.Equal(t, "1.0.0\n", output.String())
	})

	t.Run("parse and run", func(t *testing.T) {
		t.Parallel()
		var count int

		root := &Command{
			Name:  "count",
			Usage: "count [flags] [command]",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("dry-run", false, "dry run")
			}),
			Exec: func(ctx context.Context, s *State) error {
				if !s.GetFlag[bool]("dry-run") {
					count++
				}
				return nil
			},
		}
		err := Parse(root, nil)
		require.NoError(t, err)
		for range 3 {
			err := Run(context.Background(), root, nil)
			require.NoError(t, err)
		}
		require.Equal(t, 3, count)
		err = Parse(root, []string{"--dry-run"})
		require.NoError(t, err)
		err = Run(context.Background(), root, nil)
		require.NoError(t, err)
		require.Equal(t, 3, count)
	})
	t.Run("typo suggestion", func(t *testing.T) {
		t.Parallel()

		root := &Command{
			Name:  "count",
			Usage: "count [flags] [command]",
			SubCommands: []*Command{
				{
					Name:  "version",
					Usage: "show version",
					Exec: func(ctx context.Context, s *State) error {
						_, _ = s.Stdout.Write([]byte("1.0.0\n"))
						return nil
					},
				},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(root, []string{"verzion"})
		require.Error(t, err)
		require.Contains(t, err.Error(), `unknown command "verzion". Did you mean one of these?`)
		require.Contains(t, err.Error(), `	version`)
	})

	t.Run("command that panics during execution", func(t *testing.T) {
		t.Parallel()
		root := &Command{
			Name: "panic",
			Exec: func(ctx context.Context, s *State) error {
				panic("test panic")
			},
		}
		err := Parse(root, nil)
		require.NoError(t, err)
		err = Run(context.Background(), root, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "panic")
	})
	t.Run("run before parse", func(t *testing.T) {
		t.Parallel()
		root := &Command{
			Name: "test",
			Exec: func(ctx context.Context, s *State) error { return nil },
		}
		err := Run(context.Background(), root, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "command not parsed")
	})
	t.Run("concurrent state access", func(t *testing.T) {
		t.Parallel()
		root := &Command{
			Name: "concurrent",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.String("value", "default", "test value")
			}),
			Exec: func(ctx context.Context, s *State) error {
				go func() {
					_ = s.GetFlag[string]("value")
				}()
				return nil
			},
		}
		err := Parse(root, []string{"--value", "test"})
		require.NoError(t, err)
		err = Run(context.Background(), root, nil)
		require.NoError(t, err)
	})
	t.Run("io redirection", func(t *testing.T) {
		t.Parallel()
		root := &Command{
			Name: "io",
			Exec: func(ctx context.Context, s *State) error {
				_, err := s.Stdout.Write([]byte("stdout output\n"))
				if err != nil {
					return err
				}
				_, err = s.Stderr.Write([]byte("stderr output\n"))
				return err
			},
		}
		err := Parse(root, nil)
		require.NoError(t, err)

		stdout := bytes.NewBuffer(nil)
		stderr := bytes.NewBuffer(nil)
		err = Run(context.Background(), root, &RunOptions{
			Stdout: stdout,
			Stderr: stderr,
		})
		require.NoError(t, err)
		require.Equal(t, "stdout output\n", stdout.String())
		require.Equal(t, "stderr output\n", stderr.String())
	})
	t.Run("numeric flag boundary values", func(t *testing.T) {
		t.Parallel()
		root := &Command{
			Name: "numeric",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Int("int", 0, "integer value")
				f.Int64("int64", 0, "int64 value")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(root, []string{"--int", "2147483647"})
		require.NoError(t, err)
		require.Equal(t, 2147483647, root.state.GetFlag[int]("int"))

		err = Parse(root, []string{"--int", "-2147483648"})
		require.NoError(t, err)
		require.Equal(t, -2147483648, root.state.GetFlag[int]("int"))

		err = Parse(root, []string{"--int", "999999999"})
		require.NoError(t, err)
		require.Equal(t, 999999999, root.state.GetFlag[int]("int"))
	})
	t.Run("location file path is relative", func(t *testing.T) {
		t.Parallel()
		loc := location(0)
		parts := strings.SplitN(loc, " ", 2)
		require.Len(t, parts, 2, "location should return 'func file:line'")
		require.False(t, strings.HasPrefix(parts[1], "/"), "file path should be relative, not absolute: %s", parts[1])
	})
	t.Run("string flags with special characters", func(t *testing.T) {
		t.Parallel()
		root := &Command{
			Name: "special",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.String("text", "", "text value")
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		specialValues := []string{
			"text with spaces",
			"text\"with\"quotes",
			"text'with'apostrophes",
			"text\nwith\nnewlines",
			"text\twith\ttabs",
			"text@with#symbols$",
		}

		for _, val := range specialValues {
			err := Parse(root, []string{"--text", val})
			require.NoError(t, err)
			require.Equal(t, val, root.state.GetFlag[string]("text"))
		}
	})
}

func TestParseAndRun(t *testing.T) {
	t.Parallel()

	t.Run("runs command", func(t *testing.T) {
		t.Parallel()

		stdout := bytes.NewBuffer(nil)
		root := &Command{
			Name: "greet",
			Exec: func(ctx context.Context, s *State) error {
				_, err := fmt.Fprintln(s.Stdout, "hello")
				return err
			},
		}

		err := ParseAndRun(context.Background(), root, nil, &RunOptions{Stdout: stdout})
		require.NoError(t, err)
		require.Equal(t, "hello\n", stdout.String())
	})

	t.Run("prints help", func(t *testing.T) {
		t.Parallel()

		stdout := bytes.NewBuffer(nil)
		root := &Command{
			Name:        "greet",
			Description: "Print a greeting",
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}

		err := ParseAndRun(context.Background(), root, []string{"--help"}, &RunOptions{Stdout: stdout})
		require.NoError(t, err)
		require.Contains(t, stdout.String(), "Print a greeting")
		require.Contains(t, stdout.String(), "Usage:")
		require.Contains(t, stdout.String(), "greet")
	})

	t.Run("prints help for missing subcommand", func(t *testing.T) {
		t.Parallel()

		stderr := bytes.NewBuffer(nil)
		root := &Command{
			Name: "deploy",
			SubCommands: []*Command{
				{
					Name:        "service",
					Usage:       "deploy service <command> [flags]",
					Description: "Manage services.",
					Flags: FlagsFunc(func(f *flag.FlagSet) {
						f.String("config", "", "path to config file")
					}),
					FlagConfigs: []FlagConfig{
						{Name: "config", Required: true},
					},
					SubCommands: []*Command{
						{
							Name:    "restart",
							Summary: "Restart a service",
							Exec: func(ctx context.Context, s *State) error {
								return nil
							},
						},
					},
				},
			},
		}

		err := ParseAndRun(context.Background(), root, []string{"service"}, &RunOptions{Stderr: stderr})
		require.Error(t, err)
		require.EqualError(t, err, "subcommand required")
		require.Contains(t, stderr.String(), "Manage services.")
		require.Contains(t, stderr.String(), "deploy service <command> [flags]")
		require.Contains(t, stderr.String(), "restart    Restart a service")
		require.Contains(t, stderr.String(), "--config string    path to config file (required)")
		require.True(t, strings.HasSuffix(stderr.String(), "\n\n"))
	})
}

func TestStateGetFlag(t *testing.T) {
	t.Parallel()

	t.Run("nil state", func(t *testing.T) {
		defer func() {
			r := recover()
			require.NotNil(t, r)
			err, ok := r.(error)
			require.True(t, ok)
			assert.EqualError(t, err, "state is nil")
		}()
		var state *State
		_ = state.GetFlag[string]("version")
	})
	t.Run("flag not found", func(t *testing.T) {
		cmd := &Command{
			Name:  "root",
			Flags: flag.NewFlagSet("root", flag.ContinueOnError),
		}
		state := &State{
			path: []*Command{cmd},
		}
		defer func() {
			r := recover()
			require.NotNil(t, r)
			err, ok := r.(error)
			require.True(t, ok)
			assert.ErrorContains(t, err, `flag "-version" not found in command "root" flag set`)
		}()
		_ = state.GetFlag[string]("version")
	})
	t.Run("flag type mismatch", func(t *testing.T) {
		cmd := &Command{
			Name:  "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) { f.String("version", "1.0.0", "show version") }),
		}
		state := &State{
			path: []*Command{cmd},
		}
		defer func() {
			r := recover()
			require.NotNil(t, r)
			err, ok := r.(error)
			require.True(t, ok)
			assert.ErrorContains(t, err, `type mismatch for flag "-version" in command "root": registered string, requested int`)
		}()
		_ = state.GetFlag[int]("version")
	})
}

func TestStateGetFlagTypedName(t *testing.T) {
	t.Parallel()

	const verbose FlagName[bool] = "verbose"
	cmd := &Command{
		Name:  "root",
		Flags: FlagsFunc(func(f *flag.FlagSet) { f.Bool(string(verbose), false, "verbose output") }),
		Exec:  func(context.Context, *State) error { return nil },
	}

	require.NoError(t, Parse(cmd, []string{"--verbose"}))
	require.True(t, cmd.state.GetFlag(verbose))
}

func TestStateCommandContext(t *testing.T) {
	t.Parallel()

	t.Run("command and command path", func(t *testing.T) {
		t.Parallel()

		child := &Command{
			Name: "child",
			Exec: func(ctx context.Context, s *State) error {
				require.Equal(t, "child", s.Cmd.Name)
				require.Equal(t, []*Command{s.path[0], s.Cmd}, s.Cmd.Path())
				return nil
			},
		}
		root := &Command{
			Name:        "root",
			SubCommands: []*Command{child},
		}

		err := Parse(root, []string{"child"})
		require.NoError(t, err)
		err = Run(context.Background(), root, nil)
		require.NoError(t, err)
	})

	t.Run("usage uses terminal custom usage", func(t *testing.T) {
		t.Parallel()

		root := &Command{
			Name: "root",
			SubCommands: []*Command{
				{
					Name: "child",
					Help: func(c *Command) string {
						return "Usage:\n  root child\n\nExamples:\n  root child file.txt"
					},
					Exec: func(ctx context.Context, s *State) error {
						output := help(s.Cmd)
						require.Contains(t, output, "Examples:")
						require.Contains(t, output, "root child file.txt")
						return nil
					},
				},
			},
		}

		err := Parse(root, []string{"child"})
		require.NoError(t, err)
		err = Run(context.Background(), root, nil)
		require.NoError(t, err)
	})

	t.Run("usage error prints help and returns underlying error", func(t *testing.T) {
		t.Parallel()

		root := &Command{
			Name: "greet",
			Exec: func(ctx context.Context, s *State) error {
				return UsageErrorf("must supply a name")
			},
		}

		err := Parse(root, nil)
		require.NoError(t, err)
		stderr := new(strings.Builder)
		err = Run(context.Background(), root, &RunOptions{Stderr: stderr})
		require.Error(t, err)
		require.EqualError(t, err, "must supply a name")
		require.Contains(t, stderr.String(), "Usage:")
		require.Contains(t, stderr.String(), "greet")
		require.True(t, strings.HasSuffix(stderr.String(), "\n\n"))
	})

	t.Run("usage error prints terminal command help", func(t *testing.T) {
		t.Parallel()

		root := &Command{
			Name: "root",
			Flags: FlagsFunc(func(f *flag.FlagSet) {
				f.Bool("verbose", false, "enable verbose output")
			}),
			SubCommands: []*Command{
				{
					Name:        "child",
					Description: "Run the child command",
					Exec: func(ctx context.Context, s *State) error {
						return UsageErrorf("missing file")
					},
				},
			},
		}

		err := Parse(root, []string{"child"})
		require.NoError(t, err)
		stderr := new(strings.Builder)
		err = Run(context.Background(), root, &RunOptions{Stderr: stderr})
		require.Error(t, err)
		require.EqualError(t, err, "missing file")
		require.Contains(t, stderr.String(), "Run the child command")
		require.Contains(t, stderr.String(), "root child [flags]")
		require.Contains(t, stderr.String(), "Inherited Flags:")
		require.Contains(t, stderr.String(), "--verbose")
		require.True(t, strings.HasSuffix(stderr.String(), "\n\n"))
	})

	t.Run("normal error does not print help", func(t *testing.T) {
		t.Parallel()

		root := &Command{
			Name: "greet",
			Exec: func(ctx context.Context, s *State) error {
				return fmt.Errorf("boom")
			},
		}

		err := Parse(root, nil)
		require.NoError(t, err)
		stderr := new(strings.Builder)
		err = Run(context.Background(), root, &RunOptions{Stderr: stderr})
		require.Error(t, err)
		require.EqualError(t, err, "boom")
		require.Empty(t, stderr.String())
	})
}

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

		output := help(cmd)
		require.NotEmpty(t, output)
		require.Contains(t, output, "simple")
		require.Contains(t, output, "Usage:")
		require.Contains(t, output, "  simple")
		require.NotContains(t, output, "[flags]")
		require.NotContains(t, output, "Flags:")
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

		output := help(cmd)
		require.Contains(t, output, "withflags")
		require.Contains(t, output, "withflags [flags]")
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
				{Name: "child1", Description: "first child command", Exec: func(ctx context.Context, s *State) error { return nil }},
				{Name: "child2", Description: "second child command", Exec: func(ctx context.Context, s *State) error { return nil }},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := help(cmd)
		require.Contains(t, output, "parent")
		require.Contains(t, output, "child1")
		require.Contains(t, output, "child2")
		require.Contains(t, output, "first child command")
		require.Contains(t, output, "second child command")
		require.Contains(t, output, "Available Commands:")
		require.Contains(t, output, "parent <command>")
		require.NotContains(t, output, "[flags]")
	})

	t.Run("usage with flags and subcommands", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name:        "complex",
			Description: "complex command with flags and subcommands",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.Bool("global", false, "global flag")
			}),
			SubCommands: []*Command{
				{
					Name:        "sub",
					Description: "subcommand with its own flags",
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

		output := help(cmd)
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
			Name:        "longdesc",
			Description: longDesc,
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.String("long-flag", "", longDesc)
			}),
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := help(cmd)
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

		output := help(cmd)
		require.Contains(t, output, "globalonly")
		require.Contains(t, output, "-debug")
		require.Contains(t, output, "-output")
		require.Contains(t, output, "enable debug mode")
		require.Contains(t, output, "output file")
	})

	t.Run("usage with many subcommands", func(t *testing.T) {
		t.Parallel()

		var subcommands []*Command
		for i := range 10 {
			subcommands = append(subcommands, &Command{
				Name:        "cmd" + string(rune('0'+i)),
				Description: "command number " + string(rune('0'+i)),
				Exec:        func(ctx context.Context, s *State) error { return nil },
			})
		}

		cmd := &Command{
			Name:        "manychildren",
			SubCommands: subcommands,
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := help(cmd)
		require.Contains(t, output, "manychildren")
		for i := range 10 {
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

		output := help(cmd)
		require.Contains(t, output, "empty")
		require.NotEmpty(t, output)
	})

	t.Run("usage with nested command hierarchy", func(t *testing.T) {
		t.Parallel()

		child := &Command{
			Name:        "child",
			Description: "nested child command",
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}
		parent := &Command{
			Name:        "parent",
			Description: "parent command",
			SubCommands: []*Command{child},
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}
		root := &Command{
			Name:        "root",
			Description: "root command",
			SubCommands: []*Command{parent},
			Exec:        func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(root, []string{})
		require.NoError(t, err)

		output := help(root)
		require.Contains(t, output, "root")
		require.Contains(t, output, "root command")
		require.Contains(t, output, "parent")
		require.Contains(t, output, "parent command")
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

		output := help(cmd)
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
			FlagConfigs: []FlagConfig{
				{Name: "config", Required: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		output := help(cmd)
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

		output := help(cmd)
		require.Contains(t, output, "custom [options] <file>")
	})

	t.Run("help hook replaces default text", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name:        "custom",
			Description: "custom command",
			Help: func(c *Command) string {
				require.Equal(t, "custom", c.Name)
				return "custom help\n\nExamples:\n  custom example\n"
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := help(cmd)
		require.NotContains(t, output, "custom command")
		require.Contains(t, output, "custom help")
		require.Contains(t, output, "Examples:")
		require.Contains(t, output, "custom example")
		require.False(t, strings.HasSuffix(output, "\n"))
	})

	t.Run("help hook can return a plain string", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "custom",
			Help: func(c *Command) string {
				return "custom help"
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := help(cmd)
		require.Equal(t, "custom help", output)
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

		output := help(parent)
		require.Contains(t, output, "-local")
		require.Contains(t, output, "-global")
		require.Contains(t, output, "local flag")
		require.Contains(t, output, "global flag")
	})
}

func TestHelpSummaryAndDescription(t *testing.T) {
	t.Parallel()

	child := &Command{
		Name:    "list",
		Summary: "List tasks",
		Description: `List tasks in the current workspace.

By default, completed tasks are hidden.`,
		Exec: func(ctx context.Context, s *State) error { return nil },
	}
	root := &Command{
		Name:        "todo",
		Summary:     "Manage tasks",
		SubCommands: []*Command{child},
		Exec:        func(ctx context.Context, s *State) error { return nil },
	}

	require.NoError(t, Parse(root, nil))
	output := help(root)
	require.Contains(t, output, "list    List tasks")
	require.NotContains(t, output, "By default, completed tasks are hidden.")

	require.NoError(t, Parse(root, []string{"list"}))
	output = help(root)
	require.Contains(t, output, "List tasks in the current workspace.")
	require.Contains(t, output, "By default, completed tasks are hidden.")
}

func TestHelpDescriptionFallbacks(t *testing.T) {
	t.Parallel()

	t.Run("summary is shown in command help when description is empty", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name:    "greet",
			Summary: "Print a greeting",
			Exec:    func(ctx context.Context, s *State) error { return nil },
		}

		require.NoError(t, Parse(cmd, nil))
		require.Contains(t, help(cmd), "Print a greeting")
	})

	t.Run("description first line is shown in command lists when summary is empty", func(t *testing.T) {
		t.Parallel()

		root := &Command{
			Name: "todo",
			SubCommands: []*Command{
				{
					Name: "list",
					Description: `List tasks in the current workspace.

By default, completed tasks are hidden.`,
					Exec: func(ctx context.Context, s *State) error { return nil },
				},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		require.NoError(t, Parse(root, nil))
		output := help(root)
		require.Contains(t, output, "list    List tasks in the current workspace.")
		require.NotContains(t, output, "By default, completed tasks are hidden.")
	})
}

func TestFlagHelp(t *testing.T) {
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

		output := help(cmd)
		require.Contains(t, output, "Flags:")
		require.Contains(t, output, "-verbose")
		require.Contains(t, output, "-config string")
		require.Contains(t, output, "-workers int")
		require.Contains(t, output, "enable verbose output")
		require.Contains(t, output, "configuration file path")
		require.Contains(t, output, "number of worker threads")

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

		output := help(cmd)
		require.NotContains(t, output, "(default: false)")
		require.NotContains(t, output, "(default: 0)")
		require.NotContains(t, output, "(default: )")
		require.Contains(t, output, "-output string")
		require.Contains(t, output, "-count int")
		require.Contains(t, output, "-rate float64")
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
			FlagConfigs: []FlagConfig{
				{Name: "file", Required: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{"-file", "test.txt"})
		require.NoError(t, err)

		output := help(cmd)
		require.Contains(t, output, "(required)")
		require.NotContains(t, output, "(default: )")
		require.Contains(t, output, "(default: stdout)")
	})

	t.Run("long flag descriptions wrap", func(t *testing.T) {
		t.Parallel()

		cmd := &Command{
			Name: "test",
			Flags: FlagsFunc(func(fset *flag.FlagSet) {
				fset.String("asdf", "", "bar")
				fset.Bool("c", false, strings.Repeat("capitalize the input ", 6))
			}),
			FlagConfigs: []FlagConfig{
				{Name: "c", Required: true},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{"-c"})
		require.NoError(t, err)

		output := help(cmd)
		require.Contains(t, output, "Flags:")
		require.Contains(t, output, "  --asdf string    bar")
		require.Contains(t, output, "(required)")

		inFlags := false
		for line := range strings.SplitSeq(output, "\n") {
			if line == "Flags:" {
				inFlags = true
				continue
			}
			if inFlags && line == "" {
				break
			}
			if inFlags {
				require.LessOrEqualf(t, len(line), 80, "flag help line should wrap: %q", line)
			}
		}
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
			FlagConfigs: []FlagConfig{
				{Name: "verbose", Short: "v"},
				{Name: "output", Short: "o"},
			},
			Exec: func(ctx context.Context, s *State) error { return nil },
		}

		err := Parse(cmd, []string{})
		require.NoError(t, err)

		output := help(cmd)
		require.Contains(t, output, "-v, --verbose")
		require.Contains(t, output, "-o, --output string")
		require.Contains(t, output, "    --config string")
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

		output := help(cmd)
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

		output := help(cmd)
		require.NotContains(t, output, "Flags:")
		require.NotContains(t, output, "Inherited Flags:")
	})
}
