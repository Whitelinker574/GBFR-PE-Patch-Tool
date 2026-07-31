package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	virtualSigilDisabledFlag              uint32 = 0x10
	stableReleaseVirtualSigilWriteEnabled        = true
)

var virtualSigilModMu sync.Mutex

type VirtualSigilSelection struct {
	SlotID      uint32 `json:"slotId"`
	GemID       uint32 `json:"gemId"`
	Trait1      uint32 `json:"trait1"`
	Trait1Level int    `json:"trait1Level"`
	Trait2      uint32 `json:"trait2"`
	Trait2Level int    `json:"trait2Level"`
	SigilLevel  int    `json:"sigilLevel"`
}

type VirtualSigilPreset struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	CharacterHash string                  `json:"characterHash"`
	Slots         []VirtualSigilSelection `json:"slots"`
}

type VirtualSigilConfig struct {
	SchemaVersion int                                `json:"schemaVersion"`
	SlotCount     int                                `json:"slotCount"`
	Characters    map[string][]VirtualSigilSelection `json:"characters"`
	Presets       []VirtualSigilPreset               `json:"presets,omitempty"`
}

type VirtualSigilCharacter struct {
	Hash   string `json:"hash"`
	NameZh string `json:"nameZh"`
	NameEn string `json:"nameEn"`
}

type VirtualSigilInventoryItem struct {
	VirtualSigilSelection
	Hash                   string `json:"hash"`
	Name                   string `json:"name"`
	PrimaryTraitHash       string `json:"primaryTraitHash"`
	PrimaryTraitName       string `json:"primaryTraitName"`
	PrimaryTraitMaxLevel   int    `json:"primaryTraitMaxLevel"`
	SecondaryTraitHash     string `json:"secondaryTraitHash,omitempty"`
	SecondaryTraitName     string `json:"secondaryTraitName,omitempty"`
	SecondaryTraitMaxLevel int    `json:"secondaryTraitMaxLevel,omitempty"`
}

type VirtualSigilWorkspace struct {
	SavePath          string                      `json:"savePath"`
	Installed         bool                        `json:"installed"`
	Owned             bool                        `json:"owned"`
	RecoveryRequired  bool                        `json:"recoveryRequired"`
	State             string                      `json:"state"`
	Detail            string                      `json:"detail"`
	GameRunning       bool                        `json:"gameRunning"`
	Available         bool                        `json:"available"`
	UnavailableReason string                      `json:"unavailableReason,omitempty"`
	Config            VirtualSigilConfig          `json:"config"`
	Characters        []VirtualSigilCharacter     `json:"characters"`
	Inventory         []VirtualSigilInventoryItem `json:"inventory"`
}

type VirtualSigilDeployRequest struct {
	SavePath string             `json:"savePath"`
	Config   VirtualSigilConfig `json:"config"`
}

type VirtualSigilDeployResult struct {
	Active          bool `json:"active"`
	RestartRequired bool `json:"restartRequired"`
	RefreshRequired bool `json:"refreshRequired"`
	ActiveSlots     int  `json:"activeSlots"`
}

var virtualSigilCharacterNames = []VirtualSigilCharacter{
	{Hash: "2A26B1B2", NameZh: "古兰", NameEn: "Gran"}, {Hash: "A4ACBA76", NameZh: "姬塔", NameEn: "Djeeta"},
	{Hash: "18E2F9F9", NameZh: "卡塔莉娜", NameEn: "Katalina"}, {Hash: "079DF0CC", NameZh: "拉卡姆", NameEn: "Rackam"},
	{Hash: "4D0A60C3", NameZh: "伊欧", NameEn: "Io"}, {Hash: "DD7A151E", NameZh: "欧根", NameEn: "Eugen"},
	{Hash: "C8616284", NameZh: "萝赛塔", NameEn: "Rosetta"}, {Hash: "C3FFD418", NameZh: "菲莉", NameEn: "Ferry"},
	{Hash: "22E437E5", NameZh: "兰斯洛特", NameEn: "Lancelot"}, {Hash: "2EBE91D5", NameZh: "巴恩", NameEn: "Vane"},
	{Hash: "BDEF7181", NameZh: "珀西瓦尔", NameEn: "Percival"}, {Hash: "627BCB0D", NameZh: "齐格飞", NameEn: "Siegfried"},
	{Hash: "FD3BE362", NameZh: "夏洛特", NameEn: "Charlotta"}, {Hash: "FC6CDF7B", NameZh: "尤达拉哈", NameEn: "Yodarha"},
	{Hash: "E7053919", NameZh: "娜露梅", NameEn: "Narmaya"}, {Hash: "978E4B18", NameZh: "冈达葛萨", NameEn: "Ghandagoza"},
	{Hash: "0D21B430", NameZh: "泽塔", NameEn: "Zeta"}, {Hash: "F0EB77EF", NameZh: "巴萨拉卡", NameEn: "Vaseraga"},
	{Hash: "AA66178A", NameZh: "卡莉奥丝特罗", NameEn: "Cagliostro"}, {Hash: "A3A3CB2F", NameZh: "伊德", NameEn: "Id"},
	{Hash: "718E1A14", NameZh: "圣德芬", NameEn: "Sandalphon"}, {Hash: "296471BE", NameZh: "希耶提", NameEn: "Seofon"},
	{Hash: "BAD16E3B", NameZh: "索恩", NameEn: "Tweyen"}, {Hash: "1BB37EF0", NameZh: "伽兰查", NameEn: "Gallanza"},
	{Hash: "25D46F4B", NameZh: "玛琪拉菲菈", NameEn: "Maglielle"}, {Hash: "9A8AF295", NameZh: "贝阿朵丽丝", NameEn: "Beatrix"},
	{Hash: "9B15CFB1", NameZh: "尤斯提斯", NameEn: "Eustace"}, {Hash: "646C3168", NameZh: "芙劳", NameEn: "Fraux"},
	{Hash: "74DD4C79", NameZh: "菲迪埃尔", NameEn: "Fediel"},
}

