package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unsafe"
)

const (
	combatTuningHookSize                  = 5
	stableReleaseCombatTuningWriteEnabled = true
)

var (
	combatCooldownMarker              = []byte("GBFRCD01")
	combatChargeMarker                = []byte("GBFRCH01")
	combatActionSpeedMarker           = []byte("GBFRAS01")
	combatTuningInstallRemoteCodeHook = installRemoteCodeHook
	combatCooldownSpecs               = []combatTuningSiteSpec{
		{
			Label:    "冷却路径 A",
			Original: []byte{0xC5, 0xFA, 0x10, 0x4E, 0x1C},
			Pattern:  []byte{0xC5, 0xFA, 0x10, 0x4E, 0x1C, 0x74, 0, 0xC5, 0xFA, 0x10, 0x46, 0x18, 0xC5, 0xF8, 0x2E, 0xC8, 0x76},
			Mask:     []bool{true, true, true, true, true, true, false, true, true, true, true, true, true, true, true, true, true},
		},
		{
			Label:    "冷却路径 B",
			Original: []byte{0xC5, 0xFA, 0x11, 0x41, 0x1C},
			Pattern:  []byte{0xC5, 0xFA, 0x11, 0x41, 0x1C, 0x48, 0x8D, 0x4D, 0, 0xE8, 0, 0, 0, 0, 0x48, 0x8D, 0x4E, 0xE8, 0xC6},
			Mask:     []bool{true, true, true, true, true, true, true, true, false, true, false, false, false, false, true, true, true, true, true},
		},
		{
			Label:    "冷却路径 C",
			Original: []byte{0xC5, 0xF9, 0x7E, 0x41, 0x1C},
			Pattern:  []byte{0xC5, 0xF9, 0x7E, 0x41, 0x1C, 0x48, 0x8B, 0x86, 0x80, 0x0E, 0, 0, 0x48, 0x81, 0xC6, 0x80, 0x0E, 0, 0, 0x48, 0x89, 0xF1, 0xFF, 0x90, 0xB8, 0, 0, 0, 0x85},
			Mask:     []bool{true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true},
		},
	}
	combatChargeSpec = combatTuningSiteSpec{
		Label:    "三角色蓄力路径",
		Original: []byte{0xC5, 0xFA, 0x10, 0x4E, 0x24},
		Pattern: []byte{
			0xC5, 0xFA, 0x10, 0x4E, 0x24, 0xC5, 0xE8, 0x57, 0xD2, 0xC5, 0xF8, 0x2E, 0xD1, 0x77, 0,
			0xC5, 0xFA, 0x59, 0x05, 0, 0, 0, 0, 0xC5, 0xF2, 0x58, 0xC0, 0xC5, 0xF8, 0x2E, 0xD0, 0x76, 0x04,
			0xC5, 0xF8, 0x57, 0xC0, 0xC5, 0xFA, 0x11, 0x46, 0x24, 0xC5, 0xFA, 0x10, 0x56, 0x20,
		},
		Mask: []bool{
			true, true, true, true, true, true, true, true, true, true, true, true, true, true, false,
			true, true, true, true, false, false, false, false, true, true, true, true, true, true, true, true, true, true,
			true, true, true, true, true, true, true, true, true, true, true, true, true, true,
		},
	}
	combatActionSpeedSpec = combatTuningSiteSpec{
		Label:    "人物动作变速路径",
		Original: []byte{0xC5, 0xFA, 0x10, 0x40, 0x18},
		Pattern: []byte{
			0xC5, 0xFA, 0x10, 0x40, 0x18, 0xEB, 0,
			0xC5, 0, 0, 0, 0, 0, 0, 0,
			0xC5, 0, 0, 0, 0, 0x75,
		},
		Mask: []bool{
			true, true, true, true, true, true, false,
			true, false, false, false, false, false, false, false,
			true, false, false, false, false, true,
		},
	}
)

type CombatCooldownRequest struct {
	Enabled         bool    `json:"enabled"`
	NoCooldown      bool    `json:"noCooldown"`
	SpeedMultiplier float64 `json:"speedMultiplier"`
	ApplyWholeParty bool    `json:"applyWholeParty"`
}

type CombatChargeRequest struct {
	Enabled         bool    `json:"enabled"`
	Instant         bool    `json:"instant"`
	SpeedMultiplier float64 `json:"speedMultiplier"`
}

type CombatActionSpeedRequest struct {
	Enabled         bool    `json:"enabled"`
	SpeedMultiplier float64 `json:"speedMultiplier"`
	ApplyWholeParty bool    `json:"applyWholeParty"`
}

type CombatTuningFeatureStatus struct {
	Available       bool     `json:"available"`
	Enabled         bool     `json:"enabled"`
	Candidate       bool     `json:"candidate"`
	Instant         bool     `json:"instant,omitempty"`
	NoCooldown      bool     `json:"noCooldown,omitempty"`
	ApplyWholeParty bool     `json:"applyWholeParty,omitempty"`
	SpeedMultiplier float64  `json:"speedMultiplier"`
	RVAs            []uint64 `json:"rvas"`
	CurrentBytes    []string `json:"currentBytes"`
	EvidenceNote    string   `json:"evidenceNote"`
	Error           string   `json:"error,omitempty"`
}

