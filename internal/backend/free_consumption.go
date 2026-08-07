package backend

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

const (
	freeConsumptionCaveMarkerOffset = 0x20
	freeConsumptionCaveSize         = freeConsumptionCaveMarkerOffset + 8
)

var (
	freeConsumptionCaveMarker = []byte("GBFRFC01")
	freeConsumptionSites      = []freeConsumptionSiteSpec{
		{
			Label: "A 制作数量上限", RVA: 0x41D1247,
			AOB:      "8B 03 89 45 C0 44 89 7D C4 48 C7 45 C8 00 00 00 00 48 8B 96 48 03 00 00",
			Original: []byte{0x8B, 0x03, 0x89, 0x45, 0xC0},
		},
		{
			Label: "B 交易扣减", RVA: 0x3E9877A,
			AOB:      "45 2B 47 04 41 8B 17 41 B1 01 E8 ?? ?? ?? ?? 49 83 C7 10 4D 39 EF 75 DE 48 8B",
			Original: []byte{0x45, 0x2B, 0x47, 0x04}, Patch: []byte{0x90, 0x90, 0x90, 0x90},
		},
		{
			Label: "C 制作材料汇总", RVA: 0x31E71F,
			AOB:      "41 03 42 04 49 83 C1 04 49 83 C0 FC 75 D3 48 85 C9 74 1E",
			Original: []byte{0x41, 0x03, 0x42, 0x04}, Patch: []byte{0x31, 0xC0, 0x90, 0x90},
		},
		{
			Label: "D 升级材料汇总", RVA: 0x31E744,
			AOB:      "41 03 41 04 49 FF C0 4C 39 C1 75 F0 5E C3 CC CC",
			Original: []byte{0x41, 0x03, 0x41, 0x04}, Patch: []byte{0x31, 0xC0, 0x90, 0x90},
		},
		{
			Label: "E 交易数量", RVA: 0x41D23FC,
			AOB:      "44 8B 68 04 8B 06 89 85 F0 00 00 00 44 89 AD F4 00 00 00 48 C7",
			Original: []byte{0x44, 0x8B, 0x68, 0x04}, Patch: []byte{0x41, 0xB5, 0x63, 0x90},
		},
		{
			Label: "F 数量不足分支", RVA: 0x41C9CDE,
			AOB:      "7E 43 8B 45 CC 41 39 C0 0F 94 C3 45 85 C0 7E 05 41 39 C0 7F 43",
			Original: []byte{0x7E, 0x43}, Patch: []byte{0x90, 0x90},
		},
		{
			Label: "G 强化材料写入", RVA: 0x35B3E9,
			AOB:      "8B 44 24 34 89 42 04 8B 44 24 38 89 42 08 8B 44 24 3C 89 42 0C 48 83 46 08 10 66",
			Original: []byte{0x8B, 0x44, 0x24, 0x34}, Patch: []byte{0x31, 0xC0, 0x90, 0x90},
		},
		{
			Label: "H 强化返回数量", RVA: 0x35B9E4,
			AOB:      "8B 80 AC 00 00 00 48 83 C4 48 5B 5D 5F 5E 41 5C 41 5D 41 5E 41 5F C3 CC CC CC CC CC",
			Original: []byte{0x8B, 0x80, 0xAC, 0x00, 0x00, 0x00}, Patch: []byte{0x31, 0xC0, 0x90, 0x90, 0x90, 0x90},
		},
		{
			Label: "I 货币扣减", RVA: 0x1BC58C1,
			AOB:      "41 F7 D8 41 8B 16 41 B1 01 E8 ?? ?? ?? ?? 41 8B 06 3D 9A 83 07 5C 74 B7",
			Original: []byte{0x41, 0xF7, 0xD8}, Patch: []byte{0x4D, 0x31, 0xC0},
		},
		{
			Label: "J 交易库存检查", RVA: 0x400ABC,
			AOB:      "0F 8C C9 00 00 00 4C 39 EE 0F 84 A5 00 00 00 4C 8B 35 ?? ?? ?? ?? 44 8B 26",
			Original: []byte{0x0F, 0x8C, 0xC9, 0x00, 0x00, 0x00}, Patch: []byte{0x90, 0x90, 0x90, 0x90, 0x90, 0x90},
		},
		{
			Label: "K 升级扣减", RVA: 0x41C98F9,
			AOB:      "45 8B 6C 24 04 48 8B 01 49 89 CF 49 89 D6 FF 50 18 39 5D D0 0F 84 79 00",
			Original: []byte{0x45, 0x8B, 0x6C, 0x24, 0x04}, Patch: []byte{0x4D, 0x31, 0xED, 0x90, 0x90},
		},
	}
)

