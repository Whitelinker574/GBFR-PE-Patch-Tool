package backend

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

const (
	runtimeQOLMappingName         = "Local\\GBFRPlayerInfoEditQOLV2"
	runtimeQOLMagic               = uint64(0x31564C4F51465247) // GRFQOLV1
	runtimeQOLVersion             = uint32(2)
	runtimeQOLMappingSize         = 64
	runtimeQOLSessionEventName    = "runtime-qol-session"
	runtimeQOLSessionPollInterval = 250 * time.Millisecond
)

var runtimeQOLMu sync.Mutex

type RuntimeQOLConfig struct {
	DamageCapPercentage  bool `json:"damageCapPercentage"`
	DetailedEnemyHP      bool `json:"detailedEnemyHp"`
	DetailedSBA          bool `json:"detailedSba"`
	SessionCapture       bool `json:"sessionCapture"`
	NormalQuestLevelSync bool `json:"normalQuestLevelSync"`
	ReturnWrightstone    bool `json:"returnWrightstone"`
	FreeCaptain          bool `json:"freeCaptain"`
	EnemyHPPrecision     int  `json:"enemyHpPrecision"`
	SBAPrecision         int  `json:"sbaPrecision"`
}

type RuntimeQOLWorkspace struct {
	Active           bool             `json:"active"`
	Installed        bool             `json:"installed"`
	Owned            bool             `json:"owned"`
	RecoveryRequired bool             `json:"recoveryRequired"`
	State            string           `json:"state"`
	GameRunning      bool             `json:"gameRunning"`
	PID              uint32           `json:"pid"`
	Config           RuntimeQOLConfig `json:"config"`
	LatestSessionID  string           `json:"latestSessionId"`
	SessionSequence  uint64           `json:"sessionSequence"`
	Detail           string           `json:"detail"`
}

type RuntimeQOLSessionEvent struct {
	SessionID string `json:"sessionId"`
	Sequence  uint64 `json:"sequence"`
}

type runtimeQOLSessionWatcher struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func runtimeQOLSessionEventAfter(
	last uint64,
	status runtimeCompanionStatus,
	process processInstanceID,
	mappingPID uint32,
	sequence uint64,
	session string,
) (RuntimeQOLSessionEvent, bool) {
	session = strings.TrimSpace(session)
	if !strings.EqualFold(status.State, "active") ||
		!runtimeCompanionMatchesProcess(status, process) ||
		mappingPID != process.PID || sequence <= last || session == "" {
		return RuntimeQOLSessionEvent{}, false
	}
	return RuntimeQOLSessionEvent{SessionID: session, Sequence: sequence}, true
}

func (a *App) startRuntimeQOLSessionWatcher(initialSequence uint64) {
	if a.ctx == nil {
		return
	}
	a.qolSessionWatcherMu.Lock()
	defer a.qolSessionWatcherMu.Unlock()
	if previous := a.qolSessionWatcher; previous != nil {
		previous.cancel()
		<-previous.done
	}
	ctx, cancel := context.WithCancel(context.Background())
	watcher := &runtimeQOLSessionWatcher{cancel: cancel, done: make(chan struct{})}
	a.qolSessionWatcher = watcher
	go func() {
		defer close(watcher.done)
		ticker := time.NewTicker(runtimeQOLSessionPollInterval)
		defer ticker.Stop()
		lastSequence := initialSequence
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				status := readRuntimeCompanionStatus("qol")
				process, err := findRuntimeProcessInstance()
				if err != nil || !runtimeCompanionMatchesProcess(status, process) || !strings.EqualFold(status.State, "active") {
					return
				}
				mappingPID, sequence, session, err := readRuntimeQOLMapping()
				if err != nil {
					continue
				}
				event, ok := runtimeQOLSessionEventAfter(lastSequence, status, process, mappingPID, sequence, session)
				if !ok {
					continue
				}
				lastSequence = event.Sequence
				wailsruntime.EventsEmit(a.ctx, runtimeQOLSessionEventName, event)
			}
		}
	}()
}

func (a *App) stopRuntimeQOLSessionWatcher() {
	a.qolSessionWatcherMu.Lock()
	defer a.qolSessionWatcherMu.Unlock()
	watcher := a.qolSessionWatcher
	a.qolSessionWatcher = nil
	if watcher != nil {
		watcher.cancel()
		<-watcher.done
	}
}

func (a *App) startRuntimeQOLSessionWatcherForCurrent() {
	config := readRuntimeQOLConfig()
	if a.ctx == nil || !config.SessionCapture || !a.runtimeCompanionActive("qol") {
		return
	}
	_, sequence, _, err := readRuntimeQOLMapping()
	if err == nil {
		a.startRuntimeQOLSessionWatcher(sequence)
	}
}

func defaultRuntimeQOLConfig() RuntimeQOLConfig {
	return RuntimeQOLConfig{DamageCapPercentage: true, DetailedEnemyHP: true, DetailedSBA: true, SessionCapture: true, EnemyHPPrecision: 2, SBAPrecision: 2}
}

