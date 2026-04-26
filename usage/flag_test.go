package usage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlagSpec(t *testing.T) {
	t.Parallel()

	require.Equal(t, "--verbose", Flag{Name: "verbose"}.Spec(false))
	require.Equal(t, "    --config string", Flag{Name: "config", Placeholder: "string"}.Spec(true))
	require.Equal(t, "-o, --output string", Flag{Name: "output", Short: "o", Placeholder: "string"}.Spec(false))
}

func TestFlagDescription(t *testing.T) {
	t.Parallel()

	require.Equal(t, "enable verbose output", Flag{Usage: "enable verbose output"}.Description())
	require.Equal(t, "output file (default: stdout)", Flag{Usage: "output file", Default: "stdout"}.Description())
	require.Equal(t, "path to file (required)", Flag{Usage: "path to file", Required: true, Default: "ignored"}.Description())
}