type CombatTuningStatus struct {
	Cooldown    CombatTuningFeatureStatus `json:"cooldown"`
	Charge      CombatTuningFeatureStatus `json:"charge"`
	ActionSpeed CombatTuningFeatureStatus `json:"actionSpeed"`
}

type combatTuningSiteSpec struct {
	Label    string
	Original []byte
	Pattern  []byte
	Mask     []bool
}

type combatTuningSiteLease struct {
	Label     string
	EntryAddr uintptr
	Original  []byte
	Installed []byte
	CaveAddr  uintptr
	CaveCode  []byte
}

type combatTuningKind string

const (
	combatTuningKindCooldown    combatTuningKind = "cooldown"
	combatTuningKindCharge      combatTuningKind = "charge"
	combatTuningKindActionSpeed combatTuningKind = "action-speed"
)

type combatTuningLease struct {
	OwnerToken  string
	Process     processInstanceID
	Kind        combatTuningKind
	Cooldown    CombatCooldownRequest
	Charge      CombatChargeRequest
	ActionSpeed CombatActionSpeedRequest
	Sites       []combatTuningSiteLease
}

func (lease *combatTuningLease) active() bool {
	return lease != nil && len(lease.Sites) > 0
}

func (a *App) CombatTuningGetStatusOwned(token string) (CombatTuningStatus, error) {
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return CombatTuningStatus{}, err
	}
	defer a.procMu.Unlock()
	if err := a.verifyRuntimePatchExecutableLocked(a.currentProcessInstance(), "战斗参数调整"); err != nil {
		return CombatTuningStatus{}, err
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	return a.readCombatTuningStatusLocked(token)
}

func (a *App) CombatTuningSetCooldownOwned(token string, request CombatCooldownRequest) (CombatTuningStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return CombatTuningStatus{}, err
	}
	defer a.procMu.Unlock()
	if err := a.ensureLiveMemoryWritesSafe(); err != nil {
		return CombatTuningStatus{}, err
	}
	if err := a.verifyRuntimePatchExecutableLocked(a.currentProcessInstance(), "能力冷却调整"); err != nil {
		return CombatTuningStatus{}, err
	}
	if request.Enabled && !stableReleaseCombatTuningWriteEnabled {
		return CombatTuningStatus{}, errors.New("能力冷却调整在稳定版中保持禁用：仍缺少本机/全队范围与实际倍率的任务验收")
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	if err := validateCombatCooldownRequest(request); err != nil {
		return CombatTuningStatus{}, err
	}
	if a.combatTuningCooldownLease != nil {
		if err := a.restoreOneCombatTuningLeaseLocked(a.combatTuningCooldownLease, token, false); err != nil {
			return CombatTuningStatus{}, err
		}
		a.combatTuningCooldownLease = nil
	}
	if request.Enabled {
		lease, err := a.installCombatCooldownLocked(token, request)
		if err != nil {
			return CombatTuningStatus{}, err
		}
		a.combatTuningCooldownLease = lease
	}
	return a.readCombatTuningStatusLocked(token)
}

func (a *App) CombatTuningSetChargeOwned(token string, request CombatChargeRequest) (CombatTuningStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return CombatTuningStatus{}, err
	}
	defer a.procMu.Unlock()
	if err := a.ensureLiveMemoryWritesSafe(); err != nil {
		return CombatTuningStatus{}, err
	}
	if err := a.verifyRuntimePatchExecutableLocked(a.currentProcessInstance(), "三角色共享蓄力调整"); err != nil {
		return CombatTuningStatus{}, err
	}
	if request.Enabled && !stableReleaseCombatTuningWriteEnabled {
		return CombatTuningStatus{}, errors.New("三角色共享蓄力调整在稳定版中保持禁用：仍缺少三名角色的可见计时样本")
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	if err := validateCombatChargeRequest(request); err != nil {
		return CombatTuningStatus{}, err
	}
	if request.Enabled {
		if err := validateCombatChargeCatalogConflict(a.runtimePatchPatchLeases); err != nil {
			return CombatTuningStatus{}, err
		}
	}
	if a.combatTuningChargeLease != nil {
		if err := a.restoreOneCombatTuningLeaseLocked(a.combatTuningChargeLease, token, false); err != nil {
			return CombatTuningStatus{}, err
		}
		a.combatTuningChargeLease = nil
	}
	if request.Enabled {
		lease, err := a.installCombatChargeLocked(token, request)
		if err != nil {
			return CombatTuningStatus{}, err
		}
		a.combatTuningChargeLease = lease
	}
	return a.readCombatTuningStatusLocked(token)
}

