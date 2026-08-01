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
	patchCoreInjectMu           sync.Mutex
	runtimeCompanionDLLMu       sync.Mutex
	runtimeCompanionOwnerFileMu sync.Mutex
	runtimeCompanionDLLByName   = make(map[string]string)
)

type runtimeCompanionStatus struct {
	PID        uint32
	Created    uint64
	Generation string
	State      string
	Detail     string
}

type runtimeCompanionOwner struct {
	ID         string
	PID        uint32
	Created    uint64
	Generation string
}

// runtimeCompanionLease is a kernel-enforced, process-lifetime lease. The
// delete-on-close handle prevents two desktop processes from passing the
// owner-file check at the same time, and Windows releases it even when the
// desktop process is forcibly terminated.
type runtimeCompanionLease struct {
	Handle     windows.Handle
	Process    processInstanceID
	Generation string
}

type RuntimeCompanionSummary struct {
	ID               string `json:"id"`
	State            string `json:"state"`
	Active           bool   `json:"active"`
	Owned            bool   `json:"owned"`
	RecoveryRequired bool   `json:"recoveryRequired"`
	ActiveCount      int    `json:"activeCount,omitempty"`
	RecoveryCount    int    `json:"recoveryCount,omitempty"`
	Multiplier       int    `json:"multiplier,omitempty"`
	PID              uint32 `json:"pid,omitempty"`
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
func runtimeCompanionStartDecision(status runtimeCompanionStatus, process processInstanceID, ownedGeneration string) (alreadyActive bool, err error) {
	matched := runtimeCompanionMatchesProcess(status, process)
	owned := ownedGeneration != ""
	if owned && (!matched || status.Generation != ownedGeneration) {
		return false, errors.New("内置运行时启动状态与当前所有权代次不一致；请先停用并恢复，或重启游戏后再试")
	}
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

func newRuntimeCompanionGeneration() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("生成运行时所有权代次失败: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func validRuntimeCompanionGeneration(value string) bool {
	if len(value) != 32 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
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
		case "generation":
			status.Generation = strings.TrimSpace(value)
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

func runtimeCompanionLeasePath(feature string) (string, error) {
	return runtimeCompanionPath(feature + ".lease")
}

func cleanupRuntimeCompanionStatusTemporaries(feature string) error {
	dir, err := runtimeCompanionDirectory()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	prefix := feature + ".status."
	const suffix = ".tmp"
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		generation := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
		if !validRuntimeCompanionGeneration(generation) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("检查旧运行时状态临时文件失败: %w", err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("清理旧运行时状态临时文件失败: %w", err)
		}
	}
	return nil
}

func createExclusiveDeleteOnCloseFile(path string) (windows.Handle, error) {
	pathUTF16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	handle, err := windows.CreateFile(
		pathUTF16,
		windows.GENERIC_READ|windows.GENERIC_WRITE|windows.DELETE,
		0,
		nil,
		windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_TEMPORARY|windows.FILE_FLAG_DELETE_ON_CLOSE,
		0,
	)
	if err != nil {
		return 0, err
	}
	return handle, nil
}

func runtimeCompanionProcessAlive(process processInstanceID) (bool, error) {
	if process.PID == 0 || process.Created == 0 {
		return false, errors.New("运行时租约缺少完整的游戏进程身份")
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, process.PID)
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return false, nil
		}
		return false, err
	}
	defer windows.CloseHandle(handle)
	created, err := processCreationTime(handle)
	if err != nil {
		return false, err
	}
	if created != process.Created {
		return false, nil
	}
	wait, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return false, err
	}
	switch uint32(wait) {
	case uint32(windows.WAIT_TIMEOUT):
		return true, nil
	case uint32(windows.WAIT_OBJECT_0):
		return false, nil
	default:
		return false, fmt.Errorf("检查运行时租约游戏进程状态失败: wait=%d", wait)
	}
}

