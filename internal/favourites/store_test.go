package favourites

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPersistenceAndNormalization(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "preferences", "favourites.json")}
	if ids, err := s.Load(); err != nil || len(ids) != 0 {
		t.Fatalf("new store: %v %v", ids, err)
	}
	if err := s.Save([]int{133, 25, 25}); err != nil {
		t.Fatal(err)
	}
	if ids, err := s.Load(); err != nil || !reflect.DeepEqual(ids, []int{25, 133}) {
		t.Fatalf("reload: %v %v", ids, err)
	}
	if err := s.Save([]int{4}); err != nil {
		t.Fatal(err)
	}
	if ids, err := s.Load(); err != nil || !reflect.DeepEqual(ids, []int{4}) {
		t.Fatalf("replace: %v %v", ids, err)
	}
	if err := s.Save(nil); err != nil {
		t.Fatal(err)
	}
	if ids, err := s.Load(); err != nil || len(ids) != 0 {
		t.Fatalf("remove last: %v %v", ids, err)
	}
}

func TestInvalidSavePreservesExistingFile(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "favourites.json")}
	if err := s.Save([]int{25}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save([]int{-1}); err == nil {
		t.Fatal("accepted invalid ID")
	}
	ids, err := s.Load()
	if err != nil || !reflect.DeepEqual(ids, []int{25}) {
		t.Fatalf("old file changed: %v %v", ids, err)
	}
}

func TestMalformedAndFutureDocumentsFailWithoutRewrite(t *testing.T) {
	for _, body := range []string{`{`, `{"version":2,"species_ids":[25]}`, `{"version":1,"species_ids":[0]}`} {
		t.Run(body, func(t *testing.T) {
			s := Store{Path: filepath.Join(t.TempDir(), "favourites.json")}
			if err := os.WriteFile(s.Path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Load(); err == nil {
				t.Fatal("accepted invalid document")
			}
			got, err := os.ReadFile(s.Path)
			if err != nil || string(got) != body {
				t.Fatal("source document changed")
			}
		})
	}
}