// CombatTuningSetActionSpeedOwned installs or restores the 2.0.3+
// per-character action-speed hook. It changes the actor action-time field and
// never touches the process-wide game timescale.
func (a *App) CombatTuningSetActionSpeedOwned(token string, request CombatActionSpeedRequest) (CombatTuningStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return CombatTuningStatus{}, err
	}
	defer a.procMu.Unlock()
	if err := a.ensureLiveMemoryWritesSafe(); err != nil {
		return CombatTuningStatus{}, err
	}
	if err := a.verifyRuntimePatchExecutableLocked(a.currentProcessInstance(), "人物动作变速"); err != nil {
		return CombatTuningStatus{}, err
	}
	if request.Enabled && !isGame203PlusExecutableDigest(a.runtimePatchVerifiedDigest) {
		return CombatTuningStatus{}, errors.New("人物动作变速仅支持已验证的游戏 2.0.3 / 2.0.4 / 2.0.5 可执行文件")
	}
	if request.Enabled && !stableReleaseCombatTuningWriteEnabled {
		return CombatTuningStatus{}, errors.New("人物动作变速在稳定版中保持禁用：仍缺少本机/全队范围的任务验收")
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	if err := validateCombatActionSpeedRequest(request); err != nil {
		return CombatTuningStatus{}, err
	}
	if a.combatTuningActionSpeedLease != nil {
		if err := a.restoreOneCombatTuningLeaseLocked(a.combatTuningActionSpeedLease, token, false); err != nil {
			return CombatTuningStatus{}, err
		}
		a.combatTuningActionSpeedLease = nil
	}
	if request.Enabled {
		lease, err := a.installCombatActionSpeedLocked(token, request)
		if err != nil {
			return CombatTuningStatus{}, err
		}
		a.combatTuningActionSpeedLease = lease
	}
	return a.readCombatTuningStatusLocked(token)
}

func validateCombatChargeCatalogConflict(leases map[string]runtimePatchPatchLease) error {
	if _, active := leases["runtime-patch-017"]; active {
		return fmt.Errorf("三角色共享蓄力调整与冈达葛萨“瞬间直冲拳”不能同时开启；请先恢复瞬间直冲拳")
	}
	return nil
}

func validateCombatCooldownRequest(request CombatCooldownRequest) error {
	if !request.Enabled {
		return nil
	}
	if request.NoCooldown {
		return nil
	}
	if math.IsNaN(request.SpeedMultiplier) || math.IsInf(request.SpeedMultiplier, 0) ||
		request.SpeedMultiplier < 0.1 || request.SpeedMultiplier > 100 {
		return fmt.Errorf("冷却速度倍率请输入 0.1 到 100")
	}
	return nil
}

func validateCombatChargeRequest(request CombatChargeRequest) error {
	if !request.Enabled || request.Instant {
		return nil
	}
	if math.IsNaN(request.SpeedMultiplier) || math.IsInf(request.SpeedMultiplier, 0) ||
		request.SpeedMultiplier < 0.1 || request.SpeedMultiplier > 100 {
		return fmt.Errorf("蓄力速度倍率请输入 0.1 到 100")
	}
	return nil
}

func validateCombatActionSpeedRequest(request CombatActionSpeedRequest) error {
	if !request.Enabled {
		return nil
	}
	if math.IsNaN(request.SpeedMultiplier) || math.IsInf(request.SpeedMultiplier, 0) ||
		request.SpeedMultiplier < 0.1 || request.SpeedMultiplier > 5 {
		return fmt.Errorf("人物动作速度倍率请输入 0.1 到 5.0")
	}
	return nil
}

func (a *App) installCombatCooldownLocked(ownerToken string, request CombatCooldownRequest) (*combatTuningLease, error) {
	addrs, err := a.locateCombatCooldownLocked()
	if err != nil {
		return nil, err
	}
	lease := &combatTuningLease{
		OwnerToken: ownerToken,
		Process:    a.currentProcessInstance(),
		Kind:       combatTuningKindCooldown,
		Cooldown:   request,
	}
	for index, spec := range combatCooldownSpecs {
		site, prepareErr := a.prepareCombatTuningSite(addrs[index], spec, func(cave uintptr) ([]byte, error) {
			return buildCombatCooldownCave(index, cave, addrs[index]+combatTuningHookSize, request)
		})
		if prepareErr != nil {
			for _, prepared := range lease.Sites {
				_ = virtualFreeRemote(a.hProcess, prepared.CaveAddr)
			}
			return nil, prepareErr
		}
		lease.Sites = append(lease.Sites, site)
	}
	a.combatTuningCooldownLease = lease
	if err := a.publishCombatTuningLeaseLocked(lease); err != nil {
		return nil, err
	}
	return lease, nil
}

func (a *App) installCombatChargeLocked(ownerToken string, request CombatChargeRequest) (*combatTuningLease, error) {
	addr, err := a.locateCombatChargeLocked()
	if err != nil {
		return nil, err
	}
	lease := &combatTuningLease{
		OwnerToken: ownerToken,
		Process:    a.currentProcessInstance(),
		Kind:       combatTuningKindCharge,
		Charge:     request,
	}
	site, err := a.prepareCombatTuningSite(addr, combatChargeSpec, func(cave uintptr) ([]byte, error) {
		return buildCombatChargeCave(cave, addr+combatTuningHookSize, request)
	})
	if err != nil {
		return nil, err
	}
	lease.Sites = append(lease.Sites, site)
	a.combatTuningChargeLease = lease
	if err := a.publishCombatTuningLeaseLocked(lease); err != nil {
		return nil, err
	}
	return lease, nil
}