var freeConsumption204RVAs = [...]uintptr{
	0x41D21E7, 0x3E9971A, 0x31E71F, 0x31E744, 0x41D339C, 0x41CAC7E,
	0x35B3E9, 0x35B9E4, 0x1BC6861, 0x400ABC, 0x41CA899,
}

func freeConsumptionRVAForDigest(index int, digest string) uintptr {
	if strings.EqualFold(digest, game204ExecutableSHA256) && index >= 0 && index < len(freeConsumption204RVAs) {
		return freeConsumption204RVAs[index]
	}
	return freeConsumptionSites[index].RVA
}

type freeConsumptionSiteSpec struct {
	Label    string
	RVA      uintptr
	AOB      string
	Original []byte
	Patch    []byte
}

type FreeConsumptionStatus struct {
	Available    bool     `json:"available"`
	Enabled      bool     `json:"enabled"`
	RVAs         []uint64 `json:"rvas"`
	CurrentBytes []string `json:"currentBytes"`
	EvidenceNote string   `json:"evidenceNote"`
	Error        string   `json:"error"`
}

type freeConsumptionLease struct {
	OwnerToken string
	Process    processInstanceID
	State      runtimePatchPatchState
	Sites      []runtimePatchPatchSiteLease
	CaveAddr   uintptr
	CaveSize   uintptr
}

func (a *App) FreeConsumptionGetStatusOwned(token string) (FreeConsumptionStatus, error) {
	a.procMu.Lock()
	defer a.procMu.Unlock()
	if err := a.validateRuntimePatchStatusOwnerLocked(token); err != nil {
		return emptyFreeConsumptionStatus(), err
	}
	if err := a.verifyRuntimePatchExecutableLocked(a.currentProcessInstance(), "免费制作、交易和升级"); err != nil {
		return emptyFreeConsumptionStatus(), err
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	return a.readFreeConsumptionStatusLocked(token)
}

func (a *App) FreeConsumptionSetEnabledOwned(token string, enabled bool) (FreeConsumptionStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return emptyFreeConsumptionStatus(), err
	}
	defer a.procMu.Unlock()
	if enabled {
		if err := a.ensureLiveMemoryWritesSafe(); err != nil {
			return emptyFreeConsumptionStatus(), err
		}
		if err := a.verifyRuntimePatchExecutableLocked(a.currentProcessInstance(), "免费制作、交易和升级"); err != nil {
			return emptyFreeConsumptionStatus(), err
		}
		if !isGame203Or204ExecutableDigest(a.runtimePatchVerifiedDigest) {
			status := emptyFreeConsumptionStatus()
			status.Error = "免费制作、交易和升级仅支持已验证的游戏 2.0.3 / 2.0.4 可执行文件"
			return status, errors.New(status.Error)
		}
	}

	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	if !enabled {
		if err := a.restoreFreeConsumptionOwnedLocked(token, false); err != nil {
			status, statusErr := a.readFreeConsumptionStatusLocked(token)
			return status, errors.Join(err, statusErr)
		}
		return a.readFreeConsumptionStatusLocked(token)
	}
	if a.freeConsumptionLease != nil {
		status, statusErr := a.readFreeConsumptionStatusLocked(token)
		if statusErr == nil && status.Enabled {
			return status, nil
		}
		return status, errors.Join(statusErr, errors.New("免费制作、交易和升级需要先完成恢复，不能重复启用"))
	}
	return a.enableFreeConsumptionLocked(token)
}

