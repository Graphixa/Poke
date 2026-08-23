package search

import (
	"strings"
	"unicode"
)

// Normalize converts a query, display name, or slug into a comparable slug form:
// lowercase, punctuation/whitespace as hyphens, collapsed separators.
func Normalize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	prevHyphen := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// Tokens splits a normalized slug into hyphen-separated words.
func Tokens(normalized string) []string {
	if normalized == "" {
		return nil
	}
	parts := strings.Split(normalized, "-")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
