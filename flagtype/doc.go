// Package flagtype provides common [flag.Value] implementations for use with [flag.FlagSet.Var].
//
// All types implement [flag.Getter] so they work with [cli.GetFlag].
//
// The following types are available:
//   - [StringSlice] - repeatable flag that collects values into []string
//   - [Enum] - restricts values to a predefined set, retrieved as string
//   - [EnumDefault] - like [Enum] but with an initial default value
//   - [StringMap] - repeatable flag that parses key=value pairs into map[string]string
//   - [URL] - parses and validates a URL (must have scheme and host), retrieved as *url.URL
//   - [Regexp] - compiles a regular expression, retrieved as *regexp.Regexp
//
// Example registration:
//
//	Flags: cli.FlagsFunc(func(f *flag.FlagSet) {
//	    f.Var(flagtype.StringSlice(), "tag", "add a tag (repeatable)")
//	    f.Var(flagtype.Enum("json", "yaml", "table"), "format", "output format")
//	    f.Var(flagtype.EnumDefault("sql", []string{"sql", "go"}), "type", "migration type")
//	    f.Var(flagtype.StringMap(), "label", "key=value pair (repeatable)")
//	})
//
// Example retrieval in Exec:
//
//	tags   := cli.GetFlag[[]string](s, "tag")
//	format := cli.GetFlag[string](s, "format")
//	labels := cli.GetFlag[map[string]string](s, "label")
package flagtype
