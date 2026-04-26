// Package usage provides small building blocks for command help documents.
//
// Use this package from a cli.Command Help hook when the default help needs examples, extra
// sections, or a different layout.
package usage

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Help is an ordered list of blocks that can be rendered as command help.
type Help []Block

// Text returns an untitled paragraph block.
//
// Use Text for descriptions, notes, or closing hints.
func Text(lines ...string) Block {
	return Block{lines: lines}
}

// Lines returns a titled block of indented lines.
//
// Use Lines for sections such as Usage or Examples where each line should stand on its own.
func Lines(heading string, lines ...string) Block {
	return Block{Heading: heading, lines: lines, indent: true}
}

// List returns a titled list of name/summary pairs.
//
// Use List for aligned sections such as commands, flags, or named examples.
func List(heading string, items ...Item) Block {
	return Block{Heading: heading, items: items}
}

// String renders the full help document as a string.
//
// Use String in tests or when you want to pass help text to an API that expects a string.
func (h Help) String() string {
	var b strings.Builder
	_, _ = h.WriteTo(&b)
	return strings.TrimRight(b.String(), "\n")
}

// WriteTo writes the help document to w.
//
// Use WriteTo when streaming help directly to stdout, stderr, or another writer.
func (h Help) WriteTo(w io.Writer) (n int64, err error) {
	cw := &countWriter{w: w}
	for i, block := range h {
		if i > 0 {
			if _, err := fmt.Fprintln(cw); err != nil {
				return cw.n, err
			}
		}
		if _, err := block.WriteTo(cw); err != nil {
			return cw.n, err
		}
	}
	return cw.n, nil
}

// Block is one section in a help document.
type Block struct {
	// Heading is rendered above the block when set, such as "Usage:" or "Examples:".
	Heading string

	lines  []string
	indent bool
	items  []Item
}

// String renders the block as a string.
func (b Block) String() string {
	var s strings.Builder
	_, _ = b.WriteTo(&s)
	return strings.TrimRight(s.String(), "\n")
}

// WriteTo writes the block to w.
func (b Block) WriteTo(w io.Writer) (n int64, err error) {
	cw := &countWriter{w: w}
	if b.Heading != "" {
		if _, err := fmt.Fprintln(cw, b.Heading); err != nil {
			return cw.n, err
		}
	}
	if len(b.items) > 0 {
		if _, err := writeItems(cw, b.items); err != nil {
			return cw.n, err
		}
	}
	for _, line := range b.lines {
		if b.indent {
			line = "  " + line
		}
		if _, err := fmt.Fprintln(cw, line); err != nil {
			return cw.n, err
		}
	}
	return cw.n, nil
}

// Item is one row in a List block.
type Item struct {
	Name    string
	Summary string
}

func writeItems(w io.Writer, items []Item) (int64, error) {
	cw := &countWriter{w: w}
	var b bytes.Buffer
	tw := tabwriter.NewWriter(&b, 0, 0, 4, ' ', 0)
	for _, item := range items {
		if item.Summary == "" {
			if _, err := fmt.Fprintf(tw, "  %s\n", item.Name); err != nil {
				return cw.n, err
			}
			continue
		}
		if _, err := fmt.Fprintf(tw, "  %s\t%s\n", item.Name, item.Summary); err != nil {
			return cw.n, err
		}
	}
	if err := tw.Flush(); err != nil {
		return cw.n, err
	}
	if _, err := cw.Write(b.Bytes()); err != nil {
		return cw.n, err
	}
	return cw.n, nil
}

type countWriter struct {
	w io.Writer
	n int64
}

func (w *countWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}
