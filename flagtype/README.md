# flagtype

`flagtype` provides common `flag.Value` implementations for repeatable and validated flags. Each
one works with `State.GetFlag[T]`.

```go
Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
	f.Var(flagtype.StringSlice(), "tag", "add a tag (repeatable)")
	f.Var(flagtype.Enum("json", "yaml", "table"), "format", "output format")
	f.Var(flagtype.StringMap(), "label", "key=value pair (repeatable)")
}),
Exec: func(ctx context.Context, s *cli.State) error {
	tags := s.GetFlag[[]string]("tag")
	format := s.GetFlag[string]("format")
	labels := s.GetFlag[map[string]string]("label")
	_, _, _ = tags, format, labels
	return nil
},
```

The package also includes values for URLs and regular expressions. See the
[package documentation](https://pkg.go.dev/github.com/pressly/cli/flagtype) for the full API.
