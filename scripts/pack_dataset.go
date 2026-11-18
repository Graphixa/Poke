// Packs a finished dataset directory into internal/bundled/dataset.tar.gz
// so the next release build embeds it. From the repo root:
//
//	go run ./scripts/pack_dataset.go -sha <api-data commit> <dataset-dir>
//
// Release builds run -release. That repacks POKE_DATASET_DIR when it is set,
// then rejects an incomplete archive. The gzip in git is the snapshot CI embeds.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Graphixa/Poke/internal/dataset"
)

func main() {
	sha := flag.String("sha", "", "source_sha written into the packed manifest")
	out := flag.String("o", filepath.Join("internal", "bundled", "dataset.tar.gz"), "output tar.gz")
	release := flag.Bool("release", false, "repack POKE_DATASET_DIR when set, then require a complete archive")
	flag.Parse()
	if *release {
		if flag.NArg() != 0 {
			fmt.Fprintln(os.Stderr, "usage: go run ./scripts/pack_dataset.go -release [-o file]")
			os.Exit(2)
		}
		if err := releaseArchive(*out, *sha); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/pack_dataset.go [-sha commit] [-o file] <dataset-dir>")
		os.Exit(2)
	}
	if err := pack(flag.Arg(0), *out, *sha); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func releaseArchive(out, sha string) error {
	if dir := os.Getenv("POKE_DATASET_DIR"); dir != "" {
		if sha == "" {
			sha = os.Getenv("POKE_DATASET_SHA")
		}
		if err := pack(dir, out, sha); err != nil {
			return err
		}
	}
	return check(out)
}

func pack(dir, out, sha string) error {
	names := archiveNames()
	files := make(map[string][]byte, len(names)+1)
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		files[name] = body
	}
	lic, err := os.ReadFile(filepath.Join(dir, "LICENSE.txt"))
	if os.IsNotExist(err) {
		lic, err = os.ReadFile(filepath.Join("internal", "bundled", "LICENSE.pokeapi.txt"))
	}
	if err != nil {
		return fmt.Errorf("PokéAPI license: %w", err)
	}
	files["LICENSE.txt"] = lic
	if sha == "" {
		sum := sha256.Sum256(files[dataset.FileManifest])
		sha = hex.EncodeToString(sum[:])
	}
	if sha != "" {
		var man map[string]any
		if err := json.Unmarshal(files[dataset.FileManifest], &man); err != nil {
			return fmt.Errorf("manifest: %w", err)
		}
		man["source_sha"] = sha
		body, err := json.Marshal(man)
		if err != nil {
			return err
		}
		files[dataset.FileManifest] = body
	}

	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(zw)
	for _, name := range names {
		if err := writeTar(tw, name, files[name]); err != nil {
			return err
		}
	}
	if err := writeTar(tw, "LICENSE.txt", files["LICENSE.txt"]); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	tmp := out + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, out); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	fmt.Printf("wrote %s (%d bytes)\n", out, buf.Len())
	return nil
}

func check(path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("embedded dataset: %w", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("embedded dataset: %w", err)
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	files := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("embedded dataset: %w", err)
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return err
		}
		files[filepath.Base(hdr.Name)] = b
	}
	raw, ok := files[dataset.FileManifest]
	if !ok {
		return fmt.Errorf("embedded dataset: missing manifest")
	}
	var man struct {
		SchemaVersion int    `json:"schema_version"`
		Complete      bool   `json:"complete"`
		PokemonCount  int    `json:"pokemon_count"`
		SourceSHA     string `json:"source_sha"`
	}
	if err := json.Unmarshal(raw, &man); err != nil {
		return fmt.Errorf("embedded dataset: %w", err)
	}
	if man.SchemaVersion != dataset.SchemaVersion || !man.Complete || man.PokemonCount < 1 || man.SourceSHA == "" {
		return fmt.Errorf("embedded dataset: incomplete snapshot (complete=%v pokemon=%d sha=%q)", man.Complete, man.PokemonCount, man.SourceSHA)
	}
	for _, name := range append(archiveNames(), "LICENSE.txt") {
		if len(files[name]) == 0 {
			return fmt.Errorf("embedded dataset: missing %s", name)
		}
	}
	fmt.Printf("embedded dataset ok sha=%s pokemon=%d bytes=%d\n", man.SourceSHA, man.PokemonCount, len(body))
	return nil
}

func archiveNames() []string {
	return []string{
		dataset.FileManifest,
		dataset.FileBrowse,
		dataset.FilePokemon,
		dataset.FileSpecies,
		dataset.FileLearnsets,
		dataset.FileMoves,
		dataset.FileEvolutions,
		dataset.FileEncounters,
		dataset.FileVersions,
		dataset.FileVersionGroups,
		dataset.FileTypes,
		dataset.FileAbilities,
		dataset.FileForms,
	}
}

func writeTar(tw *tar.Writer, name string, body []byte) error {
	hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(body)
	return err
}