func (a *App) installCombatActionSpeedLocked(ownerToken string, request CombatActionSpeedRequest) (*combatTuningLease, error) {
	addr, err := a.locateCombatActionSpeedLocked()
	if err != nil {
		return nil, err
	}
	lease := &combatTuningLease{
		OwnerToken:  ownerToken,
		Process:     a.currentProcessInstance(),
		Kind:        combatTuningKindActionSpeed,
		ActionSpeed: request,
	}
	site, err := a.prepareCombatTuningSite(addr, combatActionSpeedSpec, func(cave uintptr) ([]byte, error) {
		return buildCombatActionSpeedCave(cave, addr+combatTuningHookSize, request)
	})
	if err != nil {
		return nil, err
	}
	lease.Sites = append(lease.Sites, site)
	a.combatTuningActionSpeedLease = lease
	if err := a.publishCombatTuningLeaseLocked(lease); err != nil {
		return nil, err
	}
	return lease, nil
}

func (a *App) prepareCombatTuningSite(addr uintptr, spec combatTuningSiteSpec, build func(uintptr) ([]byte, error)) (combatTuningSiteLease, error) {
	current, err := readCombatTuningBytes(a, addr, len(spec.Original))
	if err != nil {
		return combatTuningSiteLease{}, err
	}
	if !bytes.Equal(current, spec.Original) {
		return combatTuningSiteLease{}, fmt.Errorf("%s原始指令已变化: %s", spec.Label, bytesToHex(current))
	}
	cave, err := virtualAllocRemoteNear(a.hProcess, addr, 0x1000)
	if err != nil {
		return combatTuningSiteLease{}, fmt.Errorf("%s分配代码洞失败: %w", spec.Label, err)
	}
	code, err := build(cave)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return combatTuningSiteLease{}, err
	}
	if err := writeCodeMemory(a.hProcess, cave, code); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return combatTuningSiteLease{}, fmt.Errorf("%s写入代码洞失败: %w", spec.Label, err)
	}
	readback, err := readCombatTuningBytes(a, cave, len(code))
	if err != nil || !bytes.Equal(readback, code) {
		_ = virtualFreeRemote(a.hProcess, cave)
		if err != nil {
			return combatTuningSiteLease{}, fmt.Errorf("%s代码洞回读失败: %w", spec.Label, err)
		}
		return combatTuningSiteLease{}, fmt.Errorf("%s代码洞回读不一致", spec.Label)
	}
	patch, err := makeRelJump(addr, cave, combatTuningHookSize)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return combatTuningSiteLease{}, err
	}
	return combatTuningSiteLease{
		Label:     spec.Label,
		EntryAddr: addr,
		Original:  append([]byte(nil), spec.Original...),
		Installed: patch,
		CaveAddr:  cave,
		CaveCode:  code,
	}, nil
}

func (a *App) publishCombatTuningLeaseLocked(lease *combatTuningLease) error {
	for index := range lease.Sites {
		site := &lease.Sites[index]
		result, err := combatTuningInstallRemoteCodeHook(a.hProcess, site.EntryAddr, site.Original, site.Installed)
		if err == nil {
			continue
		}
		restoreErr := a.restoreOneCombatTuningLeaseLocked(lease, lease.OwnerToken, false)
		if restoreErr == nil {
			a.clearCombatTuningLease(lease)
			return fmt.Errorf("%s安装失败，全部入口已恢复: %w", site.Label, err)
		}
		a.poisonCurrentLiveMemoryWrites()
		if result.RequiresRecoveryLease() {
			return errors.Join(fmt.Errorf("%s安装失败: %w", site.Label, err), errRuntimeHookRollbackUnproven, restoreErr)
		}
		return errors.Join(fmt.Errorf("%s安装失败: %w", site.Label, err), restoreErr)
	}
	return nil
}

func (a *App) clearCombatTuningLease(lease *combatTuningLease) {
	if a.combatTuningCooldownLease == lease {
		a.combatTuningCooldownLease = nil
	}
	if a.combatTuningChargeLease == lease {
		a.combatTuningChargeLease = nil
	}
	if a.combatTuningActionSpeedLease == lease {
		a.combatTuningActionSpeedLease = nil
	}
}

func (a *App) readCombatTuningStatusLocked(ownerToken string) (CombatTuningStatus, error) {
	cooldown, err := a.readCombatTuningFeatureLocked(ownerToken, "cooldown")
	if err != nil {
		return CombatTuningStatus{}, err
	}
	charge, err := a.readCombatTuningFeatureLocked(ownerToken, "charge")
	if err != nil {
		return CombatTuningStatus{}, err
	}
	actionSpeed, err := a.readCombatTuningFeatureLocked(ownerToken, "action-speed")
	if err != nil {
		return CombatTuningStatus{}, err
	}
	return CombatTuningStatus{Cooldown: cooldown, Charge: charge, ActionSpeed: actionSpeed}, nil
}

