package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"

	"golang.org/x/sys/windows"
)

const (
	taskScoreMultiplierRVA = uintptr(0x1FD9339)
	taskScoreCaveSize      = 0x38
	taskSideQuestCaveSize  = 0x30
)

var (
	taskScoreOriginal  = []byte{0x89, 0x8C, 0x85, 0x84, 0x00, 0x00, 0x00}
	taskScoreAOB       = "89 8C 85 84 00 00 00 C4 C1 7A 10 04 74 C5 FA 11 44 85 64 E9 ?? ?? ?? ??"
	taskScoreMarker    = []byte("GBFRSC01")
	taskSideQuestSpecs = []taskSideQuestSiteSpec{
		{
			Label: "支线任务目标计数", RVA: 0xBABFDC,
			AOB:      "8B 41 04 89 46 60 3B 46 5C 0F 93 06 48 83 C4 30 5B 5F 5E C3",
			Original: []byte{0x8B, 0x41, 0x04, 0x89, 0x46, 0x60}, Marker: []byte("GBFRSQA1"),
		},
		{
			Label: "支线任务同步计数", RVA: 0xBAB932,
			AOB:      "FF C8 3B 46 60 0F 92 06 E9 ?? ?? ?? ?? 48 89 F1 E8 ?? ?? ?? ?? E9 ?? ?? ?? ??",
			Original: []byte{0xFF, 0xC8, 0x3B, 0x46, 0x60}, Marker: []byte("GBFRSQB1"),
		},
	}
)

func taskScoreRVAForDigest(digest string) uintptr {
	if strings.EqualFold(digest, game205ExecutableSHA256) {
		return 0x1FDA459
	}
	if strings.EqualFold(digest, game204ExecutableSHA256) {
		return 0x1FDA2D9
	}
	return taskScoreMultiplierRVA
}

func taskSideQuestRVAForDigest(index int, digest string) uintptr {
	if strings.EqualFold(digest, game205ExecutableSHA256) {
		return []uintptr{0xBAC90C, 0xBAC262}[index]
	}
	if strings.EqualFold(digest, game204ExecutableSHA256) {
		return taskSideQuestSpecs[index].RVA + 0xFA0
	}
	return taskSideQuestSpecs[index].RVA
}

type taskSideQuestSiteSpec struct {
	Label    string
	RVA      uintptr
	AOB      string
	Original []byte
	Marker   []byte
}

type TaskScoreMultiplierRequest struct {
	Enabled    bool    `json:"enabled"`
	Multiplier float64 `json:"multiplier"`
}

type TaskRuleFeatureStatus struct {
	Available    bool     `json:"available"`
	Enabled      bool     `json:"enabled"`
	Multiplier   float64  `json:"multiplier"`
	RVAs         []uint64 `json:"rvas"`
	CurrentBytes []string `json:"currentBytes"`
	EvidenceNote string   `json:"evidenceNote"`
	Error        string   `json:"error"`
}

type TaskRulesStatus struct {
	ScoreMultiplier       TaskRuleFeatureStatus `json:"scoreMultiplier"`
	SideQuestAutoComplete TaskRuleFeatureStatus `json:"sideQuestAutoComplete"`
}

type taskRuleLease struct {
	OwnerToken string
	Process    processInstanceID
	State      runtimePatchPatchState
	Multiplier float64
	Sites      []runtimePatchPatchSiteLease
	Caves      []uintptr
}

