package flagtype

import (
	"flag"
	"strings"
)

type stringSliceValue struct {
	vals []string
}

// StringSlice returns a [flag.Value] that collects values into a string slice. Each time the flag
// is set, the value is appended. This allows repeatable flags like --tag=foo --tag=bar.
//
// Use [cli.GetFlag] with type []string to retrieve the value.
func StringSlice() flag.Value {
	return &stringSliceValue{}
}

func (v *stringSliceValue) String() string {
	return strings.Join(v.vals, ",")
}

func (v *stringSliceValue) Set(s string) error {
	v.vals = append(v.vals, s)
	return nil
}

func (v *stringSliceValue) Get() any {
	return v.vals
}
