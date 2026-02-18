package flagtype

import (
	"flag"
	"regexp"
)

type regexpValue struct {
	re *regexp.Regexp
}

// Regexp returns a [flag.Value] that compiles the flag value as a regular expression. If the
// pattern is invalid, an error is returned.
//
// Use [cli.GetFlag] with type *regexp.Regexp to retrieve the value.
func Regexp() flag.Value {
	return &regexpValue{}
}

func (v *regexpValue) String() string {
	if v.re == nil {
		return ""
	}
	return v.re.String()
}

func (v *regexpValue) Set(s string) error {
	re, err := regexp.Compile(s)
	if err != nil {
		return err
	}
	v.re = re
	return nil
}

func (v *regexpValue) Get() any {
	return v.re
}
