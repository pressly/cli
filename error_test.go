package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageError(t *testing.T) {
	t.Parallel()

	err := UsageErrorf("missing %s", "name")
	require.EqualError(t, err, "missing name")

	var usageErr *usageError
	require.True(t, errors.As(err, &usageErr))
	require.EqualError(t, errors.Unwrap(err), "missing name")
}
