package ansi

// ANSI color escape sequences.
const (
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"

	ColorBoldWhite = "\033[1;37m"

	ColorReset = "\033[0m"
)

// Colorize wraps str in the given ANSI color, resetting afterwards.
// If color is empty the string is returned unchanged.
func Colorize(color, str string) string {
	if color == "" {
		return str
	}
	return color + str + ColorReset
}
