package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestWindowSizePreservesRealDesktopPreferences(t *testing.T) {
	tests := []struct {
		name       string
		config     AppConfig
		wantWidth  int
		wantHeight int
	}{
		{name: "unset", config: AppConfig{}, wantWidth: 0, wantHeight: 0},
		{name: "old small preference", config: AppConfig{WindowWidth: 940, WindowHeight: 640}, wantWidth: 960, wantHeight: 640},
		{name: "normal preference", config: AppConfig{WindowWidth: 1080, WindowHeight: 700}, wantWidth: 1080, wantHeight: 700},
		{name: "maximised desktop preference", config: AppConfig{WindowWidth: 1920, WindowHeight: 1080}, wantWidth: 1920, wantHeight: 1080},
		{name: "4k desktop preference", config: AppConfig{WindowWidth: 3840, WindowHeight: 2160}, wantWidth: 3840, wantHeight: 2160},
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

func TestAppConfigConcurrentUpdatesPreserveIndependentFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	app := &App{configPathOverride: path}
	var wait sync.WaitGroup
	for index := 0; index < 64; index++ {
		wait.Add(2)
		go func(value int) {
			defer wait.Done()
			if err := app.SetLastSavePath(fmt.Sprintf("save-%03d.dat", value)); err != nil {
				t.Errorf("set save path: %v", err)
			}
		}(index)
		go func() {
			defer wait.Done()
			if err := app.setRuntimeLoadoutDetectorPreference(true); err != nil {
				t.Errorf("set detector preference: %v", err)
			}
		}()
	}
	wait.Wait()

	config, err := app.configSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if config.LastSavePath == "" || !config.RuntimeLoadoutDetectorActive {
		t.Fatalf("concurrent updates clobbered config fields: %+v", config)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var stored AppConfig
	if err := json.Unmarshal(payload, &stored); err != nil {
		t.Fatalf("atomic config file is invalid: %v", err)
	}
	if stored != config {
		t.Fatalf("memory and disk config differ: memory=%+v disk=%+v", config, stored)
	}
}
