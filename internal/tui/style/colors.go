package style

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// Accessible type colours favour contrast on common dark terminals while
// remaining distinct under deuteranopia-friendly hue spacing. Every coloured
// type token also carries a non-colour cue (title-cased name).
var typeColors = map[string]color.Color{
	"normal":   lipgloss.Color("#A8A878"),
	"fire":     lipgloss.Color("#F08030"),
	"water":    lipgloss.Color("#6890F0"),
	"electric": lipgloss.Color("#F8D030"),
	"grass":    lipgloss.Color("#78C850"),
	"ice":      lipgloss.Color("#98D8D8"),
	"fighting": lipgloss.Color("#C03028"),
	"poison":   lipgloss.Color("#A040A0"),
	"ground":   lipgloss.Color("#E0C068"),
	"flying":   lipgloss.Color("#A890F0"),
	"psychic":  lipgloss.Color("#F85888"),
	"bug":      lipgloss.Color("#A8B820"),
	"rock":     lipgloss.Color("#B8A038"),
	"ghost":    lipgloss.Color("#705898"),
	"dragon":   lipgloss.Color("#7038F8"),
	"dark":     lipgloss.Color("#705848"),
	"steel":    lipgloss.Color("#B8B8D0"),
	"fairy":    lipgloss.Color("#EE99AC"),
}

var (
	statusError   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Bold(true)
	statusWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0C040")).Bold(true)
	statusOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("#6BCB77"))
	statusMuted   = lipgloss.NewStyle().Faint(true)
	statusLoading = lipgloss.NewStyle().Foreground(lipgloss.Color("#7EC8E3"))
	activeTab     = lipgloss.NewStyle().Bold(true).Underline(true)
	inactiveTab   = lipgloss.NewStyle().Faint(true)
)

// TypeLabel returns a coloured type name; the text itself is the non-colour cue.
func TypeLabel(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	display := title(key)
	if c, ok := typeColors[key]; ok {
		return lipgloss.NewStyle().Foreground(c).Bold(true).Render(display)
	}
	return display
}

// TypeSummary joins types with colour; empty yields an em dash.
func TypeSummary(types []string) string {
	if len(types) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(types))
	for _, t := range types {
		parts = append(parts, TypeLabel(t))
	}
	return strings.Join(parts, "/")
}

// StatusError prefixes a clear textual marker so colour is not required.
func StatusError(msg string) string {
	return statusError.Render("[ERROR] ") + msg
}

// StatusWarn prefixes a textual warning marker.
func StatusWarn(msg string) string {
	return statusWarn.Render("[!] ") + msg
}

// StatusLoading renders a loading cue with a textual marker.
func StatusLoading(msg string) string {
	if msg == "" {
		msg = "Loading…"
	}
	return statusLoading.Render("… ") + msg
}

// StatusOK is a quiet success cue.
func StatusOK(msg string) string {
	return statusOK.Render("OK ") + msg
}

// StatusMuted is faint secondary text.
func StatusMuted(msg string) string {
	return statusMuted.Render(msg)
}

// TabToken styles an active or inactive tab label with underline (non-colour).
func TabToken(label string, active bool) string {
	token := "[" + label + "]"
	if active {
		return activeTab.Render(token)
	}
	return inactiveTab.Render(token)
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
