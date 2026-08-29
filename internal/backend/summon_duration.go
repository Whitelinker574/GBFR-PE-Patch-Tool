package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
)

const (
	summonDurationRVA      = uintptr(0x18AB87)
	summonDurationCaveSize = 0x40
)

var (
	summonDurationOriginal    = []byte{0xC5, 0xFA, 0x10, 0x15, 0x19, 0x95, 0xAB, 0x07}
	summonDuration204Original = []byte{0xC5, 0xFA, 0x10, 0x15, 0x99, 0xA7, 0xAB, 0x07}
	summonDuration205Original = []byte{0xC5, 0xFA, 0x10, 0x15, 0x19, 0xAA, 0xAB, 0x07}
	summonDurationAOB         = "C5 FA 10 15 ?? ?? ?? ?? C5 EA 59 15 ?? ?? ?? ?? C5 F2 58 CA C5 FA 11 8B F4 1E 00 00 C5 F8 2E C1 76 0A"
	summonDurationMarker      = []byte("GBFRSD01")
)

func summonDurationOriginalForDigest(digest string) []byte {
	if strings.EqualFold(digest, game205ExecutableSHA256) {
		return append([]byte(nil), summonDuration205Original...)
	}
	if strings.EqualFold(digest, game204ExecutableSHA256) {
		return append([]byte(nil), summonDuration204Original...)
	}
	return append([]byte(nil), summonDurationOriginal...)
}

type SummonDurationRequest struct {
	Enabled            bool    `json:"enabled"`
	Infinite           bool    `json:"infinite"`
	DurationMultiplier float64 `json:"durationMultiplier"`
}

type SummonDurationStatus struct {
	Available          bool    `json:"available"`
	Enabled            bool    `json:"enabled"`
	Infinite           bool    `json:"infinite"`
	DurationMultiplier float64 `json:"durationMultiplier"`
	RVA                uint64  `json:"rva"`
	CurrentBytes       string  `json:"currentBytes"`
	EvidenceNote       string  `json:"evidenceNote"`
	Error              string  `json:"error"`
}

type summonDurationLease struct {
	OwnerToken string
	Process    processInstanceID
	State      runtimePatchPatchState
	Request    SummonDurationRequest
	Site       runtimePatchPatchSiteLease
	CaveAddr   uintptr
}

