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

	// DefaultSixelColors is the production sixel palette size.
	DefaultSixelColors = 64
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
	Render  Render  `yaml:"render"`
	TUI     TUI     `yaml:"tui"`
	Theme   Theme   `yaml:"theme"`
	Network Network `yaml:"network"`
	Setup   Setup   `yaml:"setup"`
}

// Setup tracks first-run wizard state and official-artwork consent.
type Setup struct {
	// Completed is true after the first-run wizard finishes.
	Completed bool `yaml:"completed"`
	// ArtworkConsent is none | accepted | declined.
	ArtworkConsent string `yaml:"artwork_consent"`
	// ArtworkMode is off | blocking | background.
	ArtworkMode string `yaml:"artwork_mode"`
}

const (
	ArtworkConsentNone     = "none"
	ArtworkConsentAccepted = "accepted"
	ArtworkConsentDeclined = "declined"

	ArtworkModeOff        = "off"
	ArtworkModeBlocking   = "blocking"
	ArtworkModeBackground = "background"
)

type Cache struct {
	AutoRefresh    bool     `yaml:"auto_refresh"`
	IndexTTL       Duration `yaml:"index_ttl"`
	DetailTTL      Duration `yaml:"detail_ttl"`
	MoveTTL        Duration `yaml:"move_ttl"`
	DossierPrewarm string   `yaml:"dossier_prewarm"` // all | window | off
}

type Render struct {
	// SpriteMode selects sprite output: auto | image | text | chafa | halfblock | off.
	SpriteMode string `yaml:"sprite_mode"`
	// SpriteSource selects pixel (nearest) or artwork (filtered) resize/assets.
	// Product default is artwork; pixel remains for tests and fallbacks.
	SpriteSource string `yaml:"sprite_source"`
	// Color selects colour encoding: auto | truecolor | 256 | mono
	Color string `yaml:"color"`
	// SpritePrewarm controls background chafa warm: all_pixel | window | off.
	SpritePrewarm string `yaml:"sprite_prewarm"`
	// SpriteHD controls HD assets for native image: auto | off.
	SpriteHD string `yaml:"sprite_hd"`
	// SixelColors selects sixel palette size: 64 (default) | 128 | 256.
	SixelColors int `yaml:"sixel_colors"`
}

type TUI struct {
	ShowSprite bool `yaml:"show_sprite"`
}

