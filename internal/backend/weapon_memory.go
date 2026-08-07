package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unsafe"
)

const (
	// Verified against the locked GAME 2.0.3 executable and live weapon-list
	// selection. The instruction pair captures RDX while the inventory refreshes
	// the currently selected weapon instance.
	weaponMemoryHookRVA        = uintptr(0x415118C)
	weaponMemorySaveRVA        = uintptr(0x796E60)
	weaponMemoryHookSize       = uintptr(6)
	weaponMemoryRecordSize     = 0xCC
	weaponMemoryCaveDataOffset = uintptr(0x40)
	weaponMemoryOriginalOffset = uintptr(17)
	weaponMemoryMarkerOffset   = uintptr(0x30)
)

var (
	weaponMemoryOriginalBytes = []byte{0x48, 0x89, 0xD7, 0x48, 0x89, 0xCE}
	weaponMemoryGuardBytes    = []byte{
		0x48, 0x89, 0xD7, 0x48, 0x89, 0xCE, 0x83, 0x7A, 0x40, 0x00, 0x7E, 0x67,
		0x48, 0x8B, 0x4E, 0x50, 0x48, 0x85, 0xC9, 0x74, 0x07, 0xB2, 0x01,
	}
	weaponMemoryMarker      = []byte("GBFRWPM3")
	weaponMemoryLifecycleMu sync.Mutex
)

func weaponMemoryRuntimeIdentity(digest string) (hookRVA, saveRVA uintptr, version string) {
	if strings.EqualFold(digest, game204ExecutableSHA256) {
		return 0x415212C, 0x797E00, "2.0.4"
	}
	return weaponMemoryHookRVA, weaponMemorySaveRVA, "2.0.3"
}

type weaponMemoryVerifiedSkill struct {
	NameCN   string
	NameEN   string
	MaxLevel int
}

// These weapon-only rows are present in the installed 2.0.x weapon/skill
// tables but are intentionally absent from the normal sigil catalog.  Keep
// them separate: being a valid weapon skill does not make a trait a valid
// sigil shell or secondary trait.
var weaponMemoryVerifiedSkills = map[uint32]weaponMemoryVerifiedSkill{
	0x1E1CECCE: {NameCN: "浩劫新星", NameEN: "Catastrophe Nova", MaxLevel: 35},
	0x3B71AF12: {NameCN: "伤害上限·轰天", NameEN: "DMG Cap Ecru", MaxLevel: 15},
	0xFFF8CF64: {NameCN: "伤害上限·疾天", NameEN: "DMG Cap Sage", MaxLevel: 15},
	0x235D86EF: {NameCN: "超新星", NameEN: "Supernova", MaxLevel: 15},
	0xAEFEB1BC: {NameCN: "伤害上限·苍天", NameEN: "DMG Cap Cobalt", MaxLevel: 15},
	0x0151CF9E: {NameCN: "伤害上限·红天", NameEN: "DMG Cap Cardinal", MaxLevel: 15},
	0xBBD77C33: {NameCN: "超凡强击", NameEN: "Unbound Strike", MaxLevel: 15},
	0x020DB733: {NameCN: "超凡技艺", NameEN: "Unbound Technique", MaxLevel: 15},
	0x3F682593: {NameCN: "超凡奥秘", NameEN: "Unbound Exertion", MaxLevel: 15},
	0x79027FC8: {NameCN: "超凡破限", NameEN: "Unbound Master", MaxLevel: 55},

	// Observed in a GAME 2.0.3 live weapon record. The unpacked
	// skill/weapon tables still use 020DB733, so retain this as an explicit
	// runtime alias instead of replacing the canonical table hash.
	0x02D0B733: {NameCN: "超凡技艺", NameEN: "Unbound Technique", MaxLevel: 15},
}

// The runtime spelling differs by one nibble from the unpacked-table hash.
// Treat the pair as search aliases while preserving the exact value selected
// by the selection: the editor must never silently rewrite one observed hash into
// the other.
var weaponMemoryTraitAliases = map[uint32][]uint32{
	0x020DB733: {0x02D0B733},
	0x02D0B733: {0x020DB733},
}

var weaponMemoryUnusedTraitHashes = map[uint32]struct{}{
	0x7279E478: {}, // SKILL_002_00: blank/unused DEF row in the game tables.
}

type WeaponMemoryOption struct {
	Hash          uint32   `json:"hash"`
	DisplayName   string   `json:"displayName"`
	MaxLevel      *int     `json:"maxLevel,omitempty"`
	AllowedLevels []int    `json:"allowedLevels,omitempty"`
	SearchTerms   []string `json:"searchTerms"`
}

type WeaponMemoryOptions struct {
	Traits []WeaponMemoryOption `json:"traits"`
}

type WeaponMemorySkillStatus struct {
	Index int    `json:"index"`
	Hash  uint32 `json:"hash"`
	Name  string `json:"name"`
	Level uint32 `json:"level"`
}

