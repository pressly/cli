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
	if err := f.Parse(arguments); err != nil {
		return err
	}
	if f.NArg() == 0 {
		return nil
	}
	var args []string
	remainingArgs := f.Args()
	for i := 0; i < len(remainingArgs); i++ {
		arg := remainingArgs[i]
		// If the arg looks like a flag, parses like a flag, and quacks like a flag, then it
		// probably is a flag.
		//
		// Note, there's an edge cases here which we EXPLICITLY do not handle, and quite honestly
		// 99.999% of the time you wouldn't build a CLI with this behavior.
		//
		// If you want to treat an unknown flag as a positional argument. For example:
		//
		//  $ ./cmd --valid=true arg1 --unknown-flag=foo arg2
		//
		// Right now, this will trigger an error. But *some* users might want that unknown flag to
		// be treated as a positional argument. It's trivial to add this behavior, by using VisitAll
		// to iterate over all defined flags (regardless if they are set), and then checking if the
		// flag is in the map of known flags.
		if len(arg) > 1 && arg[0] == '-' {
			// If we encounter a "--", treat all subsequent arguments as positional. The "--" itself
			// is stripped, consistent with the standard library's behavior.
			if arg == "--" {
				args = append(args, remainingArgs[i+1:]...)
				break
			}
			if err := f.Parse(remainingArgs[i:]); err != nil {
				return err
			}
			remainingArgs = f.Args()
			i = -1 // Reset to handle newly parsed arguments.
			continue
		}
		args = append(args, arg)
	}
	if len(args) > 0 {
		// Use "--" as a sentinel to set the FlagSet's internal args field without unsafe
		// reflection. When flag.Parse encounters "--" it stops processing and stores the remaining
		// arguments as positional args, which is exactly what we need.
		return f.Parse(append([]string{"--"}, args...))
	}
	return nil
}
