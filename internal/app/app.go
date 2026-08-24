package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sync"

	"github.com/Graphixa/Pokedex/internal/cli"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/theme"
	"github.com/Graphixa/Pokedex/internal/usererr"
)

type Runtime struct {
	InvocationPath string
	Args           []string
	Getenv         func(string) string
	Directories    config.DirectoryResolver
	Editor         config.CommandRunner
	Handler        cli.RootHandler
	Search         cli.SearchHandler
	Random         cli.RandomHandler
	Cache          cli.CacheCommands
	Theme          cli.ThemeHandler
	Rand           IntnRand
	Stdout         io.Writer
	Stderr         io.Writer
	Test           cli.TestHandler
	// DebugLogPath enables file diagnostics when non-nil after CLI parse.
	// Prefer wiring via cli persistent --debug; this field is for tests.
	DebugLogPath string
}

func OSRuntime() Runtime {
	return Runtime{
		InvocationPath: os.Args[0],
		Args:           os.Args[1:],
		Getenv:         os.Getenv,
		Directories:    config.OSDirectoryResolver(),
		Editor:         config.ExecRunner{},
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
		Rand:           rand.New(rand.NewSource(rand.Int63())),
	}
}

func Execute(ctx context.Context, runtime Runtime) error {
	paths, err := runtime.Directories.Resolve()
	if err != nil {
		return err
	}
	services := lazyServices(paths)
	cacheCmds := runtime.Cache
	if cacheCmds == nil {
		cacheCmds = cacheCommands{
			services: services,
			stdout:   runtime.Stdout,
			stderr:   runtime.Stderr,
		}
	}

	handler := runtime.Handler
	searchHandler := runtime.Search
	randomHandler := runtime.Random
	themeHandler := runtime.Theme
	testHandler := runtime.Test
	if handler == nil || searchHandler == nil || randomHandler == nil || themeHandler == nil || testHandler == nil {
		appHandler := &Handler{
			Services: services,
			Stdout:   runtime.Stdout,
			Stderr:   runtime.Stderr,
			Rand:     runtime.Rand,
			Paths:    paths,
		}
		if appHandler.Browse == nil {
			appHandler.Browse = appHandler.openBrowse
		}
		if handler == nil {
			handler = appHandler
		}
		if searchHandler == nil {
			searchHandler = appHandler
		}
		if randomHandler == nil {
			randomHandler = appHandler
		}
		if themeHandler == nil {
			themeHandler = appHandler
		}
		if testHandler == nil {
			testHandler = appHandler
		}
	}

	// Apply configured theme before any TUI runs.
	if svc, err := services(); err == nil {
		ApplyTheme(paths, svc.Config)
	}

	root := cli.NewRoot(cli.Options{
		InvocationPath: runtime.InvocationPath,
		Paths:          paths,
		Getenv:         runtime.Getenv,
		Editor:         runtime.Editor,
		Handler:        handler,
		Search:         searchHandler,
		Random:         randomHandler,
		Cache:          cacheCmds,
		Theme:          themeHandler,
		Test:           testHandler,
		Stdout:         runtime.Stdout,
		Stderr:         runtime.Stderr,
		OnDebug: func(path string) error {
			resolved, err := debuglog.Enable(path)
			if err != nil {
				return err
			}
			if runtime.Stderr != nil {
				fmt.Fprintf(runtime.Stderr, "Debug log: %s\n", resolved)
			}
			return nil
		},
	})
	root.SetArgs(runtime.Args)
	return root.ExecuteContext(ctx)
}

// ApplyTheme loads and installs the configured colour theme.
func ApplyTheme(paths config.Paths, cfg config.Config) {
	dir := theme.ThemesDir(filepath.Dir(paths.ConfigFile))
	f, _, err := theme.Resolve(dir, cfg.Theme.Name)
	if err != nil {
		return
	}
	theme.Apply(f)
}

func lazyServices(paths config.Paths) func() (*Services, error) {
	var (
		once sync.Once
		svc  *Services
		err  error
	)
	return func() (*Services, error) {
		once.Do(func() {
			svc, err = LoadServices(paths)
		})
		return svc, err
	}
}

type exitCoder interface {
	ExitCode() int
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var coded exitCoder
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	return 1
}

func Main(ctx context.Context, runtime Runtime) int {
	err := Execute(ctx, runtime)
	if err == nil {
		return 0
	}
	if runtime.Stderr != nil {
		fmt.Fprintf(runtime.Stderr, "Error: %s\n", usererr.Format(err))
	}
	return ExitCode(err)
}