type WeaponMemoryStatus struct {
	OwnerToken    string                    `json:"ownerToken,omitempty"`
	Found         bool                      `json:"found"`
	Hooked        bool                      `json:"hooked"`
	Address       uint64                    `json:"address"`
	RVA           uint64                    `json:"rva"`
	SelectedAddr  uint64                    `json:"selectedAddr"`
	SaveRVA       uint64                    `json:"saveRva"`
	CurrentBytes  string                    `json:"currentBytes"`
	CaptureSource string                    `json:"captureSource"`
	SourceVersion string                    `json:"sourceVersion"`
	WeaponID      uint32                    `json:"weaponId"`
	WeaponSlot    int32                     `json:"weaponSlot"`
	WeaponLevel   uint32                    `json:"weaponLevel"`
	Skills        []WeaponMemorySkillStatus `json:"skills"`
}

func (a *App) WeaponMemoryGetOptions() (WeaponMemoryOptions, error) {
	catalog, err := LoadCatalog()
	if err != nil {
		return WeaponMemoryOptions{}, err
	}
	result := WeaponMemoryOptions{Traits: make([]WeaponMemoryOption, 0, len(catalog.Traits))}
	seen := make(map[uint32]struct{}, len(catalog.Traits))
	for index := range catalog.Traits {
		trait := &catalog.Traits[index]
		hash, err := ParseHashHex(trait.Hash)
		if err != nil || isEmptyWeaponMemorySkill(hash) {
			continue
		}
		if _, exists := seen[hash]; exists {
			continue
		}
		seen[hash] = struct{}{}
		name := weaponMemorySkillDisplayName(catalog, hash)
		result.Traits = append(result.Traits, WeaponMemoryOption{
			Hash: hash, DisplayName: name, SearchTerms: weaponMemoryOptionSearchTerms(hash, name),
			MaxLevel: trait.MaxLevel, AllowedLevels: append([]int(nil), trait.AllowedLevels...),
		})
	}
	// The weapon table contains legitimate weapon-exclusive skills that do not
	// belong in Catalog.Traits.  Union those rows into the live weapon editor so
	// a value that can be read from a weapon can also be selected and named.
	data, err := loadLoadoutWeaponStats()
	if err != nil {
		return WeaponMemoryOptions{}, err
	}
	for hashText := range data.TraitIDs {
		hash, parseErr := ParseHashHex(hashText)
		if parseErr != nil || isEmptyWeaponMemorySkill(hash) {
			continue
		}
		if _, unused := weaponMemoryUnusedTraitHashes[hash]; unused {
			continue
		}
		if _, exists := seen[hash]; exists {
			continue
		}
		name := weaponMemorySkillDisplayName(catalog, hash)
		if name == "" {
			continue
		}
		seen[hash] = struct{}{}
		option := WeaponMemoryOption{Hash: hash, DisplayName: name, SearchTerms: weaponMemoryOptionSearchTerms(hash, name)}
		if verified, ok := weaponMemoryVerifiedSkills[hash]; ok && verified.MaxLevel > 0 {
			maxLevel := verified.MaxLevel
			option.MaxLevel = &maxLevel
		}
		result.Traits = append(result.Traits, option)
	}
	// Runtime aliases are evidence-backed read values, not rows in the unpacked
	// table, so add them after the table union.
	for hash, verified := range weaponMemoryVerifiedSkills {
		if _, exists := seen[hash]; exists {
			continue
		}
		seen[hash] = struct{}{}
		maxLevel := verified.MaxLevel
		result.Traits = append(result.Traits, WeaponMemoryOption{
			Hash: hash, DisplayName: weaponMemorySkillDisplayName(catalog, hash), MaxLevel: &maxLevel,
			SearchTerms: weaponMemoryOptionSearchTerms(hash, weaponMemorySkillDisplayName(catalog, hash)),
		})
	}
	sort.SliceStable(result.Traits, func(i, j int) bool {
		if result.Traits[i].DisplayName != result.Traits[j].DisplayName {
			return result.Traits[i].DisplayName < result.Traits[j].DisplayName
		}
		return result.Traits[i].Hash < result.Traits[j].Hash
	})
	return result, nil
}

func weaponMemoryOptionSearchTerms(hash uint32, displayName string) []string {
	values := []string{
		displayName,
		fmt.Sprintf("%08X", hash),
		fmt.Sprintf("0x%08X", hash),
		fmt.Sprintf("%d", hash),
	}
	for _, alias := range weaponMemoryTraitAliases[hash] {
		values = append(values,
			fmt.Sprintf("%08X", alias),
			fmt.Sprintf("0x%08X", alias),
			fmt.Sprintf("%d", alias),
		)
	}
	return values
}

func weaponMemorySkillDisplayName(catalog *Catalog, hash uint32) string {
	if hash == 0 || hash == EmptyHash {
		return ""
	}
	if verified, ok := weaponMemoryVerifiedSkills[hash]; ok {
		if useChinese() {
			return verified.NameCN
		}
		return verified.NameEN
	}
	if catalog != nil && catalog.LookupTraitByHash(hash) != nil {
		return loadoutTraitDisplayName(catalog, hash)
	}
	return localizedRuntimeName(hash)
}

