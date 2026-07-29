package backend

import (
	"os"
	"strings"
	"testing"
)

func TestCameraRuntimeIsBuiltIntoPatchCore(t *testing.T) {
	data, err := os.ReadFile("../../src_dll/patch_core/dllmain.cpp")
	if err != nil || !strings.Contains(string(data), "runtime_camera") || !strings.Contains(string(data), "RunCameraRuntime") {
		t.Fatalf("camera runtime is not integrated into patch_core: err=%v", err)
	}
}

func TestNormalizeCameraConfig(t *testing.T) {
	valid, err := normalizeCameraConfig(CameraDeployRequest{MaxDistance: 8, TargetHeight: 2.4, ZoomStep: 0.03})
	if err != nil || valid.SchemaVersion != 1 || valid.MaxDistance != 8 || valid.TargetHeight != 2.4 || valid.ZoomStep != 0.03 {
		t.Fatalf("valid camera config rejected: %+v err=%v", valid, err)
	}
	for _, request := range []CameraDeployRequest{
		{MaxDistance: 0.4, TargetHeight: 2, ZoomStep: 0.02},
		{MaxDistance: 6, TargetHeight: 5.1, ZoomStep: 0.02},
		{MaxDistance: 6, TargetHeight: 2, ZoomStep: 0},
	} {
		if _, err := normalizeCameraConfig(request); err == nil {
			t.Fatalf("invalid camera config accepted: %+v", request)
		}
	}
}

func TestCameraLiveLifecycle(t *testing.T) {
	if os.Getenv("GBFR_CAMERA_QA") != "1" {
		t.Skip("set GBFR_CAMERA_QA=1 with the 2.0.2 game running")
	}
	app := &App{}
	config := readCameraConfig()
	if _, err := app.DeployCameraMod(CameraDeployRequest{MaxDistance: config.MaxDistance, TargetHeight: config.TargetHeight, ZoomStep: config.ZoomStep}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := app.RemoveCameraMod(""); err != nil {
			t.Errorf("stop camera runtime: %v", err)
		}
	}()
	pid, err := findProcessByName(charaProcessName)
	if err != nil {
		t.Fatal(err)
	}
	status := readRuntimeCompanionStatus("camera")
	if status.PID != pid || status.State != "active" {
		t.Fatalf("camera runtime did not become active: %+v", status)
	}
}
