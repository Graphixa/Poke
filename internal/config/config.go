package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	appDirectory = "pokedex"
	configFile   = "config.yaml"
)

// Duration is a YAML duration encoded with Go duration syntax, such as 24h.
type Duration struct {
	time.Duration
}

func (d Duration) MarshalYAML() (any, error) {
	return d.String(), nil
}

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	value, err := time.ParseDuration(node.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", node.Value, err)
	}
	d.Duration = value
	return nil
}

type Config struct {
	Cache   Cache   `yaml:"cache"`
	Data    Data    `yaml:"data"`
	Render  Render  `yaml:"render"`
	TUI     TUI     `yaml:"tui"`
	Theme   Theme   `yaml:"theme"`
	CLI     CLI     `yaml:"cli"`
	Network Network `yaml:"network"`
}

type Cache struct {
	AutoRefresh    bool     `yaml:"auto_refresh"`
	IndexTTL       Duration `yaml:"index_ttl"`
	DetailTTL      Duration `yaml:"detail_ttl"`
	MoveTTL        Duration `yaml:"move_ttl"`
	RefreshOnStart bool     `yaml:"refresh_on_start"`
}

type Data struct {
	Language     string `yaml:"language"`
	VersionGroup string `yaml:"version_group"`
}

type Render struct {
	// SpriteMode selects sprite output: auto | off.
	// auto = native image when supported, else chafa, else placeholder.
	SpriteMode string `yaml:"sprite_mode"`
	// SpriteSource selects pixel (nearest) or artwork (filtered) assets.
	SpriteSource string `yaml:"sprite_source"`
	// Color selects colour encoding: auto | truecolor | 256 | mono
	Color string `yaml:"color"`
	// Dithering is reserved; v1 only supports "none".
	Dithering string `yaml:"dithering"`
	// Animate is reserved; v1 only supports "auto"/"off" (animation not required).
	Animate string `yaml:"animate"`
	// Sprite selects which PokeAPI sprite field to use (v1: front_default).
	Sprite string `yaml:"sprite"`

	// Deprecated fields retained for older config files (ignored at runtime).
	Images              string `yaml:"images"`
	Halfblock           bool   `yaml:"halfblock"`
	PlaceholderFallback bool   `yaml:"placeholder_fallback"`
}

type TUI struct {
	ShowSprite         bool   `yaml:"show_sprite"`
	MinWidthForSprite  int    `yaml:"min_width_for_sprite"`
	MinHeightForSprite int    `yaml:"min_height_for_sprite"`
	FooterHelp         string `yaml:"footer_help"`
}

// Theme selects the TUI colour palette (FontGet-compatible YAML themes).
type Theme struct {
	Name string `yaml:"name"`
}

type CLI struct {
	DefaultMode string `yaml:"default_mode"`
}

type Network struct {
	Timeout        Duration `yaml:"timeout"`
	Retries        int      `yaml:"retries"`
	MaxConcurrency int      `yaml:"max_concurrency"`
	UserAgent      string   `yaml:"user_agent"`
}

func Default() Config {
	return Config{
		Cache: Cache{
			AutoRefresh:    true,
			IndexTTL:       Duration{24 * time.Hour},
			DetailTTL:      Duration{7 * 24 * time.Hour},
			MoveTTL:        Duration{30 * 24 * time.Hour},
			RefreshOnStart: true,
		},
		Data: Data{Language: "en", VersionGroup: "latest"},
		Render: Render{
			SpriteMode:          "auto",
			SpriteSource:        "pixel",
			Color:               "auto",
			Dithering:           "none",
			Animate:             "auto",
			Halfblock:           true,
			PlaceholderFallback: true,
			Sprite:              "front_default",
		},
		TUI: TUI{
			ShowSprite:         true,
			MinWidthForSprite:  100,
			MinHeightForSprite: 50,
			FooterHelp:         "compact",
		},
		Theme: Theme{Name: "catppuccin"},
		CLI:   CLI{DefaultMode: "browse"},
		Network: Network{
			Timeout:        Duration{10 * time.Second},
			Retries:        3,
			MaxConcurrency: 4,
			UserAgent:      "poke-cli",
		},
	}
}