func (a *App) WeaponMemoryScan() (WeaponMemoryStatus, error) {
	if err := a.acquireGameProcessLease(); err != nil {
		return WeaponMemoryStatus{}, err
	}
	defer a.procMu.Unlock()
	weaponMemoryLifecycleMu.Lock()
	defer weaponMemoryLifecycleMu.Unlock()
	return a.scanWeaponMemoryLocked()
}

func (a *App) scanWeaponMemoryLocked() (WeaponMemoryStatus, error) {
	if a.hProcess == 0 || a.moduleBase == 0 {
		return WeaponMemoryStatus{}, fmt.Errorf("未连接游戏进程")
	}
	hookRVA, _, _ := weaponMemoryRuntimeIdentity(a.runtimePatchVerifiedDigest)
	addr, ok := checkedRuntimePatchMonitorAddress(a.moduleBase, hookRVA)
	if !ok {
		return WeaponMemoryStatus{}, fmt.Errorf("武器实时编辑入口地址溢出")
	}
	guard := make([]byte, len(weaponMemoryGuardBytes))
	if err := readProcessMemory(a.hProcess, addr, unsafe.Pointer(&guard[0]), uintptr(len(guard))); err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("读取武器焦点指令失败: %w", err)
	}
	original := isWeaponMemoryGuard(guard, false)
	hooked := isWeaponMemoryGuard(guard, true)
	if !original && !hooked {
		return WeaponMemoryStatus{}, fmt.Errorf("武器焦点指令字节异常 (%s)：此入口只支持经过完整守卫的 GAME 2.0.3 / 2.0.4", bytesToHex(guard))
	}
	a.weaponMemoryHookAddr = addr
	if original {
		a.weaponMemoryOriginal = append(a.weaponMemoryOriginal[:0], guard[:weaponMemoryHookSize]...)
		a.weaponMemoryCaveAddr = 0
	} else {
		cave := relJumpTarget(addr, guard[:weaponMemoryHookSize])
		recovered, err := a.recoverWeaponMemoryHookLocked(cave)
		if err != nil {
			a.weaponMemoryHookAddr = 0
			return WeaponMemoryStatus{}, fmt.Errorf("武器读取 Hook 无法接管: %w", err)
		}
		a.weaponMemoryCaveAddr = cave
		a.weaponMemoryOriginal = recovered
	}
	return a.readWeaponMemoryStatusLocked()
}

func (a *App) WeaponMemoryGetStatus() (WeaponMemoryStatus, error) {
	if err := a.acquireGameProcessLease(); err != nil {
		return WeaponMemoryStatus{}, err
	}
	defer a.procMu.Unlock()
	weaponMemoryLifecycleMu.Lock()
	defer weaponMemoryLifecycleMu.Unlock()
	if a.weaponMemoryHookAddr == 0 {
		return a.scanWeaponMemoryLocked()
	}
	return a.readWeaponMemoryStatusLocked()
}

func (a *App) WeaponMemoryEnable() (WeaponMemoryStatus, error) {
	if err := a.acquireLegacyRuntimeMutationLease(runtimeOwnerWeapon); err != nil {
		return WeaponMemoryStatus{}, err
	}
	defer a.procMu.Unlock()
	weaponMemoryLifecycleMu.Lock()
	defer weaponMemoryLifecycleMu.Unlock()
	status, err := a.weaponMemoryEnableLocked()
	if err == nil {
		a.weaponMemoryOwnerToken = ""
	}
	return status, err
}

func (a *App) WeaponMemoryAcquire(requestID uint64) (WeaponMemoryStatus, error) {
	if err := a.acquireOwnedGameProcessLease(requestID); err != nil {
		return WeaponMemoryStatus{}, err
	}
	defer a.procMu.Unlock()
	weaponMemoryLifecycleMu.Lock()
	defer weaponMemoryLifecycleMu.Unlock()
	status, err := a.weaponMemoryEnableLocked()
	if err != nil {
		return WeaponMemoryStatus{}, err
	}
	token := a.nextRuntimeOwnerToken("weapon")
	a.charaOwnerToken = ""
	a.weaponMemoryOwnerToken = token
	status.OwnerToken = token
	return status, nil
}

func (a *App) resolveWeaponSaveFunction203Locked() (uintptr, error) {
	_, saveRVA, _ := weaponMemoryRuntimeIdentity(a.runtimePatchVerifiedDigest)
	addr, ok := checkedRuntimePatchMonitorAddress(a.moduleBase, saveRVA)
	if !ok {
		return 0, fmt.Errorf("武器保存函数地址溢出")
	}
	actual := make([]byte, len(gameSaveFunctionPrologue))
	if err := readProcessMemory(a.hProcess, addr, unsafe.Pointer(&actual[0]), uintptr(len(actual))); err != nil {
		return 0, fmt.Errorf("读取 GAME 2.0.3 / 2.0.4 武器保存函数失败: %w", err)
	}
	if !bytes.Equal(actual, gameSaveFunctionPrologue) {
		return 0, fmt.Errorf("GAME 2.0.3 / 2.0.4 武器保存函数签名不匹配: %s", bytesToHex(actual))
	}
	if err := a.validateRemoteFunctionStart(addr, "GAME 2.0.3 / 2.0.4 武器保存函数"); err != nil {
		return 0, err
	}
	a.itemSaveFunctionAddr = addr
	return addr, nil
}

