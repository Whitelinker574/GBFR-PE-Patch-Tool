package backend

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	confluxTimerManagerPointerRVA         = uintptr(0x07C23E38)
	confluxTimerConfigOffset              = uintptr(0x2DA4)
	confluxTimerModeOffset                = uintptr(0x2DE0)
	confluxTimerActiveOffset              = uintptr(0x346C)
	confluxTimerConfigFloats              = 12
	confluxTimerConfigBytes               = confluxTimerConfigFloats * 4
	confluxTimerActiveBytes               = 8
	confluxTimerEndlessMode       uint32  = 1
	confluxTimerFastSeconds       float32 = 2
)

var (
	confluxTimerOriginalValues = [...]float32{3, 60, 60, 30, 30, 30, 30, 30, 30, 60, 30, 30}
	confluxTimerFastValues     = [...]float32{1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	errConfluxTimerNotReady    = errors.New("极沌空域计时器尚未就绪；进入极沌空域任务后重试")
)

type confluxTimerObservedState uint8

const (
	confluxTimerStateOff confluxTimerObservedState = iota + 1
	confluxTimerStateOn
	confluxTimerStateMixed
	confluxTimerStateUnknown
)

type confluxTimerLeaseState uint8

const (
	confluxTimerLeaseRecovery confluxTimerLeaseState = iota + 1
	confluxTimerLeaseEnabled
)

type confluxTimerSites struct {
	Manager uintptr
	Config  uintptr
	Mode    uintptr
	Active  uintptr
}

type confluxTimerLease struct {
	OwnerToken     string
	Process        processInstanceID
	Sites          confluxTimerSites
	State          confluxTimerLeaseState
	Original       []byte
	PreviousActive []byte
	WrittenActive  []byte
}

type ConfluxTimerStatus struct {
	Verified       bool    `json:"verified"`
	Available      bool    `json:"available"`
	Enabled        bool    `json:"enabled"`
	Owned          bool    `json:"owned"`
	Mode           uint32  `json:"mode"`
	InitialSeconds float32 `json:"initialSeconds"`
	CurrentSeconds float32 `json:"currentSeconds"`
	Error          string  `json:"error"`
}

type confluxTimerMemory interface {
	Read(addr uintptr, size int) ([]byte, error)
	Write(addr uintptr, data []byte) error
}

type confluxTimerProcessMemory struct{ handle windows.Handle }

func (memory confluxTimerProcessMemory) Read(addr uintptr, size int) ([]byte, error) {
	if memory.handle == 0 || addr == 0 || size <= 0 {
		return nil, fmt.Errorf("极沌空域计时器读取参数无效")
	}
	result := make([]byte, size)
	if err := readProcessMemory(memory.handle, addr, unsafe.Pointer(&result[0]), uintptr(size)); err != nil {
		return nil, err
	}
	return result, nil
}

func (memory confluxTimerProcessMemory) Write(addr uintptr, data []byte) error {
	if memory.handle == 0 || addr == 0 || len(data) == 0 {
		return fmt.Errorf("极沌空域计时器写入参数无效")
	}
	return writeProcessMemory(memory.handle, addr, unsafe.Pointer(&data[0]), uintptr(len(data)))
}

func encodeConfluxTimerValues(values [confluxTimerConfigFloats]float32) []byte {
	result := make([]byte, confluxTimerConfigBytes)
	for index, value := range values {
		binary.LittleEndian.PutUint32(result[index*4:], math.Float32bits(value))
	}
	return result
}

func classifyConfluxTimerConfig(config []byte) confluxTimerObservedState {
	if len(config) != confluxTimerConfigBytes {
		return confluxTimerStateUnknown
	}
	original := encodeConfluxTimerValues(confluxTimerOriginalValues)
	fast := encodeConfluxTimerValues(confluxTimerFastValues)
	if bytes.Equal(config, original) {
		return confluxTimerStateOff
	}
	if bytes.Equal(config, fast) {
		return confluxTimerStateOn
	}
	originalFields, fastFields := 0, 0
	for index := 0; index < confluxTimerConfigFloats; index++ {
		field := config[index*4 : index*4+4]
		switch {
		case bytes.Equal(field, original[index*4:index*4+4]):
			originalFields++
		case bytes.Equal(field, fast[index*4:index*4+4]):
			fastFields++
		default:
			return confluxTimerStateUnknown
		}
	}
	if originalFields > 0 && fastFields > 0 {
		return confluxTimerStateMixed
	}
	return confluxTimerStateUnknown
}

func decodeConfluxTimerActive(active []byte) (float32, float32, error) {
	if len(active) != confluxTimerActiveBytes {
		return 0, 0, fmt.Errorf("极沌空域活动计时器长度无效")
	}
	initial := math.Float32frombits(binary.LittleEndian.Uint32(active[:4]))
	current := math.Float32frombits(binary.LittleEndian.Uint32(active[4:]))
	if math.IsNaN(float64(initial)) || math.IsInf(float64(initial), 0) || initial < 0 ||
		math.IsNaN(float64(current)) || math.IsInf(float64(current), 0) || current < 0 {
		return 0, 0, fmt.Errorf("极沌空域活动计时器数值异常")
	}
	return initial, current, nil
}

func shortenConfluxTimerActive(active []byte) ([]byte, error) {
	initial, current, err := decodeConfluxTimerActive(active)
	if err != nil {
		return nil, err
	}
	result := make([]byte, confluxTimerActiveBytes)
	binary.LittleEndian.PutUint32(result[:4], math.Float32bits(min(initial, confluxTimerFastSeconds)))
	binary.LittleEndian.PutUint32(result[4:], math.Float32bits(min(current, confluxTimerFastSeconds)))
	return result, nil
}

func addConfluxTimerOffset(base, offset uintptr) (uintptr, error) {
	result := base + offset
	if base == 0 || result < base {
		return 0, fmt.Errorf("极沌空域计时器地址溢出")
	}
	return result, nil
}

func resolveConfluxTimerSites(memory confluxTimerMemory, moduleBase uintptr) (confluxTimerSites, error) {
	pointerAddress, err := addConfluxTimerOffset(moduleBase, confluxTimerManagerPointerRVA)
	if err != nil {
		return confluxTimerSites{}, err
	}
	pointer, err := memory.Read(pointerAddress, int(unsafe.Sizeof(uintptr(0))))
	if err != nil {
		return confluxTimerSites{}, fmt.Errorf("读取极沌空域计时器管理器失败: %w", err)
	}
	manager := uintptr(binary.LittleEndian.Uint64(pointer))
	if manager == 0 {
		return confluxTimerSites{}, errConfluxTimerNotReady
	}
	config, err := addConfluxTimerOffset(manager, confluxTimerConfigOffset)
	if err != nil {
		return confluxTimerSites{}, err
	}
	mode, err := addConfluxTimerOffset(manager, confluxTimerModeOffset)
	if err != nil {
		return confluxTimerSites{}, err
	}
	active, err := addConfluxTimerOffset(manager, confluxTimerActiveOffset)
	if err != nil {
		return confluxTimerSites{}, err
	}
	return confluxTimerSites{Manager: manager, Config: config, Mode: mode, Active: active}, nil
}

func reconcileConfluxTimerLease(memory confluxTimerMemory, moduleBase uintptr, lease *confluxTimerLease) (*confluxTimerLease, confluxTimerSites, bool, error) {
	currentSites, err := resolveConfluxTimerSites(memory, moduleBase)
	if err != nil {
		if errors.Is(err, errConfluxTimerNotReady) {
			return nil, confluxTimerSites{}, true, nil
		}
		return nil, confluxTimerSites{}, false, err
	}
	if lease == nil {
		return nil, currentSites, false, nil
	}
	if lease.Sites.Manager == currentSites.Manager {
		return cloneConfluxTimerLease(lease), currentSites, false, nil
	}
	config, err := memory.Read(currentSites.Config, confluxTimerConfigBytes)
	if err != nil {
		return nil, currentSites, false, fmt.Errorf("读取换代后的极沌空域等待配置失败: %w", err)
	}
	switch classifyConfluxTimerConfig(config) {
	case confluxTimerStateOff:
		return nil, currentSites, true, nil
	case confluxTimerStateOn:
		rebased := cloneConfluxTimerLease(lease)
		rebased.Sites = currentSites
		// The active countdown belongs to the retired manager. Only the global
		// configuration can safely be restored on the replacement object.
		rebased.State = confluxTimerLeaseEnabled
		return rebased, currentSites, false, nil
	default:
		return nil, currentSites, false, errors.Join(
			fmt.Errorf("极沌空域计时器管理器已换代且配置不是可安全接管的状态"),
			errLiveMemoryRollbackUnproven,
		)
	}
}

func queryProcessImagePath(handle windows.Handle) (string, error) {
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return "", err
	}
	if size == 0 || int(size) > len(buffer) {
		return "", fmt.Errorf("游戏可执行文件路径为空")
	}
	return windows.UTF16ToString(buffer[:size]), nil
}

func hashFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%X", hasher.Sum(nil)), nil
}

func legacyRuntimeExecutableError(featureName, digest string) error {
	if strings.EqualFold(digest, game203ExecutableSHA256) {
		return fmt.Errorf("%s暂未支持游戏 2.0.3：离线存档、配装、分享与 Logs 已适配；实时功能仍在重新定位和实机验收，为保护游戏进程不会连接或写入", featureName)
	}
	return fmt.Errorf("%s仅支持已验证的游戏 2.0.2 可执行文件；当前游戏版本不会连接或写入", featureName)
}

func verifyLegacyRuntimeExecutableHandle(handle windows.Handle, featureName string) error {
	path, err := queryProcessImagePath(handle)
	if err != nil {
		return fmt.Errorf("读取游戏可执行文件路径失败: %w", err)
	}
	digest, err := hashFileSHA256(path)
	if err != nil {
		return fmt.Errorf("校验游戏可执行文件失败: %w", err)
	}
	if !strings.EqualFold(digest, runtimePatchCatalogGameSHA256) {
		return legacyRuntimeExecutableError(featureName, digest)
	}
	return nil
}

func (a *App) verifyRuntimePatchExecutableLocked(process processInstanceID, featureName string) error {
	if sameProcessInstance(a.runtimePatchVerifiedProcess, process) {
		return nil
	}
	if err := verifyLegacyRuntimeExecutableHandle(a.hProcess, featureName); err != nil {
		return err
	}
	a.runtimePatchVerifiedProcess = process
	return nil
}

