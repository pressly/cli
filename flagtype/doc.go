// Package flagtype provides common [flag.Value] implementations. Each also implements [flag.Getter]
// for use with cli.State.GetFlag.
//
//	Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
//	    f.Var(flagtype.StringSlice(), "tag", "add a tag (repeatable)")
//	    f.Var(flagtype.Enum("json", "yaml", "table"), "format", "output format")
//	    f.Var(flagtype.StringMap(), "label", "key=value pair (repeatable)")
//	})
//
// Inside Exec:
//
//	tags   := s.GetFlag[[]string]("tag")
//	format := s.GetFlag[string]("format")
package flagtype