func defaultVirtualSigilConfig() VirtualSigilConfig {
	return VirtualSigilConfig{SchemaVersion: 1, SlotCount: 4, Characters: map[string][]VirtualSigilSelection{}, Presets: []VirtualSigilPreset{}}
}

func virtualSigilJSONPath() (string, error)   { return runtimeCompanionPath("virtual-sigils.json") }
func virtualSigilBinaryPath() (string, error) { return runtimeCompanionPath("virtual-sigils.bin") }

func readVirtualSigilConfig() VirtualSigilConfig {
	value := defaultVirtualSigilConfig()
	path, pathErr := virtualSigilJSONPath()
	if pathErr != nil {
		return value
	}
	data, err := os.ReadFile(path)
	if err == nil {
		var parsed VirtualSigilConfig
		if json.Unmarshal(data, &parsed) == nil && parsed.SchemaVersion == 1 && parsed.SlotCount >= 1 && parsed.SlotCount <= 8 {
			if parsed.Characters == nil {
				parsed.Characters = map[string][]VirtualSigilSelection{}
			}
			if parsed.Presets == nil {
				parsed.Presets = []VirtualSigilPreset{}
			}
			value = parsed
		}
	}
	return value
}

func writeVirtualSigilConfig(path string, config VirtualSigilConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(filepath.Dir(path), ".virtual-sigils-*.json")
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
	return replaceFileAtomic(tempName, path)
}

func loadVirtualSigilInventory(savePath string) ([]VirtualSigilInventoryItem, error) {
	if strings.TrimSpace(savePath) == "" {
		return []VirtualSigilInventoryItem{}, nil
	}
	save, err := LoadSave(savePath)
	if err != nil {
		return nil, fmt.Errorf("读取虚拟因子来源存档失败: %w", err)
	}
	cat, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	index := buildLoadoutIndex(save)
	wornBy := entriesByUnitID(save.findAllUnitsByType(GemWornByIDType))
	flagsByUnit := entriesByUnitID(save.findAllUnitsByType(GemFlagsIDType))
	traitHashByUnit := entriesByUnitID(save.findAllUnitsByType(TraitHashIDType))
	traitLevelByUnit := entriesByUnitID(save.findAllUnitsByType(TraitLevelIDType))
	items := make([]VirtualSigilInventoryItem, 0, len(index.gemBySlotID))
	for slotID, gemUnitID := range index.gemBySlotID {
		worn, wornOK := wornBy[gemUnitID]
		flags, flagsOK := flagsByUnit[gemUnitID]
		if !virtualSigilInventoryStateUsable(worn, wornOK, flags, flagsOK) {
			continue
		}
		gemEntry := index.gemHash[gemUnitID]
		if gemEntry == nil || slotID == 0 {
			continue
		}
		gemID := gemEntry.Uint32()
		primaryHash, primaryLevel, secondaryHash, secondaryLevel := indexedSigilTraits(traitHashByUnit, traitLevelByUnit, gemUnitID)
		if gemID == 0 || gemID == EmptyHash || primaryHash == 0 || primaryHash == EmptyHash || primaryLevel <= 0 {
			continue
		}
		sigilLevel := 0
		if level := index.gemLevel[gemUnitID]; level != nil {
			sigilLevel = int(level.Int32())
		}
		if sigilLevel <= 0 {
			continue
		}
		primary := cat.LookupTraitByHash(primaryHash)
		secondary := cat.LookupTraitByHash(secondaryHash)
		primaryName, secondaryName := loadoutTraitDisplayName(cat, primaryHash), loadoutTraitDisplayName(cat, secondaryHash)
		name := loadoutSigilDisplayNameFromTraits(gemID, primaryName, secondaryName)
		item := VirtualSigilInventoryItem{
			VirtualSigilSelection: VirtualSigilSelection{SlotID: slotID, GemID: gemID, Trait1: primaryHash, Trait1Level: primaryLevel, Trait2: secondaryHash, Trait2Level: secondaryLevel, SigilLevel: sigilLevel},
			Hash:                  fmt.Sprintf("%08X", gemID), Name: name, PrimaryTraitHash: fmt.Sprintf("%08X", primaryHash), PrimaryTraitName: primaryName,
			SecondaryTraitName: secondaryName,
		}
		if primary != nil && primary.MaxLevel != nil {
			item.PrimaryTraitMaxLevel = *primary.MaxLevel
		}
		if secondaryHash != 0 && secondaryHash != EmptyHash {
			item.SecondaryTraitHash = fmt.Sprintf("%08X", secondaryHash)
			if secondary != nil && secondary.MaxLevel != nil {
				item.SecondaryTraitMaxLevel = *secondary.MaxLevel
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].SlotID < items[j].SlotID
	})
	return items, nil
}

