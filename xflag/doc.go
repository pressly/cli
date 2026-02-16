// Package xflag extends the standard library's flag package to support parsing flags interleaved
// with positional arguments. By default, Go's flag package stops parsing flags at the first
// non-flag argument, which is unintuitive for most CLI users. This package provides [ParseToEnd] as
// a drop-in replacement that handles flags anywhere in the argument list.
package xflag
