// Builds a dataset directory from a local PokeAPI/api-data checkout by
// serving those files to the existing bootstrap. From the repo root:
//
//	go run ./scripts/snapshot -root <api-data> -cache <empty-dir>
//
// Prints the finished dataset directory. Pack it with scripts/pack_dataset.go.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/pokeapi"
)

func main() {
	root := flag.String("root", "", "api-data checkout")
	cacheDir := flag.String("cache", "", "empty directory for the build")
	flag.Parse()
	if *root == "" || *cacheDir == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/snapshot -root <api-data> -cache <dir>")
		os.Exit(2)
	}
	dir, err := buildSnapshot(context.Background(), *root, *cacheDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(dir)
}

func buildSnapshot(ctx context.Context, root, cacheDir string) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	srv := &http.Server{
		Handler:           apiDataHandler(root),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	client := pokeapi.NewClient(pokeapi.Options{
		BaseURL: "http://" + ln.Addr().String() + "/api/v2",
		Retries: 1,
		HTTPClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
	})
	cfg := config.Default()
	cfg.Network.MaxConcurrency = 8
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return "", err
	}
	store := cache.NewStore(cacheDir, nil)
	svc := cache.NewService(store, client, cfg, nil)
	if err := svc.BootstrapDataset(ctx, nil); err != nil {
		return "", err
	}
	id, err := store.ActiveDatasetID()
	if err != nil || id == "" {
		return "", fmt.Errorf("snapshot: no active dataset")
	}
	return filepath.Join(cacheDir, "datasets", id), nil
}