func (a *App) TaskRulesGetStatusOwned(token string) (TaskRulesStatus, error) {
	a.procMu.Lock()
	defer a.procMu.Unlock()
	if err := a.validateRuntimePatchStatusOwnerLocked(token); err != nil {
		return TaskRulesStatus{}, err
	}
	if err := a.verifyRuntimePatchExecutableLocked(a.currentProcessInstance(), "任务规则增强"); err != nil {
		return TaskRulesStatus{}, err
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	return a.readTaskRulesStatusLocked(token)
}

func (a *App) TaskRulesSetScoreMultiplierOwned(token string, request TaskScoreMultiplierRequest) (TaskRulesStatus, error) {
	if err := validateTaskScoreMultiplierRequest(request); err != nil {
		return TaskRulesStatus{}, err
	}
	return a.setTaskRuleOwned(token, request.Enabled, "score", func() error {
		return a.enableTaskScoreMultiplierLocked(token, request.Multiplier)
	})
}

func (a *App) TaskRulesSetSideQuestAutoCompleteOwned(token string, enabled bool) (TaskRulesStatus, error) {
	return a.setTaskRuleOwned(token, enabled, "side-quest", func() error {
		return a.enableTaskSideQuestAutoCompleteLocked(token)
	})
}

func validateTaskScoreMultiplierRequest(request TaskScoreMultiplierRequest) error {
	if !request.Enabled {
		return nil
	}
	if math.IsNaN(request.Multiplier) || math.IsInf(request.Multiplier, 0) || request.Multiplier < 0.1 || request.Multiplier > 16 {
		return errors.New("任务分数倍率请输入 0.1 到 16.0")
	}
	return nil
}

func (a *App) setTaskRuleOwned(token string, enabled bool, kind string, install func() error) (TaskRulesStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return TaskRulesStatus{}, err
	}
	defer a.procMu.Unlock()
	if enabled {
		if err := a.ensureLiveMemoryWritesSafe(); err != nil {
			return TaskRulesStatus{}, err
		}
		if err := a.verifyRuntimePatchExecutableLocked(a.currentProcessInstance(), "任务规则增强"); err != nil {
			return TaskRulesStatus{}, err
		}
		if !isGame203PlusExecutableDigest(a.runtimePatchVerifiedDigest) {
			return TaskRulesStatus{}, errors.New("任务规则增强仅支持已验证的游戏 2.0.3 / 2.0.4 / 2.0.5 可执行文件")
		}
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()

	lease := a.taskScoreMultiplierLease
	if kind == "side-quest" {
		lease = a.taskSideQuestAutoCompleteLease
	}
	if !enabled {
		if err := a.restoreOneTaskRuleOwnedLocked(lease, token, false); err != nil {
			status, statusErr := a.readTaskRulesStatusLocked(token)
			return status, errors.Join(err, statusErr)
		}
		if kind == "score" {
			a.taskScoreMultiplierLease = nil
		} else {
			a.taskSideQuestAutoCompleteLease = nil
		}
		return a.readTaskRulesStatusLocked(token)
	}
	if lease != nil {
		status, statusErr := a.readTaskRulesStatusLocked(token)
		return status, errors.Join(statusErr, errors.New("该任务规则已经启用或等待恢复，请先关闭后再修改"))
	}
	if err := install(); err != nil {
		status, statusErr := a.readTaskRulesStatusLocked(token)
		return status, errors.Join(err, statusErr)
	}
	status, statusErr := a.readTaskRulesStatusLocked(token)
	featureStatus := status.ScoreMultiplier
	installedLease := a.taskScoreMultiplierLease
	if kind == "side-quest" {
		featureStatus = status.SideQuestAutoComplete
		installedLease = a.taskSideQuestAutoCompleteLease
	}
	if statusErr == nil && featureStatus.Enabled {
		return status, nil
	}
	var featureErr error
	if featureStatus.Error != "" {
		featureErr = errors.New(featureStatus.Error)
	}
	rollbackErr := a.restoreOneTaskRuleOwnedLocked(installedLease, token, false)
	if rollbackErr != nil {
		a.poisonCurrentLiveMemoryWrites()
		return status, errors.Join(statusErr, featureErr, errRuntimeHookRollbackUnproven, rollbackErr)
	}
	if statusErr == nil && featureStatus.Error == "" {
		statusErr = errors.New("任务规则安装后未进入启用状态")
	}
	restoredStatus, restoredStatusErr := a.readTaskRulesStatusLocked(token)
	return restoredStatus, errors.Join(statusErr, featureErr, restoredStatusErr)
}

func (a *App) enableTaskScoreMultiplierLocked(ownerToken string, multiplier float64) error {
	address, err := a.locateTaskScoreMultiplierLocked()
	if err != nil {
		return err
	}
	cave, err := virtualAllocRemoteNear(a.hProcess, address, 0x1000)
	if err != nil {
		return fmt.Errorf("分配任务分数代码洞失败: %w", err)
	}
	code, err := buildTaskScoreCave(cave, address+uintptr(len(taskScoreOriginal)), multiplier)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return err
	}
	if err := writeAndValidateTaskRuleCave(a, cave, code, func(current []byte) error {
		return validateTaskScoreCaveBytes(cave, address, current, float32(multiplier))
	}); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return err
	}
	patch, err := makeRelJump(address, cave, len(taskScoreOriginal))
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return err
	}
	lease := &taskRuleLease{
		OwnerToken: ownerToken, Process: a.currentProcessInstance(), State: runtimePatchPatchRecovery,
		Multiplier: multiplier,
		Sites: []runtimePatchPatchSiteLease{{
			Address: address, RVA: uint64(taskScoreRVAForDigest(a.runtimePatchVerifiedDigest)),
			Original: append([]byte(nil), taskScoreOriginal...), Patch: patch,
		}},
		Caves: []uintptr{cave},
	}
	a.taskScoreMultiplierLease = lease
	if err := a.installTaskRuleLeaseLocked(lease, "task-score"); err != nil {
		return err
	}
	return nil
}