func readConfluxTimerStatus(memory confluxTimerMemory, sites confluxTimerSites, owned bool) (ConfluxTimerStatus, error) {
	modeBytes, err := memory.Read(sites.Mode, 4)
	if err != nil {
		return ConfluxTimerStatus{}, fmt.Errorf("读取极沌空域模式失败: %w", err)
	}
	config, err := memory.Read(sites.Config, confluxTimerConfigBytes)
	if err != nil {
		return ConfluxTimerStatus{}, fmt.Errorf("读取极沌空域等待配置失败: %w", err)
	}
	active, err := memory.Read(sites.Active, confluxTimerActiveBytes)
	if err != nil {
		return ConfluxTimerStatus{}, fmt.Errorf("读取极沌空域当前倒计时失败: %w", err)
	}
	initial, current, err := decodeConfluxTimerActive(active)
	if err != nil {
		return ConfluxTimerStatus{}, err
	}
	status := ConfluxTimerStatus{
		Verified: true, Mode: binary.LittleEndian.Uint32(modeBytes), InitialSeconds: initial, CurrentSeconds: current, Owned: owned,
	}
	switch classifyConfluxTimerConfig(config) {
	case confluxTimerStateOff:
		status.Available = status.Mode == confluxTimerEndlessMode
	case confluxTimerStateOn:
		status.Enabled = true
		status.Available = owned
		if !owned {
			status.Error = "计时器已被其他工具修改，本工具不会接管未知写入"
		}
	case confluxTimerStateMixed:
		status.Error = "计时器配置处于混合状态；拒绝继续写入"
	case confluxTimerStateUnknown:
		status.Error = "计时器配置不是已验证的原始值或快速值"
	}
	if status.Mode != confluxTimerEndlessMode && status.Error == "" {
		status.Error = "当前不在极沌空域 Endless 模式"
	}
	return status, nil
}

func cloneConfluxTimerLease(lease *confluxTimerLease) *confluxTimerLease {
	if lease == nil {
		return nil
	}
	cloned := *lease
	cloned.Original = append([]byte(nil), lease.Original...)
	cloned.PreviousActive = append([]byte(nil), lease.PreviousActive...)
	cloned.WrittenActive = append([]byte(nil), lease.WrittenActive...)
	return &cloned
}

