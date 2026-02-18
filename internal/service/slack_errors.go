package service

import (
	"fmt"
	"strings"
)

func slackScopeHint(needed, provided string) string {
	needed = strings.TrimSpace(needed)
	provided = strings.TrimSpace(provided)
	if needed == "" && provided == "" {
		return ""
	}
	if provided == "" {
		return fmt.Sprintf(" (needed=%s)", needed)
	}
	if needed == "" {
		return fmt.Sprintf(" (provided=%s)", provided)
	}
	return fmt.Sprintf(" (needed=%s provided=%s)", needed, provided)
}
