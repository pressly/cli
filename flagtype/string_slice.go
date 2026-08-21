package flagtype

import (
	"flag"
	"strings"
)

type stringSliceValue struct {
	vals []string
}

// StringSlice returns a repeatable [flag.Value] retrieved as []string.
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