func (a *App) readCombatTuningFeatureLocked(ownerToken string, kind combatTuningKind) (CombatTuningFeatureStatus, error) {
	status := CombatTuningFeatureStatus{
		Candidate:       true,
		SpeedMultiplier: 2,
	}
	var lease *combatTuningLease
	var specs []combatTuningSiteSpec
	var addrs []uintptr
	var err error
	switch kind {
	case combatTuningKindCooldown:
		lease = a.combatTuningCooldownLease
		specs = combatCooldownSpecs
		status.EvidenceNote = "三个 2.0.2 EXE 入口与恢复路径已核对；本机/全队识别和实际冷却倍率仍待任务实测。"
		if lease == nil {
			addrs, err = a.locateCombatCooldownLocked()
		}
	case combatTuningKindCharge:
		lease = a.combatTuningChargeLease
		specs = []combatTuningSiteSpec{combatChargeSpec}
		status.EvidenceNote = "2.0.2 EXE 共享蓄力入口与恢复路径已核对；仅作为伊欧、巴萨拉卡、冈达葛萨候选，实际角色范围待实测。"
		if lease == nil {
			if _, active := a.runtimePatchPatchLeases["runtime-patch-017"]; active {
				status.Error = "冈达葛萨“瞬间直冲拳”正在使用另一条蓄力路径；恢复该功能后才能启用共享蓄力调整。"
				return status, nil
			}
		}
		if lease == nil {
			var addr uintptr
			addr, err = a.locateCombatChargeLocked()
			addrs = []uintptr{addr}
		}
	case combatTuningKindActionSpeed:
		lease = a.combatTuningActionSpeedLease
		specs = []combatTuningSiteSpec{combatActionSpeedSpec}
		status.SpeedMultiplier = 1.5
		status.EvidenceNote = "2.0.3 / 2.0.4 / 2.0.5 EXE 人物动作字段入口、唯一签名与恢复路径已核对；本机/全队作用范围和实际战斗节奏仍待任务实测。"
		if lease == nil && a.runtimePatchVerifiedDigest != "" &&
			!isGame203PlusExecutableDigest(a.runtimePatchVerifiedDigest) {
			status.Error = "人物动作变速仅支持已验证的游戏 2.0.3 / 2.0.4 / 2.0.5 可执行文件"
			return status, nil
		}
		if lease == nil {
			var addr uintptr
			addr, err = a.locateCombatActionSpeedLocked()
			addrs = []uintptr{addr}
		}
	default:
		return CombatTuningFeatureStatus{}, fmt.Errorf("未知战斗参数类型: %q", kind)
	}
	if lease == nil && !stableReleaseCombatTuningWriteEnabled {
		status.Available = false
		status.Error = "稳定版未开放：该候选入口仍缺少可见游戏效果验收"
		return status, nil
	}
	if err != nil {
		status.Error = err.Error()
		return status, nil
	}
	if lease != nil {
		if !sameProcessInstance(lease.Process, a.currentProcessInstance()) {
			return CombatTuningFeatureStatus{}, fmt.Errorf("%s恢复租约属于另一个游戏进程实例", kind)
		}
		if ownerToken == "" || lease.OwnerToken != ownerToken {
			return CombatTuningFeatureStatus{}, errRuntimeOwnerLeaseStale
		}
		status.Enabled = true
		if kind == combatTuningKindCooldown {
			status.NoCooldown = lease.Cooldown.NoCooldown
			status.ApplyWholeParty = lease.Cooldown.ApplyWholeParty
			status.SpeedMultiplier = lease.Cooldown.SpeedMultiplier
		} else if kind == combatTuningKindCharge {
			status.Instant = lease.Charge.Instant
			status.SpeedMultiplier = lease.Charge.SpeedMultiplier
		} else {
			status.ApplyWholeParty = lease.ActionSpeed.ApplyWholeParty
			status.SpeedMultiplier = lease.ActionSpeed.SpeedMultiplier
		}
		for _, site := range lease.Sites {
			current, readErr := readCombatTuningBytes(a, site.EntryAddr, len(site.Installed))
			if readErr != nil {
				return CombatTuningFeatureStatus{}, readErr
			}
			if !bytes.Equal(current, site.Installed) || relJumpTarget(site.EntryAddr, current) != site.CaveAddr {
				return CombatTuningFeatureStatus{}, fmt.Errorf("%s入口不再属于本任务: %s", site.Label, bytesToHex(current))
			}
			cave, readErr := readCombatTuningBytes(a, site.CaveAddr, len(site.CaveCode))
			if readErr != nil || !bytes.Equal(cave, site.CaveCode) {
				if readErr != nil {
					return CombatTuningFeatureStatus{}, readErr
				}
				return CombatTuningFeatureStatus{}, fmt.Errorf("%s代码洞回读不一致", site.Label)
			}
			status.RVAs = append(status.RVAs, uint64(site.EntryAddr-a.moduleBase))
			status.CurrentBytes = append(status.CurrentBytes, bytesToHex(current))
		}
		status.Available = true
		return status, nil
	}
	for index, addr := range addrs {
		current, readErr := readCombatTuningBytes(a, addr, len(specs[index].Original))
		if readErr != nil {
			status.Error = readErr.Error()
			return status, nil
		}
		if !bytes.Equal(current, specs[index].Original) {
			status.Error = fmt.Sprintf("%s原始指令已变化: %s", specs[index].Label, bytesToHex(current))
			return status, nil
		}
		status.RVAs = append(status.RVAs, uint64(addr-a.moduleBase))
		status.CurrentBytes = append(status.CurrentBytes, bytesToHex(current))
	}
	status.Available = len(addrs) == len(specs)
	return status, nil
}

