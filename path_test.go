package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

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

		// Test path from root command (which contains state)
		path := root.Path()
		require.Len(t, path, 3)
		require.Equal(t, "root", path[0].Name)
		require.Equal(t, "parent", path[1].Name)
		require.Equal(t, "child", path[2].Name)

		// Navigate to terminal command to verify it's the child
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

		// Path should return nil before parsing
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

		// Parse to parent level, not child
		err := Parse(root, []string{"parent"})
		require.NoError(t, err)

		terminal := root.terminal()
		require.Equal(t, parent, terminal)

		path := root.Path()
		require.Len(t, path, 2)
		require.Equal(t, "root", path[0].Name)
		require.Equal(t, "parent", path[1].Name)

		// Child's path should be nil since it hasn't been parsed in context
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

		// Parse to first child
		err := Parse(root, []string{"child1"})
		require.NoError(t, err)

		terminal := root.terminal()
		require.Equal(t, child1, terminal)

		path := root.Path()
		require.Len(t, path, 2)
		require.Equal(t, "root", path[0].Name)
		require.Equal(t, "child1", path[1].Name)

		// Parse to second child
		err = Parse(root, []string{"child2"})
		require.NoError(t, err)

		terminal = root.terminal()
		require.Equal(t, child2, terminal)

		path = root.Path()
		require.Len(t, path, 2)
		require.Equal(t, "root", path[0].Name)
		require.Equal(t, "child2", path[1].Name)
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

		// Parse multiple times to different levels
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

		// terminal() should return the command itself before parsing
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

		// Parse only to parent level
		err := Parse(root, []string{"parent"})
		require.NoError(t, err)

		terminal := root.terminal()
		require.Equal(t, parent, terminal)
		require.NotEqual(t, child, terminal)
	})
}