func virtualSigilInventoryStateUsable(worn *unitEntry, wornOK bool, flags *unitEntry, flagsOK bool) bool {
	if !wornOK || worn == nil || !flagsOK || flags == nil {
		return false
	}
	wornHash := worn.Uint32()
	return wornHash == EmptyHash && flags.Uint32()&virtualSigilDisabledFlag == 0
}

func normalizeVirtualSigilConfig(config VirtualSigilConfig, inventory []VirtualSigilInventoryItem) (VirtualSigilConfig, int, error) {
	if config.SchemaVersion != 1 {
		return VirtualSigilConfig{}, 0, fmt.Errorf("不支持的虚拟因子配置版本: %d", config.SchemaVersion)
	}
	if config.SlotCount < 1 || config.SlotCount > 8 {
		return VirtualSigilConfig{}, 0, errors.New("虚拟因子槽数量必须在 1 到 8 之间")
	}
	if config.Characters == nil {
		config.Characters = map[string][]VirtualSigilSelection{}
	}
	available := make(map[uint32]VirtualSigilSelection, len(inventory))
	for _, item := range inventory {
		available[item.SlotID] = item.VirtualSigilSelection
	}
	owners := make(map[uint32]string)
	active := 0
	normalized := make(map[string][]VirtualSigilSelection, len(config.Characters))
	for rawHash, rawSlots := range config.Characters {
		hash, err := ParseHashHex(rawHash)
		if err != nil || characterNameByHash[hash] == "" {
			return VirtualSigilConfig{}, 0, fmt.Errorf("虚拟因子角色 hash 无效: %s", rawHash)
		}
		key := fmt.Sprintf("%08X", hash)
		slots := make([]VirtualSigilSelection, config.SlotCount)
		copy(slots, rawSlots)
		for index, selection := range slots {
			if selection.SlotID == 0 {
				slots[index] = VirtualSigilSelection{}
				continue
			}
			expected, exists := available[selection.SlotID]
			if !exists || selection != expected {
				return VirtualSigilConfig{}, 0, fmt.Errorf("角色 %s 的虚拟槽 %d 不再对应来源存档中的同一颗未装备因子", key, index+1)
			}
			if previous := owners[selection.SlotID]; previous != "" {
				return VirtualSigilConfig{}, 0, fmt.Errorf("物理因子 SlotID %d 同时被 %s 与 %s 占用", selection.SlotID, previous, key)
			}
			owners[selection.SlotID] = key
			active++
		}
		normalized[key] = slots
	}
	config.Characters = normalized
	seenPresets := make(map[string]bool)
	for index := range config.Presets {
		preset := &config.Presets[index]
		preset.ID = strings.TrimSpace(preset.ID)
		preset.Name = strings.TrimSpace(preset.Name)
		if preset.ID == "" || seenPresets[preset.ID] {
			return VirtualSigilConfig{}, 0, errors.New("虚拟因子预设 ID 为空或重复")
		}
		seenPresets[preset.ID] = true
		if preset.Name == "" || utf8.RuneCountInString(preset.Name) > 48 {
			return VirtualSigilConfig{}, 0, errors.New("虚拟因子预设名称必须为 1 到 48 个字符")
		}
		hash, err := ParseHashHex(preset.CharacterHash)
		if err != nil || characterNameByHash[hash] == "" {
			return VirtualSigilConfig{}, 0, fmt.Errorf("预设 %s 的角色 hash 无效", preset.Name)
		}
		preset.CharacterHash = fmt.Sprintf("%08X", hash)
		if len(preset.Slots) > config.SlotCount {
			preset.Slots = preset.Slots[:config.SlotCount]
		}
		localSeen := make(map[uint32]bool)
		for slotIndex, selection := range preset.Slots {
			if selection.SlotID == 0 {
				continue
			}
			expected, exists := available[selection.SlotID]
			if !exists || selection != expected || localSeen[selection.SlotID] {
				return VirtualSigilConfig{}, 0, fmt.Errorf("预设 %s 的虚拟槽 %d 已失效或重复", preset.Name, slotIndex+1)
			}
			localSeen[selection.SlotID] = true
		}
	}
	return config, active, nil
}

