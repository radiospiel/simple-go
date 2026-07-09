package utils

import (
	"fmt"
	"regexp"
)

// ReplacePlaceholders replaces all placeholders in s with their values from args.
// placeholderFormat is a regexp with exactly one capture group identifying the key,
// e.g. `{{([^{]+)}}` or `\$\{([^}]+)\}`.
// Returns an error if s contains a placeholder whose key is not in args.
func ReplacePlaceholders(s string, args map[string]string, placeholderFormat string) (string, error) {
	re := regexp.MustCompile(placeholderFormat)
	var missingKey string
	result := re.ReplaceAllStringFunc(s, func(match string) string {
		sub := re.FindStringSubmatch(match)
		key := sub[1]
		if v, ok := args[key]; ok {
			return v
		}
		if missingKey == "" {
			missingKey = key
		}
		return match
	})
	if missingKey != "" {
		return "", fmt.Errorf("required placeholder missing: %s", missingKey)
	}
	return result, nil
}
