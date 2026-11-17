package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Graphixa/Poke/pkg/version"
	"golang.org/x/mod/semver"
)

// executablePath resolves the current binary. Tests override this.
var executablePath = os.Executable

// Result is the outcome of checking GitHub Releases for a newer binary.
type Result struct {
	Available   bool
	Current     string
	Latest      string
	NeedsUpdate bool
}

// CheckForUpdates reports whether a newer release is available.
func CheckForUpdates(ctx context.Context) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	currentVersionStr := version.Version
	client, err := newReleaseClient()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize updater: %w", err)
	}
	latestVersion, err := client.latestVersion(ctx)
	if err != nil {
		if errors.Is(err, errReleaseNotFound) {
			return &Result{
				Available:   false,
				Current:     currentVersionStr,
				Latest:      "",
				NeedsUpdate: false,
			}, nil
		}
		return nil, fmt.Errorf("failed to check for updates: %w", mapLibraryError(err))
	}

	currentVersion, err := parseVersion(currentVersionStr)
	if err != nil {
		return &Result{
			Available:   true,
			Current:     currentVersionStr,
			Latest:      latestVersion,
			NeedsUpdate: latestVersion != currentVersionStr,
		}, nil
	}

	return &Result{
		Available:   true,
		Current:     currentVersionStr,
		Latest:      latestVersion,
		NeedsUpdate: versionGreater(latestVersion, currentVersion),
	}, nil
}

// UpdateToLatest downloads and installs the latest release binary.
func UpdateToLatest(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	client, err := newReleaseClient()
	if err != nil {
		return fmt.Errorf("failed to initialize updater: %w", err)
	}
	latestVersion, err := client.latestVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", mapLibraryError(err))
	}
	return applyVersion(ctx, client, latestVersion)
}

// UpdateToVersion downloads and installs a specific release version.
func UpdateToVersion(ctx context.Context, targetVersion string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	targetSemver, err := parseVersion(targetVersion)
	if err != nil {
		return fmt.Errorf("invalid version format '%s': %w", targetVersion, err)
	}
	client, err := newReleaseClient()
	if err != nil {
		return fmt.Errorf("failed to initialize updater: %w", err)
	}
	return applyVersion(ctx, client, targetSemver)
}

func applyVersion(ctx context.Context, client *releaseClient, releaseVersion string) error {
	archiveName := currentArchiveName(releaseVersion)
	checksumBytes, err := client.checksums(ctx, releaseVersion)
	if err != nil {
		return mapLibraryError(fmt.Errorf("failed to download checksums.txt: %w", err))
	}
	expected, err := checksumForFile(checksumBytes, archiveName)
	if err != nil {
		return err
	}

	archiveBytes, err := client.archive(ctx, releaseVersion, archiveName)
	if err != nil {
		return mapLibraryError(fmt.Errorf("failed to download %s: %w", archiveName, err))
	}
	if err := verifySHA256(archiveBytes, expected); err != nil {
		return mapLibraryError(err)
	}

	binary, err := extractBinary(archiveBytes, archiveName)
	if err != nil {
		return fmt.Errorf("failed to extract binary from %s: %w", archiveName, err)
	}

	cmdPath, err := executablePath()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	if err := replaceExecutable(cmdPath, binary); err != nil {
		return mapLibraryError(err)
	}
	cleanupOldBinary(cmdPath)
	return nil
}

func cleanupOldBinary(execPath string) {
	oldPath := execPath + ".old"
	if _, err := os.Stat(oldPath); err == nil {
		_ = os.Remove(oldPath)
	}
}

func parseVersion(versionStr string) (string, error) {
	trimmed := strings.TrimSpace(versionStr)
	if trimmed == "" {
		return "", fmt.Errorf("empty version")
	}
	if trimmed == "dev" || strings.HasPrefix(trimmed, "dev-") || strings.HasPrefix(trimmed, "dev+") {
		return "0.0.0", nil
	}
	mod := toModVersion(trimmed)
	if !semver.IsValid(mod) {
		return "", fmt.Errorf("invalid semantic version %q", versionStr)
	}
	return strings.TrimPrefix(semver.Canonical(mod), "v"), nil
}

func toModVersion(versionStr string) string {
	trimmed := strings.TrimSpace(versionStr)
	trimmed = strings.TrimPrefix(trimmed, "v")
	return "v" + trimmed
}

func versionGreater(a, b string) bool {
	return semver.Compare(toModVersion(a), toModVersion(b)) > 0
}

func mapLibraryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, errReleaseNotFound) {
		return fmt.Errorf("release not found: the specified version may not exist")
	}

	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "permission denied") || strings.Contains(errStr, "Access is denied"):
		return fmt.Errorf("insufficient permissions: try running as administrator/sudo")
	case strings.Contains(errStr, "file is locked") || strings.Contains(errStr, "being used by another process"):
		return fmt.Errorf("poke is currently running: please close other instances and try again")
	case strings.Contains(errStr, "checksum verification failed"):
		return fmt.Errorf("download verification failed: the downloaded file may be corrupted")
	case strings.Contains(errStr, "403") || strings.Contains(errStr, "forbidden"):
		return fmt.Errorf("access denied: GitHub releases may be unavailable")
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "connection") || strings.Contains(errStr, "timeout") || strings.Contains(errStr, "DeadlineExceeded"):
		return fmt.Errorf("network error: check your internet connection and try again")
	}
	return fmt.Errorf("update failed: %w", err)
}
