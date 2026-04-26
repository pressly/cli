package usage

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHelpString(t *testing.T) {
	t.Parallel()

	h := Help{
		Text("print a greeting"),
		Lines("Usage:", "greet [flags] <name>"),
		Flags("Flags:", []Flag{
			{Name: "verbose", Short: "v", Usage: "enable verbose output"},
			{Name: "output", Placeholder: "string", Usage: "output file", Required: true},
		}),
		Commands("Available Commands:", []Command{
			{Name: "hello", Summary: "print hello"},
		}),
	}

	output := h.String()
	require.Contains(t, output, "print a greeting")
	require.Contains(t, output, "Usage:")
	require.Contains(t, output, "greet [flags] <name>")
	require.Contains(t, output, "-v, --verbose")
	require.Contains(t, output, "--output string")
	require.Contains(t, output, "output file (required)")
	require.Contains(t, output, "Available Commands:")
	require.Contains(t, output, "hello")
	require.False(t, strings.HasSuffix(output, "\n"))
}

func TestBlockString(t *testing.T) {
	t.Parallel()

	output := Lines("Examples:", "greet margo").String()
	require.Equal(t, "Examples:\n  greet margo", output)
}

func TestListWithoutSummary(t *testing.T) {
	t.Parallel()

	output := List("Commands:", Item{Name: "serve"}).String()
	require.Equal(t, "Commands:\n  serve", output)
}
