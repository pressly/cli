package flagtype

import (
	"flag"
	"fmt"
	"slices"
	"strings"
)

type enumValue struct {
	val     string
	allowed []string
}

// Enum returns a [flag.Value] that restricts the flag to one of the allowed values. If a value not
// in the allowed list is provided, an error is returned listing valid options.
//
// Use [cli.GetFlag] with type string to retrieve the value.
func Enum(allowed ...string) flag.Value {
	return &enumValue{allowed: allowed}
}

func (v *enumValue) String() string {
	return v.val
}

func (v *enumValue) Set(s string) error {
	if !slices.Contains(v.allowed, s) {
		return fmt.Errorf("invalid value %q, must be one of: %s", s, strings.Join(v.allowed, ", "))
	}
	v.val = s
	return nil
}

func (v *enumValue) Get() any {
	return v.val
}
