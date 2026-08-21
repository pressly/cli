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

// Enum returns a [flag.Value] restricted to allowed.
func Enum(allowed ...string) flag.Value {
	return &enumValue{allowed: allowed}
}

// EnumDefault is like [Enum] with a default value. It panics if defaultVal is not allowed.
func EnumDefault(defaultVal string, allowed []string) flag.Value {
	if !slices.Contains(allowed, defaultVal) {
		panic(fmt.Sprintf("flagtype: default value %q is not in allowed values: %s",
			defaultVal, strings.Join(allowed, ", ")))
	}
	return &enumValue{val: defaultVal, allowed: allowed}
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