func (a *App) GetVirtualSigilWorkspace(_ string, savePath string) (*VirtualSigilWorkspace, error) {
	virtualSigilModMu.Lock()
	defer virtualSigilModMu.Unlock()
	inventory, err := loadVirtualSigilInventory(savePath)
	if err != nil {
		return nil, err
	}
	_, processErr := findProcessByName(charaProcessName)
	active := runtimeCompanionPresent("virtual-sigils")
	process, processIdentityErr := findRuntimeProcessInstance()
	owned := processIdentityErr == nil && a.runtimeCompanionOwned("virtual-sigils", process)
	status := readRuntimeCompanionStatus("virtual-sigils")
	recoveryRequired := processIdentityErr == nil && runtimeCompanionRecoveryRequired(status, process)
	characters := append([]VirtualSigilCharacter(nil), virtualSigilCharacterNames...)
	unavailableReason := ""
	if !stableReleaseVirtualSigilWriteEnabled && !owned {
		unavailableReason = "稳定版暂不开放新的虚拟因子会话：切角色、切场景、同角色队友隔离和多 Hook 长测尚未完成"
	}
	return &VirtualSigilWorkspace{SavePath: savePath, Installed: active, Owned: owned, RecoveryRequired: recoveryRequired, State: status.State, Detail: status.Detail, GameRunning: processErr == nil, Available: stableReleaseVirtualSigilWriteEnabled || owned, UnavailableReason: unavailableReason, Config: readVirtualSigilConfig(), Characters: characters, Inventory: inventory}, nil
}

func (a *App) DeployVirtualSigilMod(request VirtualSigilDeployRequest) (*VirtualSigilDeployResult, error) {
	if !stableReleaseVirtualSigilWriteEnabled {
		return nil, errors.New("虚拟因子运行时在稳定版中保持禁用：切角色、切场景、同角色队友隔离和多 Hook 长测尚未完成")
	}
	virtualSigilModMu.Lock()
	defer virtualSigilModMu.Unlock()
	inventory, err := loadVirtualSigilInventory(request.SavePath)
	if err != nil {
		return nil, err
	}
	config, active, err := normalizeVirtualSigilConfig(request.Config, inventory)
	if err != nil {
		return nil, err
	}
	previous := readVirtualSigilConfig()
	jsonPath, err := virtualSigilJSONPath()
	if err != nil {
		return nil, err
	}
	if err := writeVirtualSigilConfig(jsonPath, config); err != nil {
		return nil, err
	}
	binaryPath, err := virtualSigilBinaryPath()
	if err != nil {
		return nil, err
	}
	binaryConfig, err := encodeVirtualSigilRuntime(config, true)
	if err != nil {
		return nil, err
	}
	restart := a.runtimeCompanionActive("virtual-sigils") && previous.SlotCount != config.SlotCount
	if restart {
		disabled, _ := encodeVirtualSigilRuntime(previous, false)
		if err := writeRuntimeCompanionFile(binaryPath, disabled); err != nil {
			return nil, err
		}
		if process, processErr := findRuntimeProcessInstance(); processErr == nil {
			if err := waitRuntimeCompanionStopped("virtual-sigils", process); err != nil {
				return nil, err
			}
		}
	}
	if err := writeRuntimeCompanionFile(binaryPath, binaryConfig); err != nil {
		return nil, err
	}
	if err := a.startRuntimeCompanion("virtual-sigils", "runtime_virtual_sigils"); err != nil {
		disabled, _ := encodeVirtualSigilRuntime(config, false)
		_ = writeRuntimeCompanionFile(binaryPath, disabled)
		return nil, err
	}
	return &VirtualSigilDeployResult{Active: true, RestartRequired: false, RefreshRequired: active > 0, ActiveSlots: active}, nil
}

func (a *App) RemoveVirtualSigilMod(_ string) error {
	virtualSigilModMu.Lock()
	defer virtualSigilModMu.Unlock()
	config := readVirtualSigilConfig()
	data, err := encodeVirtualSigilRuntime(config, false)
	if err != nil {
		return err
	}
	path, err := virtualSigilBinaryPath()
	if err != nil {
		return err
	}
	return a.stopOwnedRuntimeCompanion("virtual-sigils", func() error { return writeRuntimeCompanionFile(path, data) })
}