func normalizeRuntimeQOLConfig(value RuntimeQOLConfig) (RuntimeQOLConfig, error) {
	if value.NormalQuestLevelSync {
		return RuntimeQOLConfig{}, errors.New("普通任务等级同步尚缺任务类型白名单实测，当前构建不允许启用")
	}
	if value.ReturnWrightstone {
		return RuntimeQOLConfig{}, errors.New("重镶返还尚缺完整交易提交与背包增量实测，当前构建不允许启用")
	}
	if value.EnemyHPPrecision < 0 || value.EnemyHPPrecision > 4 {
		return RuntimeQOLConfig{}, errors.New("敌人 HP 小数位必须在 0 到 4 之间")
	}
	if value.SBAPrecision < 0 || value.SBAPrecision > 4 {
		return RuntimeQOLConfig{}, errors.New("奥义槽小数位必须在 0 到 4 之间")
	}
	if !value.DamageCapPercentage && !value.DetailedEnemyHP && !value.DetailedSBA && !value.SessionCapture && !value.NormalQuestLevelSync && !value.ReturnWrightstone && !value.FreeCaptain {
		return RuntimeQOLConfig{}, errors.New("至少选择一项便利功能")
	}
	return value, nil
}

func runtimeQOLConfigPath() (string, error) { return runtimeCompanionPath("qol.ini") }

func readRuntimeQOLConfig() RuntimeQOLConfig {
	value := defaultRuntimeQOLConfig()
	path, err := runtimeQOLConfigPath()
	if err != nil {
		return value
	}
	section := readRuntimeINI(path)["qol"]
	if section == nil {
		return value
	}
	parseFlag := func(key string, fallback bool) bool {
		if section[key] == "1" {
			return true
		}
		if section[key] == "0" {
			return false
		}
		return fallback
	}
	value.DamageCapPercentage = parseFlag("damageCapPercentage", value.DamageCapPercentage)
	value.DetailedEnemyHP = parseFlag("detailedEnemyHp", value.DetailedEnemyHP)
	value.DetailedSBA = parseFlag("detailedSba", value.DetailedSBA)
	value.SessionCapture = parseFlag("sessionCapture", value.SessionCapture)
	value.NormalQuestLevelSync = parseFlag("normalQuestLevelSync", value.NormalQuestLevelSync)
	value.ReturnWrightstone = parseFlag("returnWrightstone", value.ReturnWrightstone)
	value.NormalQuestLevelSync = false
	value.ReturnWrightstone = false
	value.FreeCaptain = parseFlag("freeCaptain", value.FreeCaptain)
	_, _ = fmt.Sscanf(section["enemyHpPrecision"], "%d", &value.EnemyHPPrecision)
	_, _ = fmt.Sscanf(section["sbaPrecision"], "%d", &value.SBAPrecision)
	if normalized, err := normalizeRuntimeQOLConfig(value); err == nil {
		return normalized
	}
	return defaultRuntimeQOLConfig()
}

func writeRuntimeQOLConfig(value RuntimeQOLConfig, enabled bool) error {
	path, err := runtimeQOLConfigPath()
	if err != nil {
		return err
	}
	flag := func(value bool) int {
		if value {
			return 1
		}
		return 0
	}
	data := fmt.Sprintf("[qol]\r\nenabled=%d\r\ndamageCapPercentage=%d\r\ndetailedEnemyHp=%d\r\ndetailedSba=%d\r\nsessionCapture=%d\r\nnormalQuestLevelSync=%d\r\nreturnWrightstone=%d\r\nfreeCaptain=%d\r\nenemyHpPrecision=%d\r\nsbaPrecision=%d\r\n",
		flag(enabled), flag(value.DamageCapPercentage), flag(value.DetailedEnemyHP), flag(value.DetailedSBA), flag(value.SessionCapture), flag(value.NormalQuestLevelSync), flag(value.ReturnWrightstone), flag(value.FreeCaptain), value.EnemyHPPrecision, value.SBAPrecision)
	return writeRuntimeCompanionFile(path, []byte(data))
}

func decodeRuntimeQOLMapping(data []byte) (uint32, uint64, string, error) {
	if len(data) < runtimeQOLMappingSize {
		return 0, 0, "", errors.New("便利运行时共享内存不完整")
	}
	if binary.LittleEndian.Uint64(data[0:8]) != runtimeQOLMagic || binary.LittleEndian.Uint32(data[8:12]) != runtimeQOLVersion {
		return 0, 0, "", errors.New("便利运行时共享内存版本不兼容")
	}
	pid := binary.LittleEndian.Uint32(data[12:16])
	if binary.LittleEndian.Uint64(data[16:24])&1 != 0 {
		return 0, 0, "", errors.New("便利运行时共享状态正在更新")
	}
	sequence := binary.LittleEndian.Uint64(data[56:64])
	session := strings.TrimSpace(strings.TrimRight(string(data[24:56]), "\x00"))
	return pid, sequence, session, nil
}