func (a *App) SummonDurationGetStatusOwned(token string) (SummonDurationStatus, error) {
	a.procMu.Lock()
	defer a.procMu.Unlock()
	if err := a.validateRuntimePatchStatusOwnerLocked(token); err != nil {
		return emptySummonDurationStatus(), err
	}
	if err := a.verifyRuntimePatchExecutableLocked(a.currentProcessInstance(), "召唤持续时间"); err != nil {
		return emptySummonDurationStatus(), err
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	return a.readSummonDurationStatusLocked(token)
}

func (a *App) SummonDurationSetOwned(token string, request SummonDurationRequest) (SummonDurationStatus, error) {
	if err := validateSummonDurationRequest(request); err != nil {
		return emptySummonDurationStatus(), err
	}
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return emptySummonDurationStatus(), err
	}
	defer a.procMu.Unlock()
	if request.Enabled {
		if err := a.ensureLiveMemoryWritesSafe(); err != nil {
			return emptySummonDurationStatus(), err
		}
		if err := a.verifyRuntimePatchExecutableLocked(a.currentProcessInstance(), "召唤持续时间"); err != nil {
			return emptySummonDurationStatus(), err
		}
		if !isGame203PlusExecutableDigest(a.runtimePatchVerifiedDigest) {
			return emptySummonDurationStatus(), errors.New("召唤持续时间仅支持已验证的游戏 2.0.3 / 2.0.4 / 2.0.5 可执行文件")
		}
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	if !request.Enabled {
		if err := a.restoreSummonDurationOwnedLocked(token, false); err != nil {
			status, statusErr := a.readSummonDurationStatusLocked(token)
			return status, errors.Join(err, statusErr)
		}
		return a.readSummonDurationStatusLocked(token)
	}
	if a.summonDurationLease != nil {
		status, statusErr := a.readSummonDurationStatusLocked(token)
		return status, errors.Join(statusErr, errors.New("召唤持续时间已经启用或等待恢复，请先关闭后再修改"))
	}
	if err := a.enableSummonDurationLocked(token, request); err != nil {
		status, statusErr := a.readSummonDurationStatusLocked(token)
		return status, errors.Join(err, statusErr)
	}
	status, statusErr := a.readSummonDurationStatusLocked(token)
	if statusErr == nil && status.Enabled {
		return status, nil
	}
	rollbackErr := a.restoreSummonDurationOwnedLocked(token, false)
	var statusErrDetail error
	if status.Error != "" {
		statusErrDetail = errors.New(status.Error)
	}
	if statusErr == nil && statusErrDetail == nil {
		statusErr = errors.New("召唤持续时间安装后未进入启用状态")
	}
	if rollbackErr != nil {
		a.poisonCurrentLiveMemoryWrites()
		return status, errors.Join(statusErr, statusErrDetail, errRuntimeHookRollbackUnproven, rollbackErr)
	}
	restored, restoredErr := a.readSummonDurationStatusLocked(token)
	return restored, errors.Join(statusErr, statusErrDetail, restoredErr)
}

func validateSummonDurationRequest(request SummonDurationRequest) error {
	if !request.Enabled {
		return nil
	}
	if math.IsNaN(request.DurationMultiplier) || math.IsInf(request.DurationMultiplier, 0) ||
		request.DurationMultiplier < 0.1 || request.DurationMultiplier > 16 {
		return errors.New("召唤持续时间倍率请输入 0.1 到 16.0")
	}
	return nil
}

func emptySummonDurationStatus() SummonDurationStatus {
	return SummonDurationStatus{
		DurationMultiplier: 2,
		EvidenceNote:       "2.0.3 / 2.0.4 / 2.0.5 召唤持续时间递减入口已锁定；倍率与无限持续共用同一可恢复 Hook。",
	}
}

func (a *App) enableSummonDurationLocked(ownerToken string, request SummonDurationRequest) error {
	address, err := a.locateSummonDurationLocked()
	if err != nil {
		return err
	}
	cave, err := virtualAllocRemoteNear(a.hProcess, address, 0x1000)
	if err != nil {
		return fmt.Errorf("分配召唤持续时间代码洞失败: %w", err)
	}
	original := summonDurationOriginalForDigest(a.runtimePatchVerifiedDigest)
	code, err := buildSummonDurationCaveWithOriginal(cave, address, request, original)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return err
	}
	if err := writeCodeMemory(a.hProcess, cave, code); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return err
	}
	memory := runtimePatchProcessMemory{handle: a.hProcess}
	currentCave, err := memory.ReadCode(cave, len(code))
	if err != nil || !bytes.Equal(currentCave, code) {
		_ = virtualFreeRemote(a.hProcess, cave)
		return errors.Join(errors.New("召唤持续时间代码洞写后回读失败"), err)
	}
	if err := validateSummonDurationCaveBytesWithOriginal(cave, address, currentCave, request, original); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return err
	}
	patch, err := makeRelJump(address, cave, len(original))
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return err
	}
	lease := &summonDurationLease{
		OwnerToken: ownerToken, Process: a.currentProcessInstance(), State: runtimePatchPatchRecovery,
		Request: request, CaveAddr: cave,
		Site: runtimePatchPatchSiteLease{
			Address: address, RVA: uint64(summonDurationRVA),
			Original: append([]byte(nil), original...), Patch: patch,
		},
	}
	a.summonDurationLease = lease
	if overlap := findRuntimePatchActiveAddressOverlap([]runtimePatchPatchSiteLease{lease.Site}, a.runtimePatchPatchLeases, ""); overlap != "" {
		_ = virtualFreeRemote(a.hProcess, cave)
		a.summonDurationLease = nil
		return fmt.Errorf("召唤持续时间与已启用补丁 %s 地址重叠", overlap)
	}
	if err := installRuntimePatchSites(memory, []runtimePatchPatchSiteLease{lease.Site}); err != nil {
		if errors.Is(err, errLiveMemoryRollbackUnproven) {
			a.poisonCurrentLiveMemoryWrites()
			return err
		}
		a.retireRuntimeCaveLocked(cave, "summon-duration install rollback")
		a.summonDurationLease = nil
		return err
	}
	lease.State = runtimePatchPatchEnabled
	return nil
}

