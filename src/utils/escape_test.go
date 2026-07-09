package utils_test

import (
	"testing"

	"github.com/radiospiel/simple-go/src/assert"
	"github.com/radiospiel/simple-go/src/utils"
)

func TestEscapeShellArg(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// No escaping needed for simple strings
		{"hello", "hello"},
		{"foo-bar", "foo-bar"},
		{"path/to/file", "path/to/file"},

		// Strings with spaces get quoted
		{"hello world", "'hello world'"},
		{"foo bar baz", "'foo bar baz'"},

		// Strings with tabs/newlines get quoted
		{"hello\tworld", "'hello\tworld'"},
		{"hello\nworld", "'hello\nworld'"},
		{"hello\rworld", "'hello\rworld'"},

		// Strings with double quotes get quoted
		{`say "hello"`, `'say "hello"'`},

		// Strings with single quotes use double-quote wrapping
		{"it's", `"it's"`},
		{"don't stop", `"don't stop"`},

		// Strings with both single and double quotes
		{`it's "cool"`, `'it'"'"'s "cool"'`},

		// Empty string
		{"", "''"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := utils.EscapeShellArg2(tt.input, 0)
			assert.Equals(t, result, tt.expected, "EscapeShellArg2(%q)", tt.input)
		})
	}
}
