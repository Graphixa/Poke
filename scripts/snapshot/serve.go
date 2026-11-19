package main

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// apiDataHandler serves a PokeAPI/api-data checkout as the REST paths the
// existing client requests. Query strings are ignored. Each URL maps to
// data/api/v2/<path>/index.json.
func apiDataHandler(root string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, ok := apiDataFile(root, r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, file)
	})
}

func apiDataFile(root, urlPath string) (string, bool) {
	p := path.Clean("/" + urlPath)
	const prefix = "/api/v2/"
	if p != "/api/v2" && !strings.HasPrefix(p, prefix) {
		return "", false
	}
	rest := strings.Trim(strings.TrimPrefix(p, prefix), "/")
	if rest == "" || rest == "." {
		return "", false
	}
	file := filepath.Join(root, "data", "api", "v2", filepath.FromSlash(rest), "index.json")
	rel, err := filepath.Rel(root, file)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return file, true
}
