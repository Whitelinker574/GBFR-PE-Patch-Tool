package backend

import "testing"

func TestDefaultWindowOpensInTheCompleteWorkbenchLayout(t *testing.T) {
	if defaultAppWidth < 1280 || defaultAppHeight < 800 {
		t.Fatalf("default window %dx%d is too small for the complete workbench", defaultAppWidth, defaultAppHeight)
	}
	if minAppWidth != 960 || minAppHeight != 620 {
		t.Fatalf("compact fallback changed unexpectedly: %dx%d", minAppWidth, minAppHeight)
	}
	if defaultAppWidth > maxAppWidth || defaultAppHeight > maxAppHeight {
		t.Fatalf("default window exceeds persisted size bounds")
	}
}