func readRuntimeQOLMapping() (uint32, uint64, string, error) {
	name, err := windows.UTF16PtrFromString(runtimeQOLMappingName)
	if err != nil {
		return 0, 0, "", err
	}
	handleRaw, _, callErr := kernel32OpenFileMappingW.Call(uintptr(windows.FILE_MAP_READ), 0, uintptr(unsafe.Pointer(name)))
	if handleRaw == 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return 0, 0, "", callErr
		}
		return 0, 0, "", errors.New("便利运行时尚未建立共享状态")
	}
	handle := windows.Handle(handleRaw)
	defer windows.CloseHandle(handle)
	address, err := windows.MapViewOfFile(handle, windows.FILE_MAP_READ, 0, 0, runtimeQOLMappingSize)
	if err != nil {
		return 0, 0, "", err
	}
	defer windows.UnmapViewOfFile(address)
	view := unsafe.Slice((*byte)(unsafe.Pointer(address)), runtimeQOLMappingSize)
	for attempt := 0; attempt < 8; attempt++ {
		before := atomic.LoadUint64((*uint64)(unsafe.Pointer(address + 16)))
		if before&1 != 0 {
			runtime.Gosched()
			continue
		}
		data := append([]byte(nil), view...)
		after := atomic.LoadUint64((*uint64)(unsafe.Pointer(address + 16)))
		if before == after && after&1 == 0 {
			binary.LittleEndian.PutUint64(data[16:24], after)
			return decodeRuntimeQOLMapping(data)
		}
		runtime.Gosched()
	}
	return 0, 0, "", errors.New("便利运行时共享状态更新频繁，请重试")
}

func (a *App) GetRuntimeQOLWorkspace() (*RuntimeQOLWorkspace, error) {
	runtimeQOLMu.Lock()
	defer runtimeQOLMu.Unlock()
	status := readRuntimeCompanionStatus("qol")
	active := a.runtimeCompanionActive("qol")
	process, processErr := findRuntimeProcessInstance()
	installed := processErr == nil && runtimeCompanionInstalled(status, process)
	owned := processErr == nil && a.runtimeCompanionOwned("qol", process)
	recoveryRequired := processErr == nil && runtimeCompanionRecoveryRequired(status, process)
	workspace := &RuntimeQOLWorkspace{Active: active, Installed: installed, Owned: owned, RecoveryRequired: recoveryRequired, State: status.State, GameRunning: processErr == nil, PID: status.PID, Config: readRuntimeQOLConfig(), Detail: status.Detail}
	if active {
		pid, sequence, session, err := readRuntimeQOLMapping()
		if err != nil {
			return nil, err
		}
		if pid != status.PID {
			return nil, errors.New("便利运行时共享状态属于另一游戏进程")
		}
		workspace.SessionSequence, workspace.LatestSessionID = sequence, session
	}
	return workspace, nil
}

func (a *App) DeployRuntimeQOL(request RuntimeQOLConfig) (*RuntimeQOLWorkspace, error) {
	runtimeQOLMu.Lock()
	defer runtimeQOLMu.Unlock()
	a.stopRuntimeQOLSessionWatcher()
	watcherStarted := false
	defer func() {
		if !watcherStarted {
			a.startRuntimeQOLSessionWatcherForCurrent()
		}
	}()
	config, err := normalizeRuntimeQOLConfig(request)
	if err != nil {
		return nil, err
	}
	if a.runtimeCompanionActive("qol") {
		if err := removeRuntimeQOLLocked(a, readRuntimeQOLConfig()); err != nil {
			return nil, err
		}
	}
	if err := writeRuntimeQOLConfig(config, true); err != nil {
		return nil, err
	}
	if err := a.startRuntimeCompanion("qol", "runtime_qol"); err != nil {
		_ = writeRuntimeQOLConfig(config, false)
		return nil, err
	}
	status := readRuntimeCompanionStatus("qol")
	pid, sequence, session, err := readRuntimeQOLMapping()
	if err != nil {
		if rollbackErr := removeRuntimeQOLLocked(a, config); rollbackErr != nil {
			return nil, fmt.Errorf("读取便利运行时状态失败：%w；停用回滚也失败：%v", err, rollbackErr)
		}
		return nil, fmt.Errorf("读取便利运行时状态失败，已自动停用并恢复：%w", err)
	}
	workspace := &RuntimeQOLWorkspace{Active: true, GameRunning: true, PID: pid, Config: config, LatestSessionID: session, SessionSequence: sequence, Detail: status.Detail}
	if config.SessionCapture {
		a.startRuntimeQOLSessionWatcher(sequence)
	}
	watcherStarted = true
	return workspace, nil
}

func removeRuntimeQOLLocked(a *App, config RuntimeQOLConfig) error {
	return a.stopOwnedRuntimeCompanion("qol", func() error { return writeRuntimeQOLConfig(config, false) })
}

func (a *App) RemoveRuntimeQOL(_ string) error {
	runtimeQOLMu.Lock()
	defer runtimeQOLMu.Unlock()
	a.stopRuntimeQOLSessionWatcher()
	err := removeRuntimeQOLLocked(a, readRuntimeQOLConfig())
	if err != nil {
		a.startRuntimeQOLSessionWatcherForCurrent()
	}
	return err
}
