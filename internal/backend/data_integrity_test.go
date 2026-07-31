package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBackendJSONDataFilesAreValid(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("data", "*.json"))
	if err != nil {
		t.Fatalf("list backend JSON data: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("backend JSON data directory is empty")
	}
	for _, path := range files {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if !json.Valid([]byte(cleanJSON(string(data)))) {
				t.Fatalf("%s is not valid UTF-8 JSON", path)
			}
		})
	}
}
