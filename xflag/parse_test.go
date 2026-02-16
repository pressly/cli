package xflag

import (
	"flag"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseToEnd(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		fs := flag.NewFlagSet("name", flag.ContinueOnError)
		debugP := fs.Bool("debug", false, "debug mode")
		require.NoError(t, ParseToEnd(fs, []string{}))
		require.False(t, *debugP)
		require.Equal(t, 0, fs.NFlag())
	})
	t.Run("no args", func(t *testing.T) {
		fs := flag.NewFlagSet("name", flag.ContinueOnError)
		debugP := fs.Bool("debug", false, "debug mode")
		err := ParseToEnd(fs, []string{"--debug", "true"})
		require.NoError(t, err)
		require.Equal(t, 1, fs.NFlag())
		require.True(t, *debugP)
	})
	t.Run("before with args", func(t *testing.T) {
		fs := flag.NewFlagSet("name", flag.ContinueOnError)
		debugP := fs.Bool("debug", false, "debug mode")
		err := ParseToEnd(fs, []string{"--debug=true", "arg1", "arg2"})
		require.NoError(t, err)
		require.True(t, *debugP)
		require.Equal(t, 1, fs.NFlag())
		require.Equal(t, []string{"arg1", "arg2"}, fs.Args())
	})
	t.Run("after with args", func(t *testing.T) {
		fs := flag.NewFlagSet("name", flag.ContinueOnError)
		debugP := fs.Bool("debug", false, "debug mode")
		err := ParseToEnd(fs, []string{"arg1", "arg2", "--debug"})
		require.NoError(t, err)
		require.True(t, *debugP)

		f := fs.Lookup("debug")
		require.Equal(t, "true", f.Value.String())
		require.Equal(t, 1, fs.NFlag())
		require.Equal(t, []string{"arg1", "arg2"}, fs.Args())
	})
	t.Run("before and after with args", func(t *testing.T) {
		fs, c := newFlagset()
		args := []string{
			"--flag1=value1",
			"--flag3=true",
			"arg1",
			"arg2",
			"--flag2=value2",
			"--flag4=false",
			"arg3",
		}
		err := ParseToEnd(fs, args)
		require.NoError(t, err)
		require.Equal(t, config{
			flag1: "value1",
			flag3: true,
			flag2: "value2",
			flag4: false,
		}, *c)
		require.Equal(t, []string{"arg1", "arg2", "arg3"}, fs.Args())
		require.Equal(t, 4, fs.NFlag())
	})
	t.Run("break", func(t *testing.T) {
		fs, c := newFlagset()
		args := []string{
			"--flag1=value1",
			"--flag3=true",
			"arg1",
			"arg2",
			"--flag2=value2",
			"--flag4=false",
			"arg3",
			"--",
			"arg4",
			"arg5",
			"--flag4=true", // This is now a positional argument no matter what.
		}
		err := ParseToEnd(fs, args)
		require.NoError(t, err)
		require.Equal(t, config{
			flag1: "value1",
			flag3: true,
			flag2: "value2",
			flag4: false,
		}, *c)
		require.Equal(t, []string{"arg1", "arg2", "arg3", "arg4", "arg5", "--flag4=true"}, fs.Args())
		require.Equal(t, 4, fs.NFlag())
	})
	t.Run("unknown flag before", func(t *testing.T) {
		fs, _ := newFlagset()
		args := []string{
			"--flag1=value1",
			"--some-unknown-flag=foo", // This gets treated as a flag.
			"arg1",
		}
		err := ParseToEnd(fs, args)
		require.Error(t, err)
		require.Equal(t, err.Error(), "flag provided but not defined: -some-unknown-flag")
	})
	t.Run("unknown flag after", func(t *testing.T) {
		fs, _ := newFlagset()
		args := []string{
			"--flag1=value1",
			"arg1",
			"--some-unknown-flag=foo", // This gets treated as a flag.
		}
		err := ParseToEnd(fs, args)
		require.Error(t, err)
		require.Equal(t, err.Error(), "flag provided but not defined: -some-unknown-flag")
	})
	t.Run("only positional args", func(t *testing.T) {
		fs, c := newFlagset()
		err := ParseToEnd(fs, []string{"arg1", "arg2", "arg3"})
		require.NoError(t, err)
		// All flags should retain defaults.
		require.Equal(t, config{flag1: "asdf", flag2: "qwerty", flag3: false, flag4: true}, *c)
		require.Equal(t, 0, fs.NFlag())
		require.Equal(t, []string{"arg1", "arg2", "arg3"}, fs.Args())
	})
	t.Run("single dash flag syntax", func(t *testing.T) {
		fs, c := newFlagset()
		args := []string{
			"-flag1=value1",
			"arg1",
			"-flag3",
			"arg2",
		}
		err := ParseToEnd(fs, args)
		require.NoError(t, err)
		require.Equal(t, "value1", c.flag1)
		require.True(t, c.flag3)
		require.Equal(t, []string{"arg1", "arg2"}, fs.Args())
	})
	t.Run("space separated flags interleaved", func(t *testing.T) {
		fs, c := newFlagset()
		args := []string{
			"arg1",
			"--flag1", "value1",
			"arg2",
			"--flag2", "value2",
			"arg3",
		}
		err := ParseToEnd(fs, args)
		require.NoError(t, err)
		require.Equal(t, "value1", c.flag1)
		require.Equal(t, "value2", c.flag2)
		require.Equal(t, []string{"arg1", "arg2", "arg3"}, fs.Args())
	})
	t.Run("standalone dash is positional", func(t *testing.T) {
		fs, c := newFlagset()
		args := []string{"--flag1=value1", "-", "arg1"}
		err := ParseToEnd(fs, args)
		require.NoError(t, err)
		require.Equal(t, "value1", c.flag1)
		require.Equal(t, []string{"-", "arg1"}, fs.Args())
	})
	t.Run("flags after double dash terminator", func(t *testing.T) {
		fs, c := newFlagset()
		// The initial f.Parse consumes --flag1 and stops at "--", leaving ["--flag3"] as remaining
		// args (the "--" is stripped by std lib). The loop then parses --flag3 as a flag,
		// collecting zero positional args.
		args := []string{"--flag1=value1", "--", "--flag3"}
		err := ParseToEnd(fs, args)
		require.NoError(t, err)
		require.Equal(t, "value1", c.flag1)
		require.True(t, c.flag3)
		require.Equal(t, 0, fs.NArg())
	})
	t.Run("duplicate flags last wins", func(t *testing.T) {
		fs, c := newFlagset()
		args := []string{
			"--flag1=first",
			"arg1",
			"--flag1=second",
		}
		err := ParseToEnd(fs, args)
		require.NoError(t, err)
		require.Equal(t, "second", c.flag1)
		require.Equal(t, []string{"arg1"}, fs.Args())
	})
}

type config struct {
	flag1 string
	flag2 string
	flag3 bool
	flag4 bool
}

func newFlagset() (*flag.FlagSet, *config) {
	fs := flag.NewFlagSet("name", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	c := new(config)
	fs.StringVar(&c.flag1, "flag1", "asdf", "flag1 usage")
	fs.StringVar(&c.flag2, "flag2", "qwerty", "flag2 usage")
	fs.BoolVar(&c.flag3, "flag3", false, "flag3 usage")
	fs.BoolVar(&c.flag4, "flag4", true, "flag4 urage")
	return fs, c
}