func restoreConfluxTimerLease(memory confluxTimerMemory, lease *confluxTimerLease) error {
	if lease == nil {
		return nil
	}
	config, err := memory.Read(lease.Sites.Config, confluxTimerConfigBytes)
	if err != nil {
		return errors.Join(fmt.Errorf("读取极沌空域恢复点失败: %w", err), errLiveMemoryRollbackUnproven)
	}
	observedState := classifyConfluxTimerConfig(config)
	if observedState == confluxTimerStateUnknown || (lease.State == confluxTimerLeaseEnabled && observedState == confluxTimerStateMixed) {
		return errors.Join(fmt.Errorf("极沌空域计时器已被其他写入改变，拒绝覆盖"), errLiveMemoryRollbackUnproven)
	}
	if !bytes.Equal(config, lease.Original) {
		if err := memory.Write(lease.Sites.Config, lease.Original); err != nil {
			return errors.Join(fmt.Errorf("恢复极沌空域等待配置失败: %w", err), errLiveMemoryRollbackUnproven)
		}
	}
	if lease.State == confluxTimerLeaseRecovery && len(lease.PreviousActive) == confluxTimerActiveBytes {
		if err := memory.Write(lease.Sites.Active, lease.PreviousActive); err != nil {
			return errors.Join(fmt.Errorf("回滚极沌空域活动倒计时失败: %w", err), errLiveMemoryRollbackUnproven)
		}
	}
	verified, err := memory.Read(lease.Sites.Config, confluxTimerConfigBytes)
	if err != nil || !bytes.Equal(verified, lease.Original) {
		return errors.Join(err, fmt.Errorf("极沌空域等待配置恢复回读不一致"), errLiveMemoryRollbackUnproven)
	}
	if lease.State == confluxTimerLeaseRecovery && len(lease.PreviousActive) == confluxTimerActiveBytes {
		active, readErr := memory.Read(lease.Sites.Active, confluxTimerActiveBytes)
		if readErr != nil || !confluxTimerActiveDidNotIncrease(lease.PreviousActive, active) {
			return errors.Join(readErr, fmt.Errorf("极沌空域活动倒计时回滚回读不一致"), errLiveMemoryRollbackUnproven)
		}
	}
	return nil
}

func confluxTimerActiveDidNotIncrease(limit, observed []byte) bool {
	limitInitial, limitCurrent, limitErr := decodeConfluxTimerActive(limit)
	observedInitial, observedCurrent, observedErr := decodeConfluxTimerActive(observed)
	if limitErr != nil || observedErr != nil {
		return false
	}
	return observedInitial <= limitInitial && observedCurrent <= limitCurrent
}

func installConfluxTimerLease(memory confluxTimerMemory, lease *confluxTimerLease) error {
	if lease == nil || len(lease.Original) != confluxTimerConfigBytes || len(lease.PreviousActive) != confluxTimerActiveBytes || len(lease.WrittenActive) != confluxTimerActiveBytes {
		return fmt.Errorf("极沌空域计时器恢复租约不完整")
	}
	rollback := func(operationErr error) error {
		if restoreErr := restoreConfluxTimerLease(memory, lease); restoreErr != nil {
			return errors.Join(operationErr, restoreErr)
		}
		return operationErr
	}
	if err := memory.Write(lease.Sites.Config, encodeConfluxTimerValues(confluxTimerFastValues)); err != nil {
		return rollback(fmt.Errorf("写入极沌空域快速配置失败: %w", err))
	}
	if err := memory.Write(lease.Sites.Active, lease.WrittenActive); err != nil {
		return rollback(fmt.Errorf("缩短极沌空域当前倒计时失败: %w", err))
	}
	verifiedConfig, configErr := memory.Read(lease.Sites.Config, confluxTimerConfigBytes)
	verifiedActive, activeErr := memory.Read(lease.Sites.Active, confluxTimerActiveBytes)
	if configErr != nil || activeErr != nil || classifyConfluxTimerConfig(verifiedConfig) != confluxTimerStateOn || !confluxTimerActiveDidNotIncrease(lease.WrittenActive, verifiedActive) {
		return rollback(errors.Join(configErr, activeErr, fmt.Errorf("极沌空域快速等待写后回读不一致")))
	}
	return nil
}

func (a *App) ConfluxTimerGetStatusOwned(token string) (ConfluxTimerStatus, error) {
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return ConfluxTimerStatus{}, err
	}
	defer a.procMu.Unlock()
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	process := a.currentProcessInstance()
	if !sameProcessInstance(a.runtimePatchVerifiedProcess, process) {
		return ConfluxTimerStatus{Error: "尚未校验游戏版本；点击“验证并读取”后再使用"}, nil
	}
	return a.readConfluxTimerStatusOwnedLocked(token, process)
}

