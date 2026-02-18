package flagtype

import (
	"flag"
	"net/url"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringSlice(t *testing.T) {
	t.Parallel()

	t.Run("single value", func(t *testing.T) {
		t.Parallel()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.Var(StringSlice(), "tag", "")
		err := fs.Parse([]string{"--tag=foo"})
		require.NoError(t, err)
		got := fs.Lookup("tag").Value.(flag.Getter).Get().([]string)
		assert.Equal(t, []string{"foo"}, got)
	})
	t.Run("multiple values", func(t *testing.T) {
		t.Parallel()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.Var(StringSlice(), "tag", "")
		err := fs.Parse([]string{"--tag=foo", "--tag=bar", "--tag=baz"})
		require.NoError(t, err)
		got := fs.Lookup("tag").Value.(flag.Getter).Get().([]string)
		assert.Equal(t, []string{"foo", "bar", "baz"}, got)
	})
	t.Run("string output", func(t *testing.T) {
		t.Parallel()
		v := StringSlice()
		require.NoError(t, v.Set("a"))
		require.NoError(t, v.Set("b"))
		assert.Equal(t, "a,b", v.String())
	})
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		v := StringSlice()
		assert.Equal(t, "", v.String())
		got := v.(flag.Getter).Get().([]string)
		assert.Nil(t, got)
	})
}

func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("valid value", func(t *testing.T) {
		t.Parallel()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.Var(Enum("json", "yaml", "table"), "format", "")
		err := fs.Parse([]string{"--format=yaml"})
		require.NoError(t, err)
		got := fs.Lookup("format").Value.(flag.Getter).Get().(string)
		assert.Equal(t, "yaml", got)
	})
	t.Run("invalid value", func(t *testing.T) {
		t.Parallel()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(nopWriter{})
		fs.Var(Enum("json", "yaml"), "format", "")
		err := fs.Parse([]string{"--format=xml"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be one of")
		assert.Contains(t, err.Error(), "json, yaml")
	})
	t.Run("empty default", func(t *testing.T) {
		t.Parallel()
		v := Enum("a", "b")
		assert.Equal(t, "", v.String())
		assert.Equal(t, "", v.(flag.Getter).Get())
	})
}

func TestStringMap(t *testing.T) {
	t.Parallel()

	t.Run("single pair", func(t *testing.T) {
		t.Parallel()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.Var(StringMap(), "label", "")
		err := fs.Parse([]string{"--label=env=prod"})
		require.NoError(t, err)
		got := fs.Lookup("label").Value.(flag.Getter).Get().(map[string]string)
		assert.Equal(t, map[string]string{"env": "prod"}, got)
	})
	t.Run("multiple pairs", func(t *testing.T) {
		t.Parallel()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.Var(StringMap(), "label", "")
		err := fs.Parse([]string{"--label=env=prod", "--label=tier=web"})
		require.NoError(t, err)
		got := fs.Lookup("label").Value.(flag.Getter).Get().(map[string]string)
		assert.Equal(t, map[string]string{"env": "prod", "tier": "web"}, got)
	})
	t.Run("value contains equals", func(t *testing.T) {
		t.Parallel()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.Var(StringMap(), "label", "")
		err := fs.Parse([]string{"--label=query=a=b"})
		require.NoError(t, err)
		got := fs.Lookup("label").Value.(flag.Getter).Get().(map[string]string)
		assert.Equal(t, map[string]string{"query": "a=b"}, got)
	})
	t.Run("missing equals", func(t *testing.T) {
		t.Parallel()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(nopWriter{})
		fs.Var(StringMap(), "label", "")
		err := fs.Parse([]string{"--label=nope"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing '='")
	})
	t.Run("empty key", func(t *testing.T) {
		t.Parallel()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(nopWriter{})
		fs.Var(StringMap(), "label", "")
		err := fs.Parse([]string{"--label==value"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty key")
	})
	t.Run("string output sorted", func(t *testing.T) {
		t.Parallel()
		v := StringMap()
		require.NoError(t, v.Set("b=2"))
		require.NoError(t, v.Set("a=1"))
		assert.Equal(t, "a=1,b=2", v.String())
	})
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		v := StringMap()
		assert.Equal(t, "", v.String())
		assert.Nil(t, v.(flag.Getter).Get())
	})
}

func TestURL(t *testing.T) {
	t.Parallel()

	t.Run("valid url", func(t *testing.T) {
		t.Parallel()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.Var(URL(), "endpoint", "")
		err := fs.Parse([]string{"--endpoint=https://example.com/api"})
		require.NoError(t, err)
		got := fs.Lookup("endpoint").Value.(flag.Getter).Get().(*url.URL)
		require.NotNil(t, got)
		assert.Equal(t, "https", got.Scheme)
		assert.Equal(t, "example.com", got.Host)
		assert.Equal(t, "/api", got.Path)
	})
	t.Run("missing scheme", func(t *testing.T) {
		t.Parallel()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(nopWriter{})
		fs.Var(URL(), "endpoint", "")
		err := fs.Parse([]string{"--endpoint=example.com"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must have a scheme and host")
	})
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		v := URL()
		assert.Equal(t, "", v.String())
		assert.Nil(t, v.(flag.Getter).Get())
	})
}

func TestRegexp(t *testing.T) {
	t.Parallel()

	t.Run("valid pattern", func(t *testing.T) {
		t.Parallel()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.Var(Regexp(), "pattern", "")
		err := fs.Parse([]string{"--pattern=^foo.*bar$"})
		require.NoError(t, err)
		got := fs.Lookup("pattern").Value.(flag.Getter).Get().(*regexp.Regexp)
		require.NotNil(t, got)
		assert.True(t, got.MatchString("fooXbar"))
		assert.False(t, got.MatchString("baz"))
	})
	t.Run("invalid pattern", func(t *testing.T) {
		t.Parallel()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(nopWriter{})
		fs.Var(Regexp(), "pattern", "")
		err := fs.Parse([]string{"--pattern=[invalid"})
		require.Error(t, err)
	})
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		v := Regexp()
		assert.Equal(t, "", v.String())
		assert.Nil(t, v.(flag.Getter).Get())
	})
}

// nopWriter discards all writes, used to suppress flag.FlagSet error output in tests.
type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
