package sprite_test

import (
	"testing"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spritechafa "github.com/Graphixa/Pokedex/internal/render/sprite/chafa"
	"github.com/Graphixa/Pokedex/internal/render/sprite/placeholder"
)

func TestDetectCapabilitiesWindowsTerminalDumbTERM(t *testing.T) {
	t.Setenv("TERM", "dumb")
	t.Setenv("COLORTERM", "")
	t.Setenv("TERM_PROGRAM", "WindowsTerminal")
	t.Setenv("WT_SESSION", "")
	t.Setenv("ConEmuANSI", "")
	t.Setenv("ConEmuPID", "")

	caps := sprite.DetectCapabilities(false)
	if !caps.TrueColor {
		t.Fatalf("TrueColor = false, want true for Windows Terminal")
	}
	if !caps.ANSI256 {
		t.Fatalf("ANSI256 = false, want true for Windows Terminal")
	}
}

func TestDetectCapabilitiesWTSession(t *testing.T) {
	t.Setenv("TERM", "")
	t.Setenv("COLORTERM", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("WT_SESSION", "01234567-89ab-cdef-0123-456789abcdef")
	t.Setenv("ConEmuANSI", "")
	t.Setenv("ConEmuPID", "")

	caps := sprite.DetectCapabilities(false)
	if !caps.TrueColor || !caps.ANSI256 {
		t.Fatalf("caps=%+v, want colour for WT_SESSION", caps)
	}
}

func TestDetectCapabilitiesDumbWithoutHost(t *testing.T) {
	t.Setenv("TERM", "dumb")
	t.Setenv("COLORTERM", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("WT_SESSION", "")
	t.Setenv("ConEmuANSI", "")
	t.Setenv("ConEmuPID", "")

	caps := sprite.DetectCapabilities(false)
	if caps.TrueColor || caps.ANSI256 {
		t.Fatalf("caps=%+v, want no colour for dumb TERM without host hints", caps)
	}
}

func TestModeResolverForcedChafaWithoutColorCaps(t *testing.T) {
	modes := sprite.NewModeResolver(
		spritechafa.Renderer{},
		placeholder.Renderer{},
	)
	caps := sprite.Capabilities{}
	r := modes.Resolve(caps, sprite.ModeChafa)
	if r == nil || r.Name() != "chafa" {
		t.Fatalf("forced chafa = %v, want chafa even without colour caps", r)
	}
}

func TestModeResolverAutoWindowsLikeCaps(t *testing.T) {
	modes := sprite.NewModeResolver(
		spritechafa.Renderer{},
		placeholder.Renderer{},
	)
	caps := sprite.Capabilities{TrueColor: true, ANSI256: true}
	r := modes.Resolve(caps, sprite.ModeAuto)
	if r == nil || r.Name() != "chafa" {
		t.Fatalf("auto = %v, want chafa", r)
	}
}