func (a *App) readSummonDurationStatusLocked(ownerToken string) (SummonDurationStatus, error) {
	status := emptySummonDurationStatus()
	status.Available = isGame203PlusExecutableDigest(a.runtimePatchVerifiedDigest)
	if !status.Available {
		status.Error = "仅支持已验证的游戏 2.0.3 / 2.0.4 / 2.0.5 可执行文件"
		return status, nil
	}
	lease := a.summonDurationLease
	if lease == nil {
		return status, nil
	}
	if !runtimeOwnerTokenMatches(lease.OwnerToken, ownerToken) {
		return status, errRuntimeOwnerLeaseStale
	}
	if !sameProcessInstance(lease.Process, a.currentProcessInstance()) {
		return status, errors.New("召唤持续时间租约属于另一个游戏进程实例")
	}
	if lease.Site.Address == 0 || lease.CaveAddr == 0 || len(lease.Site.Original) != len(summonDurationOriginal) {
		return status, errors.Join(errors.New("召唤持续时间恢复租约不完整"), errLiveMemoryRollbackUnproven)
	}
	status.Infinite = lease.Request.Infinite
	status.DurationMultiplier = lease.Request.DurationMultiplier
	status.RVA = lease.Site.RVA
	if lease.State != runtimePatchPatchEnabled {
		status.Error = appendRuntimePatchStatusError(status.Error, "需要恢复后才能再次启用")
	}
	memory := runtimePatchProcessMemory{handle: a.hProcess}
	entry, err := memory.ReadCode(lease.Site.Address, len(lease.Site.Patch))
	if err != nil {
		return status, err
	}
	status.CurrentBytes = bytesToHex(entry)
	entryOwned := bytes.Equal(entry, lease.Site.Patch)
	if !entryOwned {
		status.Error = appendRuntimePatchStatusError(status.Error, "入口已被恢复或外部修改")
	}
	cave, err := memory.ReadCode(lease.CaveAddr, summonDurationCaveSize)
	if err != nil {
		return status, err
	}
	if err := validateSummonDurationCaveBytesWithOriginal(lease.CaveAddr, lease.Site.Address, cave, lease.Request, lease.Site.Original); err != nil {
		status.Error = appendRuntimePatchStatusError(status.Error, "代码洞所有权校验失败: "+err.Error())
	}
	status.Enabled = lease.State == runtimePatchPatchEnabled && entryOwned && status.Error == ""
	return status, nil
}

func (a *App) restoreSummonDurationOwnedLocked(ownerToken string, force bool) error {
	lease := a.summonDurationLease
	if lease == nil {
		return nil
	}
	if !sameProcessInstance(lease.Process, a.currentProcessInstance()) {
		return errors.New("召唤持续时间恢复租约属于另一个游戏进程实例")
	}
	if !force && !runtimeOwnerTokenMatches(lease.OwnerToken, ownerToken) {
		return errRuntimeOwnerLeaseStale
	}
	if lease.Site.Address == 0 || lease.CaveAddr == 0 || len(lease.Site.Original) != len(summonDurationOriginal) {
		return errors.Join(errors.New("召唤持续时间恢复租约不完整"), errLiveMemoryRollbackUnproven)
	}
	lease.State = runtimePatchPatchRecovery
	if err := restoreRuntimePatchSites(runtimePatchProcessMemory{handle: a.hProcess}, []runtimePatchPatchSiteLease{lease.Site}); err != nil {
		if errors.Is(err, errLiveMemoryRollbackUnproven) {
			a.poisonCurrentLiveMemoryWrites()
		}
		return err
	}
	a.retireRuntimeCaveLocked(lease.CaveAddr, "summon-duration release")
	a.summonDurationLease = nil
	return nil
}

func (a *App) dropSummonDurationOwnerLocked(ownerToken string, force bool) {
	if lease := a.summonDurationLease; lease != nil && (force || runtimeOwnerTokenMatches(lease.OwnerToken, ownerToken)) {
		a.summonDurationLease = nil
		a.summonDurationAddr = 0
	}
}

func (a *App) locateSummonDurationLocked() (uintptr, error) {
	if a.summonDurationAddr != 0 {
		return a.summonDurationAddr, nil
	}
	pattern, err := parseRuntimePatchPattern(summonDurationAOB)
	if err != nil {
		return 0, err
	}
	address, err := a.scanRuntimePatchPatternUnique(pattern, "召唤持续时间")
	if err != nil {
		return 0, err
	}
	if got := address - a.moduleBase; got != summonDurationRVA {
		return 0, fmt.Errorf("召唤持续时间 RVA=0x%X，预期 0x%X", got, summonDurationRVA)
	}
	original := summonDurationOriginalForDigest(a.runtimePatchVerifiedDigest)
	current, err := runtimePatchProcessMemory{handle: a.hProcess}.ReadCode(address, len(original))
	if err != nil {
		return 0, err
	}
	if !bytes.Equal(current, original) {
		return 0, fmt.Errorf("召唤持续时间原字节不匹配: %s", bytesToHex(current))
	}
	a.summonDurationAddr = address
	return address, nil
}

