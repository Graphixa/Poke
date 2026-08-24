package sprite

import (
	"os"
	"strings"
)

// Capabilities describes terminal rendering features detected once at startup.
// Detection chooses a rendering class, not a terminal brand.
type Capabilities struct {
	NativeImage bool
	TrueColor   bool
	ANSI256     bool
}

// DetectCapabilities builds a conservative capability snapshot from the
// environment. Uncertain native-image support is reported as false.
// nativeImage should be true only when a native Image renderer is available.
func DetectCapabilities(nativeImage bool) Capabilities {
	trueColor := detectTrueColor()
	ansi256 := trueColor || detectANSI256()
	return Capabilities{
		NativeImage: nativeImage,
		TrueColor:   trueColor,
		ANSI256:     ansi256,
	}
}

func detectTrueColor() bool {
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	if ct == "truecolor" || ct == "24bit" {
		return true
	}
	term := strings.ToLower(os.Getenv("TERM"))
	if strings.Contains(term, "truecolor") || term == "xterm-kitty" || term == "wezterm" {
		return true
	}
	// Modern GUI terminals support 24-bit colour even when TERM is dumb/empty
	// (common under Windows Terminal, ConEmu, and some IDE hosts).
	if modernColorTerminal() {
		return true
	}
	return false
}

func detectANSI256() bool {
	if modernColorTerminal() {
		return true
	}
	term := strings.ToLower(os.Getenv("TERM"))
	if term == "" || term == "dumb" {
		return false
	}
	// Most modern TERM values support 256 colours; be conservative for classic.
	switch term {
	case "vt100", "vt102", "ansi", "linux":
		return false
	}
	return true
}

// modernColorTerminal reports hosts that are known to support rich colour
// output regardless of a missing/dumb TERM value.
func modernColorTerminal() bool {
	if os.Getenv("WT_SESSION") != "" {
		return true
	}
	if os.Getenv("ConEmuANSI") != "" || os.Getenv("ConEmuPID") != "" {
		return true
	}
	switch strings.ToLower(os.Getenv("TERM_PROGRAM")) {
	case "windowsterminal", "wezterm", "iterm.app", "ghostty", "alacritty", "vscode", "warpterminal":
		return true
	}
	return false
}