func (a *App) acquireRuntimeCompanionLease(feature string, process processInstanceID) (bool, error) {
	a.runtimeCompanionLeaseMu.Lock()
	defer a.runtimeCompanionLeaseMu.Unlock()
	if lease, ok := a.runtimeCompanionLeases[feature]; ok {
		if lease.Handle != 0 && lease.Process == process {
			return false, nil
		}
		if lease.Handle != 0 {
			alive, err := runtimeCompanionProcessAlive(lease.Process)
			if err != nil {
				return false, fmt.Errorf("验证旧运行时租约失败: %w", err)
			}
			if alive {
				return false, errors.New("当前工具实例仍持有另一存活游戏进程的运行时租约")
			}
			_ = windows.CloseHandle(lease.Handle)
		}
		delete(a.runtimeCompanionLeases, feature)
	}
	path, err := runtimeCompanionLeasePath(feature)
	if err != nil {
		return false, err
	}
	handle, err := createExclusiveDeleteOnCloseFile(path)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return false, errors.New("该游戏进程的运行时组件正由另一个工具实例管理")
		}
		return false, fmt.Errorf("创建运行时独占租约失败: %w", err)
	}
	generation, err := newRuntimeCompanionGeneration()
	if err != nil {
		_ = windows.CloseHandle(handle)
		return false, err
	}
	if a.runtimeCompanionLeases == nil {
		a.runtimeCompanionLeases = make(map[string]runtimeCompanionLease)
	}
	a.runtimeCompanionLeases[feature] = runtimeCompanionLease{
		Handle:     handle,
		Process:    process,
		Generation: generation,
	}
	return true, nil
}

func (a *App) runtimeCompanionLease(feature string, process processInstanceID) (runtimeCompanionLease, bool) {
	a.runtimeCompanionLeaseMu.Lock()
	defer a.runtimeCompanionLeaseMu.Unlock()
	lease, ok := a.runtimeCompanionLeases[feature]
	if !ok || lease.Handle == 0 || lease.Process != process || !validRuntimeCompanionGeneration(lease.Generation) {
		return runtimeCompanionLease{}, false
	}
	return lease, true
}

func (a *App) runtimeCompanionLeaseByFeature(feature string) (runtimeCompanionLease, bool) {
	a.runtimeCompanionLeaseMu.Lock()
	defer a.runtimeCompanionLeaseMu.Unlock()
	lease, ok := a.runtimeCompanionLeases[feature]
	if !ok || lease.Handle == 0 || !validRuntimeCompanionGeneration(lease.Generation) {
		return runtimeCompanionLease{}, false
	}
	return lease, true
}

func (a *App) dropRuntimeCompanionLease(feature, generation string) {
	a.runtimeCompanionLeaseMu.Lock()
	lease, ok := a.runtimeCompanionLeases[feature]
	if ok && lease.Generation == generation {
		delete(a.runtimeCompanionLeases, feature)
	} else {
		ok = false
	}
	a.runtimeCompanionLeaseMu.Unlock()
	if ok && lease.Handle != 0 {
		_ = windows.CloseHandle(lease.Handle)
	}
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
		case "generation":
			owner.Generation = strings.TrimSpace(value)
		}
	}
	return owner
}

func (a *App) ownsRuntimeCompanion(feature string, process processInstanceID) bool {
	lease, ok := a.runtimeCompanionLease(feature, process)
	if !ok {
		return false
	}
	owner := readRuntimeCompanionOwner(feature)
	return owner.ID != "" &&
		owner.ID == a.runtimeCompanionOwnerID() &&
		owner.PID == process.PID &&
		owner.Created == process.Created &&
		owner.Generation == lease.Generation
}

func (a *App) releaseRuntimeCompanionOwnership(feature string) {
	runtimeCompanionOwnerFileMu.Lock()
	defer runtimeCompanionOwnerFileMu.Unlock()
	lease, ok := a.runtimeCompanionLeaseByFeature(feature)
	if !ok {
		return
	}
	defer a.dropRuntimeCompanionLease(feature, lease.Generation)
	path, err := runtimeCompanionOwnerPath(feature)
	if err != nil {
		return
	}
	owner := readRuntimeCompanionOwner(feature)
	if owner.ID == "" ||
		owner.ID != a.runtimeCompanionOwnerID() ||
		owner.Generation != lease.Generation {
		return
	}
	_ = os.Remove(path)
}

