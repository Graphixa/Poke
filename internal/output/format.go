package output

import (
	"fmt"
	"strings"
	"unicode"
)

// TitleFromSlug turns "lightning-rod" into "Lightning Rod".
func TitleFromSlug(slug string) string {
	if slug == "" {
		return ""
	}
	parts := strings.Split(slug, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

// GameLabel disambiguates Japan cartridges whose English names collide
// with the international Red/Blue releases (PokeAPI: red-japan, blue-japan).
// english is the API English name when known; empty falls back to the slug.
func GameLabel(slug, english string) string {
	japan := strings.HasSuffix(slug, "-japan")
	base := strings.TrimSpace(english)
	if base == "" {
		s := slug
		if japan {
			s = strings.TrimSuffix(s, "-japan")
		}
		base = TitleFromSlug(s)
	}
	if base == "" {
		return ""
	}
	if japan && !strings.Contains(strings.ToLower(base), "japan") {
		return base + " (Japan)"
	}
	return base
}

// FormatDex renders a national dex number like #025.
func FormatDex(id int) string {
	return fmt.Sprintf("#%03d", id)
}

// FormatGeneration turns "generation-iv" into "Generation IV".
func FormatGeneration(slug string) string {
	n := GenerationNumeral(slug)
	if n == "" {
		return ""
	}
	return "Generation " + n
}

// GenerationNumeral turns "generation-iv" into "IV".
func GenerationNumeral(slug string) string {
	s := strings.TrimSpace(strings.ToLower(slug))
	if s == "" {
		return ""
	}
	const prefix = "generation-"
	if !strings.HasPrefix(s, prefix) {
		return ""
	}
	roman := strings.TrimPrefix(s, prefix)
	if roman == "" {
		return ""
	}
	return strings.ToUpper(roman)
}

// JoinTypes formats type names for display: "Fire/Flying".
func JoinTypes(types []string) string {
	if len(types) == 0 {
		return ""
	}
	parts := make([]string, len(types))
	for i, t := range types {
		parts[i] = TitleFromSlug(t)
	}
	return strings.Join(parts, "/")
}

// CleanFlavor collapses whitespace and form-feed characters from flavour text.
// Soft hyphens (U+00AD) mark Game Boy line-wrap syllable breaks ("sun\xad\nlight");
// drop them so syllables rejoin, and strip leftovers — Windows Terminal smears
// glyphs when SHY remains in the buffer (vertical ghosting on the next word).
func CleanFlavor(text string) string {
	const softHyphen = "\u00ad"
	text = strings.ReplaceAll(text, softHyphen+"\r\n", "")
	text = strings.ReplaceAll(text, softHyphen+"\n", "")
	text = strings.ReplaceAll(text, softHyphen+"\r", "")
	text = strings.ReplaceAll(text, softHyphen, "")
	text = strings.ReplaceAll(text, "\f", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	fields := strings.Fields(text)
	return strings.Join(fields, " ")
}