// restoreCombatTuningOwnedLocked runs with procMu and runtimePatchMu held.
func (a *App) restoreCombatTuningOwnedLocked(ownerToken string, force bool) error {
	var result error
	if lease := a.combatTuningActionSpeedLease; lease != nil {
		if err := a.restoreOneCombatTuningLeaseLocked(lease, ownerToken, force); err != nil {
			result = errors.Join(result, fmt.Errorf("action speed: %w", err))
		} else {
			a.combatTuningActionSpeedLease = nil
		}
	}
	if lease := a.combatTuningChargeLease; lease != nil {
		if err := a.restoreOneCombatTuningLeaseLocked(lease, ownerToken, force); err != nil {
			result = errors.Join(result, fmt.Errorf("charge: %w", err))
		} else {
			a.combatTuningChargeLease = nil
		}
	}
	if lease := a.combatTuningCooldownLease; lease != nil {
		if err := a.restoreOneCombatTuningLeaseLocked(lease, ownerToken, force); err != nil {
			result = errors.Join(result, fmt.Errorf("cooldown: %w", err))
		} else {
			a.combatTuningCooldownLease = nil
		}
	}
	return result
}

func (a *App) restoreOneCombatTuningLeaseLocked(lease *combatTuningLease, ownerToken string, force bool) error {
	if lease == nil || !lease.active() {
		return nil
	}
	if !sameProcessInstance(lease.Process, a.currentProcessInstance()) {
		return fmt.Errorf("恢复租约属于另一个游戏进程实例")
	}
	if !force && (ownerToken == "" || lease.OwnerToken != ownerToken) {
		return errRuntimeOwnerLeaseStale
	}
	var joined error
	failedReverse := make([]combatTuningSiteLease, 0, len(lease.Sites))
	for index := len(lease.Sites) - 1; index >= 0; index-- {
		site := &lease.Sites[index]
		current, err := readCombatTuningBytes(a, site.EntryAddr, len(site.Original))
		if err != nil {
			joined = errors.Join(joined, fmt.Errorf("%s入口读取失败: %w", site.Label, err))
			failedReverse = append(failedReverse, *site)
			continue
		}
		if !bytes.Equal(current, site.Original) {
			if !bytes.Equal(current, site.Installed) || relJumpTarget(site.EntryAddr, current) != site.CaveAddr {
				joined = errors.Join(joined, fmt.Errorf("%s入口既不是自有跳转也不是原始指令: %s", site.Label, bytesToHex(current)))
				failedReverse = append(failedReverse, *site)
				continue
			}
		}
		cave, err := readCombatTuningBytes(a, site.CaveAddr, len(site.CaveCode))
		if err != nil || !bytes.Equal(cave, site.CaveCode) {
			if err != nil {
				joined = errors.Join(joined, fmt.Errorf("%s代码洞读取失败: %w", site.Label, err))
			} else {
				joined = errors.Join(joined, fmt.Errorf("%s代码洞所有权校验失败", site.Label))
			}
			failedReverse = append(failedReverse, *site)
			continue
		}
		if !bytes.Equal(current, site.Original) {
			if err := writeAndVerifyRuntimeHookEntry(a.hProcess, site.EntryAddr, site.Original); err != nil {
				joined = errors.Join(joined, fmt.Errorf("%s恢复失败: %w", site.Label, err))
				failedReverse = append(failedReverse, *site)
				continue
			}
		}
		a.retireRuntimeCaveLocked(site.CaveAddr, "combat-tuning "+string(lease.Kind))
	}
	lease.Sites = lease.Sites[:0]
	for index := len(failedReverse) - 1; index >= 0; index-- {
		lease.Sites = append(lease.Sites, failedReverse[index])
	}
	return joined
}

func (a *App) dropCombatTuningOwnerLocked(ownerToken string, force bool) {
	if lease := a.combatTuningCooldownLease; lease != nil && (force || ownerToken != "" && lease.OwnerToken == ownerToken) {
		a.combatTuningCooldownLease = nil
		a.combatTuningCooldownAddrs = nil
	}
	if lease := a.combatTuningChargeLease; lease != nil && (force || ownerToken != "" && lease.OwnerToken == ownerToken) {
		a.combatTuningChargeLease = nil
		a.combatTuningChargeAddr = 0
	}
	if lease := a.combatTuningActionSpeedLease; lease != nil && (force || ownerToken != "" && lease.OwnerToken == ownerToken) {
		a.combatTuningActionSpeedLease = nil
		a.combatTuningActionSpeedAddr = 0
	}
}