func (a *App) readConfluxTimerStatusOwnedLocked(token string, process processInstanceID) (ConfluxTimerStatus, error) {
	memory := confluxTimerProcessMemory{handle: a.hProcess}
	if lease := a.confluxTimerLease; lease != nil && runtimeOwnerTokenMatches(lease.OwnerToken, token) && sameProcessInstance(lease.Process, process) {
		reconciled, currentSites, retired, reconcileErr := reconcileConfluxTimerLease(memory, a.moduleBase, lease)
		if reconcileErr != nil {
			return ConfluxTimerStatus{Verified: true, Owned: true, Error: reconcileErr.Error()}, nil
		}
		if retired {
			a.confluxTimerLease = nil
			if currentSites.Manager == 0 {
				return ConfluxTimerStatus{Verified: true, Error: errConfluxTimerNotReady.Error()}, nil
			}
			status, err := readConfluxTimerStatus(memory, currentSites, false)
			if err != nil {
				return ConfluxTimerStatus{Verified: true, Error: err.Error()}, nil
			}
			return status, nil
		}
		a.confluxTimerLease = reconciled
		status, err := readConfluxTimerStatus(memory, reconciled.Sites, true)
		if err != nil {
			return ConfluxTimerStatus{Verified: true, Owned: true, Error: err.Error()}, nil
		}
		return status, nil
	}
	sites, err := resolveConfluxTimerSites(memory, a.moduleBase)
	if err != nil {
		return ConfluxTimerStatus{Verified: true, Error: err.Error()}, nil
	}
	status, err := readConfluxTimerStatus(memory, sites, false)
	if err != nil {
		return ConfluxTimerStatus{Verified: true, Error: err.Error()}, nil
	}
	return status, nil
}

func (a *App) ConfluxTimerVerifyStatusOwned(token string) (ConfluxTimerStatus, error) {
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return ConfluxTimerStatus{}, err
	}
	defer a.procMu.Unlock()
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	process := a.currentProcessInstance()
	if err := a.verifyRuntimePatchExecutableLocked(process, "极沌空域快速等待"); err != nil {
		return ConfluxTimerStatus{Error: err.Error()}, nil
	}
	return a.readConfluxTimerStatusOwnedLocked(token, process)
}

