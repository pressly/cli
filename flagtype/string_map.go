package flagtype

import (
	"flag"
	"fmt"
	"sort"
	"strings"
)

type stringMapValue struct {
	m map[string]string
}

// StringMap returns a [flag.Value] that parses key=value pairs into a map. The flag can be repeated
// to add multiple entries, like --label=env=prod --label=tier=web. The value is split on the first
// "=" character, so values may contain additional "=" characters.
//
// Use [cli.GetFlag] with type map[string]string to retrieve the value.
func StringMap() flag.Value {
	return &stringMapValue{}
}

func (v *stringMapValue) String() string {
	if v.m == nil {
		return ""
	}
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(v.m))
	for k := range v.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
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
