# 001 - flagtype API

**Date:** 2026-02-18

## Context

Users of pressly/cli must manually implement `flag.Value` (and `flag.Getter`) for common types like
string slices, enums, and maps. This is repetitive boilerplate that most CLI tools need.

## Decision

Use stdlib-native constructors that return `flag.Value`, registered via `f.Var()`.

```go
Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
    f.Bool("verbose", false, "enable verbose output")
    f.Var(flagtype.StringSlice(), "tag", "add a tag (repeatable)")
    f.Var(flagtype.Enum("json", "yaml", "table"), "format", "output format")
    f.Var(flagtype.StringMap(), "label", "key=value pair (repeatable)")
})
```

The flagtype package has no knowledge of `flag.FlagSet`. Each constructor returns a value that
implements `flag.Value` and `flag.Getter`. Storage is internal -- no destination pointers needed
since values are retrieved via `cli.GetFlag[T]`.

## Alternatives considered

### A: flagtype takes a FlagSet

```go
Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
    f.Bool("verbose", false, "enable verbose output")
    flagtype.StringSlice(f, "tag", "add a tag (repeatable)")
    flagtype.Enum(f, "format", "output format", "json", "yaml", "table")
})
```

One-liner registration, no `f.Var()` ceremony. Rejected because it introduces a second calling
convention in the same block -- stdlib flags use `f.Type(name, default, usage)` while flagtype would
use `flagtype.Type(f, name, usage)`. The argument ordering inconsistency makes it harder to read at
a glance.

### B: FlagSet wrapper

```go
Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
    f.Bool("verbose", false, "enable verbose output")
    ft := flagtype.From(f)
    ft.StringSlice("tag", "add a tag (repeatable)")
    ft.Enum("format", "output format", "json", "yaml", "table")
})
```

Feels like a natural extension of FlagSet. Rejected because it requires managing two objects in the
same closure -- `f` for standard types and `ft` for custom types. Also adds a layer of indirection
that doesn't pull its weight.

### C: Declarative flag list

```go
Flags: []cli.Flag{
    cli.String("output", "", "output file"),
    cli.Bool("verbose", false, "enable verbose output"),
    flagtype.StringSlice("tag", "add a tag (repeatable)"),
    flagtype.Enum("format", "output format", "json", "yaml", "table"),
}
```

Fully declarative, no callback, no FlagSet. Rejected because it's a significant departure from the
stdlib `flag` package and would require rethinking the core `Command` type. Essentially a different
framework.

### D: Destination pointer pattern

```go
var tags []string
var re *regexp.Regexp
f.Var(flagtype.StringSlice(&tags), "tag", "add a tag (repeatable)")
f.Var(flagtype.Regexp(&re), "pattern", "regex pattern")
```

The initial implementation. Each constructor takes a pointer to the destination variable. Rejected
because pointer types like `*regexp.Regexp` and `*url.URL` require double pointers
(`**regexp.Regexp`), which is awkward. Since values are always retrieved via `cli.GetFlag[T]`, the
destination pointer serves no purpose.

## Why this approach

- **Zero new concepts.** Anyone who knows `flag.Var` already knows how to use flagtype.
- **No coupling.** flagtype has no dependency on the cli package or `flag.FlagSet`.
- **Consistent with stdlib.** Custom flag types in Go have always been registered via `f.Var()`.
  This follows that convention exactly.
- **No double pointers.** Internal storage means the API is clean for all types, including pointer
  types like `*url.URL` and `*regexp.Regexp`.
