package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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
				if !GetFlag[bool](s, "dry-run") {
					count++
				}
				return nil
			},
		}
		err := Parse(root, nil)
		require.NoError(t, err)
		// Run the command 3 times
		for i := 0; i < 3; i++ {
			err := Run(context.Background(), root, nil)
			require.NoError(t, err)
		}
		require.Equal(t, 3, count)
		// Run with dry-run flag
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
	t.Run("run with nil context", func(t *testing.T) {
		t.Parallel()
		root := &Command{
			Name: "test",
			Exec: func(ctx context.Context, s *State) error {
				if ctx == nil {
					return errors.New("context is nil")
				}
				return nil
			},
		}
		err := Parse(root, nil)
		require.NoError(t, err)
		err = Run(nil, root, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "context is nil")
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
				// Simulate concurrent access to state
				go func() {
					_ = GetFlag[string](s, "value")
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

		// Test max int
		err := Parse(root, []string{"--int", "2147483647"})
		require.NoError(t, err)
		require.Equal(t, 2147483647, GetFlag[int](root.state, "int"))

		// Test min int
		err = Parse(root, []string{"--int", "-2147483648"})
		require.NoError(t, err)
		require.Equal(t, -2147483648, GetFlag[int](root.state, "int"))

		// Test that parsing still works with large values (may not overflow in Go flag package)
		err = Parse(root, []string{"--int", "999999999"})
		require.NoError(t, err)
		require.Equal(t, 999999999, GetFlag[int](root.state, "int"))
	})
	t.Run("location file path is relative", func(t *testing.T) {
		t.Parallel()
		loc := location(0)
		// location returns "funcName file:line"
		parts := strings.SplitN(loc, " ", 2)
		require.Len(t, parts, 2, "location should return 'func file:line'")
		// File path should be relative, not an absolute path
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
			require.Equal(t, val, GetFlag[string](root.state, "text"))
		}
	})
}
