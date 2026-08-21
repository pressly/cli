package flagtype

import (
	"flag"
	"fmt"
	"net/url"
)

type urlValue struct {
	u *url.URL
}

// URL returns a [flag.Value] that requires a URL with a scheme and host. Its value is retrieved as
// *url.URL.
func URL() flag.Value {
	return &urlValue{}
}

func (v *urlValue) String() string {
	if v.u == nil {
		return ""
	}
	return v.u.String()
}

func (v *urlValue) Set(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", s, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid URL %q: must have a scheme and host", s)
	}
	v.u = u
	return nil
}

func (v *urlValue) Get() any {
	return v.u
}
