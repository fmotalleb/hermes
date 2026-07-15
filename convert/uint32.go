// Package convert provides utilities for converting string values to uint32,
// with support for fallback defaults on empty or invalid input.
package convert

import (
	"cmp"
	"strconv"
)

// Uint32 parses a string as a uint32. Returns an error if the string is not a valid number.
func Uint32(s string) (uint32, error) {
	r, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(r), nil
}

// Uint32Or parses a string as a uint32, returning the default value if the string is empty or invalid.
func Uint32Or(s string, def uint32) uint32 {
	if s == "" {
		return def
	}
	r, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return def
	}
	return cmp.Or(uint32(r), def)
}
