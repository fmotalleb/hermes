// Package convert provides utilities for converting string values to uint32,
// with support for fallback defaults on empty or invalid input.
package convert

import (
	"cmp"
	"strconv"
)

func Uint32(s string) (uint32, error) {
	r, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(r), nil
}

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