func emptyFreeConsumptionStatus() FreeConsumptionStatus {
	return FreeConsumptionStatus{
		RVAs:         make([]uint64, 0),
		CurrentBytes: make([]string, 0),
		EvidenceNote: "2.0.3 / 2.0.4 的 11 条制作、交易和升级消费路径已锁定；默认关闭，开启后作为一个整体安装和恢复。",
	}
}

func (a *App) enableFreeConsumptionLocked(ownerToken string) (FreeConsumptionStatus, error) {
	addresses, err := a.locateFreeConsumptionSitesLocked()
	if err != nil {
		return emptyFreeConsumptionStatus(), err
	}
	cave, err := virtualAllocRemoteNear(a.hProcess, addresses[0], 0x1000)
	if err != nil {
		return emptyFreeConsumptionStatus(), fmt.Errorf("分配免费消费代码洞失败: %w", err)
	}
	caveCode, err := buildFreeConsumptionCave(cave, addresses[0]+uintptr(len(freeConsumptionSites[0].Original)))
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return emptyFreeConsumptionStatus(), err
	}
	if err := writeCodeMemory(a.hProcess, cave, caveCode); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return emptyFreeConsumptionStatus(), fmt.Errorf("写入免费消费代码洞失败: %w", err)
	}
	if err := validateFreeConsumptionCaveLocked(runtimePatchProcessMemory{handle: a.hProcess}, cave, addresses[0]); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return emptyFreeConsumptionStatus(), fmt.Errorf("免费消费代码洞写后校验失败: %w", err)
	}

	sites := make([]runtimePatchPatchSiteLease, len(freeConsumptionSites))
	for index, spec := range freeConsumptionSites {
		patch := append([]byte(nil), spec.Patch...)
		if index == 0 {
			patch, err = makeRelJump(addresses[index], cave, len(spec.Original))
			if err != nil {
				_ = virtualFreeRemote(a.hProcess, cave)
				return emptyFreeConsumptionStatus(), err
			}
		}
		sites[index] = runtimePatchPatchSiteLease{
			Address: addresses[index], RVA: uint64(freeConsumptionRVAForDigest(index, a.runtimePatchVerifiedDigest)),
			Original: append([]byte(nil), spec.Original...), Patch: patch,
		}
	}
	if err := validateRuntimePatchSiteRanges(sites); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return emptyFreeConsumptionStatus(), err
	}
	if overlap := findRuntimePatchActiveAddressOverlap(sites, a.runtimePatchPatchLeases, ""); overlap != "" {
		_ = virtualFreeRemote(a.hProcess, cave)
		return emptyFreeConsumptionStatus(), fmt.Errorf("免费制作、交易和升级与已启用补丁 %s 地址重叠", overlap)
	}

	lease := &freeConsumptionLease{
		OwnerToken: ownerToken,
		Process:    a.currentProcessInstance(),
		State:      runtimePatchPatchRecovery,
		Sites:      sites,
		CaveAddr:   cave,
		CaveSize:   freeConsumptionCaveSize,
	}
	a.freeConsumptionLease = lease
	if err := installRuntimePatchSites(runtimePatchProcessMemory{handle: a.hProcess}, sites); err != nil {
		if errors.Is(err, errLiveMemoryRollbackUnproven) {
			a.poisonCurrentLiveMemoryWrites()
			status, statusErr := a.readFreeConsumptionStatusLocked(ownerToken)
			return status, errors.Join(err, statusErr)
		}
		a.retireRuntimeCaveLocked(cave, "free-consumption install rollback")
		a.freeConsumptionLease = nil
		return emptyFreeConsumptionStatus(), err
	}
	lease.State = runtimePatchPatchEnabled
	status, statusErr := a.readFreeConsumptionStatusLocked(ownerToken)
	if statusErr == nil && status.Enabled {
		return status, nil
	}
	rollbackErr := a.restoreFreeConsumptionOwnedLocked(ownerToken, false)
	if rollbackErr != nil {
		a.poisonCurrentLiveMemoryWrites()
		return status, errors.Join(statusErr, errRuntimeHookRollbackUnproven, rollbackErr)
	}
	if statusErr == nil {
		statusErr = errors.New("免费制作、交易和升级安装后未进入启用状态")
	}
	return emptyFreeConsumptionStatus(), fmt.Errorf("免费消费安装后验证失败，原始入口已恢复: %w", statusErr)
}