// Theme selects the TUI colour palette (FontGet-compatible YAML themes).
type Theme struct {
	Name string `yaml:"name"`
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
			DossierPrewarm: "window",
		},
		Render: Render{
			SpriteMode:    "auto",
			SpriteSource:  "artwork",
			Color:         "auto",
			SpritePrewarm: "window",
			SpriteHD:      "auto",
			SixelColors:   DefaultSixelColors,
		},
		TUI: TUI{
			ShowSprite: true,
		},
		Theme: Theme{Name: "catppuccin"},
		Network: Network{
			Timeout:        Duration{10 * time.Second},
			Retries:        3,
			MaxConcurrency: 8,
			UserAgent:      "poke-cli",
		},
		Setup: Setup{
			Completed:      false,
			ArtworkConsent: ArtworkConsentNone,
			ArtworkMode:    ArtworkModeOff,
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
	switch c.Render.SpriteMode {
	case "", "auto", "off", "image", "text", "chafa", "halfblock":
		// Allowed.
	default:
		problems = append(problems, `render.sprite_mode must be one of "auto", "image", "text", "chafa", "halfblock", "off"`)
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
	switch c.Render.SpritePrewarm {
	case "", "all_pixel", "window", "off":
	default:
		problems = append(problems, `render.sprite_prewarm must be one of "all_pixel", "window", "off"`)
	}
	switch c.Render.SpriteHD {
	case "", "auto", "off":
	default:
		problems = append(problems, `render.sprite_hd must be one of "auto", "off"`)
	}
	switch c.Cache.DossierPrewarm {
	case "", "all", "window", "off":
	default:
		problems = append(problems, `cache.dossier_prewarm must be one of "all", "window", "off"`)
	}
	switch c.Setup.ArtworkConsent {
	case "", ArtworkConsentNone, ArtworkConsentAccepted, ArtworkConsentDeclined:
	default:
		problems = append(problems, `setup.artwork_consent must be one of "none", "accepted", "declined"`)
	}
	switch c.Setup.ArtworkMode {
	case "", ArtworkModeOff, ArtworkModeBlocking, ArtworkModeBackground:
	default:
		problems = append(problems, `setup.artwork_mode must be one of "off", "blocking", "background"`)
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

// EffectiveSpriteMode returns auto, image, chafa, halfblock, or off.
func (c Config) EffectiveSpriteMode() string {
	switch c.Render.SpriteMode {
	case "off":
		return "off"
	case "image":
		return "image"
	case "text", "chafa":
		return "chafa"
	case "halfblock":
		return "halfblock"
	default:
		return "auto"
	}
}

// EffectiveSixelColors returns 64, 128, or 256. Unsupported values fall back to 64.
func (c Config) EffectiveSixelColors() int {
	switch c.Render.SixelColors {
	case 64, 128, 256:
		return c.Render.SixelColors
	default:
		return DefaultSixelColors
	}
}

// EffectiveSpritePrewarm returns all_pixel | window | off.
func (c Config) EffectiveSpritePrewarm() string {
	switch c.Render.SpritePrewarm {
	case "all_pixel", "off":
		return c.Render.SpritePrewarm
	default:
		return "window"
	}
}

// EffectiveDossierPrewarm returns all | window | off.
func (c Config) EffectiveDossierPrewarm() string {
	switch c.Cache.DossierPrewarm {
	case "all", "window", "off":
		return c.Cache.DossierPrewarm
	default:
		return "window"
	}
}

// SpriteHDEnabled reports whether bulk artwork warm is allowed.
// Resolve always prefers official artwork; this gates pack download only.
func (c Config) SpriteHDEnabled() bool {
	return c.Render.SpriteHD != "off"
}

// NeedsSetup reports whether the first-run wizard should run.
func (c Config) NeedsSetup() bool {
	return !c.Setup.Completed
}

// EffectiveArtworkConsent returns none | accepted | declined.
func (c Config) EffectiveArtworkConsent() string {
	switch c.Setup.ArtworkConsent {
	case ArtworkConsentAccepted, ArtworkConsentDeclined:
		return c.Setup.ArtworkConsent
	default:
		return ArtworkConsentNone
	}
}

// EffectiveArtworkMode returns off | blocking | background.
func (c Config) EffectiveArtworkMode() string {
	switch c.Setup.ArtworkMode {
	case ArtworkModeBlocking, ArtworkModeBackground:
		return c.Setup.ArtworkMode
	default:
		return ArtworkModeOff
	}
}

// ApplyArtworkConsent updates setup + render.sprite_hd to match a wizard choice.
func (c *Config) ApplyArtworkConsent(consent, mode string) {
	if c == nil {
		return
	}
	switch consent {
	case ArtworkConsentAccepted:
		c.Setup.ArtworkConsent = ArtworkConsentAccepted
		c.Render.SpriteHD = "auto"
		switch mode {
		case ArtworkModeBackground:
			c.Setup.ArtworkMode = ArtworkModeBackground
		default:
			c.Setup.ArtworkMode = ArtworkModeBlocking
		}
	case ArtworkConsentDeclined:
		c.Setup.ArtworkConsent = ArtworkConsentDeclined
		c.Setup.ArtworkMode = ArtworkModeOff
		c.Render.SpriteHD = "off"
	default:
		c.Setup.ArtworkConsent = ArtworkConsentNone
		c.Setup.ArtworkMode = ArtworkModeOff
		c.Render.SpriteHD = "off"
	}
}

// MarkSetupComplete flags the first-run wizard as finished.
func (c *Config) MarkSetupComplete() {
	if c == nil {
		return
	}
	c.Setup.Completed = true
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
func Load(path string) (Config, []string, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil, nil
	}
	if err != nil {
		return Config{}, nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil, nil
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
	cfg, _, err := Load(path)
	if err != nil {
		return err
	}
	cfg.Theme.Name = name
	return Save(path, cfg)
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

func Edit(path string, getenv func(string) string) error {
	if _, err := Initialize(path); err != nil {
		return err
	}
	command := SelectEditor(getenv, runtime.GOOS)
	if len(command) == 0 {
		return errors.New("no editor command configured")
	}
	args := append(command[1:], path)
	cmd := exec.Command(command[0], args...) // #nosec G204 -- user $VISUAL/$EDITOR or platform default
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
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