func (a *App) enableTaskSideQuestAutoCompleteLocked(ownerToken string) error {
	addresses, err := a.locateTaskSideQuestSitesLocked()
	if err != nil {
		return err
	}
	caves := make([]uintptr, len(taskSideQuestSpecs))
	sites := make([]runtimePatchPatchSiteLease, len(taskSideQuestSpecs))
	for index, spec := range taskSideQuestSpecs {
		cave, allocErr := virtualAllocRemoteNear(a.hProcess, addresses[index], 0x1000)
		if allocErr != nil {
			freeUnpublishedTaskRuleCaves(a.hProcess, caves)
			return fmt.Errorf("分配支线任务站点 %c 代码洞失败: %w", 'A'+rune(index), allocErr)
		}
		caves[index] = cave
		code, buildErr := buildTaskSideQuestCave(index, cave, addresses[index]+uintptr(len(spec.Original)))
		if buildErr != nil {
			freeUnpublishedTaskRuleCaves(a.hProcess, caves)
			return buildErr
		}
		if writeErr := writeAndValidateTaskRuleCave(a, cave, code, func(current []byte) error {
			return validateTaskSideQuestCaveBytes(index, cave, addresses[index], current)
		}); writeErr != nil {
			freeUnpublishedTaskRuleCaves(a.hProcess, caves)
			return writeErr
		}
		patch, jumpErr := makeRelJump(addresses[index], cave, len(spec.Original))
		if jumpErr != nil {
			freeUnpublishedTaskRuleCaves(a.hProcess, caves)
			return jumpErr
		}
		sites[index] = runtimePatchPatchSiteLease{
			Address: addresses[index], RVA: uint64(taskSideQuestRVAForDigest(index, a.runtimePatchVerifiedDigest)),
			Original: append([]byte(nil), spec.Original...), Patch: patch,
		}
	}
	lease := &taskRuleLease{
		OwnerToken: ownerToken, Process: a.currentProcessInstance(), State: runtimePatchPatchRecovery,
		Sites: sites, Caves: caves,
	}
	a.taskSideQuestAutoCompleteLease = lease
	return a.installTaskRuleLeaseLocked(lease, "task-side-quest")
}