func (a *App) locateCombatCooldownLocked() ([]uintptr, error) {
	if len(a.combatTuningCooldownAddrs) == len(combatCooldownSpecs) {
		return append([]uintptr(nil), a.combatTuningCooldownAddrs...), nil
	}
	addrs := make([]uintptr, 0, len(combatCooldownSpecs))
	for _, spec := range combatCooldownSpecs {
		addr, err := a.scanPatternUnique(spec.Pattern, spec.Mask, spec.Label)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, addr)
	}
	a.combatTuningCooldownAddrs = append([]uintptr(nil), addrs...)
	return addrs, nil
}

func (a *App) locateCombatChargeLocked() (uintptr, error) {
	if a.combatTuningChargeAddr != 0 {
		return a.combatTuningChargeAddr, nil
	}
	addr, err := a.scanPatternUnique(combatChargeSpec.Pattern, combatChargeSpec.Mask, combatChargeSpec.Label)
	if err != nil {
		return 0, err
	}
	a.combatTuningChargeAddr = addr
	return addr, nil
}

func (a *App) locateCombatActionSpeedLocked() (uintptr, error) {
	if a.combatTuningActionSpeedAddr != 0 {
		return a.combatTuningActionSpeedAddr, nil
	}
	addr, err := a.scanPatternUnique(combatActionSpeedSpec.Pattern, combatActionSpeedSpec.Mask, combatActionSpeedSpec.Label)
	if err != nil {
		return 0, err
	}
	a.combatTuningActionSpeedAddr = addr
	return addr, nil
}

func readCombatTuningBytes(a *App, addr uintptr, size int) ([]byte, error) {
	if a == nil || addr == 0 || size <= 0 {
		return nil, fmt.Errorf("运行时调节读取参数无效")
	}
	data := make([]byte, size)
	if err := readProcessMemory(a.hProcess, addr, unsafe.Pointer(&data[0]), uintptr(size)); err != nil {
		return nil, err
	}
	return data, nil
}

func buildCombatCooldownCave(index int, cave, returnAddr uintptr, request CombatCooldownRequest) ([]byte, error) {
	if index < 0 || index >= len(combatCooldownSpecs) {
		return nil, fmt.Errorf("冷却代码洞索引无效")
	}
	code := make([]byte, 0, 80)
	shortJumps := make([]int, 0, 1)
	// Path A is the local actor's continuous cooldown path in the locked 2.0.2
	// executable. Keep its verified local-player discriminator even when the
	// optional party scope is selected; party scope expands only the two party
	// array write paths below. Removing this guard would also admit unresolved
	// non-party actors that share the same routine.
	if index == 0 {
		code = append(code, 0x53)                   // push rbx
		code = append(code, 0x48, 0x8B, 0x5E, 0x40) // mov rbx,[rsi+40]
		code = append(code, 0x48, 0x85, 0xDB)       // test rbx,rbx
		code = append(code, 0x74, 0)                // je original
		shortJumps = append(shortJumps, len(code)-1)
		code = append(code, 0x81, 0xBB, 0x10, 0x02, 0, 0, 1, 0, 0, 0) // cmp dword [rbx+210],1
		code = append(code, 0x75, 0)
		shortJumps = append(shortJumps, len(code)-1)
	} else if !request.ApplyWholeParty {
		if index == 1 {
			code = append(code, 0x81, 0xBF, 0x10, 0x02, 0, 0, 1, 0, 0, 0) // cmp dword [rdi+210],1
		} else {
			code = append(code, 0x81, 0xBE, 0x10, 0x02, 0, 0, 1, 0, 0, 0) // cmp dword [rsi+210],1
		}
		code = append(code, 0x75, 0)
		shortJumps = append(shortJumps, len(code)-1)
	}
	var factorDisp int
	if request.NoCooldown {
		if index == 0 {
			code = append(code, 0x0F, 0x57, 0xF6) // xorps xmm6,xmm6
		} else {
			code = append(code, 0x0F, 0x57, 0xC0) // xorps xmm0,xmm0
		}
	} else {
		if index == 0 {
			code = append(code, 0xF3, 0x0F, 0x5E, 0x35, 0, 0, 0, 0) // divss xmm6,[rip+factor]
		} else {
			code = append(code, 0xF3, 0x0F, 0x5E, 0x05, 0, 0, 0, 0) // divss xmm0,[rip+factor]
		}
		factorDisp = len(code) - 4
	}
	skipOffset := len(code)
	for _, displacement := range shortJumps {
		if err := patchCombatShortJump(code, displacement, skipOffset); err != nil {
			return nil, err
		}
	}
	if index == 0 {
		code = append(code, 0x5B) // pop rbx
	}
	code = append(code, combatCooldownSpecs[index].Original...)
	jump, err := makeRelJump(cave+uintptr(len(code)), returnAddr, 5)
	if err != nil {
		return nil, err
	}
	code = append(code, jump...)
	code = append(code, combatCooldownMarker...)
	if !request.NoCooldown {
		factorOffset := len(code)
		var raw [4]byte
		binary.LittleEndian.PutUint32(raw[:], math.Float32bits(float32(request.SpeedMultiplier)))
		code = append(code, raw[:]...)
		if err := patchCombatRIPRelative(code, factorDisp, cave, factorOffset); err != nil {
			return nil, err
		}
	}
	return code, nil
}

