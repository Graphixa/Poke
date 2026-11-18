// Package bundled is the dataset snapshot compiled into the binary.
package bundled

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"io"
	"path"
)

//go:embed dataset.tar.gz
var gz []byte

// Bytes is the embedded dataset tar.gz.
func Bytes() []byte { return gz }

// SHA is the embedded manifest source_sha, or "" when the gzip cannot be read.
func SHA() string {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return ""
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF || err != nil {
			return ""
		}
		if path.Base(path.Clean(hdr.Name)) != "manifest.json" {
			continue
		}
		var man struct {
			SourceSHA string `json:"source_sha"`
		}
		if json.NewDecoder(tr).Decode(&man) != nil {
			return ""
		}
		return man.SourceSHA
	}
}
