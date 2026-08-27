package xflag

import (
	"flag"
)

// ParseToEnd is a drop-in replacement for flag.Parse. It improves upon the standard behavior by
// parsing flags even when they are interspersed with positional arguments. This overcomes Go's
// default limitation of stopping flag parsing upon encountering the first positional argument. For
// more details, see:
//
//   - https://github.com/golang/go/issues/4513
//   - https://github.com/golang/go/issues/63138
//
// This is a bit unfortunate, but most users nowadays consuming CLI tools expect this behavior.
func ParseToEnd(f *flag.FlagSet, arguments []string) error {
	arguments, trailingArgs := splitAtDelimiter(arguments)
	var args []string
	parseOnce := true
	for parseOnce || len(arguments) > 0 {
		parseOnce = false
		// If the next argument looks like a flag, parses like a flag, and quacks like a flag,
		// then it probably is a flag. Let the standard parser make that determination. When it
		// instead stops at a positional argument, preserve that argument and resume parsing after
		// it on the next iteration.
		//
		// There is one edge case here which we EXPLICITLY do not handle, and quite honestly
		// 99.999% of the time you wouldn't build a CLI with this behavior: treating an unknown flag
		// as a positional argument. For example:
		//
		//	$ ./cmd --valid=true arg1 --unknown-flag=foo arg2
		//
		// This triggers an error. Some users might want the unknown flag to be treated as a
		// positional argument instead. That behavior could be added by using VisitAll to collect
		// the defined flags before deciding whether to pass a flag-looking argument to Parse.
		if err := f.Parse(arguments); err != nil {
			return err
		}

		arguments = f.Args()
		if len(arguments) == 0 {
			break
		}

		args = append(args, arguments[0])
		arguments = arguments[1:]
	}
	args = append(args, trailingArgs...)
	if len(args) > 0 {
		// Use "--" as a sentinel to set the FlagSet's internal args field without unsafe
		// reflection. When flag.Parse encounters "--" it stops processing and stores the remaining
		// arguments as positional args, which is exactly what we need.
		return f.Parse(append([]string{"--"}, args...))
	}
	return nil
}

func splitAtDelimiter(arguments []string) (before, after []string) {
	for i, arg := range arguments {
		if arg == "--" {
			return arguments[:i], arguments[i+1:]
		}
	}
	return arguments, nil
}