func (a *App) readFreeConsumptionStatusLocked(ownerToken string) (FreeConsumptionStatus, error) {
	status := emptyFreeConsumptionStatus()
	status.Available = isGame203Or204ExecutableDigest(a.runtimePatchVerifiedDigest)
	if !status.Available {
		status.Error = "免费制作、交易和升级仅支持已验证的游戏 2.0.3 / 2.0.4 可执行文件"
		return status, nil
	}
	lease := a.freeConsumptionLease
	if lease == nil {
		return status, nil
	}
	if !runtimeOwnerTokenMatches(lease.OwnerToken, ownerToken) {
		return status, errRuntimeOwnerLeaseStale
	}
	if !sameProcessInstance(lease.Process, a.currentProcessInstance()) {
		return status, errors.New("免费消费恢复租约属于另一个游戏进程实例")
	}
	if len(lease.Sites) != len(freeConsumptionSites) || lease.CaveAddr == 0 || lease.CaveSize != freeConsumptionCaveSize {
		return status, errors.Join(errors.New("免费消费恢复租约不完整"), errLiveMemoryRollbackUnproven)
	}
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
			return status, fmt.Errorf("读取免费消费站点 %c 失败: %w", 'A'+rune(index), err)
		}
		status.CurrentBytes[index] = bytesToHex(current)
		if !bytes.Equal(current, site.Patch) {
			allPatched = false
			status.Error = appendRuntimePatchStatusError(status.Error, fmt.Sprintf("站点 %c 已被恢复或外部修改", 'A'+rune(index)))
		}
	}
	if err := validateFreeConsumptionCaveLocked(memory, lease.CaveAddr, lease.Sites[0].Address); err != nil {
		allPatched = false
		status.Error = appendRuntimePatchStatusError(status.Error, "代码洞所有权校验失败: "+err.Error())
	}
	status.Enabled = allPatched && status.Error == ""
	return status, nil
}

// restoreFreeConsumptionOwnedLocked runs with procMu and runtimePatchMu held.
func (a *App) restoreFreeConsumptionOwnedLocked(ownerToken string, force bool) error {
	lease := a.freeConsumptionLease
	if lease == nil {
		return nil
	}
	if !sameProcessInstance(lease.Process, a.currentProcessInstance()) {
		return errors.New("免费消费恢复租约属于另一个游戏进程实例")
	}
	if !force && !runtimeOwnerTokenMatches(lease.OwnerToken, ownerToken) {
		return errRuntimeOwnerLeaseStale
	}
	if len(lease.Sites) != len(freeConsumptionSites) || lease.CaveAddr == 0 || lease.CaveSize != freeConsumptionCaveSize {
		return errors.Join(errors.New("免费消费恢复租约不完整"), errLiveMemoryRollbackUnproven)
	}
	lease.State = runtimePatchPatchRecovery
	err := restoreRuntimePatchSites(runtimePatchProcessMemory{handle: a.hProcess}, lease.Sites)
	if err != nil {
		if errors.Is(err, errLiveMemoryRollbackUnproven) {
			a.poisonCurrentLiveMemoryWrites()
		}
		return err
	}
	a.retireRuntimeCaveLocked(lease.CaveAddr, "free-consumption release")
	a.freeConsumptionLease = nil
	return nil
}