func (a *App) weaponMemoryEnableLocked() (WeaponMemoryStatus, error) {
	var status WeaponMemoryStatus
	var err error
	if a.weaponMemoryHookAddr == 0 {
		status, err = a.scanWeaponMemoryLocked()
	} else {
		status, err = a.readWeaponMemoryStatusLocked()
	}
	if err != nil || status.Hooked {
		return status, err
	}
	if _, err := a.resolveWeaponSaveFunction203Locked(); err != nil {
		return WeaponMemoryStatus{}, err
	}
	original := make([]byte, weaponMemoryHookSize)
	if err := readProcessMemory(a.hProcess, a.weaponMemoryHookAddr, unsafe.Pointer(&original[0]), uintptr(len(original))); err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("读取武器焦点原始指令失败: %w", err)
	}
	if !isWeaponMemoryOriginal(original) {
		return WeaponMemoryStatus{}, fmt.Errorf("武器焦点原始指令已变化: %s", bytesToHex(original))
	}
	cave, err := virtualAllocRemoteNear(a.hProcess, a.weaponMemoryHookAddr, 0x1000)
	if err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("分配武器读取代码洞失败: %w", err)
	}
	code, err := buildWeaponMemoryCave(cave, a.weaponMemoryHookAddr+weaponMemoryHookSize, original)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return WeaponMemoryStatus{}, err
	}
	if err := writeCodeMemory(a.hProcess, cave, code); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return WeaponMemoryStatus{}, fmt.Errorf("写入武器读取代码洞失败: %w", err)
	}
	patch, err := makeRelJump(a.weaponMemoryHookAddr, cave, int(weaponMemoryHookSize))
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return WeaponMemoryStatus{}, err
	}
	installResult, err := installRemoteCodeHook(a.hProcess, a.weaponMemoryHookAddr, original, patch)
	if err != nil {
		return WeaponMemoryStatus{}, runtimeHookInstallFailure(
			"武器读取 Hook", installResult, err,
			func() { _ = virtualFreeRemote(a.hProcess, cave) },
			func() { a.retireRuntimeCaveLocked(cave, "weapon-memory install rollback") },
			func() {
				a.weaponMemoryCaveAddr = cave
				a.weaponMemoryOriginal = append(a.weaponMemoryOriginal[:0], original...)
			},
			a.poisonCurrentLiveMemoryWrites,
		)
	}
	a.weaponMemoryCaveAddr = cave
	a.weaponMemoryOriginal = append(a.weaponMemoryOriginal[:0], original...)
	return finalizeRuntimeHookEnable(
		"武器读取 Hook", a.readWeaponMemoryStatusLocked, a.releaseWeaponMemoryHookLocked, a.poisonCurrentLiveMemoryWrites,
	)
}

func (a *App) WeaponMemoryUpdate(update WeaponMemoryUpdate) (WeaponMemoryStatus, error) {
	return a.weaponMemoryUpdate("", false, update)
}

func (a *App) WeaponMemoryUpdateOwned(token string, update WeaponMemoryUpdate) (WeaponMemoryStatus, error) {
	return a.weaponMemoryUpdate(token, true, update)
}

