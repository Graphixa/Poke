package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Graphixa/Pokedex/internal/update"
	"golang.org/x/mod/semver"
)

func runUpdateCommand(ctx context.Context, options Options, checkOnly, autoYes bool, targetVersion string) error {
	u := options.Updater
	out := options.Stdout
	if out == nil {
		out = io.Discard
	}

	if targetVersion != "" {
		return updateToVersion(ctx, options, targetVersion, autoYes)
	}

	current, latest, needsUpdate, err := u.Check(ctx)
	_ = u.RecordCheck(update.GetLastCheckedTimestamp(), "")
	if err != nil {
		return fmt.Errorf("unable to check for updates: %w", err)
	}
	if latest == "" {
		fmt.Fprintln(out, "No releases found on GitHub.")
		return nil
	}
	if !needsUpdate {
		fmt.Fprintf(out, "poke is up to date (v%s)\n", displayVersion(current))
		return nil
	}

	fmt.Fprintf(out, "Installed: v%s\n", displayVersion(current))
	fmt.Fprintf(out, "Available: v%s\n", displayVersion(latest))
	fmt.Fprintf(out, "\nChangelog: https://github.com/Graphixa/Pokedex/releases/tag/v%s\n", displayVersion(latest))

	if checkOnly {
		return nil
	}
	if !autoYes {
		ok, err := confirmUpdate(options, fmt.Sprintf("Update poke from v%s to v%s?", displayVersion(current), displayVersion(latest)))
		if err != nil {
			return err
		}
		if !ok {
			next := update.GetUpdateDeclinedUntilTimestamp(u.CheckIntervalHours())
			_ = u.RecordCheck(update.GetLastCheckedTimestamp(), next)
			fmt.Fprintln(out, "Update cancelled.")
			return nil
		}
	}
	fmt.Fprintf(out, "Updating poke from v%s to v%s...\n", displayVersion(current), displayVersion(latest))
	if err := u.ApplyLatest(ctx); err != nil {
		return err
	}
	fmt.Fprintf(out, "Successfully updated to poke v%s\n", displayVersion(latest))
	return nil
}

func updateToVersion(ctx context.Context, options Options, targetVersion string, autoYes bool) error {
	u := options.Updater
	out := options.Stdout
	if out == nil {
		out = io.Discard
	}
	current, _, _, err := u.Check(ctx)
	if err != nil {
		// Still allow targeting a known version offline-ish; Check failure is soft for --version path.
		current = ""
	}
	_ = u.RecordCheck(update.GetLastCheckedTimestamp(), "")

	target := displayVersion(targetVersion)
	curr := displayVersion(current)
	if curr != "" && target == curr {
		fmt.Fprintf(out, "poke is already at v%s\n", curr)
		return nil
	}

	isDowngrade := false
	if curr != "" {
		c := "v" + curr
		tg := "v" + target
		if semver.IsValid(c) && semver.IsValid(tg) && semver.Compare(tg, c) < 0 {
			isDowngrade = true
		}
	}

	fmt.Fprintf(out, "Installed: v%s\n", curr)
	fmt.Fprintf(out, "Available: v%s\n", target)
	if isDowngrade {
		fmt.Fprintln(out, "Warning: downgrading may cause config or cache issues.")
	}
	if isDowngrade || !autoYes {
		ok, err := confirmUpdate(options, fmt.Sprintf("Install poke v%s?", target))
		if err != nil {
			return err
		}
		if !ok {
			next := update.GetUpdateDeclinedUntilTimestamp(u.CheckIntervalHours())
			_ = u.RecordCheck(update.GetLastCheckedTimestamp(), next)
			fmt.Fprintln(out, "Update cancelled.")
			return nil
		}
	}
	fmt.Fprintf(out, "Updating poke to v%s...\n", target)
	if err := u.ApplyVersion(ctx, target); err != nil {
		return err
	}
	fmt.Fprintf(out, "Successfully updated to poke v%s\n", target)
	return nil
}

func confirmUpdate(options Options, prompt string) (bool, error) {
	in := options.Stdin
	out := options.Stdout
	if out == nil {
		out = io.Discard
	}
	if in == nil {
		return false, fmt.Errorf("no stdin for confirmation; re-run with --yes")
	}
	fmt.Fprintf(out, "%s [y/N] ", prompt)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && len(strings.TrimSpace(line)) == 0 {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func displayVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}
