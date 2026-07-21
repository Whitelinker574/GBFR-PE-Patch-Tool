package backend

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

type badgeCatalogTestName struct {
	ZH string `json:"zh"`
	EN string `json:"en"`
}

func TestBadgeCatalogPR26Contract(t *testing.T) {
	const badgeCatalogVectorLength = 1700
	raw, err := os.ReadFile("data/badges.json")
	if err != nil {
		t.Fatalf("read extracted PR #26 badge catalog: %v", err)
	}
	var catalog map[string]badgeCatalogTestName
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatalf("decode badge catalog: %v", err)
	}
	if len(catalog) != 1616 {
		t.Fatalf("badge count=%d, want locked PR #26 count 1616", len(catalog))
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != "2aab04739254bebb26d48db9ec5f9cd804cda6a37a09654de2344cd41f4d833f" {
		t.Fatalf("badge catalog sha256=%s, checked catalog data changed without review", got)
	}
	for key, name := range catalog {
		id, err := strconv.Atoi(key)
		if err != nil || id < 0 || id >= badgeCatalogVectorLength {
			t.Fatalf("badge id %q outside 0..%d", key, badgeCatalogVectorLength-1)
		}
		if strings.TrimSpace(name.ZH) == "" || strings.TrimSpace(name.EN) == "" {
			t.Fatalf("badge %d has incomplete localized names: %+v", id, name)
		}
	}
}
