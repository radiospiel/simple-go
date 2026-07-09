package utils

import (
	"strings"
)

// BytesToString converts a byte slice to a string.
func BytesToString(b []byte) string {
	var buf strings.Builder
	buf.Write(b)
	return buf.String()
}

// StringToBytes converts a string to a byte.
func StringToBytes(s string) []byte {
	b := make([]byte, len(s))
	copy(b, s)
	return b
}
