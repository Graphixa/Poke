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
	renderingWarmers := map[string]bool{
		modulePath + "/internal/render/sprite/nativewarm": true,
		modulePath + "/internal/render/sprite/assetwarm":  true,
		modulePath + "/internal/render/sprite/warm":       true,
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
			case renderingWarmers[pkg] && (imp == cachePkg || strings.HasPrefix(imp, cachePkg+"/")):
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
	forbidden := map[string]bool{
		"Progress":            true,
		"Stage":               true,
		"DatasetProgress":     true,
		"DatasetStage":        true,
		"ProgressFunc":        true,
		"DatasetProgressFunc": true,
	}
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
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			cacheNames := importedPackageNames(f, modulePath+"/internal/cache")
			ast.Inspect(f, func(node ast.Node) bool {
				sel, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if ok && cacheNames[pkg.Name] && forbidden[sel.Sel.Name] {
					t.Errorf("%s references cache.%s; warmers own their progress types", rel(root, path), sel.Sel.Name)
				}
				return true
			})
		}
	}
}

func TestProductionDoesNotCastNonDatasetStages(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	cachePkg := modulePath + "/internal/cache"
	allowed := map[string]bool{
		"index": true, "enrich": true, "records": true,
		"evolution": true, "moves": true, "dataset": true,
	}
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		cacheNames := importedPackageNames(f, cachePkg)
		ast.Inspect(f, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "DatasetStage" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			lit, literal := call.Args[0].(*ast.BasicLit)
			if !ok || !cacheNames[pkg.Name] || !literal || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err == nil && !allowed[value] {
				t.Errorf("%s casts %q to cache.DatasetStage; dataset stages cannot carry non-dataset work", rel(root, path), value)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
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
	return pkg == cachePkg || strings.HasPrefix(pkg, cachePkg+"/") ||
		pkg == datasetPkg || strings.HasPrefix(pkg, datasetPkg+"/")
}

func importedPackageNames(f *ast.File, importPath string) map[string]bool {
	names := make(map[string]bool)
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != importPath {
			continue
		}
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names[name] = true
	}
	return names
}

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}
