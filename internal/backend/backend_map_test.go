package backend

import (
	"os"
	"strings"
	"testing"
)

func TestBackendMapListsCurrentFeatureFamilies(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	required := []string{
		"Audio control",
		"Camera and spatial tools",
		"Conflux timer",
		"Natural drop deployment",
		"Runtime companions",
		"Save comparison",
		"Virtual sigils",
		"Optimizer evidence",
		"Logs battle archive",
		"Combat reference",
		"GBFR data index",
	}
	for _, family := range required {
		if !strings.Contains(text, "| "+family+" |") {
			t.Errorf("backend map is missing feature family %q", family)
		}
	}
}
