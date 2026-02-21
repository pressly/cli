# 002 - Usage error from Exec

**Date:** 2026-02-18

## Context

Commands often need to validate arguments and show usage alongside an error message. Currently
there's no way to access usage from within `Exec` -- users must capture the `*Command` variable in a
closure:

```go
func newCmd() *cli.Command {
    var cmd *cli.Command
    cmd = &cli.Command{
        Name: "greet",
        Exec: func(ctx context.Context, s *cli.State) error {
            if len(s.Args) == 0 {
                fmt.Fprintln(s.Stderr, cli.DefaultUsage(cmd))
                return errors.New("must supply a name")
            }
            // ...
        },
    }
    return cmd
}
```

This is awkward and breaks the clean `return &cli.Command{...}` pattern that works everywhere else.

## Decision

Two changes, one foundational and one convenience.

### 1. Expose `Cmd` on State

Add the terminal (current) command as an exported field on `*State`:

```go
type State struct {
    Args           []string
    Cmd            *Command
    Stdin          io.Reader
    Stdout, Stderr io.Writer
    path           []*Command
}
```

During `Parse`, set `cmd.state` on every command in the path (they all share the same `*State`).
This makes `DefaultUsage` work with any command, not just root -- the `state` limitation was an
implementation detail, not a design constraint.

Users who need full control can use `DefaultUsage(s.Cmd)` directly:

```go
Exec: func(ctx context.Context, s *cli.State) error {
    fmt.Fprintln(s.Stderr, cli.DefaultUsage(s.Cmd))
    return errors.New("must supply a name")
}
```

### 2. Add `UsageErrorf` convenience method

For the common case of "show usage and return an error", add a one-liner on `*State`:

```go
func (s *State) UsageErrorf(format string, args ...any) error
```

Usage:

```go
Exec: func(ctx context.Context, s *cli.State) error {
    if len(s.Args) == 0 {
        return s.UsageErrorf("must supply a name")
    }
    // ...
}
```

Output:

```
<usage text>

error: must supply a name
```

Internally, `UsageErrorf` writes usage to `s.Stderr` (via `DefaultUsage(s.Cmd)`), then returns
`fmt.Errorf(format, args...)`.

## Alternatives considered

### A: Special error type handled by Run/ParseAndRun

```go
return cli.UsageErrorf(s, "must supply a name")
```

Returns a `*UsageError` wrapping the real error. `Run` detects it, prints usage to `s.Stderr`,
returns the unwrapped error. Clean separation -- no side effects at the call site. Rejected because
it adds a new exported error type, framework interception logic in `Run`, and users who call `Run`
directly need to understand the unwrapping behavior. More surface area than necessary.

### B: Expose `s.Usage() string` and let users format

```go
fmt.Fprintln(s.Stderr, s.Usage())
return errors.New("must supply a name")
```

Most flexible, least magic. Rejected because it's two lines instead of one, and there's no single
"one way to do it" -- users will format differently, forget one half, or mix up stdout/stderr.

### C: `s.PrintUsage()` plus normal error return

```go
s.PrintUsage()
return errors.New("must supply a name")
```

Like B but hides the `fmt.Fprintln` boilerplate. Still two lines and two concepts (print usage, then
return error) instead of one.

### D: UsageErrorf without exposing Cmd

Only add `UsageErrorf` on State without exposing `Cmd`. Solves the common case but leaves users with
no escape hatch for less common needs (inspecting the command's name, flags, or building custom
usage). Exposing `Cmd` costs one exported field and enables both the convenience method and direct
access.

## Why this approach

- **Layered.** `Cmd` on State is the foundational primitive. `UsageErrorf` is a convenience built on
  top. Users pick the level of control they need.
- **One line for the common case.** `return s.UsageErrorf("...")` reads as "return a usage error" --
  same pattern as `return fmt.Errorf("...")`.
- **No new types.** Returns a plain `error`. No framework interception, no special error unwrapping.
- **Fixes an artificial limitation.** `DefaultUsage` not working with non-root commands was an
  implementation detail (only root had `state` set), not a design constraint. Setting `state` on all
  commands in the path during Parse removes this restriction.
- **Consistent with existing patterns.** State fields and methods are already how users interact
  with command context (`GetFlag`, `Args`, I/O streams).
