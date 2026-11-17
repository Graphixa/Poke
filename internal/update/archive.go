package update

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// currentGOOS / currentGOARCH select the release archive. Tests override these.
var (
	currentGOOS   = runtime.GOOS
	currentGOARCH = runtime.GOARCH
)

// executableBaseName returns the release binary basename to extract (poke / poke.exe).
// Tests may override via executablePath returning a custom name.
var executableBaseName = func() string {
	path, err := executablePath()
	if err != nil || path == "" {
		if currentGOOS == "windows" {
			return "poke.exe"
		}
		return "poke"
	}
	base := filepath.Base(path)
	lower := strings.ToLower(base)
	switch lower {
	case "poke", "poke.exe", "pokedex", "pokedex.exe":
		return base
	default:
		if currentGOOS == "windows" {
			return "poke.exe"
		}
		return "poke"
	}
}

// ReleaseArchiveName matches archives.name_template: poke_{{ .Version }}_{{ .Os }}_{{ .Arch }}.
func ReleaseArchiveName(version, goos, goarch string) string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("poke_%s_%s_%s%s", version, goos, goarch, ext)
}

func currentArchiveName(version string) string {
	return ReleaseArchiveName(version, currentGOOS, currentGOARCH)
}

func currentExecutableName() string {
	return executableBaseName()
}
