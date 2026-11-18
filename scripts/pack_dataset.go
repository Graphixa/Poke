// Packs a finished dataset directory into internal/bundled/dataset.tar.gz
// so the next release build embeds it. From the repo root:
//
//	go run ./scripts/pack_dataset.go -sha <api-data commit> <dataset-dir>
//
// The gzip committed in git is a tiny incomplete fixture so go test compiles.
// Do not commit the full snapshot this script writes. Pass -o to write elsewhere.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Graphixa/Poke/internal/dataset"
)

func main() {
	sha := flag.String("sha", "", "source_sha written into the packed manifest")
	out := flag.String("o", filepath.Join("internal", "bundled", "dataset.tar.gz"), "output tar.gz")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/pack_dataset.go [-sha commit] [-o file] <dataset-dir>")
		os.Exit(2)
	}
	if err := pack(flag.Arg(0), *out, *sha); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func pack(dir, out, sha string) error {
	names := []string{
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
	files := make(map[string][]byte, len(names)+1)
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		files[name] = body
	}
	if lic, err := os.ReadFile(filepath.Join(dir, "LICENSE.txt")); err == nil {
		files["LICENSE.txt"] = lic
	} else if !os.IsNotExist(err) {
		return err
	} else {
		fmt.Fprintln(os.Stderr, "warning: LICENSE.txt not in dataset dir; archive has no PokéAPI license")
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
	if lic, ok := files["LICENSE.txt"]; ok {
		if err := writeTar(tw, "LICENSE.txt", lic); err != nil {
			return err
		}
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

func writeTar(tw *tar.Writer, name string, body []byte) error {
	hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(body)
	return err
}
