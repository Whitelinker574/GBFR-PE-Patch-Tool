package backend

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

var (
	patchCoreInjectMu         sync.Mutex
	runtimeCompanionDLLMu     sync.Mutex
	runtimeCompanionDLLByName = make(map[string]string)
)

type runtimeCompanionStatus struct {
	PID     uint32
	Created uint64
	State   string
	Detail  string
}

type runtimeCompanionOwner struct {
	ID      string
	PID     uint32
	Created uint64
}

type RuntimeCompanionSummary struct {
	ID               string `json:"id"`
	State            string `json:"state"`
	Active           bool   `json:"active"`
	Owned            bool   `json:"owned"`
	RecoveryRequired bool   `json:"recoveryRequired"`
}

func runtimeCompanionMatchesProcess(status runtimeCompanionStatus, process processInstanceID) bool {
	return status.PID != 0 && status.Created != 0 &&
		status.PID == process.PID && status.Created == process.Created
}

func runtimeCompanionNeedsStop(status runtimeCompanionStatus, process processInstanceID) bool {
	return runtimeCompanionMatchesProcess(status, process) && !strings.EqualFold(status.State, "inactive")
}

func runtimeCompanionInstalled(status runtimeCompanionStatus, process processInstanceID) bool {
	return runtimeCompanionNeedsStop(status, process)
}

func runtimeCompanionRecoveryRequired(status runtimeCompanionStatus, process processInstanceID) bool {
	if !runtimeCompanionMatchesProcess(status, process) {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(status.State))
	return state == "restore_failed" || state == "error"
}

// runtimeCompanionStartDecision is evaluated before claiming/injecting. An
// owner without a matching status is an unresolved startup, not a fresh slot:
// injecting again could leave two copies of the same Hook in one process.
func runtimeCompanionStartDecision(status runtimeCompanionStatus, process processInstanceID, owned bool) (alreadyActive bool, err error) {
	matched := runtimeCompanionMatchesProcess(status, process)
	if matched {
		switch strings.ToLower(strings.TrimSpace(status.State)) {
		case "active":
			if owned {
				return true, nil
			}
			return false, errors.New("该游戏进程的运行时组件由另一个工具实例管理")
		case "error", "restore_failed":
			return false, fmt.Errorf("内置运行时处于不可恢复状态，请先重启游戏: %s", status.Detail)
		case "inactive":
			return false, nil
		}
	}
	if owned {
		return false, errors.New("内置运行时启动状态未知；请先停用并恢复，或重启游戏后再试")
	}
	return false, nil
}

