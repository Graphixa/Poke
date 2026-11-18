package style

import (
	"fmt"
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
// Empty Error/Warning/Success fall back to DefaultPalette so status ink always exists.
func SetPalette(p Palette) {
	d := DefaultPalette()
	if p.Error == "" {
		p.Error = d.Error
	}
	if p.Warning == "" {
		p.Warning = d.Warning
	}
	if p.Success == "" {
		p.Success = d.Success
	}
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

// ErrorColor is status / negative emphasis (theme error).
func ErrorColor() color.Color {
	mu.RLock()
	defer mu.RUnlock()
	return fg(active.Error)
}

// SuccessColor is status / positive emphasis (theme success).
func SuccessColor() color.Color {
	mu.RLock()
	defer mu.RUnlock()
	return fg(active.Success)
}

func statusStyles() (err, warn, muted, loading lipgloss.Style) {
	mu.RLock()
	p := active
	mu.RUnlock()
	err = lipgloss.NewStyle().Foreground(fg(p.Error)).Bold(true)
	warn = lipgloss.NewStyle().Foreground(fg(p.Warning)).Bold(true)
	// Same ink as borders (BorderColor → Placeholders). No Faint: themes already set Placeholders muted.
	muted = lipgloss.NewStyle().Foreground(fg(p.Placeholders))
	loading = lipgloss.NewStyle().Foreground(fg(p.Secondary))
	return
}

// TypeLabel returns a coloured type name; the text itself is the non-colour cue.
func TypeLabel(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	return TypeLabelText(key, title(key))
}

// TypeLabelText colours display using the palette entry for name. It lets
// compact layouts abbreviate a label without duplicating the type palette.
func TypeLabelText(name, display string) string {
	key := strings.ToLower(strings.TrimSpace(name))
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
	err, _, _, _ := statusStyles()
	return err.Render("[ERROR] ") + msg
}

func StatusWarn(msg string) string {
	_, warn, _, _ := statusStyles()
	return warn.Render("[!] ") + msg
}

func StatusLoading(msg string) string {
	if msg == "" {
		msg = "Loading…"
	}
	_, _, _, loading := statusStyles()
	return loading.Render("… ") + msg
}

func StatusMuted(msg string) string {
	_, _, muted, _ := statusStyles()
	return muted.Render(msg)
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// BaseColor returns the theme base colour for lipgloss backgrounds.
func BaseColor() color.Color {
	mu.RLock()
	defer mu.RUnlock()
	return fg(active.Base)
}

// ComponentsColor is the main UI text / chip fill (light on dark themes).
func ComponentsColor() color.Color {
	mu.RLock()
	defer mu.RUnlock()
	return fg(active.Components)
}

// BaseRGBA returns the theme base colour as an opaque RGBA for sprite letterboxing.
func BaseRGBA() color.RGBA {
	mu.RLock()
	hex := active.Base
	mu.RUnlock()
	return parseHexRGBA(hex)
}

func parseHexRGBA(hex string) color.RGBA {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		return color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255}
	}
	var r, g, b uint8
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return color.RGBA{R: r, G: g, B: b, A: 255}
}
