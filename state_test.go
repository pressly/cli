package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/pressly/cli/usage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFlag(t *testing.T) {
	t.Parallel()

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
		// Panic because author tried to access a flag that doesn't exist in any of the commands
		_ = GetFlag[string](state, "version")
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
		// Panic because author tried to access a registered flag with the wrong type
		_ = GetFlag[int](state, "version")
	})
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
					Help: func(c *Command, h usage.Help) usage.Help {
						return append(h, usage.Lines("Examples:", "root child file.txt"))
					},
					Exec: func(ctx context.Context, s *State) error {
						output := Help(s.Cmd).String()
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
					Name:      "child",
					ShortHelp: "Run the child command",
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