func (a *App) installTaskRuleLeaseLocked(lease *taskRuleLease, label string) error {
	if lease == nil || len(lease.Sites) == 0 || len(lease.Sites) != len(lease.Caves) {
		return errors.New("任务规则安装租约不完整")
	}
	if overlap := findRuntimePatchActiveAddressOverlap(lease.Sites, a.runtimePatchPatchLeases, ""); overlap != "" {
		freeUnpublishedTaskRuleCaves(a.hProcess, lease.Caves)
		a.clearTaskRuleLease(lease)
		return fmt.Errorf("任务规则与已启用补丁 %s 地址重叠", overlap)
	}
	if err := installRuntimePatchSites(runtimePatchProcessMemory{handle: a.hProcess}, lease.Sites); err != nil {
		if errors.Is(err, errLiveMemoryRollbackUnproven) {
			a.poisonCurrentLiveMemoryWrites()
			return err
		}
		for _, cave := range lease.Caves {
			a.retireRuntimeCaveLocked(cave, label+" install rollback")
		}
		a.clearTaskRuleLease(lease)
		return err
	}
	lease.State = runtimePatchPatchEnabled
	return nil
}

func (a *App) readTaskRulesStatusLocked(ownerToken string) (TaskRulesStatus, error) {
	available := isGame203PlusExecutableDigest(a.runtimePatchVerifiedDigest)
	status := TaskRulesStatus{
		ScoreMultiplier: emptyTaskRuleFeatureStatus(available, 2,
			"2.0.3 / 2.0.4 / 2.0.5 任务分数写入入口已锁定；倍率只改变任务分数，不改变任务奖励物品数量。"),
		SideQuestAutoComplete: emptyTaskRuleFeatureStatus(available, 0,
			"2.0.3 / 2.0.4 / 2.0.5 两条支线任务计数路径已锁定；开启后会把当前目标进度补到要求值。"),
	}
	var joined error
	if readErr := a.readOneTaskRuleLeaseLocked(ownerToken, a.taskScoreMultiplierLease, &status.ScoreMultiplier); readErr != nil {
		joined = errors.Join(joined, readErr)
	}
	if readErr := a.readOneTaskRuleLeaseLocked(ownerToken, a.taskSideQuestAutoCompleteLease, &status.SideQuestAutoComplete); readErr != nil {
		joined = errors.Join(joined, readErr)
	}
	return status, joined
}

func emptyTaskRuleFeatureStatus(available bool, multiplier float64, note string) TaskRuleFeatureStatus {
	status := TaskRuleFeatureStatus{
		Available: available, Multiplier: multiplier, RVAs: make([]uint64, 0),
		CurrentBytes: make([]string, 0), EvidenceNote: note,
	}
	if !available {
		status.Error = "仅支持已验证的游戏 2.0.3 / 2.0.4 / 2.0.5 可执行文件"
	}
	return status
}

func (a *App) readOneTaskRuleLeaseLocked(ownerToken string, lease *taskRuleLease, status *TaskRuleFeatureStatus) error {
	if lease == nil {
		return nil
	}
	if !runtimeOwnerTokenMatches(lease.OwnerToken, ownerToken) {
		return errRuntimeOwnerLeaseStale
	}
	if !sameProcessInstance(lease.Process, a.currentProcessInstance()) {
		return errors.New("任务规则租约属于另一个游戏进程实例")
	}
	if len(lease.Sites) == 0 || len(lease.Sites) != len(lease.Caves) {
		return errors.Join(errors.New("任务规则恢复租约不完整"), errLiveMemoryRollbackUnproven)
	}
	status.Multiplier = lease.Multiplier
	status.RVAs = make([]uint64, len(lease.Sites))
	status.CurrentBytes = make([]string, len(lease.Sites))
	allPatched := lease.State == runtimePatchPatchEnabled
	if lease.State != runtimePatchPatchEnabled {
		status.Error = appendRuntimePatchStatusError(status.Error, "需要恢复后才能再次启用")
	}
	memory := runtimePatchProcessMemory{handle: a.hProcess}
	for index, site := range lease.Sites {
		status.RVAs[index] = site.RVA
		current, err := memory.ReadCode(site.Address, len(site.Patch))
		if err != nil {
			return err
		}
		status.CurrentBytes[index] = bytesToHex(current)
		if !bytes.Equal(current, site.Patch) {
			allPatched = false
			status.Error = appendRuntimePatchStatusError(status.Error, fmt.Sprintf("站点 %d 已被恢复或外部修改", index+1))
		}
	}
	if err := a.validateTaskRuleLeaseCavesLocked(lease); err != nil {
		allPatched = false
		status.Error = appendRuntimePatchStatusError(status.Error, "代码洞所有权校验失败: "+err.Error())
	}
	status.Enabled = allPatched && status.Error == ""
	return nil
}