func buildCombatChargeCave(cave, returnAddr uintptr, request CombatChargeRequest) ([]byte, error) {
	code := append([]byte(nil), combatChargeSpec.Original...)
	factorDisp := 0
	if request.Instant {
		code = append(code, 0x0F, 0x57, 0xC9) // xorps xmm1,xmm1
	} else {
		code = append(code, 0xF3, 0x0F, 0x59, 0x05, 0, 0, 0, 0) // mulss xmm0,[rip+factor]
		factorDisp = len(code) - 4
	}
	jump, err := makeRelJump(cave+uintptr(len(code)), returnAddr, 5)
	if err != nil {
		return nil, err
	}
	code = append(code, jump...)
	code = append(code, combatChargeMarker...)
	if !request.Instant {
		factorOffset := len(code)
		var raw [4]byte
		binary.LittleEndian.PutUint32(raw[:], math.Float32bits(float32(request.SpeedMultiplier)))
		code = append(code, raw[:]...)
		if err := patchCombatRIPRelative(code, factorDisp, cave, factorOffset); err != nil {
			return nil, err
		}
	}
	return code, nil
}

func buildCombatActionSpeedCave(cave, returnAddr uintptr, request CombatActionSpeedRequest) ([]byte, error) {
	if err := validateCombatActionSpeedRequest(request); err != nil {
		return nil, err
	}
	code := make([]byte, 0, 80)
	shortJumps := make([]int, 0, 3)
	code = append(code, 0x51)                   // push rcx
	code = append(code, 0x48, 0x8B, 0x4B, 0x58) // mov rcx,[rbx+58]
	code = append(code, 0x48, 0x85, 0xC9)       // test rcx,rcx
	code = append(code, 0x74, 0)                // je original
	shortJumps = append(shortJumps, len(code)-1)
	code = append(code, 0x80, 0xB9, 0xFE, 0x01, 0, 0, 1) // cmp byte [rcx+1FE],1
	code = append(code, 0x75, 0)                         // jne original
	shortJumps = append(shortJumps, len(code)-1)
	if !request.ApplyWholeParty {
		code = append(code, 0x83, 0xB9, 0x10, 0x02, 0, 0, 1) // cmp dword [rcx+210],1
		code = append(code, 0x75, 0)                         // jne original
		shortJumps = append(shortJumps, len(code)-1)
	}
	code = append(code, 0xF3, 0x0F, 0x10, 0x05, 0, 0, 0, 0) // movss xmm0,[rip+factor]
	factorDisp := len(code) - 4
	code = append(code, 0xF3, 0x0F, 0x11, 0x40, 0x18) // movss [rax+18],xmm0
	originalOffset := len(code)
	for _, displacement := range shortJumps {
		if err := patchCombatShortJump(code, displacement, originalOffset); err != nil {
			return nil, err
		}
	}
	code = append(code, 0x59) // pop rcx
	code = append(code, combatActionSpeedSpec.Original...)
	jump, err := makeRelJump(cave+uintptr(len(code)), returnAddr, 5)
	if err != nil {
		return nil, err
	}
	code = append(code, jump...)
	code = append(code, combatActionSpeedMarker...)
	factorOffset := len(code)
	var raw [4]byte
	binary.LittleEndian.PutUint32(raw[:], math.Float32bits(float32(request.SpeedMultiplier)))
	code = append(code, raw[:]...)
	if err := patchCombatRIPRelative(code, factorDisp, cave, factorOffset); err != nil {
		return nil, err
	}
	return code, nil
}

func patchCombatShortJump(code []byte, displacementOffset, targetOffset int) error {
	delta := targetOffset - (displacementOffset + 1)
	if displacementOffset < 0 || displacementOffset >= len(code) || delta < -128 || delta > 127 {
		return fmt.Errorf("短跳转超出范围")
	}
	code[displacementOffset] = byte(int8(delta))
	return nil
}

func patchCombatRIPRelative(code []byte, displacementOffset int, cave uintptr, targetOffset int) error {
	if displacementOffset < 0 || displacementOffset+4 > len(code) {
		return fmt.Errorf("RIP 相对地址偏移无效")
	}
	instructionEnd := cave + uintptr(displacementOffset+4)
	target := cave + uintptr(targetOffset)
	delta := int64(target) - int64(instructionEnd)
	if delta < math.MinInt32 || delta > math.MaxInt32 {
		return fmt.Errorf("RIP 相对地址超出范围")
	}
	binary.LittleEndian.PutUint32(code[displacementOffset:displacementOffset+4], uint32(int32(delta)))
	return nil
}
