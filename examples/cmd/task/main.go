package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pressly/cli"
)

// todo
// ├── (-file, required)
// ├── list
// │   ├── today
// │   └── overdue
// │   └── (-tags)
// │
// └── task
// 		├── add <text>
// 		│   └── (-tags)
// 		├── done <id>
// 		└── remove <id> (-force, -all)

func main() {
	root := &cli.Command{
		Name:      "todo",
		Usage:     "todo <command> [flags]",
		ShortHelp: "A simple CLI for managing your tasks",
		Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
			f.Bool("verbose", false, "enable verbose output")
			f.Bool("version", false, "print the version")
		}),
		Exec: func(ctx context.Context, s *cli.State) error {
			if cli.GetFlag[bool](s, "version") {
				fmt.Fprintf(s.Stdout, "todo v1.0.0\n")
				return nil
			}
			fmt.Fprintf(s.Stderr, "todo: subcommand required, use --help for more information\n")
			return nil
		},
		SubCommands: []*cli.Command{
			list(),
			task(),
		},
	}

	if err := cli.ParseAndRun(context.Background(), root, os.Args[1:], nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func list() *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "todo list <command> [flags]",
		ShortHelp: "List tasks",
		Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
			f.String("file", "", "path to the tasks file")
			f.String("tags", "", "filter tasks by tags")
		}),
		FlagsMetadata: []cli.FlagMetadata{
			{Name: "file", Required: true},
		},
		Exec: func(ctx context.Context, s *cli.State) error {
			fmt.Fprintf(s.Stderr, "todo list: subcommand required, use --help for more information\n")
			return nil
		},
		SubCommands: []*cli.Command{
			listToday(),
			listOverdue(),
		},
	}
}

func getTasksFromFile(s *cli.State) (*TaskList, error) {
	file := cli.GetFlag[string](s, "file")
	return Load(file)
}

func listToday() *cli.Command {
	return &cli.Command{
		Name:      "today",
		Usage:     "todo list today [flags]",
		ShortHelp: "List tasks due today",
		Exec: func(ctx context.Context, s *cli.State) error {
			tasks, err := getTasksFromFile(s)
			if err != nil {
				return err
			}
			today := tasks.ListToday()
			if len(today) == 0 {
				fmt.Fprintf(s.Stdout, "No tasks due today, enjoy your day!\n")
				return nil
			}
			fmt.Fprintf(s.Stdout, "Tasks due today:\n")
			for _, task := range today {
				fmt.Fprintf(s.Stdout, "  %s\n", task.String())
			}
			return nil
		},
	}
}

func listOverdue() *cli.Command {
	return &cli.Command{
		Name:      "overdue",
		Usage:     "todo list overdue [flags]",
		ShortHelp: "List overdue tasks",
		Exec: func(ctx context.Context, s *cli.State) error {
			tasks, err := getTasksFromFile(s)
			if err != nil {
				return err
			}
			overdue := tasks.ListOverdue()
			if len(overdue) == 0 {
				fmt.Fprintf(s.Stdout, "No overdue tasks, enjoy your day!\n")
				return nil
			}
			fmt.Fprintf(s.Stdout, "Overdue tasks:\n")
			for _, task := range overdue {
				fmt.Fprintf(s.Stdout, "  %s\n", task.String())
			}
			return nil
		},
	}
}

func task() *cli.Command {
	return &cli.Command{
		Name:  "task",
		Usage: "todo task <command> [flags]",
		Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
			f.String("file", "", "path to the tasks file")
		}),
		FlagsMetadata: []cli.FlagMetadata{
			{Name: "file", Required: true},
		},
		ShortHelp: "Manage tasks",
		SubCommands: []*cli.Command{
			taskAdd(),
			taskDone(),
			taskRemove(),
		},
	}
}

func taskAdd() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "todo task add <text> [flags]",
		ShortHelp: "Add a new task",
		Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
			f.String("tags", "", "comma-separated list of tags")
		}),
		Exec: func(ctx context.Context, s *cli.State) error {
			var (
				tagsText = cli.GetFlag[string](s, "tags")
				file     = cli.GetFlag[string](s, "file")
			)
			var tags []string
			if tagsText != "" {
				tags = strings.Split(tagsText, ",")
			}
			tasks, err := getTasksFromFile(s)
			if err != nil {
				return err
			}
			id := tasks.LatestID() + 1
			tasks.Add(Task{
				ID:      id,
				Text:    strings.Join(s.Args, " "),
				Tags:    tags,
				Created: time.Now(),
				Status:  Pending,
			})
			if err := Save(file, tasks); err != nil {

				return err
			}
			fmt.Fprintf(s.Stdout, "Task added with ID %d\n", id)
			return nil
		},
	}
}

func taskDone() *cli.Command {
	return &cli.Command{
		Name:      "done",
		Usage:     "todo task done <id> [flags]",
		ShortHelp: "Mark a task as done",
		Exec: func(ctx context.Context, s *cli.State) error {
			if len(s.Args) == 0 {
				return errors.New("task ID required")
			}
			tasks, err := getTasksFromFile(s)
			if err != nil {
				return err
			}
			id := s.Args[0]
			parsedID, err := strconv.Atoi(id)
			if err != nil {
				return fmt.Errorf("invalid task ID: %w", err)
			}
			return tasks.Done(parsedID)
		},
	}
}

func taskRemove() *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Usage:     "todo task remove <id> [flags]",
		ShortHelp: "Remove a task",
		Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
			f.Bool("force", false, "force removal without confirmation")
			f.Bool("all", false, "remove all tasks")
		}),
		Exec: func(ctx context.Context, s *cli.State) error {
			var (
				force = cli.GetFlag[bool](s, "force")
				all   = cli.GetFlag[bool](s, "all")
				file  = cli.GetFlag[string](s, "file")
			)
			if len(s.Args) == 0 && !all {
				return errors.New("task ID required, or use --all to remove all tasks")
			}
			if all {
				if !force {

					reader := bufio.NewReader(os.Stdin)
					fmt.Print("Are you sure you want to clear all tasks? (y/N): ")
					response, err := reader.ReadString('\n')
					if err != nil {
						return fmt.Errorf("failed to read input: %w", err)
					}
					response = strings.TrimSpace(strings.ToLower(response))
					if response != "y" {
						fmt.Fprintf(s.Stdout, "Operation cancelled\n")
						return nil
					}
				}
				// add a confirmation prompt
				return Save(file, &TaskList{})
			}
			return nil
		},
	}
}