func (a *App) validateTaskRuleLeaseCavesLocked(lease *taskRuleLease) error {
	memory := runtimePatchProcessMemory{handle: a.hProcess}
	if lease == a.taskScoreMultiplierLease {
		code, err := memory.ReadCode(lease.Caves[0], taskScoreCaveSize)
		if err != nil {
			return err
		}
		return validateTaskScoreCaveBytes(lease.Caves[0], lease.Sites[0].Address, code, float32(lease.Multiplier))
	}
	if lease == a.taskSideQuestAutoCompleteLease {
		for index, cave := range lease.Caves {
			code, err := memory.ReadCode(cave, taskSideQuestCaveSize)
			if err != nil {
				return err
			}
			if err := validateTaskSideQuestCaveBytes(index, cave, lease.Sites[index].Address, code); err != nil {
				return err
			}
		}
		return nil
	}
	return errors.New("未知任务规则恢复租约")
}

func (a *App) restoreOneTaskRuleOwnedLocked(lease *taskRuleLease, ownerToken string, force bool) error {
	if lease == nil {
		return nil
	}
	if !sameProcessInstance(lease.Process, a.currentProcessInstance()) {
		return errors.New("任务规则恢复租约属于另一个游戏进程实例")
	}
	if !force && !runtimeOwnerTokenMatches(lease.OwnerToken, ownerToken) {
		return errRuntimeOwnerLeaseStale
	}
	if len(lease.Sites) == 0 || len(lease.Sites) != len(lease.Caves) {
		return errors.Join(errors.New("任务规则恢复租约不完整"), errLiveMemoryRollbackUnproven)
	}
	lease.State = runtimePatchPatchRecovery
	if err := restoreRuntimePatchSites(runtimePatchProcessMemory{handle: a.hProcess}, lease.Sites); err != nil {
		if errors.Is(err, errLiveMemoryRollbackUnproven) {
			a.poisonCurrentLiveMemoryWrites()
		}
		return err
	}
	for _, cave := range lease.Caves {
		a.retireRuntimeCaveLocked(cave, "task-rule release")
	}
	a.clearTaskRuleLease(lease)
	return nil
}

func (a *App) restoreTaskRulesOwnedLocked(ownerToken string, force bool) error {
	var joined error
	if lease := a.taskSideQuestAutoCompleteLease; lease != nil {
		joined = errors.Join(joined, a.restoreOneTaskRuleOwnedLocked(lease, ownerToken, force))
	}
	if lease := a.taskScoreMultiplierLease; lease != nil {
		joined = errors.Join(joined, a.restoreOneTaskRuleOwnedLocked(lease, ownerToken, force))
	}
	return joined
}

func (a *App) clearTaskRuleLease(lease *taskRuleLease) {
	if a.taskScoreMultiplierLease == lease {
		a.taskScoreMultiplierLease = nil
	}
	if a.taskSideQuestAutoCompleteLease == lease {
		a.taskSideQuestAutoCompleteLease = nil
	}
}

func (a *App) dropTaskRuleOwnersLocked(ownerToken string, force bool) {
	for _, lease := range []*taskRuleLease{a.taskScoreMultiplierLease, a.taskSideQuestAutoCompleteLease} {
		if lease != nil && (force || runtimeOwnerTokenMatches(lease.OwnerToken, ownerToken)) {
			a.clearTaskRuleLease(lease)
		}
	}
	if force {
		a.taskScoreMultiplierAddr = 0
		a.taskSideQuestAutoCompleteAddrs = nil
	}
}

func (a *App) locateTaskScoreMultiplierLocked() (uintptr, error) {
	if a.taskScoreMultiplierAddr != 0 {
		return a.taskScoreMultiplierAddr, nil
	}
	address, err := locateTaskRuleSiteLocked(a, taskScoreAOB, "任务分数倍率", taskScoreRVAForDigest(a.runtimePatchVerifiedDigest), taskScoreOriginal)
	if err != nil {
		return 0, err
	}
	a.taskScoreMultiplierAddr = address
	return address, nil
}