func (c Config) Validate() error {
	var problems []string
	positiveDuration := func(name string, value time.Duration) {
		if value <= 0 {
			problems = append(problems, name+" must be greater than zero")
		}
	}
	positiveDuration("cache.index_ttl", c.Cache.IndexTTL.Duration)
	positiveDuration("cache.detail_ttl", c.Cache.DetailTTL.Duration)
	positiveDuration("cache.move_ttl", c.Cache.MoveTTL.Duration)
	positiveDuration("network.timeout", c.Network.Timeout.Duration)
	if c.Data.Language != "en" {
		problems = append(problems, `data.language must be "en" in v1`)
	}
	if c.Data.VersionGroup == "" {
		problems = append(problems, "data.version_group must not be empty")
	}
	switch c.Render.SpriteMode {
	case "", "auto", "off":
		// Allowed.
	case "image", "chafa", "halfblock", "braille", "ascii":
		// Deprecated; EffectiveSpriteMode remaps to auto (or off via legacy).
	default:
		problems = append(problems, `render.sprite_mode must be one of "auto", "off"`)
	}
	switch c.Render.SpriteSource {
	case "", "pixel", "artwork":
	default:
		problems = append(problems, `render.sprite_source must be one of "pixel", "artwork"`)
	}
	switch c.Render.Color {
	case "", "auto", "truecolor", "256", "mono":
	default:
		problems = append(problems, `render.color must be one of "auto", "truecolor", "256", "mono"`)
	}
	switch c.Render.Dithering {
	case "", "none":
	default:
		problems = append(problems, `render.dithering must be "none" in v1`)
	}
	switch c.Render.Animate {
	case "", "auto", "on", "off":
	default:
		problems = append(problems, `render.animate must be one of "auto", "on", "off"`)
	}
	// render.images is deprecated and ignored (native protocol is auto-detected).
	if c.Render.Sprite != "front_default" {
		problems = append(problems, `render.sprite must be "front_default" in v1`)
	}
	if c.TUI.MinWidthForSprite <= 0 || c.TUI.MinHeightForSprite <= 0 {
		problems = append(problems, "tui sprite dimensions must be greater than zero")
	}
	if c.TUI.FooterHelp != "compact" {
		problems = append(problems, `tui.footer_help must be "compact"`)
	}
	if c.CLI.DefaultMode != "browse" {
		problems = append(problems, `cli.default_mode must be "browse"`)
	}
	if c.Network.Retries < 0 {
		problems = append(problems, "network.retries must not be negative")
	}
	if c.Network.MaxConcurrency <= 0 {
		problems = append(problems, "network.max_concurrency must be greater than zero")
	}
	if strings.TrimSpace(c.Network.UserAgent) == "" {
		problems = append(problems, "network.user_agent must not be empty")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// EffectiveSpriteMode returns auto or off. Native image vs chafa is chosen by
// capability detection, not config. Deprecated mode names remap to auto.
func (c Config) EffectiveSpriteMode() string {
	mode := c.Render.SpriteMode
	switch mode {
	case "off":
		return "off"
	case "":
		// Legacy configs without sprite_mode.
		if !c.Render.Halfblock && (c.Render.Images == "off" || c.Render.Images == "") {
			return "off"
		}
		return "auto"
	default:
		// auto, image, chafa, and old text modes all mean "use the auto chain".
		return "auto"
	}
}

type Paths struct {
	ConfigFile string
	CacheDir   string
}

type DirectoryResolver struct {
	UserConfigDir func() (string, error)
	UserCacheDir  func() (string, error)
}

func OSDirectoryResolver() DirectoryResolver {
	return DirectoryResolver{
		UserConfigDir: os.UserConfigDir,
		UserCacheDir:  os.UserCacheDir,
	}
}

func (r DirectoryResolver) Resolve() (Paths, error) {
	if r.UserConfigDir == nil || r.UserCacheDir == nil {
		return Paths{}, errors.New("config and cache directory resolvers are required")
	}
	configDir, err := r.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user config directory: %w", err)
	}
	cacheDir, err := r.UserCacheDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user cache directory: %w", err)
	}
	return Paths{
		ConfigFile: filepath.Join(configDir, appDirectory, configFile),
		CacheDir:   filepath.Join(cacheDir, appDirectory),
	}, nil
}

// Load overlays a YAML file on the defaults. Unknown fields are ignored so
// newer config files remain usable by older patch releases.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	migrateLegacySpriteGates(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

// migrateLegacySpriteGates raises older sprite gates to the sprite-first ~100×50
// contract (never show below a 40×40 frame when the panel is enabled).
func migrateLegacySpriteGates(cfg *Config) {
	if cfg == nil {
		return
	}
	w, h := cfg.TUI.MinWidthForSprite, cfg.TUI.MinHeightForSprite
	legacy := (w == 110 && h == 52) || (w == 80 && h == 28)
	if legacy {
		cfg.TUI.MinWidthForSprite = 100
		cfg.TUI.MinHeightForSprite = 50
	}
}

// Initialize atomically creates a default config and never replaces an
// existing file. The bool reports whether a file was created.
func Initialize(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, fmt.Errorf("create config directory: %w", err)
	}
	data, err := yaml.Marshal(Default())
	if err != nil {
		return false, fmt.Errorf("encode default config: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return false, fmt.Errorf("create temporary config: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return false, fmt.Errorf("set config permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return false, fmt.Errorf("write temporary config: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return false, fmt.Errorf("sync temporary config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return false, fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Link(tempName, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, nil
		}
		return false, fmt.Errorf("install config atomically: %w", err)
	}
	return true, nil
}

// Save writes cfg to path atomically (temp file + rename).
func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	_ = temp.Chmod(0o600)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("install config: %w", err)
	}
	return nil
}

// SetThemeName updates only the theme name and persists the full config.
func SetThemeName(path, name string) error {
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	cfg.Theme.Name = name
	return Save(path, cfg)
}

type CommandRunner interface {
	Run(name string, args ...string) error
}

type ExecRunner struct{}

func (ExecRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func SelectEditor(getenv func(string) string, goos string) []string {
	for _, variable := range []string{"VISUAL", "EDITOR"} {
		if command := strings.TrimSpace(getenv(variable)); command != "" {
			return splitCommand(command)
		}
	}
	if goos == "windows" {
		return []string{"notepad"}
	}
	return []string{"vi"}
}

func Edit(path string, getenv func(string) string, runner CommandRunner) error {
	if _, err := Initialize(path); err != nil {
		return err
	}
	command := SelectEditor(getenv, runtime.GOOS)
	if len(command) == 0 {
		return errors.New("no editor command configured")
	}
	args := append(command[1:], path)
	if err := runner.Run(command[0], args...); err != nil {
		return fmt.Errorf("open config in %s: %w", command[0], err)
	}
	return nil
}

func splitCommand(value string) []string {
	var fields []string
	var current strings.Builder
	var quote rune
	flush := func() {
		if current.Len() > 0 {
			fields = append(fields, current.String())
			current.Reset()
		}
	}
	for _, r := range value {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '\'' || r == '"'):
			quote = r
		case quote == 0 && (r == ' ' || r == '\t'):
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return fields
}