func buildSummonDurationCave(cave, entry uintptr, request SummonDurationRequest) ([]byte, error) {
	return buildSummonDurationCaveWithOriginal(cave, entry, request, summonDurationOriginal)
}

func buildSummonDurationCaveWithOriginal(cave, entry uintptr, request SummonDurationRequest, original []byte) ([]byte, error) {
	if err := validateSummonDurationRequest(request); err != nil {
		return nil, err
	}
	if len(original) != len(summonDurationOriginal) || !bytes.Equal(original[:4], summonDurationOriginal[:4]) {
		return nil, errors.New("召唤持续时间原指令版本无效")
	}
	code := make([]byte, 0, summonDurationCaveSize)
	code = append(code, original...)
	originalTarget := uintptr(int64(entry+8) + int64(int32(binary.LittleEndian.Uint32(original[4:8]))))
	newDisp := int64(originalTarget) - int64(cave+8)
	if newDisp < math.MinInt32 || newDisp > math.MaxInt32 {
		return nil, errors.New("召唤持续时间原始常量超出代码洞 RIP 相对寻址范围")
	}
	binary.LittleEndian.PutUint32(code[4:8], uint32(int32(newDisp)))
	if request.Infinite {
		code = append(code, 0x0F, 0x57, 0xD2) // xorps xmm2,xmm2
	} else {
		code = append(code, 0xF3, 0x0F, 0x5E, 0x15, 0, 0, 0, 0) // divss xmm2,[rip+factor]
	}
	jumpOffset := len(code)
	jump, err := makeRelJump(cave+uintptr(jumpOffset), entry+uintptr(len(original)), 5)
	if err != nil {
		return nil, err
	}
	code = append(code, jump...)
	code = append(code, summonDurationMarker...)
	factorOffset := len(code)
	var raw [4]byte
	binary.LittleEndian.PutUint32(raw[:], math.Float32bits(float32(request.DurationMultiplier)))
	code = append(code, raw[:]...)
	for len(code) < summonDurationCaveSize {
		code = append(code, 0x90)
	}
	if !request.Infinite {
		factorDispOffset := 12
		if err := patchCombatRIPRelative(code, factorDispOffset, cave, factorOffset); err != nil {
			return nil, err
		}
	}
	return code, nil
}

func validateSummonDurationCaveBytes(cave, entry uintptr, code []byte, request SummonDurationRequest) error {
	return validateSummonDurationCaveBytesWithOriginal(cave, entry, code, request, summonDurationOriginal)
}

func validateSummonDurationCaveBytesWithOriginal(cave, entry uintptr, code []byte, request SummonDurationRequest, original []byte) error {
	if len(original) != len(summonDurationOriginal) || len(code) < summonDurationCaveSize || !bytes.Equal(code[:4], original[:4]) {
		return errors.New("召唤持续时间代码洞过短或原指令不匹配")
	}
	originalTarget := uintptr(int64(entry+8) + int64(int32(binary.LittleEndian.Uint32(original[4:8]))))
	relocatedTarget := uintptr(int64(cave+8) + int64(int32(binary.LittleEndian.Uint32(code[4:8]))))
	if relocatedTarget != originalTarget {
		return errors.New("召唤持续时间原指令 RIP 目标不匹配")
	}
	markerOffset := bytes.Index(code, summonDurationMarker)
	if markerOffset < 0 || markerOffset+12 > len(code) {
		return errors.New("召唤持续时间代码洞所有权标记缺失")
	}
	factor := math.Float32frombits(binary.LittleEndian.Uint32(code[markerOffset+8 : markerOffset+12]))
	if factor != float32(request.DurationMultiplier) {
		return errors.New("召唤持续时间倍率常量不匹配")
	}
	jumpOffset := 11
	if !request.Infinite {
		jumpOffset = 16
		factorTarget := uintptr(int64(cave+16) + int64(int32(binary.LittleEndian.Uint32(code[12:16]))))
		if factorTarget != cave+uintptr(markerOffset+8) {
			return errors.New("召唤持续时间倍率地址不匹配")
		}
	}
	if code[jumpOffset] != 0xE9 || relJumpTarget(cave+uintptr(jumpOffset), code[jumpOffset:jumpOffset+5]) != entry+uintptr(len(original)) {
		return errors.New("召唤持续时间代码洞回跳地址不匹配")
	}
	return nil
}
