package seedcontent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func productionDocuments(t *testing.T) []Document {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "..", "seeds", "*.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var documents []Document
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		d, err := Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		documents = append(documents, d)
	}
	return documents
}
func TestProductionCatalogIsCompleteAndConnected(t *testing.T) {
	if err := ValidateCatalog(productionDocuments(t), true); err != nil {
		t.Fatal(err)
	}
}
