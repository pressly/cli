package flagtype

import (
	"flag"
	"regexp"
)

type regexpValue struct {
	re *regexp.Regexp
}

// Regexp returns a [flag.Value] that compiles its input as a regular expression.
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
