// Package cli defines the command surface and delegates product behavior to an
// injected application handler.
package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/pkg/version"
	"github.com/spf13/cobra"
)

type InlineMode string

const (
	InlineNone      InlineMode = ""
	InlineInfo      InlineMode = "info"
	InlineStats     InlineMode = "stats"
	InlineEvolution InlineMode = "evolution"
)

type RootRequest struct {
	Target     string
	InlineMode InlineMode
	Invocation string
}

type Options struct {
	InvocationPath string
	Paths          config.Paths
	Getenv         func(string) string
	App            AppHandler
	Cache          CacheCommands
	Updater        Updater
	Stdout         io.Writer
	Stderr         io.Writer
	Stdin          io.Reader
	// OnDebug is invoked when --debug is set. path may be empty for the default log file.
	OnDebug func(path string) error
}

// CacheCommands handles cache maintenance subcommands.
type CacheCommands interface {
	Clear() error
	Refresh(context.Context) error
}

// Updater installs poke binary updates from GitHub Releases.
type Updater interface {
	Check(ctx context.Context) (current, latest string, needsUpdate bool, err error)
	ApplyLatest(ctx context.Context) error
	ApplyVersion(ctx context.Context, version string) error
	RecordCheck(lastChecked, nextCheck string) error
	CheckIntervalHours() int
}

func NewRoot(options Options) *cobra.Command {
	invocation := InvocationName(options.InvocationPath)
	cmd := &cobra.Command{
		Use:           "poke [pokemon-name-or-number]",
		Short:         "Browse and inspect Pokémon from your terminal",
		Long:          "Browse and inspect Pokémon from your terminal.\n\nThe canonical executable is poke; pokedex is an equivalent alias.",
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version.String(),
	}
	cmd.SetVersionTemplate("{{.Version}}\n")
	cmd.SetOut(options.Stdout)
	cmd.SetErr(options.Stderr)

	var info, stats, evolution bool
	var debugFlag string
	cmd.Flags().BoolVar(&info, "info", false, "print compact Pokémon information")
	cmd.Flags().BoolVar(&stats, "stats", false, "print base stats")
	cmd.Flags().BoolVar(&evolution, "evolution", false, "print the evolution tree")
	cmd.PersistentFlags().StringVar(&debugFlag, "debug", "", "write request/cache/render diagnostics to a file (optional path)")
	// Allow --debug with no value → default log path under the OS temp directory.
	cmd.PersistentFlags().Lookup("debug").NoOptDefVal = " "
	cmd.MarkFlagsMutuallyExclusive("info", "stats", "evolution")
	cmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		flag := cmd.Flags().Lookup("debug")
		if flag == nil || !flag.Changed {
			return nil
		}
		path := strings.TrimSpace(debugFlag)
		if options.OnDebug == nil {
			return fmt.Errorf("debug logging is unavailable")
		}
		return options.OnDebug(path)
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		mode := InlineNone
		switch {
		case info:
			mode = InlineInfo
		case stats:
			mode = InlineStats
		case evolution:
			mode = InlineEvolution
		}
		target := ""
		if len(args) == 1 {
			target = args[0]
		}
		if mode != InlineNone && target == "" {
			return fmt.Errorf("--%s requires a Pokémon name or number", mode)
		}
		if options.App == nil {
			return nil
		}
		return options.App.RunRoot(cmd.Context(), RootRequest{
			Target: target, InlineMode: mode, Invocation: invocation,
		})
	}

	cmd.AddCommand(newVersionCommand(options.Stdout))
	cmd.AddCommand(newConfigCommand(options))
	cmd.AddCommand(newCacheCommand(options))
	cmd.AddCommand(newRefreshCommand(options))
	cmd.AddCommand(newUpdateCommand(options))
	cmd.AddCommand(newSearchCommand(options))
	cmd.AddCommand(newRandomCommand(options))
	cmd.AddCommand(newThemeCommand(options))
	return cmd
}

func newSearchCommand(options Options) *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search the species index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.App == nil {
				return fmt.Errorf("search is unavailable")
			}
			return options.App.Search(cmd.Context(), args[0])
		},
	}
}