func (a *App) weaponMemoryUpdate(token string, owned bool, update WeaponMemoryUpdate) (WeaponMemoryStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	weaponMemoryWriteMu.Lock()
	defer weaponMemoryWriteMu.Unlock()
	var leaseErr error
	if owned {
		leaseErr = a.acquireOwnedRuntimeWriteLease(runtimeOwnerWeapon, token)
	} else {
		leaseErr = a.acquireLegacyRuntimeMutationLease(runtimeOwnerWeapon)
	}
	if leaseErr != nil {
		return WeaponMemoryStatus{}, leaseErr
	}
	defer a.procMu.Unlock()
	if err := a.ensureLiveMemoryWritesSafe(); err != nil {
		return WeaponMemoryStatus{}, err
	}
	weaponMemoryLifecycleMu.Lock()
	defer weaponMemoryLifecycleMu.Unlock()
	if err := validateWeaponMemoryUpdate(update); err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("武器技能写入参数无效: %w", err)
	}
	status, err := a.readWeaponMemoryStatusLocked()
	if err != nil {
		return WeaponMemoryStatus{}, err
	}
	if !status.Hooked || status.SelectedAddr == 0 {
		return WeaponMemoryStatus{}, fmt.Errorf("请先开启读取，并在游戏内武器列表中选中一把武器")
	}
	if a.weaponMemoryCaveAddr == 0 {
		return WeaponMemoryStatus{}, fmt.Errorf("武器读取代码洞尚未就绪")
	}
	var selected uintptr
	if err := readProcessMemory(a.hProcess, a.weaponMemoryCaveAddr+weaponMemoryCaveDataOffset, unsafe.Pointer(&selected), unsafe.Sizeof(selected)); err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("写入前复核选中武器指针失败: %w", err)
	}
	base, err := validateWeaponMemorySelection(uintptr(update.ExpectedSelectedAddr), uintptr(status.SelectedAddr), selected)
	if err != nil {
		return WeaponMemoryStatus{}, err
	}
	windowAddr := base + weaponMemorySkillWindowOffset
	original := make([]byte, weaponMemorySkillWindowSize)
	if err := readProcessMemory(a.hProcess, windowAddr, unsafe.Pointer(&original[0]), uintptr(len(original))); err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("读取武器技能原记录失败: %w", err)
	}
	desired, err := encodeWeaponMemorySkillWindow(original, update)
	if err != nil {
		return WeaponMemoryStatus{}, err
	}
	if err := snapshotBeforeLiveSaveChange("游戏内武器技能写入前自动备份"); err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("自动备份失败，已取消写入: %w", err)
	}
	confirmedStatus, err := a.readWeaponMemoryStatusLocked()
	if err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("自动备份后复核武器状态失败: %w", err)
	}
	var confirmedSelected uintptr
	if err := readProcessMemory(a.hProcess, a.weaponMemoryCaveAddr+weaponMemoryCaveDataOffset, unsafe.Pointer(&confirmedSelected), unsafe.Sizeof(confirmedSelected)); err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("自动备份后复核武器指针失败: %w", err)
	}
	confirmed := make([]byte, weaponMemorySkillWindowSize)
	if err := readProcessMemory(a.hProcess, windowAddr, unsafe.Pointer(&confirmed[0]), uintptr(len(confirmed))); err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("自动备份后复核武器技能失败: %w", err)
	}
	if err := validateWeaponMemorySnapshot(base, uintptr(confirmedStatus.SelectedAddr), confirmedSelected, original, confirmed); err != nil {
		return WeaponMemoryStatus{}, err
	}
	writer := func(window []byte) error {
		if len(window) != weaponMemorySkillWindowSize {
			return fmt.Errorf("武器技能窗口长度异常: %d", len(window))
		}
		return writeProcessMemory(a.hProcess, windowAddr, unsafe.Pointer(&window[0]), uintptr(len(window)))
	}
	reader := func() ([]byte, error) {
		window := make([]byte, weaponMemorySkillWindowSize)
		if err := readProcessMemory(a.hProcess, windowAddr, unsafe.Pointer(&window[0]), uintptr(len(window))); err != nil {
			return nil, err
		}
		return window, nil
	}
	if err := writeWeaponMemorySkillWindowAtomic(original, desired, writer, func() error { return a.saveWeaponMemorySkills(base) }, reader); err != nil {
		if isRemoteCallIndeterminate(err) || errors.Is(err, errLiveMemoryRollbackUnproven) {
			a.poisonCurrentLiveMemoryWrites()
			_ = a.clearWeaponMemorySelectionLocked()
		}
		return WeaponMemoryStatus{}, fmt.Errorf("武器技能原子写入失败: %w", err)
	}
	result, err := a.readWeaponMemoryStatusLocked()
	if err != nil {
		return WeaponMemoryStatus{}, err
	}
	if err := a.clearWeaponMemorySelectionLocked(); err != nil {
		return WeaponMemoryStatus{}, err
	}
	result.SelectedAddr = 0
	return result, nil
}

func (a *App) saveWeaponMemorySkills(base uintptr) error {
	fn, err := a.resolveWeaponSaveFunction203Locked()
	if err != nil {
		return err
	}
	for offset := uintptr(0); offset < weaponMemorySkillWindowSize; offset += 4 {
		field := base + weaponMemorySkillWindowOffset + offset
		if err := a.callRemoteOneArg(fn, field); err != nil {
			return fmt.Errorf("保存武器技能字段 +0x%02X 失败: %w", weaponMemorySkillWindowOffset+offset, err)
		}
	}
	return nil
}

func (a *App) WeaponMemoryRelease(token string) (WeaponMemoryStatus, error) {
	a.procMu.Lock()
	if !runtimeOwnerTokenMatches(a.weaponMemoryOwnerToken, token) {
		a.procMu.Unlock()
		return WeaponMemoryStatus{}, nil
	}
	idle := a.weaponMemoryHookAddr == 0 && a.weaponMemoryCaveAddr == 0 && len(a.weaponMemoryOriginal) == 0
	if idle {
		a.weaponMemoryOwnerToken = ""
		a.procMu.Unlock()
		return WeaponMemoryStatus{}, nil
	}
	a.procMu.Unlock()
	if err := a.acquireGameProcessLease(); err != nil {
		return WeaponMemoryStatus{}, err
	}
	defer a.procMu.Unlock()
	weaponMemoryLifecycleMu.Lock()
	defer weaponMemoryLifecycleMu.Unlock()
	if !runtimeOwnerTokenMatches(a.weaponMemoryOwnerToken, token) {
		return WeaponMemoryStatus{}, nil
	}
	if err := a.releaseWeaponMemoryHookLocked(); err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("关闭武器读取失败: %w", err)
	}
	a.weaponMemoryOwnerToken = ""
	return WeaponMemoryStatus{}, nil
}

