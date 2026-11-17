package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Graphixa/Poke/pkg/version"
)

func TestShouldCheckForUpdates(t *testing.T) {
	if ShouldCheckForUpdates(false, 24, "") {
		t.Fatal("disabled must not check")
	}
	if !ShouldCheckForUpdates(true, 24, "") {
		t.Fatal("never-checked must check")
	}
	if ShouldCheckForUpdates(true, 24, GetLastCheckedTimestamp()) {
		t.Fatal("just-checked must wait for interval")
	}
}

func TestCheckAndApplyLatest(t *testing.T) {
	const latest = "9.9.9"
	binPayload := []byte("fake-poke-binary-v999")
	archiveName := ReleaseArchiveName(latest, "linux", "amd64")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: "poke", Mode: 0o755, Size: int64(len(binPayload))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(binPayload); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	_ = gz.Close()
	archiveBytes := buf.Bytes()
	sum := sha256.Sum256(archiveBytes)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archiveName)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/latest"):
			w.Header().Set("Location", "/Graphixa/Poke/releases/tag/v"+latest)
			w.WriteHeader(http.StatusFound)
		case strings.Contains(r.URL.Path, "/download/v"+latest+"/checksums.txt"):
			_, _ = w.Write([]byte(checksums))
		case strings.Contains(r.URL.Path, "/download/v"+latest+"/"+archiveName):
			_, _ = w.Write(archiveBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	prevBase := releaseBaseURL
	prevGOOS, prevGOARCH := currentGOOS, currentGOARCH
	prevExec := executablePath
	prevVer := version.Version
	t.Cleanup(func() {
		releaseBaseURL = prevBase
		currentGOOS, currentGOARCH = prevGOOS, prevGOARCH
		executablePath = prevExec
		version.Version = prevVer
	})

	releaseBaseURL = srv.URL + "/Graphixa/Poke/releases"
	currentGOOS, currentGOARCH = "linux", "amd64"
	version.Version = "1.0.0"

	dest := filepath.Join(t.TempDir(), "poke")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	executablePath = func() (string, error) { return dest, nil }

	result, err := CheckForUpdates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.NeedsUpdate || result.Latest != latest {
		t.Fatalf("result = %+v", result)
	}
	if err := UpdateToLatest(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, binPayload) {
		t.Fatalf("binary = %q want %q", got, binPayload)
	}
}

func TestParseChecksumsRejectsBadLine(t *testing.T) {
	if _, err := parseChecksums([]byte("not-a-checksum\n")); err == nil {
		t.Fatal("expected error")
	}
}
