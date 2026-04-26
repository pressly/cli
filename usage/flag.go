package usage

import "fmt"

// Flag describes one flag row in a help document.
type Flag struct {
	// Name is the long flag name without dashes, such as "verbose".
	Name string

	// Short is the optional short flag name without dashes, such as "v".
	Short string

	// Placeholder is shown after non-boolean flags, such as "string" or "int".
	Placeholder string

	// Usage describes what the flag changes.
	Usage string

	// Default is shown when the default is useful to users.
	Default string

	// Required marks the flag as required in help output.
	Required bool
}

// Flags returns a help section for flag rows.
func Flags(heading string, flags []Flag) Block {
	hasShort := false
	for _, f := range flags {
		if f.Short != "" {
			hasShort = true
			break
		}
	}
	items := make([]Item, 0, len(flags))
	for _, f := range flags {
		items = append(items, Item{
			Name:    f.Spec(hasShort),
			Summary: f.Description(),
		})
	}
	return List(heading, items...)
}

// Spec returns the flag spelling shown in help output.
func (f Flag) Spec(padShort bool) string {
	var name string
	if f.Short != "" {
		name = "-" + f.Short + ", --" + f.Name
	} else if padShort {
		name = "    --" + f.Name
	} else {
		name = "--" + f.Name
	}
	if f.Placeholder == "" {
		return name
	}
	return name + " " + f.Placeholder
}

// Description returns the help text shown after the flag spelling.
func (f Flag) Description() string {
	description := f.Usage
	if f.Required {
		return description + " (required)"
	}
	if f.Default != "" {
		return fmt.Sprintf("%s (default: %s)", description, f.Default)
	}
	return description
}