func (a *App) WeaponMemoryDisable() (WeaponMemoryStatus, error) {
	a.procMu.Lock()
	if a.weaponMemoryOwnerToken != "" {
		a.procMu.Unlock()
		return WeaponMemoryStatus{}, errRuntimeOwnerLeaseStale
	}
	idle := a.weaponMemoryHookAddr == 0 && a.weaponMemoryCaveAddr == 0 && len(a.weaponMemoryOriginal) == 0
	a.procMu.Unlock()
	if idle {
		return WeaponMemoryStatus{}, nil
	}
	if err := a.acquireLegacyRuntimeMutationLease(runtimeOwnerWeapon); err != nil {
		return WeaponMemoryStatus{}, err
	}
	defer a.procMu.Unlock()
	weaponMemoryLifecycleMu.Lock()
	defer weaponMemoryLifecycleMu.Unlock()
	if err := a.releaseWeaponMemoryHookLocked(); err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("关闭武器读取失败: %w", err)
	}
	a.weaponMemoryOwnerToken = ""
	return WeaponMemoryStatus{}, nil
}

func (a *App) readWeaponMemoryStatusLocked() (WeaponMemoryStatus, error) {
	if a.hProcess == 0 || a.weaponMemoryHookAddr == 0 {
		return WeaponMemoryStatus{}, fmt.Errorf("未定位武器焦点指令")
	}
	current := make([]byte, weaponMemoryHookSize)
	if err := readProcessMemory(a.hProcess, a.weaponMemoryHookAddr, unsafe.Pointer(&current[0]), uintptr(len(current))); err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("读取武器 Hook 指令失败: %w", err)
	}
	hooked := isWeaponMemoryJump(current)
	if !hooked && !isWeaponMemoryOriginal(current) {
		return WeaponMemoryStatus{}, fmt.Errorf("武器焦点指令字节异常: %s", bytesToHex(current))
	}
	status := newWeaponMemoryStatus(true, hooked, a.weaponMemoryHookAddr, a.moduleBase, current, a.runtimePatchVerifiedDigest)
	if !hooked {
		return status, nil
	}
	cave := relJumpTarget(a.weaponMemoryHookAddr, current)
	if cave == 0 {
		return WeaponMemoryStatus{}, fmt.Errorf("武器读取 Hook 跳转目标为空")
	}
	if a.weaponMemoryCaveAddr != 0 && a.weaponMemoryCaveAddr != cave {
		return WeaponMemoryStatus{}, fmt.Errorf("武器读取 Hook 跳转目标已从 0x%X 变为 0x%X", a.weaponMemoryCaveAddr, cave)
	}
	original, err := a.recoverWeaponMemoryHookLocked(cave)
	if err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("校验武器读取 Hook 失败: %w", err)
	}
	if len(a.weaponMemoryOriginal) == int(weaponMemoryHookSize) && !bytes.Equal(a.weaponMemoryOriginal, original) {
		return WeaponMemoryStatus{}, fmt.Errorf("武器读取 Hook 保存的原始指令已变化")
	}
	a.weaponMemoryCaveAddr = cave
	a.weaponMemoryOriginal = original
	var selected uintptr
	if err := readProcessMemory(a.hProcess, cave+weaponMemoryCaveDataOffset, unsafe.Pointer(&selected), unsafe.Sizeof(selected)); err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("读取选中武器指针失败: %w", err)
	}
	status.SelectedAddr = uint64(selected)
	if selected == 0 {
		return status, nil
	}
	record := make([]byte, weaponMemoryRecordSize)
	if err := readProcessMemory(a.hProcess, selected, unsafe.Pointer(&record[0]), uintptr(len(record))); err != nil {
		return WeaponMemoryStatus{}, fmt.Errorf("读取选中武器数据失败: %w", err)
	}
	status.WeaponID = binary.LittleEndian.Uint32(record[0x04:0x08])
	status.WeaponSlot = int32(binary.LittleEndian.Uint32(record[0x00:0x04]))
	status.WeaponLevel = binary.LittleEndian.Uint32(record[0x58:0x5C])
	status.Skills = decodeWeaponMemorySkills(record[weaponMemorySkillWindowOffset : weaponMemorySkillWindowOffset+weaponMemorySkillWindowSize])
	return status, nil
}

