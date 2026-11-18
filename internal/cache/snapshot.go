package cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Graphixa/Poke/internal/bundled"
)

const snapshotMax = 32 << 20

// SnapshotOverride replaces SnapshotURL. Tests set it so refresh does not call GitHub.
var SnapshotOverride string

var snapshotHTTP = &http.Client{Timeout: 2 * time.Minute}

var embeddedSnapshot = func() []byte { return bundled.Bytes() }

// RefreshSnapshot downloads the repository gzip when its ETag changed and
// unpacks it when the manifest is usable. A 304, a network error, or a
// rejected snapshot leaves the active dataset in place and does not crawl.
// An empty cache falls back to the embedded snapshot.
func (s *Service) RefreshSnapshot(ctx context.Context, progress DatasetProgressFunc) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	report(progress, DatasetProgress{Stage: DatasetStageImport})
	code, body, etag, err := s.downloadSnapshot(ctx)
	if err != nil || (code != http.StatusOK && code != http.StatusNotModified) {
		return s.keepOrEmbed(ctx)
	}
	if code == http.StatusNotModified {
		return nil
	}
	if err := s.store.Unpack(ctx, body); err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		if rejectedSnapshot(err) {
			return s.keepOrEmbed(ctx)
		}
		return err
	}
	if etag != "" {
		_ = os.WriteFile(filepath.Join(s.store.Path(), "snapshot.etag"), []byte(etag), 0o600)
	}
	return nil
}

func (s *Service) keepOrEmbed(ctx context.Context) error {
	if s.store.HasActiveDataset() {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.store.Unpack(ctx, embeddedSnapshot())
}

func rejectedSnapshot(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "cache: embedded dataset:")
}

func (s *Service) downloadSnapshot(ctx context.Context) (int, []byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, snapshotEndpoint(), nil) // #nosec G107 G704 -- default URL is a const; override is tests only
	if err != nil {
		return 0, nil, "", err
	}
	ua := strings.TrimSpace(s.cfg.Network.UserAgent)
	if ua == "" {
		ua = "poke-cli"
	}
	req.Header.Set("User-Agent", ua)
	if raw, err := os.ReadFile(filepath.Join(s.store.Path(), "snapshot.etag")); err == nil {
		if tag := strings.TrimSpace(string(raw)); tag != "" {
			req.Header.Set("If-None-Match", tag)
		}
	}
	resp, err := snapshotHTTP.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil, "", nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, snapshotMax+1))
	if err != nil {
		return 0, nil, "", err
	}
	if len(body) > snapshotMax {
		return 0, nil, "", fmt.Errorf("cache: snapshot larger than %d bytes", snapshotMax)
	}
	return http.StatusOK, body, resp.Header.Get("ETag"), nil
}

func snapshotEndpoint() string {
	if SnapshotOverride != "" {
		return SnapshotOverride
	}
	return bundled.SnapshotURL
}
