package debuglog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnableAndSanitize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "poke-debug.log")
	got, err := Enable(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("path = %q", got)
	}
	Requestf("GET /pokemon/1 status=200")
	Cachef("hit species id=1")
	Renderf("chafa cells=12 hash=abc")
	Requestf("authorization=secret-token-value")

	_ = Global().Close()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"[request]", "[cache]", "[render]", "GET /pokemon/1", "hit species", "chafa"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if strings.Contains(text, "secret-token-value") {
		t.Fatal("secret leaked into debug log")
	}
	if !strings.Contains(text, "<redacted>") {
		t.Fatal("expected redaction marker")
	}
}

func TestDiscardByDefault(t *testing.T) {
	Requestf("should not panic when discarded")
}