func decodeWeaponMemorySkills(window []byte) []WeaponMemorySkillStatus {
	result := make([]WeaponMemorySkillStatus, 0, weaponMemoryPhysicalSlotCount)
	var catalog *Catalog
	if loaded, err := LoadCatalog(); err == nil {
		catalog = loaded
	}
	for index := 0; index < weaponMemoryPhysicalSlotCount; index++ {
		offset := index * 8
		hash := binary.LittleEndian.Uint32(window[offset : offset+4])
		name := ""
		if !isEmptyWeaponMemorySkill(hash) {
			name = weaponMemorySkillDisplayName(catalog, hash)
		}
		if name == "" {
			if isEmptyWeaponMemorySkill(hash) {
				if useChinese() {
					name = "空槽"
				} else {
					name = "Empty"
				}
			} else if localized := localizedRuntimeName(hash); localized != "" {
				name = localized
			} else {
				name = fmt.Sprintf("0x%08X", hash)
			}
		}
		result = append(result, WeaponMemorySkillStatus{
			Index: index, Hash: hash, Name: name,
			Level: binary.LittleEndian.Uint32(window[offset+4 : offset+8]),
		})
	}
	return result
}

func newWeaponMemoryStatus(found, hooked bool, hookAddr, moduleBase uintptr, current []byte, digest string) WeaponMemoryStatus {
	_, saveRVA, sourceVersion := weaponMemoryRuntimeIdentity(digest)
	return WeaponMemoryStatus{
		Found: found, Hooked: hooked, Address: uint64(hookAddr), RVA: uint64(hookAddr - moduleBase),
		SaveRVA: uint64(saveRVA), CurrentBytes: bytesToHex(current),
		CaptureSource: "game-" + sourceVersion + "-current-selected-weapon", SourceVersion: sourceVersion,
		Skills: make([]WeaponMemorySkillStatus, 0, weaponMemoryPhysicalSlotCount),
	}
}

func isWeaponMemoryOriginal(buf []byte) bool {
	return len(buf) >= int(weaponMemoryHookSize) && bytes.Equal(buf[:weaponMemoryHookSize], weaponMemoryOriginalBytes)
}

func isWeaponMemoryGuard(buf []byte, hooked bool) bool {
	if len(buf) < len(weaponMemoryGuardBytes) {
		return false
	}
	if !hooked {
		return bytes.Equal(buf[:len(weaponMemoryGuardBytes)], weaponMemoryGuardBytes)
	}
	return isWeaponMemoryJump(buf[:weaponMemoryHookSize]) && bytes.Equal(buf[weaponMemoryHookSize:len(weaponMemoryGuardBytes)], weaponMemoryGuardBytes[weaponMemoryHookSize:])
}

func isWeaponMemoryJump(buf []byte) bool {
	if len(buf) < int(weaponMemoryHookSize) || buf[0] != 0xE9 {
		return false
	}
	for index := 5; index < int(weaponMemoryHookSize); index++ {
		if buf[index] != 0x90 {
			return false
		}
	}
	return true
}

func buildWeaponMemoryCave(cave, returnAddr uintptr, original []byte) ([]byte, error) {
	if len(original) != int(weaponMemoryHookSize) || !isWeaponMemoryOriginal(original) {
		return nil, fmt.Errorf("武器焦点原始指令长度或签名异常")
	}
	code := make([]byte, 0, weaponMemoryCaveDataOffset+8)
	code = append(code, 0x41, 0x52)
	code = append(code, 0x49, 0xBA)
	code = binary.LittleEndian.AppendUint64(code, uint64(cave+weaponMemoryCaveDataOffset))
	code = append(code, 0x49, 0x89, 0x12)
	code = append(code, 0x41, 0x5A)
	code = append(code, original...)
	jmp, err := makeRelJump(cave+uintptr(len(code)), returnAddr, 5)
	if err != nil {
		return nil, err
	}
	code = append(code, jmp...)
	for len(code) < int(weaponMemoryMarkerOffset) {
		code = append(code, 0)
	}
	code = append(code, weaponMemoryMarker...)
	for len(code) < int(weaponMemoryCaveDataOffset)+8 {
		code = append(code, 0)
	}
	return code, nil
}

func decodeWeaponMemoryCave(cave uintptr, code []byte) ([]byte, error) {
	minimum := int(weaponMemoryMarkerOffset) + len(weaponMemoryMarker)
	if cave == 0 || len(code) < minimum {
		return nil, fmt.Errorf("武器代码洞长度不足")
	}
	if !bytes.Equal(code[0:4], []byte{0x41, 0x52, 0x49, 0xBA}) || !bytes.Equal(code[12:17], []byte{0x49, 0x89, 0x12, 0x41, 0x5A}) {
		return nil, fmt.Errorf("武器代码洞寄存器保护签名不匹配")
	}
	if uintptr(binary.LittleEndian.Uint64(code[4:12])) != cave+weaponMemoryCaveDataOffset {
		return nil, fmt.Errorf("武器代码洞数据地址不匹配")
	}
	if !bytes.Equal(code[weaponMemoryMarkerOffset:weaponMemoryMarkerOffset+uintptr(len(weaponMemoryMarker))], weaponMemoryMarker) {
		return nil, fmt.Errorf("武器代码洞拥有权标记不匹配")
	}
	original := append([]byte(nil), code[weaponMemoryOriginalOffset:weaponMemoryOriginalOffset+weaponMemoryHookSize]...)
	if !isWeaponMemoryOriginal(original) {
		return nil, fmt.Errorf("武器代码洞中的原始指令不匹配: %s", bytesToHex(original))
	}
	jumpOffset := weaponMemoryOriginalOffset + weaponMemoryHookSize
	if len(code) < int(jumpOffset)+5 || code[jumpOffset] != 0xE9 {
		return nil, fmt.Errorf("武器代码洞回跳签名不匹配")
	}
	return original, nil
}

