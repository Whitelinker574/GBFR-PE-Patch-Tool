package backend

import (
	"context"
	"encoding/binary"
	"os"
	"strings"
	"sync"
	"time"
)

const persistentRuntimeCompanionPollInterval = 2 * time.Second

func shouldReconnectPersistentRuntimeCompanion(enabled bool, status runtimeCompanionStatus, process processInstanceID) bool {
	if !enabled || process.PID == 0 || process.Created == 0 {
		return false
	}
	if !runtimeCompanionMatchesProcess(status, process) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(status.State), "inactive")
}

func runtimeCompanionINIEnabled(file, section string) bool {
	path, err := runtimeCompanionPath(file)
	if err != nil {
		return false
	}
	return readRuntimeINI(path)[section]["enabled"] == "1"
}

func virtualSigilRuntimeEnabled() bool {
	path, err := virtualSigilBinaryPath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 16 || string(data[:8]) != "GBFRVS02" {
		return false
	}
	return binary.LittleEndian.Uint32(data[8:12]) == 2 && binary.LittleEndian.Uint32(data[12:16]) == 1
}

func (a *App) reconnectPersistentRuntimeCompanion(mu *sync.Mutex, process processInstanceID, feature, command string, enabled func() bool) {
	mu.Lock()
	defer mu.Unlock()
	if !shouldReconnectPersistentRuntimeCompanion(enabled(), readRuntimeCompanionStatus(feature), process) {
		return
	}
	_ = a.startRuntimeCompanion(feature, command)
}

func (a *App) reconnectPersistentCameraRuntime(process processInstanceID) {
	a.reconnectPersistentRuntimeCompanion(&cameraModMu, process, "camera", "runtime_camera", func() bool {
		return runtimeCompanionINIEnabled("camera.ini", "camera")
	})
}

func (a *App) reconnectPersistentAudioRuntime(process processInstanceID) {
	a.reconnectPersistentRuntimeCompanion(&audioMixerMu, process, "audio", "runtime_audio", func() bool {
		return runtimeCompanionINIEnabled("audio.ini", "audio")
	})
}

func (a *App) reconnectPersistentVirtualSigilRuntime(process processInstanceID) {
	a.reconnectPersistentRuntimeCompanion(&virtualSigilModMu, process, "virtual-sigils", "runtime_virtual_sigils", virtualSigilRuntimeEnabled)
}

func (a *App) reconnectPersistentWeaponSkillsRuntime(process processInstanceID) {
	weaponRuntimeSkillsMu.Lock()
	defer weaponRuntimeSkillsMu.Unlock()
	if !shouldReconnectPersistentRuntimeCompanion(weaponRuntimeSkillsEnabled(), readRuntimeCompanionStatus("weapon-skills"), process) {
		return
	}
	_ = a.startRuntimeCompanionForDigest("weapon-skills", "runtime_weapon_skills", game203ExecutableSHA256)
}

func (a *App) reconnectPersistentRuntimeCompanions() {
	process, err := findRuntimeProcessInstance()
	if err != nil {
		return
	}
	a.reconnectPersistentCameraRuntime(process)
	a.reconnectPersistentAudioRuntime(process)
	a.reconnectPersistentVirtualSigilRuntime(process)
	a.reconnectPersistentWeaponSkillsRuntime(process)
}

func (a *App) startPersistentRuntimeCompanionSupervisor() {
	a.runtimeCompanionSupervisorMu.Lock()
	defer a.runtimeCompanionSupervisorMu.Unlock()
	if a.runtimeCompanionSupervisor != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.runtimeCompanionSupervisor = cancel
	a.runtimeCompanionSupervisorWG.Add(1)
	go func() {
		defer a.runtimeCompanionSupervisorWG.Done()
		ticker := time.NewTicker(persistentRuntimeCompanionPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.reconnectPersistentRuntimeCompanions()
			}
		}
	}()
}

func (a *App) stopPersistentRuntimeCompanionSupervisor() {
	a.runtimeCompanionSupervisorMu.Lock()
	cancel := a.runtimeCompanionSupervisor
	a.runtimeCompanionSupervisor = nil
	a.runtimeCompanionSupervisorMu.Unlock()
	if cancel != nil {
		cancel()
		a.runtimeCompanionSupervisorWG.Wait()
	}
}
