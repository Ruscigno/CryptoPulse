// Package match centralizes the screening match-mode rule so the config
// validator, the HTTP layer, and the screener all share one definition
// (single source of truth) and cannot drift apart.
package match

import (
	"strconv"
	"strings"
)

// Valid reports whether mode is a supported match mode: "any", "all", or
// "min:N" with N >= 1.
func Valid(mode string) bool {
	if mode == "any" || mode == "all" {
		return true
	}
	n, ok := minN(mode)
	return ok && n >= 1
}

// Qualifies reports whether `triggered` of `requested` indicators satisfies the
// match mode. "all" requires every requested indicator to trigger. An invalid
// or non-positive "min:N" never qualifies (defense in depth, independent of the
// HTTP/config validation layer).
func Qualifies(triggered, requested int, mode string) bool {
	switch {
	case mode == "any":
		return triggered >= 1
	case mode == "all":
		return requested > 0 && triggered == requested
	default:
		if n, ok := minN(mode); ok && n >= 1 {
			return triggered >= n
		}
		return false
	}
}

func minN(mode string) (int, bool) {
	if !strings.HasPrefix(mode, "min:") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(mode, "min:"))
	if err != nil {
		return 0, false
	}
	return n, true
}
