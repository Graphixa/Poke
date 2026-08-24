package style

import (
	"image/color"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
)

// Palette is the active theme colours (FontGet-inspired).
type Palette struct {
	Primary      string
	Secondary    string
	Components   string
	Placeholders string
	Base         string
	Warning      string
	Error        string
	Success      string
}

var (
	mu         sync.RWMutex
	active     = DefaultPalette()
	typeColors = map[string]color.Color{
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
)

// DefaultPalette is Catppuccin-inspired defaults used when no theme is loaded.
func DefaultPalette() Palette {
	return Palette{
		Primary:      "#cba6f7",
		Secondary:    "#94e2d5",
		Components:   "#cdd6f4",
		Placeholders: "#7f849c",
		Base:         "#313244",
		Warning:      "#f9e2af",
		Error:        "#e78284",
		Success:      "#a6e3a1",
	}
}

// SetPalette installs the active theme colours for the process.
func SetPalette(p Palette) {
	mu.Lock()
	defer mu.Unlock()
	active = p
}

// CurrentPalette returns a copy of the active palette.
func CurrentPalette() Palette {
	mu.RLock()
	defer mu.RUnlock()
	return active
}

func fg(hex string) color.Color {
	if hex == "" {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(hex)
}

// PrimaryColor is the main accent (titles, active emphasis).
func PrimaryColor() color.Color {
	mu.RLock()
	defer mu.RUnlock()
	return fg(active.Primary)
}

// BorderColor is used for frames and separators.
func BorderColor() color.Color {
	mu.RLock()
	defer mu.RUnlock()
	return fg(active.Placeholders)
}

// SecondaryColor is used for labels / secondary accents.
func SecondaryColor() color.Color {
	mu.RLock()
	defer mu.RUnlock()
	return fg(active.Secondary)
}

func statusStyles() (err, warn, ok, muted, loading, activeTab, inactiveTab lipgloss.Style) {
	mu.RLock()
	p := active
	mu.RUnlock()
	err = lipgloss.NewStyle().Foreground(fg(p.Error)).Bold(true)
	warn = lipgloss.NewStyle().Foreground(fg(p.Warning)).Bold(true)
	ok = lipgloss.NewStyle().Foreground(fg(p.Success))
	muted = lipgloss.NewStyle().Foreground(fg(p.Placeholders)).Faint(true)
	loading = lipgloss.NewStyle().Foreground(fg(p.Secondary))
	activeTab = lipgloss.NewStyle().Foreground(fg(p.Primary)).Bold(true).Underline(true)
	inactiveTab = lipgloss.NewStyle().Foreground(fg(p.Placeholders))
	return
}

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

func StatusError(msg string) string {
	err, _, _, _, _, _, _ := statusStyles()
	return err.Render("[ERROR] ") + msg
}

func StatusWarn(msg string) string {
	_, warn, _, _, _, _, _ := statusStyles()
	return warn.Render("[!] ") + msg
}

func StatusLoading(msg string) string {
	if msg == "" {
		msg = "Loading…"
	}
	_, _, _, _, loading, _, _ := statusStyles()
	return loading.Render("… ") + msg
}

func StatusOK(msg string) string {
	_, _, ok, _, _, _, _ := statusStyles()
	return ok.Render("OK ") + msg
}

func StatusMuted(msg string) string {
	_, _, _, muted, _, _, _ := statusStyles()
	return muted.Render(msg)
}

// TabToken styles an active or inactive tab label with underline (non-colour).
func TabToken(label string, active bool) string {
	token := "[" + label + "]"
	_, _, _, _, _, activeTab, inactiveTab := statusStyles()
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