func (a *App) recoverWeaponMemoryHookLocked(cave uintptr) ([]byte, error) {
	if cave == 0 {
		return nil, fmt.Errorf("武器代码洞地址为空")
	}
	code := make([]byte, weaponMemoryCaveDataOffset)
	if err := readProcessMemory(a.hProcess, cave, unsafe.Pointer(&code[0]), uintptr(len(code))); err != nil {
		return nil, fmt.Errorf("读取武器代码洞失败: %w", err)
	}
	original, err := decodeWeaponMemoryCave(cave, code)
	if err != nil {
		return nil, err
	}
	jumpOffset := weaponMemoryOriginalOffset + weaponMemoryHookSize
	if target := relJumpTarget(cave+jumpOffset, code[jumpOffset:jumpOffset+5]); target != a.weaponMemoryHookAddr+weaponMemoryHookSize {
		return nil, fmt.Errorf("武器代码洞回跳地址不匹配")
	}
	return original, nil
}

func (a *App) clearWeaponMemorySelectionLocked() error {
	if a.hProcess == 0 || a.weaponMemoryCaveAddr == 0 {
		return nil
	}
	var zero uintptr
	if err := writeProcessMemory(a.hProcess, a.weaponMemoryCaveAddr+weaponMemoryCaveDataOffset, unsafe.Pointer(&zero), unsafe.Sizeof(zero)); err != nil {
		return fmt.Errorf("清空旧的选中武器指针失败: %w", err)
	}
	return nil
}

func (a *App) releaseWeaponMemoryHook() error {
	weaponMemoryLifecycleMu.Lock()
	defer weaponMemoryLifecycleMu.Unlock()
	return a.releaseWeaponMemoryHookLocked()
}

func (a *App) releaseWeaponMemoryHookLocked() error {
	if a.weaponMemoryHookAddr == 0 {
		if a.weaponMemoryCaveAddr != 0 || len(a.weaponMemoryOriginal) != 0 {
			return fmt.Errorf("武器 Hook 入口未知，但仍保留代码洞或原始指令恢复状态")
		}
		return nil
	}
	if a.hProcess == 0 {
		return fmt.Errorf("缺少游戏进程句柄，无法恢复武器 Hook")
	}
	current := make([]byte, weaponMemoryHookSize)
	if err := readProcessMemory(a.hProcess, a.weaponMemoryHookAddr, unsafe.Pointer(&current[0]), uintptr(len(current))); err != nil {
		return err
	}
	if isWeaponMemoryOriginal(current) {
		a.weaponMemoryHookAddr = 0
		a.weaponMemoryCaveAddr = 0
		a.weaponMemoryOriginal = nil
		return nil
	}
	if !isWeaponMemoryJump(current) {
		return fmt.Errorf("武器 Hook 入口已被其他代码修改: %s", bytesToHex(current))
	}
	cave := relJumpTarget(a.weaponMemoryHookAddr, current)
	if a.weaponMemoryCaveAddr != 0 && cave != a.weaponMemoryCaveAddr {
		return fmt.Errorf("武器 Hook 跳转目标已被替换，拒绝覆盖外部 Hook")
	}
	original, err := a.recoverWeaponMemoryHookLocked(cave)
	if err != nil {
		return err
	}
	if len(a.weaponMemoryOriginal) == int(weaponMemoryHookSize) && !bytes.Equal(a.weaponMemoryOriginal, original) {
		return fmt.Errorf("武器 Hook 原始指令缓存与代码洞不一致，拒绝恢复")
	}
	_ = a.clearWeaponMemorySelectionLocked()
	if err := writeCodeMemory(a.hProcess, a.weaponMemoryHookAddr, original); err != nil {
		return fmt.Errorf("恢复武器焦点原始指令失败: %w", err)
	}
	restored := make([]byte, weaponMemoryHookSize)
	if err := readProcessMemory(a.hProcess, a.weaponMemoryHookAddr, unsafe.Pointer(&restored[0]), uintptr(len(restored))); err != nil {
		return fmt.Errorf("恢复武器焦点原始指令后回读失败: %w", err)
	}
	if !bytes.Equal(restored, original) {
		return fmt.Errorf("恢复武器焦点原始指令后回读不一致: %s", bytesToHex(restored))
	}
	// Do not free a cave that a game thread may still be executing. The OS will
	// reclaim it with the process, matching the other live item editors.
	a.weaponMemoryHookAddr = 0
	a.weaponMemoryCaveAddr = 0
	a.weaponMemoryOriginal = nil
	return nil
}
