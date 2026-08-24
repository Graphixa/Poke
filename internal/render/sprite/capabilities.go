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
	Unicode     bool
	Braille     bool
}

// DetectCapabilities builds a conservative capability snapshot from the
// environment. Uncertain native-image support is reported as false.
// nativeImage should be true only when a native Image renderer is available.
func DetectCapabilities(nativeImage bool) Capabilities {
	trueColor := detectTrueColor()
	ansi256 := trueColor || detectANSI256()
	unicode := detectUnicode()
	return Capabilities{
		NativeImage: nativeImage,
		TrueColor:   trueColor,
		ANSI256:     ansi256,
		Unicode:     unicode,
		// Braille needs Unicode; assume modern terminals that advertise UTF-8.
		Braille: unicode,
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
	return false
}

func detectANSI256() bool {
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

func detectUnicode() bool {
	for _, key := range []string{"LANG", "LC_ALL", "LC_CTYPE"} {
		v := strings.ToLower(os.Getenv(key))
		if strings.Contains(v, "utf-8") || strings.Contains(v, "utf8") {
			return true
		}
	}
	// Windows terminals and most modern hosts are UTF-8 capable.
	if os.Getenv("WT_SESSION") != "" || os.Getenv("TERM_PROGRAM") != "" {
		return true
	}
	term := strings.ToLower(os.Getenv("TERM"))
	if term == "" || term == "dumb" {
		return false
	}
	return true
}