func newRandomCommand(options Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "random",
		Short: "Pick a random Pokémon from the species index",
		Args:  cobra.NoArgs,
	}
	var info, stats, evolution bool
	cmd.Flags().BoolVar(&info, "info", false, "print compact Pokémon information")
	cmd.Flags().BoolVar(&stats, "stats", false, "print base stats")
	cmd.Flags().BoolVar(&evolution, "evolution", false, "print the evolution tree")
	cmd.MarkFlagsMutuallyExclusive("info", "stats", "evolution")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		mode := InlineNone
		switch {
		case info:
			mode = InlineInfo
		case stats:
			mode = InlineStats
		case evolution:
			mode = InlineEvolution
		}
		if options.App == nil {
			return fmt.Errorf("random is unavailable")
		}
		return options.App.Random(cmd.Context(), mode)
	}
	return cmd
}

func newVersionCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			_, err := fmt.Fprintln(stdout, version.String())
			return err
		},
	}
}

func newConfigCommand(options Options) *cobra.Command {
	parent := &cobra.Command{Use: "config", Short: "Manage configuration", Args: cobra.NoArgs}
	parent.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the configuration file path",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			_, err := fmt.Fprintln(options.Stdout, options.Paths.ConfigFile)
			return err
		},
	})
	parent.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create a default configuration if missing",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			created, err := config.Initialize(options.Paths.ConfigFile)
			if err != nil {
				return err
			}
			if created {
				_, err = fmt.Fprintf(options.Stdout, "Created %s\n", options.Paths.ConfigFile)
			} else {
				_, err = fmt.Fprintf(options.Stdout, "Configuration already exists at %s\n", options.Paths.ConfigFile)
			}
			return err
		},
	})
	parent.AddCommand(&cobra.Command{
		Use:   "edit",
		Short: "Open the configuration in your editor",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if options.Getenv == nil {
				return fmt.Errorf("environment lookup is unavailable")
			}
			return config.Edit(options.Paths.ConfigFile, options.Getenv)
		},
	})
	return parent
}

func newCacheCommand(options Options) *cobra.Command {
	parent := &cobra.Command{Use: "cache", Short: "Manage cached data", Args: cobra.NoArgs}
	parent.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the cache directory path",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			_, err := fmt.Fprintln(options.Stdout, options.Paths.CacheDir)
			return err
		},
	})
	parent.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "Delete the application cache directory",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if options.Cache == nil {
				return fmt.Errorf("cache clear is unavailable")
			}
			return options.Cache.Clear()
		},
	})
	return parent
}

func newRefreshCommand(options Options) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Refresh local dataset and sync official artwork",
		Long:  "Rebuilds the species dataset snapshot and downloads new or changed official artwork into the app cache. Native Kitty/iTerm/Sixel frame caches are built in the background while browsing, not during refresh.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if options.Cache == nil {
				return fmt.Errorf("poke refresh is unavailable")
			}
			return options.Cache.Refresh(cmd.Context())
		},
	}
}

func newUpdateCommand(options Options) *cobra.Command {
	var checkOnly, autoYes bool
	var targetVersion string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update poke to the latest release",
		Long:  "Checks GitHub Releases for a newer poke binary, verifies checksums, and replaces the installed executable.\n\nUse --check to inspect only, -y to skip confirmation, or --version for a specific release.",
		Example: `  poke update
  poke update --check
  poke update -y
  poke update --version 1.2.3`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if options.Updater == nil {
				return fmt.Errorf("poke update is unavailable")
			}
			return runUpdateCommand(cmd.Context(), options, checkOnly, autoYes, targetVersion)
		},
	}
	cmd.Flags().BoolVarP(&checkOnly, "check", "c", false, "Check only; do not install")
	cmd.Flags().BoolVarP(&autoYes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().StringVar(&targetVersion, "version", "", "Update to a specific version (e.g. 1.2.3)")
	return cmd
}

func newThemeCommand(options Options) *cobra.Command {
	return &cobra.Command{
		Use:   "theme",
		Short: "Interactive theme selector with live preview",
		Long:  "Launch an interactive TUI to select and preview colour themes (FontGet-style).",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if options.App == nil {
				return fmt.Errorf("theme selector is unavailable")
			}
			return options.App.Theme(cmd.Context())
		},
	}
}

func InvocationName(path string) string {
	path = strings.ReplaceAll(path, `\`, "/")
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(path)), ".exe")
	if name == "pokedex" {
		return "pokedex"
	}
	return "poke"
}
