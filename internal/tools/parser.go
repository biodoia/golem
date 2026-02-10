package tools

import "strings"

// ParseCommand returns cmd and args when input starts with /.
func ParseCommand(input string) (string, []string, bool) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return "", nil, false
	}
	parts := strings.Fields(input[1:])
	if len(parts) == 0 {
		return "", nil, false
	}
	return parts[0], parts[1:], true
}