func (a *App) runtimeCompanionOwnerID() string {
	a.runtimeCompanionOwnerMu.Lock()
	defer a.runtimeCompanionOwnerMu.Unlock()
	if a.runtimeCompanionOwnerIDValue != "" {
		return a.runtimeCompanionOwnerIDValue
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err == nil {
		a.runtimeCompanionOwnerIDValue = hex.EncodeToString(bytes)
	} else {
		a.runtimeCompanionOwnerIDValue = fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	}
	return a.runtimeCompanionOwnerIDValue
}

func findRuntimeProcessInstance() (processInstanceID, error) {
	pid, err := findProcessByName(charaProcessName)
	if err != nil {
		return processInstanceID{}, err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return processInstanceID{}, err
	}
	defer windows.CloseHandle(handle)
	created, err := processCreationTime(handle)
	if err != nil {
		return processInstanceID{}, err
	}
	return processInstanceID{PID: pid, Created: created}, nil
}

func runtimeCompanionDirectory() (string, error) {
	dir := filepath.Join(os.TempDir(), "gbfr-player-info-edit", "runtime")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func runtimeCompanionPath(name string) (string, error) {
	dir, err := runtimeCompanionDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

func clearStaleInactiveRuntimeCompanionStatus(feature string, status runtimeCompanionStatus, process processInstanceID) error {
	if !runtimeCompanionMatchesProcess(status, process) || !strings.EqualFold(strings.TrimSpace(status.State), "inactive") {
		return nil
	}
	path, err := runtimeCompanionPath(feature + ".status")
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("清理旧运行时状态失败: %w", err)
	}
	return nil
}

func writeRuntimeCompanionFile(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".runtime-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err = temp.Write(data); err == nil {
		err = temp.Sync()
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		err = replaceFileAtomic(tempName, path)
		if err == nil {
			return nil
		}
		if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) &&
			!errors.Is(err, windows.ERROR_LOCK_VIOLATION) &&
			!errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return err
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func readRuntimeINI(path string) map[string]map[string]string {
	result := make(map[string]map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	section := ""
	for _, raw := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if result[section] == nil {
				result[section] = make(map[string]string)
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && section != "" {
			result[section][strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return result
}

func decodeRuntimeStatus(data []byte) string {
	if len(data) >= 2 && len(data)%2 == 0 {
		values := make([]uint16, len(data)/2)
		for index := range values {
			values[index] = binary.LittleEndian.Uint16(data[index*2:])
		}
		return strings.TrimPrefix(string(utf16.Decode(values)), "\ufeff")
	}
	return string(data)
}

func readRuntimeCompanionStatus(feature string) runtimeCompanionStatus {
	path, err := runtimeCompanionPath(feature + ".status")
	if err != nil {
		return runtimeCompanionStatus{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return runtimeCompanionStatus{}
	}
	status := runtimeCompanionStatus{}
	for _, line := range strings.Split(strings.ReplaceAll(decodeRuntimeStatus(data), "\r\n", "\n"), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "pid":
			parsed, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 32)
			status.PID = uint32(parsed)
		case "created":
			status.Created, _ = strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		case "state":
			status.State = strings.TrimSpace(value)
		case "detail":
			status.Detail = strings.TrimSpace(value)
		}
	}
	return status
}

func runtimeCompanionOwnerPath(feature string) (string, error) {
	return runtimeCompanionPath(feature + ".owner")
}

func readRuntimeCompanionOwner(feature string) runtimeCompanionOwner {
	path, err := runtimeCompanionOwnerPath(feature)
	if err != nil {
		return runtimeCompanionOwner{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return runtimeCompanionOwner{}
	}
	owner := runtimeCompanionOwner{}
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "owner":
			owner.ID = strings.TrimSpace(value)
		case "pid":
			parsed, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 32)
			owner.PID = uint32(parsed)
		case "created":
			owner.Created, _ = strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		}
	}
	return owner
}

func (a *App) ownsRuntimeCompanion(feature string, process processInstanceID) bool {
	owner := readRuntimeCompanionOwner(feature)
	return owner.ID != "" && owner.ID == a.runtimeCompanionOwnerID() && owner.PID == process.PID && owner.Created == process.Created
}

func (a *App) releaseRuntimeCompanionOwnership(feature string) {
	path, err := runtimeCompanionOwnerPath(feature)
	if err != nil {
		return
	}
	owner := readRuntimeCompanionOwner(feature)
	if owner.ID == "" || owner.ID != a.runtimeCompanionOwnerID() {
		return
	}
	_ = os.Remove(path)
}

func (a *App) claimRuntimeCompanionOwnership(feature string, process processInstanceID) error {
	path, err := runtimeCompanionOwnerPath(feature)
	if err != nil {
		return err
	}
	ownerID := a.runtimeCompanionOwnerID()
	for attempt := 0; attempt < 2; attempt++ {
		owner := readRuntimeCompanionOwner(feature)
		if owner.ID != "" {
			if owner.ID == ownerID && owner.PID == process.PID && owner.Created == process.Created {
				return nil
			}
			if owner.PID == process.PID && owner.Created == process.Created {
				return errors.New("该游戏进程的运行时组件已由另一个工具实例管理")
			}
			_ = os.Remove(path)
		}
		if status := readRuntimeCompanionStatus(feature); runtimeCompanionNeedsStop(status, process) {
			return errors.New("该游戏进程存在没有可验证所有者的运行时组件；请重启游戏后再试")
		}
		file, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if createErr != nil {
			if errors.Is(createErr, os.ErrExist) {
				continue
			}
			return createErr
		}
		_, writeErr := fmt.Fprintf(file, "owner=%s\npid=%d\ncreated=%d\n", ownerID, process.PID, process.Created)
		closeErr := file.Close()
		if writeErr != nil {
			_ = os.Remove(path)
			return writeErr
		}
		if closeErr != nil {
			_ = os.Remove(path)
			return closeErr
		}
		return nil
	}
	return errors.New("运行时组件所有权正在被另一个工具实例获取")
}

func (a *App) stopOwnedRuntimeCompanion(feature string, disable func() error) error {
	process, processErr := findRuntimeProcessInstance()
	if processErr == nil {
		if !a.ownsRuntimeCompanion(feature, process) {
			if err := a.claimRuntimeCompanionOwnership(feature, process); err != nil {
				return err
			}
		}
	}
	if err := disable(); err != nil {
		return err
	}
	if processErr != nil {
		a.releaseRuntimeCompanionOwnership(feature)
		cleanupRuntimeCompanionDLL(feature)
		return nil
	}
	status := readRuntimeCompanionStatus(feature)
	if !runtimeCompanionNeedsStop(status, process) {
		a.releaseRuntimeCompanionOwnership(feature)
		cleanupRuntimeCompanionDLL(feature)
		return nil
	}
	if err := stopRuntimeCompanion(feature, process); err != nil {
		return err
	}
	a.releaseRuntimeCompanionOwnership(feature)
	return nil
}

func (a *App) runtimeCompanionActive(feature string) bool {
	status := readRuntimeCompanionStatus(feature)
	process, err := findRuntimeProcessInstance()
	return err == nil && runtimeCompanionMatchesProcess(status, process) && status.State == "active" && a.ownsRuntimeCompanion(feature, process)
}

func (a *App) runtimeCompanionOwned(feature string, process processInstanceID) bool {
	return a.ownsRuntimeCompanion(feature, process)
}

func runtimeCompanionPresent(feature string) bool {
	status := readRuntimeCompanionStatus(feature)
	process, err := findRuntimeProcessInstance()
	return err == nil && runtimeCompanionInstalled(status, process)
}

// GetRuntimeCompanionSummary is the shell-level authority for persistent
// camera, audio, and virtual-sigil status. It intentionally avoids loading the
// large per-page catalogs or save inventories.
func (a *App) GetRuntimeCompanionSummary() []RuntimeCompanionSummary {
	features := []struct {
		ID      string
		Runtime string
	}{
		{ID: "camera", Runtime: "camera"},
		{ID: "audioMixer", Runtime: "audio"},
		{ID: "virtualSigils", Runtime: "virtual-sigils"},
	}
	process, processErr := findRuntimeProcessInstance()
	result := make([]RuntimeCompanionSummary, 0, len(features))
	for _, feature := range features {
		status := readRuntimeCompanionStatus(feature.Runtime)
		summary := RuntimeCompanionSummary{ID: feature.ID, State: status.State}
		if processErr == nil && runtimeCompanionMatchesProcess(status, process) {
			summary.Owned = a.runtimeCompanionOwned(feature.Runtime, process)
			summary.Active = summary.Owned && strings.EqualFold(strings.TrimSpace(status.State), "active")
			summary.RecoveryRequired = runtimeCompanionRecoveryRequired(status, process)
		}
		result = append(result, summary)
	}
	return result
}

func extractAndInjectPatchCore(hProcess windows.Handle, command string) (string, error) {
	patchCoreInjectMu.Lock()
	defer patchCoreInjectMu.Unlock()
	dllPath, err := extractPatchCoreDLL(command)
	if err != nil {
		return "", err
	}
	if err := injectDLL(hProcess, dllPath); err != nil {
		return "", err
	}
	return dllPath, nil
}

func rememberRuntimeCompanionDLL(feature, path string) {
	runtimeCompanionDLLMu.Lock()
	runtimeCompanionDLLByName[feature] = path
	runtimeCompanionDLLMu.Unlock()
}

func cleanupRuntimeCompanionDLL(feature string) {
	runtimeCompanionDLLMu.Lock()
	path := runtimeCompanionDLLByName[feature]
	runtimeCompanionDLLMu.Unlock()
	if path == "" {
		return
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := os.Remove(path)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			_ = os.Remove(path + ".command")
			runtimeCompanionDLLMu.Lock()
			if runtimeCompanionDLLByName[feature] == path {
				delete(runtimeCompanionDLLByName, feature)
			}
			runtimeCompanionDLLMu.Unlock()
			return
		}
		if time.Now().After(deadline) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (a *App) startRuntimeCompanion(feature, command string) error {
	if feature != "camera" && feature != "audio" && feature != "virtual-sigils" && feature != "damage" && feature != "qol" {
		return errors.New("未知运行时组件")
	}
	if err := a.acquireGameProcessLease(); err != nil {
		return err
	}
	process := a.currentProcessInstance()
	if err := a.verifyRuntimePatchExecutableLocked(process, runtimePatchMonitorText("内置运行时组件", "Integrated runtime component")); err != nil {
		a.procMu.Unlock()
		return err
	}
	handle := a.hProcess
	status := readRuntimeCompanionStatus(feature)
	owned := a.ownsRuntimeCompanion(feature, process)
	if alreadyActive, decisionErr := runtimeCompanionStartDecision(status, process, owned); decisionErr != nil {
		a.procMu.Unlock()
		return decisionErr
	} else if alreadyActive {
		a.procMu.Unlock()
		return nil
	}
	if err := clearStaleInactiveRuntimeCompanionStatus(feature, status, process); err != nil {
		a.procMu.Unlock()
		return err
	}
	if err := a.claimRuntimeCompanionOwnership(feature, process); err != nil {
		a.procMu.Unlock()
		return err
	}
	dllPath, err := extractAndInjectPatchCore(handle, command)
	a.procMu.Unlock()
	if err != nil {
		a.releaseRuntimeCompanionOwnership(feature)
		return fmt.Errorf("注入内置运行时失败: %w", err)
	}
	// From this point the target process may have loaded the DLL even if its
	// startup status is delayed or reports an error. Retain the extraction path
	// so a later successful restoration can remove both temporary files.
	rememberRuntimeCompanionDLL(feature, dllPath)
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		status := readRuntimeCompanionStatus(feature)
		if runtimeCompanionMatchesProcess(status, process) {
			switch status.State {
			case "active":
				return nil
			case "inactive":
				a.releaseRuntimeCompanionOwnership(feature)
				cleanupRuntimeCompanionDLL(feature)
				return fmt.Errorf("内置运行时未启动: %s", status.Detail)
			case "error", "restore_failed":
				return fmt.Errorf("内置运行时启动失败: %s", status.Detail)
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("等待内置运行时启动超时")
}

func waitRuntimeCompanionStopped(feature string, process processInstanceID) error {
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		status := readRuntimeCompanionStatus(feature)
		if !runtimeCompanionMatchesProcess(status, process) || status.State == "inactive" {
			return nil
		}
		if status.State == "error" || status.State == "restore_failed" {
			return fmt.Errorf("内置运行时停用失败: %s", status.Detail)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("等待内置运行时恢复 Hook 超时")
}

func stopRuntimeCompanion(feature string, process processInstanceID) error {
	if err := waitRuntimeCompanionStopped(feature, process); err != nil {
		return err
	}
	cleanupRuntimeCompanionDLL(feature)
	return nil
}

func restoreRuntimeCompanions(removers ...struct {
	name   string
	remove func(string) error
}) error {
	var restoreErr error
	for _, runtime := range removers {
		if runtime.remove == nil {
			continue
		}
		if err := runtime.remove(""); err != nil {
			restoreErr = errors.Join(restoreErr, fmt.Errorf("%s: %w", runtime.name, err))
		}
	}
	return restoreErr
}

func (a *App) closeRuntimeCompanions() error {
	return restoreRuntimeCompanions(
		struct {
			name   string
			remove func(string) error
		}{"convenience runtime", a.RemoveRuntimeQOL},
		struct {
			name   string
			remove func(string) error
		}{"damage capture", func(string) error { return a.RuntimeDamageCaptureStop() }},
		struct {
			name   string
			remove func(string) error
		}{"virtual sigils", a.RemoveVirtualSigilMod},
		struct {
			name   string
			remove func(string) error
		}{"audio mixer", a.RemoveAudioMixerMod},
		struct {
			name   string
			remove func(string) error
		}{"camera", a.RemoveCameraMod},
	)
}

func encodeVirtualSigilRuntime(config VirtualSigilConfig, enabled bool) ([]byte, error) {
	entries := make([]VirtualSigilConfigEntry, 0)
	for hashText, slots := range config.Characters {
		hash, err := ParseHashHex(hashText)
		if err != nil {
			return nil, err
		}
		for index, slot := range slots {
			if index >= config.SlotCount || slot.SlotID == 0 {
				continue
			}
			entries = append(entries, VirtualSigilConfigEntry{
				CharacterHash: hash, SlotIndex: uint32(index), SlotID: slot.SlotID, GemID: slot.GemID,
				Trait1: slot.Trait1, Trait1Level: int32(slot.Trait1Level), Trait2: slot.Trait2,
				Trait2Level: int32(slot.Trait2Level), SigilLevel: int32(slot.SigilLevel),
			})
		}
	}
	buffer := bytes.NewBuffer(make([]byte, 0, 24+len(entries)*36))
	buffer.WriteString("GBFRVS02")
	values := []uint32{2, 0, uint32(config.SlotCount), uint32(len(entries))}
	if enabled {
		values[1] = 1
	}
	for _, value := range values {
		_ = binary.Write(buffer, binary.LittleEndian, value)
	}
	for _, entry := range entries {
		if err := binary.Write(buffer, binary.LittleEndian, entry); err != nil {
			return nil, err
		}
	}
	return buffer.Bytes(), nil
}

type VirtualSigilConfigEntry struct {
	CharacterHash uint32
	SlotIndex     uint32
	SlotID        uint32
	GemID         uint32
	Trait1        uint32
	Trait1Level   int32
	Trait2        uint32
	Trait2Level   int32
	SigilLevel    int32
}
