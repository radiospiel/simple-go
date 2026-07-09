package utils

import "strings"

// EscapeShellArg2 escapes a string for safe display in shell-like log output.
// The second parameter is ignored (present for lo.Map compatibility).
func EscapeShellArg2(s string, _ int) string {
	return EscapeShellArg(s)
}

// EscapeShellArg escapes a string for safe display in shell-like log output.
func EscapeShellArg(s string) string {
	if s == "" {
		return "''"
	}

	if !strings.ContainsAny(s, "\"' \t\n\r\\") {
		return s
	}

	// If no single quotes, wrap in single quotes (simplest shell escaping)
	if !strings.Contains(s, "'") {
		return "'" + s + "'"
	}

	// If no double quotes, wrap in double quotes
	if !strings.Contains(s, "\"") {
		return "\"" + s + "\""
	}

	// Both quote types present: use single quotes with escaped single quotes
	// Shell idiom: replace ' with '"'"' (end single quote, double-quoted single quote, resume single quote)
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
