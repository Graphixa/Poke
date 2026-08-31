package cache

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
)

const nativeFrameSuffix = ".sixel.gz"

// nativeFrameFilename returns the on-disk name for a native render key.
func nativeFrameFilename(key string) string {
	return key + nativeFrameSuffix
}

// nativeKeyFromFilename extracts the render key from a native artefact filename.
func nativeKeyFromFilename(name string) (string, bool) {
	switch {
	case strings.HasSuffix(name, nativeFrameSuffix):
		return strings.TrimSuffix(name, nativeFrameSuffix), true
	case strings.HasSuffix(name, ".frame"):
		// Legacy pre-gzip layout; eligible for purge on incompatible version.
		return strings.TrimSuffix(name, ".frame"), true
	default:
		return "", false
	}
}

func gzipCompressFrame(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gzipDecompressFrame(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func isNativeArtefactName(name string) bool {
	_, ok := nativeKeyFromFilename(name)
	return ok
}

func validateNativeKeyFormat(key string) error {
	if key == "" {
		return fmt.Errorf("empty native key")
	}
	return nil
}
