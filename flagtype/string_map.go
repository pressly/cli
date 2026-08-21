package flagtype

import (
	"flag"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type stringMapValue struct {
	m map[string]string
}

// StringMap returns a repeatable [flag.Value] that parses key=value pairs. Values may contain "=".
func StringMap() flag.Value {
	return &stringMapValue{}
}

func (v *stringMapValue) String() string {
	if v.m == nil {
		return ""
	}
	keys := slices.Sorted(maps.Keys(v.m))
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+v.m[k])
	}
	return strings.Join(pairs, ",")
}

func (v *stringMapValue) Set(s string) error {
	key, value, ok := strings.Cut(s, "=")
	if !ok {
		return fmt.Errorf("invalid key=value pair: %q (missing '=')", s)
	}
	if key == "" {
		return fmt.Errorf("invalid key=value pair: %q (empty key)", s)
	}
	if v.m == nil {
		v.m = make(map[string]string)
	}
	v.m[key] = value
	return nil
}

func (v *stringMapValue) Get() any {
	return v.m
}
