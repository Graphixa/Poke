//go:build ignore

// Generates types.json by probing PokeAPI for each national-dex pokemon id.
// Usage: go run ./scripts/gen_typeseed.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	speciesURL = "https://pokeapi.co/api/v2/pokemon-species?limit=2000"
	workers    = 16
)

type namedURL struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type listResp struct {
	Results []namedURL `json:"results"`
}

type pokemonDTO struct {
	ID    int `json:"id"`
	Types []struct {
		Slot int `json:"slot"`
		Type namedURL `json:"type"`
	} `json:"types"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 20 * time.Second}

	ids, err := listSpeciesIDs(ctx, client)
	if err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "species=%d\n", len(ids))

	out := make(map[string][]string, len(ids))
	var mu sync.Mutex
	var done atomic.Int64
	ch := make(chan int, workers*2)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range ch {
				types, err := fetchTypes(ctx, client, id)
				if err != nil {
					fmt.Fprintf(os.Stderr, "id=%d err=%v\n", id, err)
					continue
				}
				mu.Lock()
				out[strconv.Itoa(id)] = types
				mu.Unlock()
				n := done.Add(1)
				if n%50 == 0 || int(n) == len(ids) {
					fmt.Fprintf(os.Stderr, "types %d/%d\n", n, len(ids))
				}
			}
		}()
	}
	for _, id := range ids {
		ch <- id
	}
	close(ch)
	wg.Wait()

	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, _ := strconv.Atoi(keys[i])
		b, _ := strconv.Atoi(keys[j])
		return a < b
	})
	ordered := make(map[string][]string, len(out))
	for _, k := range keys {
		ordered[k] = out[k]
	}

	data, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		fatal(err)
	}
	data = append(data, '\n')
	dest := filepath.Join("internal", "cache", "typeseed", "types.json")
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d entries)\n", dest, len(ordered))
}

func listSpeciesIDs(ctx context.Context, client *http.Client) ([]int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, speciesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "poke-cli-gen-typeseed")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("species list status %d", resp.StatusCode)
	}
	var body listResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(body.Results))
	for i := range body.Results {
		ids = append(ids, i+1) // national order matches list order for standard dex
	}
	// Prefer parsing id from URL when present; fallback to 1..N.
	for i, r := range body.Results {
		if id := idFromURL(r.URL); id > 0 {
			ids[i] = id
		}
	}
	return ids, nil
}

func idFromURL(u string) int {
	// .../pokemon-species/25/ or .../pokemon-species/25
	for i := len(u) - 1; i >= 0; i-- {
		if u[i] == '/' {
			if i == len(u)-1 {
				u = u[:i]
				continue
			}
			n, err := strconv.Atoi(u[i+1:])
			if err == nil {
				return n
			}
			return 0
		}
	}
	return 0
}

func fetchTypes(ctx context.Context, client *http.Client, id int) ([]string, error) {
	url := fmt.Sprintf("https://pokeapi.co/api/v2/pokemon/%d", id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "poke-cli-gen-typeseed")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var dto pokemonDTO
	if err := json.NewDecoder(resp.Body).Decode(&dto); err != nil {
		return nil, err
	}
	types := make([]string, 0, len(dto.Types))
	// Preserve slot order.
	sort.Slice(dto.Types, func(i, j int) bool { return dto.Types[i].Slot < dto.Types[j].Slot })
	for _, t := range dto.Types {
		if t.Type.Name != "" {
			types = append(types, t.Type.Name)
		}
	}
	return types, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
