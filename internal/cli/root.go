// Package cli defines the command surface and delegates product behavior to an
// injected application handler.
package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/pkg/version"
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
	Stdout         io.Writer
	Stderr         io.Writer
	// OnDebug is invoked when --debug is set. path may be empty for the default log file.
	OnDebug func(path string) error
}

// CacheCommands handles cache maintenance subcommands.
type CacheCommands interface {
	Refresh(context.Context) error
	Clear() error
	WarmSprites(context.Context) error
	Update(context.Context) error
}

type CacheCommandsFuncs struct {
	RefreshFunc     func(context.Context) error
	ClearFunc       func() error
	WarmSpritesFunc func(context.Context) error
	UpdateFunc      func(context.Context) error
}

func (f CacheCommandsFuncs) Refresh(ctx context.Context) error {
	if f.RefreshFunc == nil {
		return fmt.Errorf("cache refresh is unavailable")
	}
	return f.RefreshFunc(ctx)
}

func (f CacheCommandsFuncs) Clear() error {
	if f.ClearFunc == nil {
		return fmt.Errorf("cache clear is unavailable")
	}
	return f.ClearFunc()
}

func (f CacheCommandsFuncs) WarmSprites(ctx context.Context) error {
	if f.WarmSpritesFunc == nil {
		return fmt.Errorf("cache warm-sprites is unavailable")
	}
	return f.WarmSpritesFunc(ctx)
}

func (f CacheCommandsFuncs) Update(ctx context.Context) error {
	if f.UpdateFunc == nil {
		return fmt.Errorf("poke update is unavailable")
	}
	return f.UpdateFunc(ctx)
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
		Use:   "refresh",
		Short: "Rebuild the local metadata dataset snapshot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if options.Cache == nil {
				return fmt.Errorf("cache refresh is unavailable")
			}
			return options.Cache.Refresh(cmd.Context())
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
	parent.AddCommand(&cobra.Command{
		Use:   "warm-sprites",
		Short: "Download official artwork PNGs into the local cache",
		Long:  "Fetches official-artwork PNGs for the national dex into the user cache. Does not ship or redistribute sprites; all work is local.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if options.Cache == nil {
				return fmt.Errorf("cache warm-sprites is unavailable")
			}
			return options.Cache.WarmSprites(cmd.Context())
		},
	})
	return parent
}

func newUpdateCommand(options Options) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Refresh index and sync official artwork",
		Long:  "Refreshes the species index and type enrichment, then downloads new or changed official artwork into the app cache. Native Kitty/iTerm/Sixel frame caches are built in the background while browsing, not during update.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if options.Cache == nil {
				return fmt.Errorf("poke update is unavailable")
			}
			return options.Cache.Update(cmd.Context())
		},
	}
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
