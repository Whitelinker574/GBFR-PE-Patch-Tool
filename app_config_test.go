package main

import "testing"

func TestWindowSizeUsesCompactBounds(t *testing.T) {
	tests := []struct {
		name       string
		config     AppConfig
		wantWidth  int
		wantHeight int
	}{
		{name: "unset", config: AppConfig{}, wantWidth: 0, wantHeight: 0},
		{name: "old small preference", config: AppConfig{WindowWidth: 940, WindowHeight: 640}, wantWidth: 960, wantHeight: 640},
		{name: "normal preference", config: AppConfig{WindowWidth: 1080, WindowHeight: 700}, wantWidth: 1080, wantHeight: 700},
		{name: "old oversized preference", config: AppConfig{WindowWidth: 1920, WindowHeight: 1080}, wantWidth: 1320, wantHeight: 820},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			width, height := test.config.windowSize()
			if width != test.wantWidth || height != test.wantHeight {
				t.Fatalf("windowSize() = %dx%d, want %dx%d", width, height, test.wantWidth, test.wantHeight)
			}
		})
	}
}