func (a *App) ConfluxTimerSetEnabledOwned(token string, enabled bool) (ConfluxTimerStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return ConfluxTimerStatus{}, err
	}
	defer a.procMu.Unlock()
	if enabled {
		if err := a.ensureLiveMemoryWritesSafe(); err != nil {
			return ConfluxTimerStatus{}, err
		}
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	process := a.currentProcessInstance()
	if enabled {
		if err := a.verifyRuntimePatchExecutableLocked(process, "极沌空域快速等待"); err != nil {
			return ConfluxTimerStatus{}, err
		}
	}
	memory := confluxTimerProcessMemory{handle: a.hProcess}
	if !enabled {
		lease := a.confluxTimerLease
		if lease == nil {
			if !sameProcessInstance(a.runtimePatchVerifiedProcess, process) {
				return ConfluxTimerStatus{Error: "尚未校验游戏版本；点击“验证并读取”后再使用"}, nil
			}
			sites, err := resolveConfluxTimerSites(memory, a.moduleBase)
			if err != nil {
				return ConfluxTimerStatus{Error: err.Error()}, nil
			}
			return readConfluxTimerStatus(memory, sites, false)
		}
		if !runtimeOwnerTokenMatches(lease.OwnerToken, token) || !sameProcessInstance(lease.Process, process) {
			return ConfluxTimerStatus{}, errRuntimeOwnerLeaseStale
		}
		reconciled, currentSites, retired, err := reconcileConfluxTimerLease(memory, a.moduleBase, lease)
		if err != nil {
			a.poisonCurrentLiveMemoryWrites()
			return ConfluxTimerStatus{}, err
		}
		if retired {
			a.confluxTimerLease = nil
			if currentSites.Manager == 0 {
				return ConfluxTimerStatus{Verified: true, Error: errConfluxTimerNotReady.Error()}, nil
			}
			return readConfluxTimerStatus(memory, currentSites, false)
		}
		a.confluxTimerLease = reconciled
		if err := restoreConfluxTimerLease(memory, reconciled); err != nil {
			a.poisonCurrentLiveMemoryWrites()
			return ConfluxTimerStatus{}, err
		}
		a.confluxTimerLease = nil
		sites, resolveErr := resolveConfluxTimerSites(memory, a.moduleBase)
		if resolveErr != nil {
			return ConfluxTimerStatus{Error: resolveErr.Error()}, nil
		}
		return readConfluxTimerStatus(memory, sites, false)
	}
	sites, err := resolveConfluxTimerSites(memory, a.moduleBase)
	if err != nil {
		return ConfluxTimerStatus{}, err
	}
	modeBytes, err := memory.Read(sites.Mode, 4)
	if err != nil {
		return ConfluxTimerStatus{}, fmt.Errorf("读取极沌空域模式失败: %w", err)
	}
	if binary.LittleEndian.Uint32(modeBytes) != confluxTimerEndlessMode {
		return ConfluxTimerStatus{}, fmt.Errorf("当前不在极沌空域 Endless 模式")
	}
	if lease := a.confluxTimerLease; lease != nil {
		if !runtimeOwnerTokenMatches(lease.OwnerToken, token) || !sameProcessInstance(lease.Process, process) {
			return ConfluxTimerStatus{}, errRuntimeOwnerLeaseStale
		}
		reconciled, currentSites, retired, reconcileErr := reconcileConfluxTimerLease(memory, a.moduleBase, lease)
		if reconcileErr != nil {
			a.poisonCurrentLiveMemoryWrites()
			return ConfluxTimerStatus{}, reconcileErr
		}
		if !retired {
			a.confluxTimerLease = reconciled
			return readConfluxTimerStatus(memory, currentSites, true)
		}
		a.confluxTimerLease = nil
	}
	config, err := memory.Read(sites.Config, confluxTimerConfigBytes)
	if err != nil {
		return ConfluxTimerStatus{}, err
	}
	if classifyConfluxTimerConfig(config) != confluxTimerStateOff {
		return ConfluxTimerStatus{}, fmt.Errorf("极沌空域计时器不是已验证的原始配置，拒绝接管")
	}
	active, err := memory.Read(sites.Active, confluxTimerActiveBytes)
	if err != nil {
		return ConfluxTimerStatus{}, err
	}
	shortened, err := shortenConfluxTimerActive(active)
	if err != nil {
		return ConfluxTimerStatus{}, err
	}
	lease := &confluxTimerLease{
		OwnerToken: token, Process: process, Sites: sites, State: confluxTimerLeaseRecovery,
		Original: append([]byte(nil), config...), PreviousActive: append([]byte(nil), active...), WrittenActive: append([]byte(nil), shortened...),
	}
	a.confluxTimerLease = cloneConfluxTimerLease(lease)
	if err := installConfluxTimerLease(memory, a.confluxTimerLease); err != nil {
		if errors.Is(err, errLiveMemoryRollbackUnproven) {
			a.poisonCurrentLiveMemoryWrites()
		} else {
			a.confluxTimerLease = nil
		}
		return ConfluxTimerStatus{}, err
	}
	lease.State = confluxTimerLeaseEnabled
	a.confluxTimerLease = cloneConfluxTimerLease(lease)
	return readConfluxTimerStatus(memory, sites, true)
}

// restoreConfluxTimerOwnedLocked runs with procMu and runtimePatchMu held.
func (a *App) restoreConfluxTimerOwnedLocked(owner string, force bool) error {
	lease := a.confluxTimerLease
	if lease == nil || (!force && !runtimeOwnerTokenMatches(lease.OwnerToken, owner)) {
		return nil
	}
	if a.hProcess == 0 || !processHandleAlive(a.hProcess) {
		a.confluxTimerLease = nil
		return nil
	}
	if !sameProcessInstance(lease.Process, a.currentProcessInstance()) {
		return errors.Join(fmt.Errorf("极沌空域计时器属于已替换的游戏进程"), errLiveMemoryRollbackUnproven)
	}
	memory := confluxTimerProcessMemory{handle: a.hProcess}
	reconciled, _, retired, err := reconcileConfluxTimerLease(memory, a.moduleBase, lease)
	if err != nil {
		a.poisonCurrentLiveMemoryWrites()
		return err
	}
	if retired {
		a.confluxTimerLease = nil
		return nil
	}
	a.confluxTimerLease = reconciled
	if err := restoreConfluxTimerLease(memory, reconciled); err != nil {
		a.poisonCurrentLiveMemoryWrites()
		return err
	}
	a.confluxTimerLease = nil
	return nil
}

func (a *App) dropConfluxTimerOwnerLocked(owner string) {
	if a.confluxTimerLease != nil && runtimeOwnerTokenMatches(a.confluxTimerLease.OwnerToken, owner) {
		a.confluxTimerLease = nil
	}
}
