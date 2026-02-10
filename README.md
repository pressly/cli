# cli

[![GoDoc](https://godoc.org/github.com/pressly/cli?status.svg)](https://pkg.go.dev/github.com/pressly/cli#pkg-index)
[![CI](https://github.com/pressly/cli/actions/workflows/ci.yaml/badge.svg)](https://github.com/pressly/cli/actions/workflows/ci.yaml)

A Go package for building CLI applications. Extends the standard library's `flag` package to support
[flags anywhere](https://mfridman.com/blog/2024/allowing-flags-anywhere-on-the-cli/) in command
arguments.

## Features

The **bare minimum** to build a CLI application while leveraging the standard library's `flag`
package.

- Nested subcommands for organizing complex CLIs
- Flexible flag parsing, allowing flags anywhere
- Subcommands inherit flags from parent commands
- Type-safe flag access
- Automatic generation of help text and usage information
- Suggestions for misspelled or incomplete commands

### But why?

This package is intentionally minimal. It aims to be a building block for CLI applications that want
to leverage the standard library's `flag` package while providing a bit more structure and
flexibility.

- Build maintainable command-line tools quickly
- Focus on application logic rather than framework complexity
- Extend functionality **only when needed**

Sometimes less is more. While other frameworks offer extensive features, this package focuses on
core functionality.

## Installation

```bash
go get github.com/pressly/cli@latest
```

Required go version: 1.21 or higher

## Quick Start

Here's a simple example of a CLI application that echoes back the input with a required `-c` flag to
capitalize the output:

```go
root := &cli.Command{
	Name:      "echo",
	Usage:     "echo [flags] <text>...",
	ShortHelp: "echo is a simple command that prints the provided text",
	Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
		// Add a flag to capitalize the input
		f.Bool("c", false, "capitalize the input")
	}),
	FlagsMetadata: []cli.FlagMetadata{
		{Name: "c", Required: true},
	},
	Exec: func(ctx context.Context, s *cli.State) error {
		if len(s.Args) == 0 {
			return errors.New("must provide text to echo, see --help")
		}
		output := strings.Join(s.Args, " ")
		// If -c flag is set, capitalize the output
		if cli.GetFlag[bool](s, "c") {
			output = strings.ToUpper(output)
		}
		fmt.Fprintln(s.Stdout, output)
		return nil
	},
}
if err := cli.Parse(root, os.Args[1:]); err != nil {
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintf(os.Stdout, "%s\n", cli.DefaultUsage(root))
		return
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
if err := cli.Run(context.Background(), root, nil); err != nil {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
```

## Command Structure

Each command is represented by a `Command` struct:

```go
type Command struct {
	Name          string // Required
	Usage         string
	ShortHelp     string
	UsageFunc     func(*Command) string
	Flags         *flag.FlagSet
	FlagsMetadata []FlagMetadata
	SubCommands   []*Command
	Exec          func(ctx context.Context, s *State) error
}
```

The `Name` field is the command's name and is **required**.

The `Usage` and `ShortHelp` fields are used to generate help text. Nice-to-have but not required.

The `Flags` field is a `*flag.FlagSet` that defines the command's flags.

> [!TIP]
>
> There's a convenience function `FlagsFunc` that allows you to define flags inline:

```go
root := &cli.Command{
	Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
		fs.Bool("verbose", false, "enable verbose output")
		fs.String("output", "", "output file")
		fs.Int("count", 0, "number of items")
	}),
	FlagsMetadata: []cli.FlagMetadata{
		{Name: "c", Required: true},
	},
}
```

The optional `FlagsMetadata` field is a way to extend defined flags. The `flag` package alone is a
bit limiting, so we add this to provide the most common features, such as handling of required
flags.

The `SubCommands` field is a list of `*Command` structs that represent subcommands. This allows you
to organize CLI applications into a hierarchy of commands. Each subcommand can have its own flags
and business logic.

The `Exec` field is a function that is called when the command is executed. This is where you put
business logic.

## Flag Access

Flags can be accessed using the type-safe `GetFlag` function, called inside the `Exec` function:

```go
// Access boolean flag
verbose := cli.GetFlag[bool](state, "verbose")
// Access string flag
output := cli.GetFlag[string](state, "output")
// Access integer flag
count := cli.GetFlag[int](state, "count")
```

### State Inheritance

Child commands automatically inherit their parent command's flags:

```go
// Parent command with a verbose flag
root := cli.Command{
	Name: "root",
	Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
		f.Bool("verbose", false, "enable verbose mode")
	}),
}

// Child command that can access parent's verbose flag
sub := cli.Command{
	Name: "sub",
	Exec: func(ctx context.Context, s *cli.State) error {
		verbose := cli.GetFlag[bool](s, "verbose")
		if verbose {
			fmt.Println("Verbose mode enabled")
		}
		return nil
	},
}
```

## Help System

Help text is automatically generated, but you can customize it by setting the `UsageFunc` field.

There is a `DefaultUsage` function that generates a default help text for a command, which is useful
to display when `flag.ErrHelp` is returned from `Parse`:

```go
if err := cli.Parse(root, os.Args[1:]); err != nil {
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintf(os.Stdout, "%s\n", cli.DefaultUsage(root)) // Display help text and exit
		return
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
```

## Usage Syntax Conventions

When reading command usage strings, the following syntax is used:

| Syntax        | Description                |
| ------------- | -------------------------- |
| `<required>`  | Required argument          |
| `[optional]`  | Optional argument          |
| `<arg>...`    | One or more arguments      |
| `[arg]...`    | Zero or more arguments     |
| `(a\|b)`      | Must choose one of a or b  |
| `[-f <file>]` | Flag with value (optional) |
| `-f <file>`   | Flag with value (required) |

Examples:

```bash
# Multiple source files, one destination
mv <source>... <dest>

# Required flag with value, optional config
build -t <tag> [config]...

# Subcommands with own flags
docker (run|build) [--file <dockerfile>] <image>

# Multiple flag values
find [--exclude <pattern>]... <path>

# Choice between options, required path
chmod (u+x|a+r) <file>...

# Flag groups with value
kubectl [-n <namespace>] (get|delete) (pod|service) <name>
```

## Status

This project is in active development and undergoing changes as the API gets refined. Please open an
issue if you encounter any problems or have suggestions for improvement.

## Acknowledgements

There are many great CLI libraries out there, but I always felt [they were too heavy for my
needs](https://mfridman.com/blog/2021/a-simpler-building-block-for-go-clis/).

I was inspired by Peter Bourgon's [ff](https://github.com/peterbourgon/ff) library, specifically the
`v3` branch, which was soooo close to what I wanted. But the `v4` branch took a different direction
and I wanted to keep the simplicity of `v3`. This library aims to pick up where `v3` left off.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
