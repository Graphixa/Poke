package app_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/Graphixa/Pokedex"

func TestArchitectureForbiddenImports(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	cachePkg := modulePath + "/internal/cache"
	datasetPkg := modulePath + "/internal/dataset"
	appPkg := modulePath + "/internal/app"
	tuiPrefix := modulePath + "/internal/tui"
	renderPrefix := modulePath + "/internal/render"
	warmers := map[string]bool{
		modulePath + "/internal/render/sprite/nativewarm": true,
		modulePath + "/internal/render/sprite/assetwarm":  true,
		modulePath + "/internal/render/sprite/warm":       true,
		modulePath + "/internal/cache/dossierwarm":        true,
	}

	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "testdata", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		pkg := importPathForFile(root, path)
		imps, parseErr := fileImports(path)
		if parseErr != nil {
			return parseErr
		}
		for _, imp := range imps {
			switch {
			case isTUIPackage(pkg, tuiPrefix) && (imp == cachePkg || strings.HasPrefix(imp, cachePkg+"/")):
				t.Errorf("%s imports %s; TUI must not depend on cache", rel(root, path), imp)
			case warmers[pkg] && (imp == cachePkg || strings.HasPrefix(imp, cachePkg+"/")):
				t.Errorf("%s imports %s; rendering warmers must not import cache", rel(root, path), imp)
			case isCacheOrDataset(pkg, cachePkg, datasetPkg) && (imp == appPkg || strings.HasPrefix(imp, appPkg+"/") || isTUIPackage(imp, tuiPrefix)):
				t.Errorf("%s imports %s; cache/dataset must not import app or tui", rel(root, path), imp)
			case strings.HasPrefix(pkg, renderPrefix) && isTUIPackage(imp, tuiPrefix):
				t.Errorf("%s imports %s; renderer packages must not know TUI presentation", rel(root, path), imp)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAppAssemblesWarmersAndTUI(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	path := filepath.Join(root, "internal", "app", "browse.go")
	imps, err := fileImports(path)
	if err != nil {
		t.Fatal(err)
	}
	need := []string{
		modulePath + "/internal/tui",
		modulePath + "/internal/render/sprite/nativewarm",
		modulePath + "/internal/render/sprite/assetwarm",
		modulePath + "/internal/render/sprite/warm",
		modulePath + "/internal/cache/dossierwarm",
	}
	have := map[string]bool{}
	for _, imp := range imps {
		have[imp] = true
	}
	for _, pkg := range need {
		if !have[pkg] {
			t.Errorf("internal/app/browse.go must import %s (app classifies subsystem progress)", pkg)
		}
	}
	if have[modulePath+"/internal/cache"] {
		t.Error("internal/app/browse.go must not import cache; dataset progress lives on CLI/bootstrap paths")
	}
}

func TestCacheDoesNotExportGlobalProgressBus(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	dir := filepath.Join(root, "internal", "cache")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".go") || strings.HasSuffix(ent.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, ent.Name())
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts := spec.(*ast.TypeSpec)
				switch ts.Name.Name {
				case "Progress", "Stage", "ProgressFunc":
					t.Errorf("%s exports %s; cache progress must be DatasetProgress / DatasetStage", ent.Name(), ts.Name.Name)
				}
			}
		}
	}
}

func TestWarmersDoNotReferenceCacheProgressTypes(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	dirs := []string{
		filepath.Join(root, "internal", "render", "sprite", "nativewarm"),
		filepath.Join(root, "internal", "render", "sprite", "assetwarm"),
		filepath.Join(root, "internal", "render", "sprite", "warm"),
		filepath.Join(root, "internal", "cache", "dossierwarm"),
	}
	forbidden := []string{"cache.Progress", "cache.Stage", "cache.DatasetProgress", "cache.DatasetStage", "cache.ProgressFunc", "cache.DatasetProgressFunc"}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, ent := range entries {
			if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".go") || strings.HasSuffix(ent.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, ent.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			src := string(data)
			for _, needle := range forbidden {
				if strings.Contains(src, needle) {
					t.Errorf("%s references %s; warmers own their progress types", rel(root, path), needle)
				}
			}
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func importPathForFile(root, path string) string {
	relDir, err := filepath.Rel(root, filepath.Dir(path))
	if err != nil {
		return ""
	}
	return modulePath + "/" + filepath.ToSlash(relDir)
}

func fileImports(path string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(f.Imports))
	for _, imp := range f.Imports {
		s, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func isTUIPackage(pkg, tuiPrefix string) bool {
	return pkg == tuiPrefix || strings.HasPrefix(pkg, tuiPrefix+"/")
}

func isCacheOrDataset(pkg, cachePkg, datasetPkg string) bool {
	return pkg == cachePkg || strings.HasPrefix(pkg, cachePkg+"/") || pkg == datasetPkg
}

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}
