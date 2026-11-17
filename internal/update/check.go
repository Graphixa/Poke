package update

import (
	"context"
	"time"

	"github.com/Graphixa/Pokedex/pkg/version"
)

// UpdateDeclinedGracePeriodHours is used when UpdateCheckInterval is 0/missing.
const UpdateDeclinedGracePeriodHours = 24

// CheckResult is the startup-check callback payload.
type CheckResult struct {
	UpdateAvailable bool
	CurrentVersion  string
	LatestVersion   string
	Error           error
}

// ShouldCheckForUpdates reports whether an interval-based check is due.
func ShouldCheckForUpdates(checkForUpdates bool, checkInterval int, lastChecked string) bool {
	if !checkForUpdates {
		return false
	}
	if lastChecked == "" {
		return true
	}
	lastCheckedTime, err := time.Parse(time.RFC3339, lastChecked)
	if err != nil {
		return true
	}
	interval := time.Duration(checkInterval) * time.Hour
	return time.Since(lastCheckedTime) >= interval
}

// PerformStartupCheck runs a non-blocking update check. Always invokes callback
// when a check is performed so LastUpdateCheck can be persisted.
func PerformStartupCheck(ctx context.Context, checkForUpdates bool, checkInterval int, lastChecked string, callback func(*CheckResult)) {
	if !ShouldCheckForUpdates(checkForUpdates, checkInterval, lastChecked) {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	result, err := CheckForUpdates(ctx)
	checkResult := &CheckResult{
		UpdateAvailable: false,
		CurrentVersion:  version.Version,
		LatestVersion:   "",
		Error:           err,
	}
	if err == nil && result != nil {
		checkResult.UpdateAvailable = result.NeedsUpdate
		checkResult.CurrentVersion = result.Current
		checkResult.LatestVersion = result.Latest
		checkResult.Error = nil
	}
	if callback != nil {
		callback(checkResult)
	}
}

// GetLastCheckedTimestamp returns now as RFC3339 UTC.
func GetLastCheckedTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// ShouldShowUpdatePrompt is true when NextUpdateCheck has elapsed (or is empty).
func ShouldShowUpdatePrompt(nextUpdateCheck string) bool {
	if nextUpdateCheck == "" {
		return true
	}
	until, err := time.Parse(time.RFC3339, nextUpdateCheck)
	if err != nil {
		return true
	}
	return time.Now().UTC().After(until)
}

// GetUpdateDeclinedUntilTimestamp returns when prompts may resume after a decline.
func GetUpdateDeclinedUntilTimestamp(intervalHours int) string {
	hours := intervalHours
	if hours <= 0 {
		hours = UpdateDeclinedGracePeriodHours
	}
	return time.Now().UTC().Add(time.Duration(hours) * time.Hour).Format(time.RFC3339)
}