func (a *App) locateTaskSideQuestSitesLocked() ([]uintptr, error) {
	if len(a.taskSideQuestAutoCompleteAddrs) == len(taskSideQuestSpecs) {
		return append([]uintptr(nil), a.taskSideQuestAutoCompleteAddrs...), nil
	}
	addresses := make([]uintptr, len(taskSideQuestSpecs))
	for index, spec := range taskSideQuestSpecs {
		address, err := locateTaskRuleSiteLocked(a, spec.AOB, spec.Label, taskSideQuestRVAForDigest(index, a.runtimePatchVerifiedDigest), spec.Original)
		if err != nil {
			return nil, err
		}
		addresses[index] = address
	}
	a.taskSideQuestAutoCompleteAddrs = append([]uintptr(nil), addresses...)
	return addresses, nil
}

func locateTaskRuleSiteLocked(a *App, rawAOB, label string, expectedRVA uintptr, original []byte) (uintptr, error) {
	pattern, err := parseRuntimePatchPattern(rawAOB)
	if err != nil {
		return 0, err
	}
	address, err := a.scanRuntimePatchPatternUnique(pattern, label)
	if err != nil {
		return 0, err
	}
	if got := address - a.moduleBase; got != expectedRVA {
		return 0, fmt.Errorf("%s RVA=0x%X，预期 0x%X", label, got, expectedRVA)
	}
	current, err := runtimePatchProcessMemory{handle: a.hProcess}.ReadCode(address, len(original))
	if err != nil {
		return 0, err
	}
	if !bytes.Equal(current, original) {
		return 0, fmt.Errorf("%s 原字节不匹配: %s", label, bytesToHex(current))
	}
	return address, nil
}

func buildTaskScoreCave(cave, returnAddr uintptr, multiplier float64) ([]byte, error) {
	if err := validateTaskScoreMultiplierRequest(TaskScoreMultiplierRequest{Enabled: true, Multiplier: multiplier}); err != nil {
		return nil, err
	}
	code := make([]byte, 0, taskScoreCaveSize)
	code = append(code, 0xF3, 0x0F, 0x2A, 0xC1)             // cvtsi2ss xmm0,ecx
	code = append(code, 0xF3, 0x0F, 0x59, 0x05, 0, 0, 0, 0) // mulss xmm0,[rip+factor]
	factorDisp := len(code) - 4
	code = append(code, 0xF3, 0x0F, 0x2D, 0xC8) // cvtss2si ecx,xmm0
	code = append(code, taskScoreOriginal...)
	jump, err := makeRelJump(cave+uintptr(len(code)), returnAddr, 5)
	if err != nil {
		return nil, err
	}
	code = append(code, jump...)
	code = append(code, taskScoreMarker...)
	factorOffset := len(code)
	var raw [4]byte
	binary.LittleEndian.PutUint32(raw[:], math.Float32bits(float32(multiplier)))
	code = append(code, raw[:]...)
	for len(code) < taskScoreCaveSize {
		code = append(code, 0x90)
	}
	if err := patchCombatRIPRelative(code, factorDisp, cave, factorOffset); err != nil {
		return nil, err
	}
	return code, nil
}