func (a *App) claimRuntimeCompanionOwnership(feature string, process processInstanceID) error {
	runtimeCompanionOwnerFileMu.Lock()
	defer runtimeCompanionOwnerFileMu.Unlock()
	path, err := runtimeCompanionOwnerPath(feature)
	if err != nil {
		return err
	}
	leaseCreated, err := a.acquireRuntimeCompanionLease(feature, process)
	if err != nil {
		return err
	}
	claimed := false
	lease, ok := a.runtimeCompanionLease(feature, process)
	if !ok {
		return errors.New("运行时独占租约缺少有效代次")
	}
	defer func() {
		if leaseCreated && !claimed {
			a.dropRuntimeCompanionLease(feature, lease.Generation)
		}
	}()
	ownerID := a.runtimeCompanionOwnerID()
	for attempt := 0; attempt < 2; attempt++ {
		owner := readRuntimeCompanionOwner(feature)
		if owner.ID != "" {
			if owner.ID == ownerID &&
				owner.PID == process.PID &&
				owner.Created == process.Created &&
				owner.Generation == lease.Generation {
				claimed = true
				return nil
			}
			if owner.PID == process.PID && owner.Created == process.Created {
				return errors.New("该游戏进程仍保留另一个运行时所有者；请等待其完成恢复")
			}
			if owner.PID == 0 || owner.Created == 0 {
				return errors.New("运行时所有者记录不完整；为避免覆盖未知 Hook，拒绝接管")
			}
			alive, aliveErr := runtimeCompanionProcessAlive(processInstanceID{PID: owner.PID, Created: owner.Created})
			if aliveErr != nil {
				return fmt.Errorf("验证旧运行时所有者对应的游戏进程失败: %w", aliveErr)
			}
			if alive {
				return errors.New("另一个仍存活的游戏进程保留运行时所有权；拒绝跨进程接管")
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("清理已死亡游戏进程的运行时所有者记录失败: %w", err)
			}
		}
		if status := readRuntimeCompanionStatus(feature); runtimeCompanionNeedsStop(status, process) {
			return errors.New("该游戏进程存在没有可验证所有者的运行时组件；请重启游戏后再试")
		}
		if err := cleanupRuntimeCompanionStatusTemporaries(feature); err != nil {
			return err
		}
		file, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if createErr != nil {
			if errors.Is(createErr, os.ErrExist) {
				continue
			}
			return createErr
		}
		_, writeErr := fmt.Fprintf(
			file,
			"owner=%s\ngeneration=%s\npid=%d\ncreated=%d\n",
			ownerID,
			lease.Generation,
			process.PID,
			process.Created,
		)
		closeErr := file.Close()
		if writeErr != nil {
			owner := readRuntimeCompanionOwner(feature)
			if owner.ID == ownerID && owner.Generation == lease.Generation {
				_ = os.Remove(path)
			}
			return writeErr
		}
		if closeErr != nil {
			owner := readRuntimeCompanionOwner(feature)
			if owner.ID == ownerID && owner.Generation == lease.Generation {
				_ = os.Remove(path)
			}
			return closeErr
		}
		claimed = true
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
	lease, ok := a.runtimeCompanionLease(feature, process)
	if !ok {
		return errors.New("运行时组件缺少当前进程的所有权代次，拒绝等待未知 Hook")
	}
	if err := stopRuntimeCompanion(feature, process, lease.Generation); err != nil {
		return err
	}
	a.releaseRuntimeCompanionOwnership(feature)
	return nil
}

func (a *App) runtimeCompanionActive(feature string) bool {
	status := readRuntimeCompanionStatus(feature)
	process, err := findRuntimeProcessInstance()
	if err != nil || !a.ownsRuntimeCompanion(feature, process) {
		return false
	}
	lease, ok := a.runtimeCompanionLease(feature, process)
	return ok &&
		runtimeCompanionMatchesProcess(status, process) &&
		status.Generation == lease.Generation &&
		status.State == "active"
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
// camera, audio, virtual-sigil, damage-capture, and QOL status. It intentionally avoids loading the
// large per-page catalogs or save inventories.
func (a *App) GetRuntimeCompanionSummary() []RuntimeCompanionSummary {
	features := []struct {
		ID      string
		Runtime string
	}{
		{ID: "camera", Runtime: "camera"},
		{ID: "audioMixer", Runtime: "audio"},
		{ID: "virtualSigils", Runtime: "virtual-sigils"},
		{ID: "loadoutPresets", Runtime: "damage"},
		{ID: "runtimeQOL", Runtime: "qol"},
	}
	process, processErr := findRuntimeProcessInstance()
	result := make([]RuntimeCompanionSummary, 0, len(features))
	for _, feature := range features {
		status := readRuntimeCompanionStatus(feature.Runtime)
		summary := RuntimeCompanionSummary{ID: feature.ID, State: status.State}
		if processErr == nil && runtimeCompanionMatchesProcess(status, process) {
			summary.Owned = a.runtimeCompanionOwned(feature.Runtime, process)
			lease, leased := a.runtimeCompanionLease(feature.Runtime, process)
			summary.Active = summary.Owned &&
				leased &&
				status.Generation == lease.Generation &&
				strings.EqualFold(strings.TrimSpace(status.State), "active")
			summary.RecoveryRequired = runtimeCompanionRecoveryRequired(status, process)
		}
		result = append(result, summary)
	}
	a.runtimeLoadoutDetectorMu.Lock()
	detector := a.runtimeLoadoutDetector
	a.runtimeLoadoutDetectorMu.Unlock()
	if detector != nil {
		status := detector.status()
		result = append(result, RuntimeCompanionSummary{ID: "runtimeMonitor", State: status.State, Active: status.Enabled})
	} else {
		result = append(result, RuntimeCompanionSummary{ID: "runtimeMonitor", State: "stopped"})
	}
	result = append(result, a.runtimePatchCompanionSummary(), a.taskRewardMultiplierCompanionSummary())
	return result
}

func (a *App) taskRewardMultiplierCompanionSummary() RuntimeCompanionSummary {
	summary := RuntimeCompanionSummary{ID: "taskRewardMultiplier", State: "inactive", Multiplier: 1}
	a.procMu.Lock()
	defer a.procMu.Unlock()
	process := a.currentProcessInstance()
	if a.hProcess == 0 || process.PID == 0 || process.Created == 0 || !processHandleAlive(a.hProcess) {
		return summary
	}
	live, err := findRuntimeProcessInstance()
	if err != nil || !sameProcessInstance(live, process) {
		return summary
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	lease := a.taskRewardMultiplierLease
	if lease == nil || lease.OwnerToken == "" || lease.OwnerToken != a.charaOwnerToken || !sameProcessInstance(lease.Process, process) {
		return summary
	}
	summary.PID = process.PID
	summary.Multiplier = lease.Multiplier
	summary.Active = lease.Multiplier > 1
	summary.Owned = true
	if summary.Active {
		summary.State = "active"
	}
	return summary
}

func (a *App) runtimePatchCompanionSummary() RuntimeCompanionSummary {
	summary := RuntimeCompanionSummary{ID: "runtimePatches", State: "inactive"}
	a.procMu.Lock()
	defer a.procMu.Unlock()
	process := a.currentProcessInstance()
	if a.hProcess == 0 || process.PID == 0 || process.Created == 0 || !processHandleAlive(a.hProcess) {
		return summary
	}
	live, err := findRuntimeProcessInstance()
	if err != nil || !sameProcessInstance(live, process) {
		return summary
	}
	summary.PID = process.PID
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	owner := a.charaOwnerToken
	for _, lease := range a.runtimePatchPatchLeases {
		if owner == "" || lease.OwnerToken != owner || !sameProcessInstance(lease.Process, process) {
			continue
		}
		if lease.State == runtimePatchPatchEnabled {
			summary.ActiveCount++
		} else {
			summary.RecoveryCount++
		}
	}
	if lease := a.confluxTimerLease; lease != nil && owner != "" && lease.OwnerToken == owner && sameProcessInstance(lease.Process, process) {
		if lease.State == confluxTimerLeaseEnabled {
			summary.ActiveCount++
		} else {
			summary.RecoveryCount++
		}
	}
	for _, lease := range []*combatTuningLease{a.combatTuningCooldownLease, a.combatTuningChargeLease} {
		if lease != nil && lease.active() && owner != "" && lease.OwnerToken == owner && sameProcessInstance(lease.Process, process) {
			summary.ActiveCount++
		}
	}
	summary.Active = summary.ActiveCount > 0
	summary.Owned = owner != "" && (summary.ActiveCount > 0 || summary.RecoveryCount > 0)
	summary.RecoveryRequired = summary.RecoveryCount > 0
	if summary.RecoveryRequired {
		summary.State = "recovery_required"
	} else if summary.Active {
		summary.State = "active"
	}
	return summary
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

func runtimeCompanionCommand(command, generation string) (string, error) {
	if !validRuntimeCompanionGeneration(generation) {
		return "", errors.New("运行时所有权代次无效")
	}
	created, err := processCreationTime(windows.CurrentProcess())
	if err != nil {
		return "", fmt.Errorf("读取工具进程创建时间失败: %w", err)
	}
	return fmt.Sprintf(
		"%s\nowner_pid=%d\nowner_created=%d\ngeneration=%s\n",
		strings.TrimSpace(command),
		os.Getpid(),
		created,
		generation,
	), nil
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
	if feature != "camera" && feature != "audio" && feature != "virtual-sigils" && feature != "damage" && feature != "qol" && feature != "party-observer" {
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
	ownedGeneration := ""
	if a.ownsRuntimeCompanion(feature, process) {
		if lease, ok := a.runtimeCompanionLease(feature, process); ok {
			ownedGeneration = lease.Generation
		}
	}
	if alreadyActive, decisionErr := runtimeCompanionStartDecision(status, process, ownedGeneration); decisionErr != nil {
		a.procMu.Unlock()
		return decisionErr
	} else if alreadyActive {
		a.procMu.Unlock()
		return nil
	}
	if err := a.claimRuntimeCompanionOwnership(feature, process); err != nil {
		a.procMu.Unlock()
		return err
	}
	if err := clearStaleInactiveRuntimeCompanionStatus(feature, status, process); err != nil {
		a.procMu.Unlock()
		a.releaseRuntimeCompanionOwnership(feature)
		return err
	}
	lease, ok := a.runtimeCompanionLease(feature, process)
	if !ok {
		a.procMu.Unlock()
		a.releaseRuntimeCompanionOwnership(feature)
		return errors.New("运行时所有权代次在注入前丢失")
	}
	ownedCommand, err := runtimeCompanionCommand(command, lease.Generation)
	if err != nil {
		a.procMu.Unlock()
		a.releaseRuntimeCompanionOwnership(feature)
		return err
	}
	dllPath, err := extractAndInjectPatchCore(handle, ownedCommand)
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
		if runtimeCompanionMatchesProcess(status, process) && status.Generation == lease.Generation {
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

func waitRuntimeCompanionStopped(feature string, process processInstanceID, generations ...string) error {
	generation := ""
	if len(generations) > 0 {
		generation = generations[0]
	} else {
		// Compatibility for the virtual-sigil hot-restart path: capture the
		// generation before waiting so a later status from another owner cannot
		// be mistaken for this runtime having stopped.
		generation = readRuntimeCompanionStatus(feature).Generation
	}
	if !validRuntimeCompanionGeneration(generation) {
		return errors.New("等待运行时恢复时缺少有效所有权代次")
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		status := readRuntimeCompanionStatus(feature)
		if !runtimeCompanionMatchesProcess(status, process) {
			return nil
		}
		if status.Generation != generation {
			return errors.New("运行时状态已切换到其他所有权代次，拒绝把它视为当前 Hook 已恢复")
		}
		if status.State == "inactive" {
			return nil
		}
		if status.State == "error" || status.State == "restore_failed" {
			return fmt.Errorf("内置运行时停用失败: %s", status.Detail)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("等待内置运行时恢复 Hook 超时")
}

func stopRuntimeCompanion(feature string, process processInstanceID, generation string) error {
	if err := waitRuntimeCompanionStopped(feature, process, generation); err != nil {
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
