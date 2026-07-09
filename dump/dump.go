package dump

import (
	"fmt"
	"strings"

	"github.com/radiospiel/simple-go/preconditions"
	"github.com/samber/lo"
)

// Printable returns a character dump representation of the input.
// Printable ASCII characters (0x20-0x7E) are shown as-is,
// non-printable characters are shown as ".".
// Accepts both string and []byte.
func Printable[T string | []byte](input T) string {
	var bytes []byte
	switch v := any(input).(type) {
	case string:
		bytes = make([]byte, len(v))
		copy(bytes, v)
	case []byte:
		bytes = v
	default:
		// never happens, due to type constraints
		preconditions.Fail("input must be string or []byte")
	}

	xbytes := lo.Map(bytes, func(b byte, _ int) string {
		if b >= 0x20 && b <= 0x7e {
			return fmt.Sprintf("%c ", b)
		} else {
			return fmt.Sprintf(". ")
		}
	})
	return strings.Join(xbytes, " ")
}

// Hex returns a hexadecimal dump representation of the input.
// Each byte is formatted as a two-character uppercase hex value.
// Accepts both string and []byte.
func Hex[T string | []byte](input T) string {
	var bytes []byte
	switch v := any(input).(type) {
	case string:
		bytes = make([]byte, len(v))
		copy(bytes, v)
	case []byte:
		bytes = v
	default:
		// never happens, due to type constraints
		preconditions.Fail("input must be string or []byte")
	}

	xbytes := lo.Map(bytes, func(b byte, _ int) string {
		return fmt.Sprintf("%X", b)
	})
	return strings.Join(xbytes, " ")
}
