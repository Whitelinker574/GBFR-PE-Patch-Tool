package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"unsafe"
)

const (
	summonInventoryPtrRVA = 0x7C23F48
	summonRecordsOffset   = 0xC40
	summonRecordSize      = 0x1C
	summonMaxRecords      = 1000
	summonInvalidTypeHash = 0x887AE0B0
	summonSaveFunctionRVA = 0x79D820
)

type SummonInfo struct {
	Index          int    `json:"index"`
	Address        uint64 `json:"address"`
	TypeHash       uint32 `json:"typeHash"`
	Slot           uint32 `json:"slot"`
	MainTraitHash  uint32 `json:"mainTraitHash"`
	SubParamHash   uint32 `json:"subParamHash"`
	MainTraitLevel uint32 `json:"mainTraitLevel"`
	SubParamLevel  uint32 `json:"subParamLevel"`
	Rank           uint32 `json:"rank"`
}

type SummonUpdate struct {
	Index          int    `json:"index"`
	TypeHash       uint32 `json:"typeHash"`
	MainTraitHash  uint32 `json:"mainTraitHash"`
	SubParamHash   uint32 `json:"subParamHash"`
	MainTraitLevel uint32 `json:"mainTraitLevel"`
	SubParamLevel  uint32 `json:"subParamLevel"`
	Rank           uint32 `json:"rank"`
}

//go:embed data/summons.json
var summonTypesJSON []byte

//go:embed data/summon_skills.json
var summonSkillsJSON []byte

//go:embed data/summon_sub_params.json
var summonSubParamsJSON []byte

type SummonOption struct {
	Hash      uint32    `json:"hash"`
	Name      string    `json:"name"`
	MaxLevel  int       `json:"maxLevel"`
	Cost      int       `json:"cost"`
	TypeName  string    `json:"typeName"`
	IsPercent bool      `json:"isPercent"`
	Values    []float64 `json:"values"`
}

type SummonOptions struct {
	Types     []SummonOption `json:"types"`
	Traits    []SummonOption `json:"traits"`
	SubParams []SummonOption `json:"subParams"`
}

type summonTypeFile struct {
	Summons []struct {
		Hash        string `json:"hash"`
		DisplayName string `json:"displayName"`
		Cost        int    `json:"cost"`
		TypeName    string `json:"typeName"`
	} `json:"summons"`
}

type summonSkillFile struct {
	Skills []struct {
		Hash        string `json:"hash"`
		DisplayName string `json:"displayName"`
		MaxLevel    int    `json:"maxLevel"`
	} `json:"skills"`
}

type summonSubParamFile struct {
	SubParams []struct {
		Hash        string `json:"hash"`
		DisplayName string `json:"displayName"`
	} `json:"subParams"`
}

func (a *App) SummonGetOptions() (SummonOptions, error) {
	var types summonTypeFile
	var skills summonSkillFile
	var subParams summonSubParamFile
	if err := json.Unmarshal(summonTypesJSON, &types); err != nil {
		return SummonOptions{}, fmt.Errorf("解析召唤石种类映射失败: %w", err)
	}
	if err := json.Unmarshal(summonSkillsJSON, &skills); err != nil {
		return SummonOptions{}, fmt.Errorf("解析召唤石因子映射失败: %w", err)
	}
	if err := json.Unmarshal(summonSubParamsJSON, &subParams); err != nil {
		return SummonOptions{}, fmt.Errorf("解析召唤石副参数映射失败: %w", err)
	}
	options := SummonOptions{
		Types:     make([]SummonOption, 0, len(types.Summons)),
		Traits:    make([]SummonOption, 0, len(skills.Skills)),
		SubParams: make([]SummonOption, 0, len(subParams.SubParams)),
	}
	for _, item := range types.Summons {
		hash, err := ParseHashHex(item.Hash)
		if err == nil {
			options.Types = append(options.Types, SummonOption{Hash: hash, Name: item.DisplayName, Cost: item.Cost, TypeName: item.TypeName})
		}
	}
	for _, item := range skills.Skills {
		hash, err := ParseHashHex(item.Hash)
		if err == nil {
			options.Traits = append(options.Traits, SummonOption{Hash: hash, Name: item.DisplayName, MaxLevel: item.MaxLevel})
		}
	}
	for _, item := range subParams.SubParams {
		hash, err := ParseHashHex(item.Hash)
		if err == nil {
			options.SubParams = append(options.SubParams, SummonOption{Hash: hash, Name: item.DisplayName})
		}
	}
	return options, nil
}

