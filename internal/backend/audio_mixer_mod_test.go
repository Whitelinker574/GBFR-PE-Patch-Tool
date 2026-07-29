package backend

import (
	"os"
	"strings"
	"testing"
)

func TestAudioMixerEmbeddedHelperAndCatalog(t *testing.T) {
	data, err := os.ReadFile("../../src_dll/patch_core/dllmain.cpp")
	if err != nil || !strings.Contains(string(data), "runtime_audio") || !strings.Contains(string(data), "PostEventDetour") ||
		!strings.Contains(string(data), "g_audioUIEvents") || !strings.Contains(string(data), "Volume_SE") ||
		!strings.Contains(string(data), "GBFR audio event=") || !strings.Contains(string(data), "g_audioDiagnostic.load()") {
		t.Fatalf("audio runtime is not integrated into patch_core: err=%v", err)
	}
	ids := audioMixerSortedIDs()
	if len(ids) != 32 {
		t.Fatalf("expected 32 supported voice owners, got %d", len(ids))
	}
	for index := 1; index < len(ids); index++ {
		if ids[index] == ids[index-1] {
			t.Fatalf("duplicate voice owner %s", ids[index])
		}
	}
}

func TestAudioMixerLiveLifecycle(t *testing.T) {
	if os.Getenv("GBFR_AUDIO_QA") != "1" {
		t.Skip("set GBFR_AUDIO_QA=1 with the 2.0.2 game running")
	}
	app := &App{}
	volumes := make(map[string]int, len(audioMixerCharacters))
	for _, character := range audioMixerCharacters {
		volumes[character.ID] = 100
	}
	if err := app.DeployAudioMixerMod(AudioMixerDeployRequest{Volumes: volumes, UIVolume: 100}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := app.RemoveAudioMixerMod(""); err != nil {
			t.Errorf("stop audio runtime: %v", err)
		}
	}()
	pid, err := findProcessByName(charaProcessName)
	if err != nil {
		t.Fatal(err)
	}
	status := readRuntimeCompanionStatus("audio")
	if status.PID != pid || status.State != "active" {
		t.Fatalf("audio runtime did not become active: %+v", status)
	}
}

func TestNormalizeAudioMixerConfigRejectsUnknownAndOutOfRange(t *testing.T) {
	config, err := normalizeAudioMixerConfig(AudioMixerDeployRequest{Volumes: map[string]int{"pl1600": 42}, UIVolume: 65})
	if err != nil || config.CharacterVolumes["PL1600"] != 42 || config.UIVolume != 65 || len(config.CharacterVolumes) != len(audioMixerCharacters) {
		t.Fatalf("valid normalized config mismatch: %#v err=%v", config, err)
	}
	if _, err := normalizeAudioMixerConfig(AudioMixerDeployRequest{Volumes: map[string]int{"PL9999": 50}}); err == nil {
		t.Fatal("unknown character should be rejected")
	}
	if _, err := normalizeAudioMixerConfig(AudioMixerDeployRequest{Volumes: map[string]int{"PL1600": 101}}); err == nil {
		t.Fatal("out-of-range volume should be rejected")
	}
	if _, err := normalizeAudioMixerConfig(AudioMixerDeployRequest{UIVolume: 101}); err == nil {
		t.Fatal("out-of-range UI volume should be rejected")
	}
}
