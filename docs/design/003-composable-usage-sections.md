# 003 - Composable usage sections

**Date:** 2026-02-20

## Context

`DefaultUsage` builds help output as a single monolithic function. Users who want to customize
sections (add examples, environment variables, supported databases, etc.) have two options today,
both problematic:

1. **`UsageFunc` from scratch.** Rewrite the entire help output, duplicating all the flag formatting,
   command listing, and alignment logic that `DefaultUsage` already handles.

2. **`UsageFunc` wrapping `DefaultUsage`.** Call `DefaultUsage` from within `UsageFunc` to append or
   prepend content. But `DefaultUsage` checks for `UsageFunc` first and dispatches to it, causing
   infinite recursion (#15). The only workaround is temporarily nilling out `UsageFunc`:

```go
UsageFunc: func(cmd *cli.Command) string {
    cmd.UsageFunc = nil
    s := cli.DefaultUsage(cmd)
    cmd.UsageFunc = ... // restore
    return s + "\n\nExamples:\n  ..."
},
```

Neither option gives users composable pieces. There's no way to say "give me the standard flags
section" or "give me the standard commands section" and mix them with custom sections.

Separately, `DefaultUsage` only works when called with the root command because only root gets
`state` assigned during `Parse`. Subcommands have nil `state`, so `cmd.Path()` returns nil and the
section builders can't walk the hierarchy for inherited flags or command path strings.

## Decision

Four changes: one parse fix, one bug fix, one new helper package, and a refactor to connect them.

### 1. Set state on all commands in path during Parse

During `Parse`, assign `cmd.state` on every command in the resolved path, not just root. They all
share the same `*State` -- this is a one-line change. Without this, `cmd.Path()` returns nil for
subcommands and the section builders can't function.

### 2. Fix DefaultUsage recursion (#15)

Remove the `UsageFunc` check from `DefaultUsage`. The function name already means "give me the
default behavior" -- dispatching to `UsageFunc` is surprising and prevents composition. The
`UsageFunc` dispatch stays in `ParseAndRun` where it belongs.

### 3. Create `pkg/usage` with section builders

A helper package that provides the individual pieces as functions. Each takes the terminal
`*Command` (which has access to `Path()`, `Flags`, `FlagOptions`, `SubCommands`, etc.) and returns a
formatted string. Empty string when there's nothing to show.

```go
package usage

// Build joins non-empty sections with a blank line separator.
func Build(sections ...string) string

// Standard section builders.
func ShortHelp(cmd *cli.Command) string       // description text, no title
func UsageLine(cmd *cli.Command) string        // "Usage:\n  app cmd [flags]"
func Commands(cmd *cli.Command) string         // "Available Commands:\n  ..."
func Flags(cmd *cli.Command) string            // "Flags:\n  ..."
func InheritedFlags(cmd *cli.Command) string   // "Inherited Flags:\n  ..."
func SubcommandHelp(cmd *cli.Command) string   // 'Use "app cmd --help" for more...'

// Section formats a custom titled section.
func Section(title, body string) string
```

`Flags` and `InheritedFlags` walk `cmd.Path()` to categorize flags by ancestry, respecting
`FlagOption.Local`. They use `pkg/textutil` for wrapping.

`Section` formats a titled block -- the escape hatch for anything domain-specific:

```go
usage.Section("Environment Variables",
    "  GOOSE_DRIVER       database driver\n"+
    "  GOOSE_DBSTRING     connection string")
```

### 4. Refactor DefaultUsage to use pkg/usage

`DefaultUsage` becomes the default composition of sections, eliminating duplication:

```go
func DefaultUsage(root *Command) string {
    cmd := root.terminal()
    return usage.Build(
        usage.ShortHelp(cmd),
        usage.UsageLine(cmd),
        usage.Commands(cmd),
        usage.Flags(cmd),
        usage.InheritedFlags(cmd),
        usage.SubcommandHelp(cmd),
    )
}
```

The formatting logic lives in `pkg/usage` once. `DefaultUsage` is just the standard arrangement.

## Usage

Default behavior (no `UsageFunc`) is unchanged -- `DefaultUsage` handles it. When someone wants
custom sections, they set `UsageFunc` and build from pieces:

```go
&cli.Command{
    Name:      "migrate",
    ShortHelp: "Run database migrations",
    UsageFunc: func(cmd *cli.Command) string {
        return usage.Build(
            usage.ShortHelp(cmd),
            usage.UsageLine(cmd),
            usage.Commands(cmd),
            usage.Flags(cmd),
            usage.InheritedFlags(cmd),
            usage.Section("Supported Databases",
                "  PostgreSQL, MySQL, SQLite, ClickHouse"),
            usage.Section("Environment Variables",
                "  GOOSE_DRIVER       database driver\n"+
                "  GOOSE_DBSTRING     connection string"),
            usage.Section("Learn More",
                "  https://pressly.github.io/goose"),
            usage.SubcommandHelp(cmd),
        )
    },
}
```

Output:

```
Run database migrations

Usage:
  myapp migrate [flags] <command>

Available Commands:
  down    Roll back the last migration
  status  Show migration status
  up      Apply all pending migrations

Flags:
  -d, --dir string    migrations directory (default: db/migrations)

Inherited Flags:
      --verbose    enable verbose output

Supported Databases:
  PostgreSQL, MySQL, SQLite, ClickHouse

Environment Variables:
  GOOSE_DRIVER       database driver
  GOOSE_DBSTRING     connection string

Learn More:
  https://pressly.github.io/goose

Use "myapp migrate [command] --help" for more information about a command.
```

## Alternatives considered

### A: Section builders in the core package

Put the section functions directly on `cli` instead of `pkg/usage`. Rejected because it bloats the
core API surface. Users who just want defaults never need these functions. Keeping them in a helper
package means the core stays minimal -- you opt into composition by importing `pkg/usage`.

### B: Section as a struct type

```go
type Section struct {
    Title string
    Body  string
}
```

Each builder returns a `Section`, and `Build` joins them. Rejected because it adds a type for no
benefit -- the title is already baked into each builder's output (e.g., `Flags` returns `"Flags:\n
..."`). Plain strings compose with `+` and are easier to inspect and test.

### C: Builder/fluent pattern

```go
usage.New(cmd).
    ShortHelp().
    UsageLine().
    Commands().
    Section("Examples", "...").
    Flags().
    String()
```

Rejected because it adds a builder type, method chaining, and a terminal `String()` call. Functions
returning strings are simpler and more flexible -- you can store sections in variables,
conditionally include them, or reorder them without a builder API.

### D: Template-based approach

```go
UsageTemplate: `{{.ShortHelp}}

Usage:
  {{.UsageLine}}

{{.Commands}}
{{.Flags}}

Examples:
  myapp migrate up --dir ./db
`
```

Rejected because templates are harder to debug, don't compose well with programmatic logic, and
require learning template syntax. Functions are just Go code.

## What's deferred

`State.Cmd` and `State.UsageErrorf` from design doc 002 are not needed for this work. `UsageFunc`
already receives `*Command`, so the section builders have everything they need. The "usage error
from Exec" story is a separate concern that can ship independently.

## Why this approach

- **No duplication.** `DefaultUsage` composes from the same pieces users compose from. One
  implementation of flag formatting, command listing, and text wrapping.
- **Core stays minimal.** The only core changes are a one-line parse fix and removing 3 lines from
  `DefaultUsage`. All section logic lives in `pkg/usage`.
- **Plain strings.** No builder types, no section structs, no templates. Each function returns a
  string. `Build` joins non-empty strings. That's the entire API.
- **Incremental adoption.** Users who never set `UsageFunc` see no change. Users who want one extra
  section can use `DefaultUsage(cmd) + "\n\n" + usage.Section(...)`. Users who want full control
  compose from individual pieces.
- **Consistent with existing helpers.** `pkg/textutil`, `pkg/suggest`, `flagtype`, and `xflag` are
  all optional packages that build on the core without changing it. `pkg/usage` follows the same
  pattern.