func (a *App) summonInventoryAddress() (uintptr, error) {
	if err := a.ensureGameProcess(); err != nil {
		return 0, err
	}
	var inventory uintptr
	root := a.moduleBase + summonInventoryPtrRVA
	if err := readProcessMemory(a.hProcess, root, unsafe.Pointer(&inventory), unsafe.Sizeof(inventory)); err != nil {
		return 0, fmt.Errorf("读取召唤石背包指针失败: %w", err)
	}
	if inventory == 0 {
		return 0, fmt.Errorf("召唤石背包未加载，请进入游戏存档并打开召唤石背包")
	}
	return inventory, nil
}

func (a *App) readSummonRecords(inventory uintptr) ([]SummonInfo, error) {
	buf := make([]byte, summonMaxRecords*summonRecordSize)
	start := inventory + summonRecordsOffset
	if err := readProcessMemory(a.hProcess, start, unsafe.Pointer(&buf[0]), uintptr(len(buf))); err != nil {
		return nil, fmt.Errorf("读取召唤石背包失败: %w", err)
	}

	result := make([]SummonInfo, 0, summonMaxRecords)
	for i := 0; i < summonMaxRecords; i++ {
		base := i * summonRecordSize
		item := SummonInfo{
			Index:          i,
			Address:        uint64(start + uintptr(base)),
			TypeHash:       readUint32LE(buf[base:]),
			Slot:           readUint32LE(buf[base+4:]),
			MainTraitHash:  readUint32LE(buf[base+8:]),
			SubParamHash:   readUint32LE(buf[base+12:]),
			MainTraitLevel: readUint32LE(buf[base+16:]),
			SubParamLevel:  readUint32LE(buf[base+20:]),
			Rank:           readUint32LE(buf[base+24:]),
		}
		if item.TypeHash != 0 && item.TypeHash != summonInvalidTypeHash {
			result = append(result, item)
		}
	}
	return result, nil
}

func (a *App) SummonGetAll() ([]SummonInfo, error) {
	inventory, err := a.summonInventoryAddress()
	if err != nil {
		return nil, err
	}
	return a.readSummonRecords(inventory)
}

func (a *App) SummonUpdate(item SummonUpdate) (SummonInfo, error) {
	if item.Index < 0 || item.Index >= summonMaxRecords {
		return SummonInfo{}, fmt.Errorf("无效召唤石索引: %d", item.Index)
	}
	if item.TypeHash == 0 {
		return SummonInfo{}, fmt.Errorf("召唤石种类不能为空")
	}
	if item.Rank == 0 || item.Rank > 3 {
		return SummonInfo{}, fmt.Errorf("阶级必须为 1 到 3")
	}
	if item.MainTraitLevel > math.MaxInt32 || item.SubParamLevel > math.MaxInt32 {
		return SummonInfo{}, fmt.Errorf("召唤石等级或副参数等级超出范围")
	}

	inventory, err := a.summonInventoryAddress()
	if err != nil {
		return SummonInfo{}, err
	}
	items, err := a.readSummonRecords(inventory)
	if err != nil {
		return SummonInfo{}, err
	}
	found := false
	for _, existing := range items {
		if existing.Index != item.Index {
			continue
		}
		if item.TypeHash != existing.TypeHash {
			return SummonInfo{}, fmt.Errorf("召唤石种类不支持修改")
		}
		found = true
		break
	}
	if !found {
		return SummonInfo{}, fmt.Errorf("召唤石索引不存在于当前背包: %d", item.Index)
	}

	address := inventory + summonRecordsOffset + uintptr(item.Index*summonRecordSize)
	values := []struct {
		offset uintptr
		value  uint32
	}{
		{0x00, item.TypeHash},
		{0x08, item.MainTraitHash},
		{0x0C, item.SubParamHash},
		{0x10, item.MainTraitLevel},
		{0x14, item.SubParamLevel},
		{0x18, item.Rank},
	}
	for _, field := range values {
		if err := writeUint32Remote(a.hProcess, address+field.offset, field.value); err != nil {
			return SummonInfo{}, fmt.Errorf("写入召唤石字段 +0x%02X 失败: %w", field.offset, err)
		}
	}

	saveFn := a.moduleBase + summonSaveFunctionRVA
	for _, offset := range []uintptr{0x08, 0x0C, 0x10, 0x14, 0x18} {
		if err := a.callRemoteOneArg(saveFn, address+offset); err != nil {
			return SummonInfo{}, fmt.Errorf("调用召唤石保存函数失败: %w", err)
		}
	}

	items, err = a.SummonGetAll()
	if err != nil {
		return SummonInfo{}, err
	}
	for _, updated := range items {
		if updated.Index == item.Index {
			return updated, nil
		}
	}
	return SummonInfo{}, fmt.Errorf("召唤石写入后未找到索引 %d", item.Index)
}

func readUint32LE(data []byte) uint32 {
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
}
