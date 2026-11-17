package theme

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Graphixa/Pokedex/internal/tui/style"
)

//go:embed themes/*.yaml
var embeddedThemes embed.FS

// File is a theme YAML document.
type File struct {
	ThemeName string `yaml:"theme_name"`
	Style     string `yaml:"style"`
	Colors    Colors `yaml:"colors"`
}

// Colors holds the required palette keys.
type Colors struct {
	Primary      string `yaml:"primary"`
	Secondary    string `yaml:"secondary"`
	Components   string `yaml:"components"`
	Placeholders string `yaml:"placeholders"`
	Base         string `yaml:"base"`
	Warning      string `yaml:"warning"`
	Error        string `yaml:"error"`
	Success      string `yaml:"success"`
}

// Option is a selectable theme row for the theme TUI.
type Option struct {
	Name        string
	DisplayName string
	Style       string // dark / light / ""
	Source      string // embedded / user / system
}

func normalize(name string) string {
	n := strings.ReplaceAll(name, " ", "-")
	n = strings.ReplaceAll(n, "_", "-")
	for strings.Contains(n, "--") {
		n = strings.ReplaceAll(n, "--", "-")
	}
	return strings.Trim(n, "-")
}

// ToPalette maps theme colours into the TUI style palette.
func (c Colors) ToPalette() style.Palette {
	return style.Palette{
		Primary:      c.Primary,
		Secondary:    c.Secondary,
		Components:   c.Components,
		Placeholders: c.Placeholders,
		Base:         c.Base,
		Warning:      c.Warning,
		Error:        c.Error,
		Success:      c.Success,
	}
}

// Validate ensures required colour keys are present (system exempt).
func Validate(f File) error {
	c := f.Colors
	missing := []string{}
	req := map[string]string{
		"primary": c.Primary, "secondary": c.Secondary, "components": c.Components,
		"placeholders": c.Placeholders, "base": c.Base,
		"warning": c.Warning, "error": c.Error, "success": c.Success,
	}
	for k, v := range req {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("theme missing required colour keys: %s", strings.Join(missing, ", "))
}

// LoadEmbedded loads a bundled theme by name.
func LoadEmbedded(name string) (File, error) {
	name = normalize(name)
	if name == "system" {
		return System(), nil
	}
	data, err := embeddedThemes.ReadFile("themes/" + name + ".yaml")
	if err != nil {
		return File{}, fmt.Errorf("embedded theme %q not found", name)
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return File{}, err
	}
	if err := Validate(f); err != nil {
		return File{}, err
	}
	return f, nil
}

// LoadUser loads ~/.config/pokedex/themes/{name}.yaml when present.
func LoadUser(themesDir, name string) (File, error) {
	name = normalize(name)
	path := filepath.Join(themesDir, name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return File{}, err
	}
	if err := Validate(f); err != nil {
		return File{}, err
	}
	return f, nil
}

// System returns an empty-colour theme (terminal defaults).
func System() File {
	return File{ThemeName: "System", Style: "", Colors: Colors{}}
}

// Apply installs the theme into the process-wide TUI style palette.
func Apply(f File) {
	if f.ThemeName == "System" || (f.Colors.Primary == "" && f.Colors.Placeholders == "") {
		style.SetPalette(style.Palette{}) // NoColor via empty strings
		return
	}
	style.SetPalette(f.Colors.ToPalette())
}

// Resolve loads user theme, then embedded, then dracula, then system.
func Resolve(themesDir, name string) (File, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "dracula"
	}
	name = normalize(name)
	if name == "system" {
		return System(), "system", nil
	}
	if themesDir != "" {
		if f, err := LoadUser(themesDir, name); err == nil {
			return f, "user", nil
		}
	}
	if f, err := LoadEmbedded(name); err == nil {
		return f, "embedded", nil
	}
	if f, err := LoadEmbedded("dracula"); err == nil {
		return f, "default", nil
	}
	return System(), "system", nil
}

// ListOptions discovers system + embedded + user themes for the selector.
func ListOptions(themesDir, current string) ([]Option, error) {
	current = normalize(current)
	if current == "" {
		current = "dracula"
	}
	seen := map[string]bool{}
	var out []Option

	add := func(name, display, styleName, source string) {
		key := normalize(name)
		if seen[key] {
			return
		}
		seen[key] = true
		if display == "" {
			display = key
		}
		out = append(out, Option{Name: key, DisplayName: display, Style: styleName, Source: source})
	}

	add("system", "System", "", "system")

	entries, err := embeddedThemes.ReadDir("themes")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".yaml")
		f, err := LoadEmbedded(base)
		if err != nil {
			continue
		}
		display := f.ThemeName
		if display == "" {
			display = base
		}
		add(base, display, f.Style, "embedded")
	}

	if themesDir != "" {
		_ = os.MkdirAll(themesDir, 0o755)
		files, _ := os.ReadDir(themesDir)
		for _, e := range files {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			base := strings.TrimSuffix(e.Name(), ".yaml")
			f, err := LoadUser(themesDir, base)
			if err != nil {
				continue
			}
			display := f.ThemeName
			if display == "" {
				display = base
			}
			add(base, display+" (user)", f.Style, "user")
		}
	}

	// Stable-ish order: system, dark embedded, light embedded, user already interleaved by add.
	return out, nil
}

// ThemesDir returns {configDir}/themes.
func ThemesDir(configDir string) string {
	return filepath.Join(configDir, "themes")
}