func validateTaskScoreCaveBytes(cave, entry uintptr, code []byte, multiplier float32) error {
	if len(code) < taskScoreCaveSize || !bytes.Equal(code[:4], []byte{0xF3, 0x0F, 0x2A, 0xC1}) {
		return errors.New("任务分数代码洞过短或转换指令不匹配")
	}
	markerOffset := bytes.Index(code, taskScoreMarker)
	if markerOffset < 0 || markerOffset+12 > len(code) {
		return errors.New("任务分数代码洞所有权标记缺失")
	}
	factor := math.Float32frombits(binary.LittleEndian.Uint32(code[markerOffset+8 : markerOffset+12]))
	if factor != multiplier {
		return fmt.Errorf("任务分数倍率=%v，预期 %v", factor, multiplier)
	}
	factorTarget := uintptr(int64(cave+12) + int64(int32(binary.LittleEndian.Uint32(code[8:12]))))
	if factorTarget != cave+uintptr(markerOffset+8) {
		return errors.New("任务分数倍率地址不匹配")
	}
	originalOffset := bytes.Index(code, taskScoreOriginal)
	if originalOffset < 0 || originalOffset+len(taskScoreOriginal)+5 > markerOffset {
		return errors.New("任务分数原指令重放缺失")
	}
	jumpOffset := originalOffset + len(taskScoreOriginal)
	if code[jumpOffset] != 0xE9 || relJumpTarget(cave+uintptr(jumpOffset), code[jumpOffset:jumpOffset+5]) != entry+uintptr(len(taskScoreOriginal)) {
		return errors.New("任务分数代码洞回跳地址不匹配")
	}
	return nil
}

func buildTaskSideQuestCave(index int, cave, returnAddr uintptr) ([]byte, error) {
	if index < 0 || index >= len(taskSideQuestSpecs) {
		return nil, fmt.Errorf("支线任务代码洞索引无效: %d", index)
	}
	code := make([]byte, 0, taskSideQuestCaveSize)
	if index == 0 {
		code = append(code, 0x8B, 0x46, 0x5C) // mov eax,[rsi+5C]
		code = append(code, 0x39, 0x41, 0x04) // cmp [rcx+04],eax
		code = append(code, 0x7D, 0x03)       // jge original
		code = append(code, 0x89, 0x41, 0x04) // mov [rcx+04],eax
	} else {
		code = append(code, 0x85, 0xC0)       // test eax,eax
		code = append(code, 0x74, 0x03)       // jz original
		code = append(code, 0x89, 0x46, 0x60) // mov [rsi+60],eax
	}
	code = append(code, taskSideQuestSpecs[index].Original...)
	jump, err := makeRelJump(cave+uintptr(len(code)), returnAddr, 5)
	if err != nil {
		return nil, err
	}
	code = append(code, jump...)
	code = append(code, taskSideQuestSpecs[index].Marker...)
	for len(code) < taskSideQuestCaveSize {
		code = append(code, 0x90)
	}
	return code, nil
}

func validateTaskSideQuestCaveBytes(index int, cave, entry uintptr, code []byte) error {
	if index < 0 || index >= len(taskSideQuestSpecs) || len(code) < taskSideQuestCaveSize {
		return errors.New("支线任务代码洞参数无效")
	}
	spec := taskSideQuestSpecs[index]
	markerOffset := bytes.Index(code, spec.Marker)
	originalOffset := bytes.Index(code, spec.Original)
	if markerOffset < 0 || originalOffset < 0 || originalOffset+len(spec.Original)+5 > markerOffset {
		return errors.New("支线任务代码洞原指令或所有权标记缺失")
	}
	jumpOffset := originalOffset + len(spec.Original)
	if code[jumpOffset] != 0xE9 || relJumpTarget(cave+uintptr(jumpOffset), code[jumpOffset:jumpOffset+5]) != entry+uintptr(len(spec.Original)) {
		return errors.New("支线任务代码洞回跳地址不匹配")
	}
	return nil
}

func writeAndValidateTaskRuleCave(a *App, cave uintptr, code []byte, validate func([]byte) error) error {
	if err := writeCodeMemory(a.hProcess, cave, code); err != nil {
		return err
	}
	current, err := runtimePatchProcessMemory{handle: a.hProcess}.ReadCode(cave, len(code))
	if err != nil {
		return fmt.Errorf("任务规则代码洞写后回读失败: %w", err)
	}
	if !bytes.Equal(current, code) {
		return errors.New("任务规则代码洞写后回读不一致")
	}
	return validate(current)
}

func freeUnpublishedTaskRuleCaves(handle windows.Handle, caves []uintptr) {
	for _, cave := range caves {
		if cave != 0 {
			_ = virtualFreeRemote(handle, cave)
		}
	}
}
