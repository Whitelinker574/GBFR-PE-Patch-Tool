package backend

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
)

var cameraModMu sync.Mutex

type CameraConfig struct {
	SchemaVersion int     `json:"schemaVersion"`
	MaxDistance   float32 `json:"maxDistance"`
	TargetHeight  float32 `json:"targetHeight"`
	ZoomStep      float32 `json:"zoomStep"`
}

type CameraWorkspace struct {
	Installed        bool         `json:"installed"`
	Owned            bool         `json:"owned"`
	RecoveryRequired bool         `json:"recoveryRequired"`
	State            string       `json:"state"`
	GameRunning      bool         `json:"gameRunning"`
	Config           CameraConfig `json:"config"`
	Detail           string       `json:"detail"`
}

type CameraDeployRequest struct {
	MaxDistance  float32 `json:"maxDistance"`
	TargetHeight float32 `json:"targetHeight"`
	ZoomStep     float32 `json:"zoomStep"`
}

type CameraDeployResult struct {
	Active          bool `json:"active"`
	RestartRequired bool `json:"restartRequired"`
}

func defaultCameraConfig() CameraConfig {
	return CameraConfig{SchemaVersion: 1, MaxDistance: 6, TargetHeight: 1.8, ZoomStep: 0.02}
}

func normalizeCameraConfig(request CameraDeployRequest) (CameraConfig, error) {
	config := CameraConfig{SchemaVersion: 1, MaxDistance: request.MaxDistance, TargetHeight: request.TargetHeight, ZoomStep: request.ZoomStep}
	if config.MaxDistance < 0.5 || config.MaxDistance > 30 {
		return CameraConfig{}, errors.New("镜头最大距离必须在 0.5 到 30 之间")
	}
	if config.TargetHeight < 0 || config.TargetHeight > 5 {
		return CameraConfig{}, errors.New("镜头目标高度必须在 0 到 5 之间")
	}
	if config.ZoomStep < 0.001 || config.ZoomStep > 1 {
		return CameraConfig{}, errors.New("镜头缩放步长必须在 0.001 到 1 之间")
	}
	return config, nil
}

func cameraRuntimePath() (string, error) { return runtimeCompanionPath("camera.ini") }

func readCameraConfig() CameraConfig {
	value := defaultCameraConfig()
	path, err := cameraRuntimePath()
	if err != nil {
		return value
	}
	section := readRuntimeINI(path)["camera"]
	if section == nil {
		return value
	}
	if parsed, err := strconv.ParseFloat(section["maxDistance"], 32); err == nil {
		value.MaxDistance = float32(parsed)
	}
	if parsed, err := strconv.ParseFloat(section["targetHeight"], 32); err == nil {
		value.TargetHeight = float32(parsed)
	}
	if parsed, err := strconv.ParseFloat(section["zoomStep"], 32); err == nil {
		value.ZoomStep = float32(parsed)
	}
	if normalized, err := normalizeCameraConfig(CameraDeployRequest{MaxDistance: value.MaxDistance, TargetHeight: value.TargetHeight, ZoomStep: value.ZoomStep}); err == nil {
		return normalized
	}
	return defaultCameraConfig()
}

func writeCameraRuntimeConfig(config CameraConfig, enabled bool) error {
	path, err := cameraRuntimePath()
	if err != nil {
		return err
	}
	flag := 0
	if enabled {
		flag = 1
	}
	data := []byte(fmt.Sprintf("[camera]\r\nenabled=%d\r\nmaxDistance=%.9g\r\ntargetHeight=%.9g\r\nzoomStep=%.9g\r\n", flag, config.MaxDistance, config.TargetHeight, config.ZoomStep))
	return writeRuntimeCompanionFile(path, data)
}

func (a *App) GetCameraWorkspace(_ string) (*CameraWorkspace, error) {
	cameraModMu.Lock()
	defer cameraModMu.Unlock()
	_, processErr := findProcessByName(charaProcessName)
	status := readRuntimeCompanionStatus("camera")
	active := runtimeCompanionPresent("camera")
	process, processIdentityErr := findRuntimeProcessInstance()
	owned := processIdentityErr == nil && a.runtimeCompanionOwned("camera", process)
	recoveryRequired := processIdentityErr == nil && runtimeCompanionRecoveryRequired(status, process)
	detail := status.Detail
	if !active && processErr != nil {
		detail = "请先启动游戏，再从本页开启镜头运行时"
	}
	return &CameraWorkspace{Installed: active, Owned: owned, RecoveryRequired: recoveryRequired, State: status.State, GameRunning: processErr == nil, Config: readCameraConfig(), Detail: detail}, nil
}

func (a *App) DeployCameraMod(request CameraDeployRequest) (*CameraDeployResult, error) {
	cameraModMu.Lock()
	defer cameraModMu.Unlock()
	config, err := normalizeCameraConfig(request)
	if err != nil {
		return nil, err
	}
	if err := writeCameraRuntimeConfig(config, true); err != nil {
		return nil, err
	}
	if err := a.startRuntimeCompanion("camera", "runtime_camera"); err != nil {
		_ = writeCameraRuntimeConfig(config, false)
		return nil, err
	}
	return &CameraDeployResult{Active: true}, nil
}

func (a *App) RemoveCameraMod(_ string) error {
	cameraModMu.Lock()
	defer cameraModMu.Unlock()
	config := readCameraConfig()
	return a.stopOwnedRuntimeCompanion("camera", func() error { return writeCameraRuntimeConfig(config, false) })
}
