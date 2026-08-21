# xflag

`xflag.ParseToEnd` is a drop-in replacement for `flag.FlagSet.Parse` that keeps parsing after the
first positional argument.

```go
if err := xflag.ParseToEnd(flags, os.Args[1:]); err != nil {
	return err
}
```

This lets both forms work:

```text
todo add --verbose buy-milk
todo add buy-milk --verbose
```

As with the standard library, `--` stops flag parsing and leaves everything after it as positional
arguments.