func (a *App) dropFreeConsumptionOwnerLocked(ownerToken string, force bool) {
	lease := a.freeConsumptionLease
	if lease != nil && (force || runtimeOwnerTokenMatches(lease.OwnerToken, ownerToken)) {
		a.freeConsumptionLease = nil
		a.freeConsumptionSiteAddrs = nil
	}
}

func (a *App) locateFreeConsumptionSitesLocked() ([]uintptr, error) {
	if len(a.freeConsumptionSiteAddrs) == len(freeConsumptionSites) {
		return append([]uintptr(nil), a.freeConsumptionSiteAddrs...), nil
	}
	addresses := make([]uintptr, len(freeConsumptionSites))
	for index, spec := range freeConsumptionSites {
		pattern, err := parseRuntimePatchPattern(spec.AOB)
		if err != nil {
			return nil, fmt.Errorf("解析免费消费站点 %c 签名失败: %w", 'A'+rune(index), err)
		}
		address, err := a.scanRuntimePatchPatternUnique(pattern, spec.Label)
		if err != nil {
			return nil, err
		}
		expectedRVA := freeConsumptionRVAForDigest(index, a.runtimePatchVerifiedDigest)
		if got := address - a.moduleBase; got != expectedRVA {
			return nil, fmt.Errorf("免费消费站点 %c RVA=0x%X，预期 0x%X", 'A'+rune(index), got, expectedRVA)
		}
		current, err := runtimePatchProcessMemory{handle: a.hProcess}.ReadCode(address, len(spec.Original))
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(current, spec.Original) {
			return nil, fmt.Errorf("免费消费站点 %c 原字节不匹配: %s", 'A'+rune(index), bytesToHex(current))
		}
		addresses[index] = address
	}
	a.freeConsumptionSiteAddrs = append([]uintptr(nil), addresses...)
	return addresses, nil
}

func buildFreeConsumptionCave(cave, returnAddr uintptr) ([]byte, error) {
	code := make([]byte, 0, freeConsumptionCaveSize)
	code = append(code, 0x41, 0xBF, 0xE7, 0x03, 0x00, 0x00) // mov r15d,999
	code = append(code, freeConsumptionSites[0].Original...)
	jump, err := makeRelJump(cave+uintptr(len(code)), returnAddr, 5)
	if err != nil {
		return nil, err
	}
	code = append(code, jump...)
	for len(code) < freeConsumptionCaveMarkerOffset {
		code = append(code, 0x90)
	}
	code = append(code, freeConsumptionCaveMarker...)
	return code, nil
}

func validateFreeConsumptionCaveBytes(cave, entry uintptr, code []byte) error {
	if len(code) < freeConsumptionCaveSize {
		return fmt.Errorf("代码洞过短: %d", len(code))
	}
	if !bytes.Equal(code[:6], []byte{0x41, 0xBF, 0xE7, 0x03, 0x00, 0x00}) {
		return errors.New("制作数量上限指令不匹配")
	}
	if !bytes.Equal(code[6:11], freeConsumptionSites[0].Original) {
		return errors.New("站点 A 原指令重放不匹配")
	}
	if code[11] != 0xE9 || relJumpTarget(cave+11, code[11:16]) != entry+uintptr(len(freeConsumptionSites[0].Original)) {
		return errors.New("站点 A 回跳地址不匹配")
	}
	if !bytes.Equal(code[freeConsumptionCaveMarkerOffset:freeConsumptionCaveSize], freeConsumptionCaveMarker) {
		return errors.New("代码洞所有权标记不匹配")
	}
	return nil
}

type freeConsumptionCodeReader interface {
	ReadCode(addr uintptr, size int) ([]byte, error)
}

func validateFreeConsumptionCaveLocked(reader freeConsumptionCodeReader, cave, entry uintptr) error {
	if reader == nil || cave == 0 || entry == 0 {
		return errors.New("代码洞校验参数无效")
	}
	code, err := reader.ReadCode(cave, freeConsumptionCaveSize)
	if err != nil {
		return err
	}
	return validateFreeConsumptionCaveBytes(cave, entry, code)
}
