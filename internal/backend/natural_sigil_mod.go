package backend

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	naturalSigilGemRowSize        = 64
	naturalSigilRateGroup  uint32 = 0x27509C51
)

var naturalSigilRequiredTables = []struct {
	Name   string
	Size   int
	SHA256 string
}{
	{"gem.tbl", 66184, "F6B39C0FF9A190B3DD44FEFEAC84180D5FDA8254636DEEA1A8181336B1EA2C99"},
}

var naturalDropSigilTablePaths = []string{"system/table/gem.tbl"}

var naturalSigilPools = []uint32{
	0x9092654F, 0xB3976B98, 0x090F0E91, 0xF527EF32, 0x5AD4ADAD,
	0x81216A95, 0x6E52A69A, 0x36879ED7, 0x1F44C95D,
}

var naturalSigilPoolSet = func() map[uint32]bool {
	result := make(map[uint32]bool, len(naturalSigilPools))
	for _, pool := range naturalSigilPools {
		result[pool] = true
	}
	return result
}()

type NaturalDropSigilOption struct {
	SigilHash         string                   `json:"sigilHash"`
	InternalID        string                   `json:"internalId"`
	NameZh            string                   `json:"nameZh"`
	NameEn            string                   `json:"nameEn"`
	Level             int                      `json:"level"`
	NativeTransmarvel bool                     `json:"nativeTransmarvel"`
	PrimaryTrait      NaturalDropTraitOption   `json:"primaryTrait"`
	SecondaryTraits   []NaturalDropTraitOption `json:"secondaryTraits"`
}

type NaturalDropSigilSelection struct {
	SigilHash      string `json:"sigilHash"`
	SecondaryTrait string `json:"secondaryTrait"`
}

type naturalSigilTables struct {
	Gem []byte
}

func loadNaturalSigilTables(sourceDir string, strict bool) (*naturalSigilTables, []NaturalDropTableStatus, error) {
	bundled := naturalDropUsesBundledSource(sourceDir)
	statuses := make([]NaturalDropTableStatus, 0, len(naturalSigilRequiredTables))
	var gem []byte
	for _, required := range naturalSigilRequiredTables {
		var data []byte
		var err error
		if bundled {
			data, err = naturalDropBundledTable(required.Name)
		} else {
			data, err = os.ReadFile(filepath.Join(sourceDir, required.Name))
		}
		if err != nil {
			if !strict && os.IsNotExist(err) {
				statuses = append(statuses, NaturalDropTableStatus{Name: required.Name, Expected: required.SHA256})
				return nil, statuses, nil
			}
			return nil, statuses, fmt.Errorf("读取 %s 失败: %w", required.Name, err)
		}
		hash := fileSHA256(data)
		valid := len(data) == required.Size && strings.EqualFold(hash, required.SHA256)
		statuses = append(statuses, NaturalDropTableStatus{
			Name: required.Name, Size: len(data), SHA256: hash, Expected: required.SHA256, Valid: valid,
		})
		if strict && !valid {
			return nil, statuses, fmt.Errorf("%s 不是已验证的 DLC 2.0.2 原表（大小 %d，SHA-256 %s）", required.Name, len(data), hash)
		}
		if !valid {
			return nil, statuses, nil
		}
		gem = data
	}
	if _, err := tableRowCount(gem, naturalSigilGemRowSize); err != nil {
		return nil, statuses, fmt.Errorf("gem.tbl: %w", err)
	}
	return &naturalSigilTables{Gem: gem}, statuses, nil
}

func naturalSigilGemRowOffset(data []byte, key uint32) (int, bool) {
	count, err := tableRowCount(data, naturalSigilGemRowSize)
	if err != nil {
		return 0, false
	}
	for index := 0; index < count; index++ {
		offset := 8 + index*naturalSigilGemRowSize
		if binary.LittleEndian.Uint32(data[offset+8:]) == key {
			return offset, true
		}
	}
	return 0, false
}

func naturalSigilTransmarvelGachaOffset(data []byte) (int, bool) {
	count, err := tableRowCount(data, naturalWrightstoneGachaRowSize)
	if err != nil {
		return 0, false
	}
	for index := 0; index < count; index++ {
		offset := 8 + index*naturalWrightstoneGachaRowSize
		if binary.LittleEndian.Uint32(data[offset+8:]) == naturalWrightstoneGachaKey {
			return offset, true
		}
	}
	return 0, false
}

func naturalSigilChineseName(sigil *SigilDef, hash uint32) string {
	if sigil == nil {
		return ""
	}
	if name := strings.TrimSpace(runtimeNameCN[hash]); name != "" {
		return normalizeChineseSigilItemName(name)
	}
	if name := strings.TrimSpace(sigilCN[sigil.DisplayName]); name != "" {
		return normalizeChineseSigilItemName(name)
	}
	return normalizeChineseSigilItemName(strings.TrimSpace(sigil.DisplayName))
}

func naturalSigilTraitOption(trait *TraitDef) NaturalDropTraitOption {
	if trait == nil {
		return NaturalDropTraitOption{}
	}
	hash, _ := ParseHashHex(trait.Hash)
	nameEn := strings.TrimSpace(trait.DisplayName)
	nameZh := strings.TrimSpace(runtimeNameCN[hash])
	if nameZh == "" {
		nameZh = strings.TrimSpace(traitCN[nameEn])
	}
	if nameZh == "" {
		nameZh = nameEn
	}
	return NaturalDropTraitOption{
		InternalID: trait.InternalID,
		Hash:       fmt.Sprintf("0x%08X", hash),
		NameZh:     nameZh,
		NameEn:     nameEn,
	}
}

func naturalSigilDefaultLevel(sigil *SigilDef) int {
	if sigil == nil {
		return 0
	}
	if sigil.DefaultSigilLevel != nil {
		return *sigil.DefaultSigilLevel
	}
	level := 0
	for _, candidate := range sigil.AllowedSigilLevels {
		if candidate > level {
			level = candidate
		}
	}
	return level
}

func naturalSigilNativeSet(lots []byte) (map[uint32]bool, error) {
	count, err := tableRowCount(lots, naturalWrightstoneLotRowSize)
	if err != nil {
		return nil, err
	}
	result := make(map[uint32]bool)
	for index := 0; index < count; index++ {
		offset := 8 + index*naturalWrightstoneLotRowSize
		if !naturalSigilPoolSet[binary.LittleEndian.Uint32(lots[offset+8:])] {
			continue
		}
		if binary.LittleEndian.Uint32(lots[offset+16:]) == 0 {
			continue
		}
		result[binary.LittleEndian.Uint32(lots[offset+12:])] = true
	}
	return result, nil
}

func buildNaturalSigilCatalog(lots, gem []byte) ([]NaturalDropSigilOption, error) {
	if _, err := tableRowCount(gem, naturalSigilGemRowSize); err != nil {
		return nil, err
	}
	native, err := naturalSigilNativeSet(lots)
	if err != nil {
		return nil, err
	}
	catalog, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	result := make([]NaturalDropSigilOption, 0, len(catalog.Sigils))
	for index := range catalog.Sigils {
		sigil := &catalog.Sigils[index]
		hash, err := ParseHashHex(sigil.Hash)
		if err != nil {
			continue
		}
		if _, exists := naturalSigilGemRowOffset(gem, hash); !exists || !isVerifiedSigilDefinition(sigil) {
			continue
		}
		primary, err := catalog.RequireTrait(sigil.PrimaryTraitID)
		if err != nil {
			continue
		}
		secondary, err := catalog.GetAllowedSecondaryTraits(sigil)
		if err != nil {
			return nil, err
		}
		secondaryOptions := make([]NaturalDropTraitOption, 0, len(secondary))
		for _, trait := range secondary {
			secondaryOptions = append(secondaryOptions, naturalSigilTraitOption(trait))
		}
		sort.Slice(secondaryOptions, func(i, j int) bool {
			if secondaryOptions[i].NameZh != secondaryOptions[j].NameZh {
				return secondaryOptions[i].NameZh < secondaryOptions[j].NameZh
			}
			return secondaryOptions[i].Hash < secondaryOptions[j].Hash
		})
		result = append(result, NaturalDropSigilOption{
			SigilHash:         fmt.Sprintf("0x%08X", hash),
			InternalID:        sigil.InternalID,
			NameZh:            naturalSigilChineseName(sigil, hash),
			NameEn:            strings.TrimSpace(sigil.DisplayName),
			Level:             naturalSigilDefaultLevel(sigil),
			NativeTransmarvel: native[hash],
			PrimaryTrait:      naturalSigilTraitOption(primary),
			SecondaryTraits:   secondaryOptions,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].NativeTransmarvel != result[j].NativeTransmarvel {
			return result[i].NativeTransmarvel
		}
		if result[i].NameZh != result[j].NameZh {
			return result[i].NameZh < result[j].NameZh
		}
		return result[i].SigilHash < result[j].SigilHash
	})
	return result, nil
}

func naturalSigilPoolCapacity(lots []byte) (int, error) {
	count, err := tableRowCount(lots, naturalWrightstoneLotRowSize)
	if err != nil {
		return 0, err
	}
	capacity := 0
	for index := 0; index < count; index++ {
		offset := 8 + index*naturalWrightstoneLotRowSize
		if naturalSigilPoolSet[binary.LittleEndian.Uint32(lots[offset+8:])] {
			capacity++
		}
	}
	return capacity, nil
}

func patchNaturalSigilTables(
	shared *naturalWrightstoneTables,
	source *naturalSigilTables,
	selections []NaturalDropSigilSelection,
	sigilOnly bool,
) (*naturalWrightstoneTables, *naturalSigilTables, int, error) {
	if shared == nil || source == nil {
		return nil, nil, 0, errors.New("Transmarvel 因子原表为空")
	}
	capacity, err := naturalSigilPoolCapacity(shared.Lots)
	if err != nil {
		return nil, nil, 0, err
	}
	if len(selections) == 0 || len(selections) > capacity {
		return nil, nil, 0, fmt.Errorf("Transmarvel 因子配置数量必须为 1 到 %d，当前为 %d", capacity, len(selections))
	}
	options, err := buildNaturalSigilCatalog(shared.Lots, source.Gem)
	if err != nil {
		return nil, nil, 0, err
	}
	optionByHash := make(map[uint32]NaturalDropSigilOption, len(options))
	for _, option := range options {
		hash, _ := ParseHashHex(option.SigilHash)
		optionByHash[hash] = option
	}
	lots := append([]byte(nil), shared.Lots...)
	rates := append([]byte(nil), shared.RateGroups...)
	gacha := append([]byte(nil), shared.Gacha...)
	items := append([]byte(nil), shared.Items...)
	gem := append([]byte(nil), source.Gem...)
	type resolvedSelection struct {
		hash   uint32
		level  int
		option NaturalDropSigilOption
	}
	resolved := make([]resolvedSelection, 0, len(selections))
	seen := make(map[uint32]bool, len(selections))
	for _, selection := range selections {
		hash, err := parseNaturalDropSelection(selection.SigilHash, "因子")
		if err != nil {
			return nil, nil, 0, err
		}
		option, ok := optionByHash[hash]
		if !ok {
			return nil, nil, 0, fmt.Errorf("因子 0x%08X 不在已验证的 2.0.2 gem.tbl 目录", hash)
		}
		if seen[hash] {
			return nil, nil, 0, fmt.Errorf("因子 %s 重复选择", option.NameZh)
		}
		seen[hash] = true
		if strings.TrimSpace(selection.SecondaryTrait) != "" {
			traitHash, err := parseNaturalDropSelection(selection.SecondaryTrait, "因子副词条")
			if err != nil {
				return nil, nil, 0, err
			}
			allowed := false
			for _, candidate := range option.SecondaryTraits {
				candidateHash, _ := ParseHashHex(candidate.Hash)
				if candidateHash == traitHash {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil, nil, 0, fmt.Errorf("%s 不能天然生成副词条 0x%08X", option.NameZh, traitHash)
			}
			offset, ok := naturalSigilGemRowOffset(gem, hash)
			if !ok {
				return nil, nil, 0, fmt.Errorf("gem.tbl 缺少因子 0x%08X", hash)
			}
			binary.LittleEndian.PutUint32(gem[offset+4:], traitHash)
			binary.LittleEndian.PutUint32(gem[offset+36:], uint32(^uint32(0)))
		}
		resolved = append(resolved, resolvedSelection{hash: hash, level: option.Level, option: option})
	}
	lotCount, err := tableRowCount(lots, naturalWrightstoneLotRowSize)
	if err != nil {
		return nil, nil, 0, err
	}
	next := 0
	activeByPool := make(map[uint32]int, len(naturalSigilPools))
	for index := 0; index < lotCount; index++ {
		offset := 8 + index*naturalWrightstoneLotRowSize
		pool := binary.LittleEndian.Uint32(lots[offset+8:])
		if !naturalSigilPoolSet[pool] {
			continue
		}
		if next < len(resolved) {
			item := resolved[next]
			next++
			binary.LittleEndian.PutUint32(lots[offset:], 0)
			binary.LittleEndian.PutUint32(lots[offset+4:], 0)
			binary.LittleEndian.PutUint32(lots[offset+12:], item.hash)
			binary.LittleEndian.PutUint32(lots[offset+16:], 50)
			binary.LittleEndian.PutUint32(lots[offset+20:], uint32(item.level))
			binary.LittleEndian.PutUint32(lots[offset+24:], 0)
			activeByPool[pool]++
		} else {
			binary.LittleEndian.PutUint32(lots[offset+16:], 0)
		}
	}
	if next != len(resolved) {
		return nil, nil, 0, errors.New("gacha_lot.tbl 的 Transmarvel 因子池容量不足")
	}
	rateCount, err := tableRowCount(rates, naturalWrightstoneRateRowSize)
	if err != nil {
		return nil, nil, 0, err
	}
	foundPools := make(map[uint32]bool, len(naturalSigilPools))
	for index := 0; index < rateCount; index++ {
		offset := 8 + index*naturalWrightstoneRateRowSize
		if binary.LittleEndian.Uint32(rates[offset:]) != naturalSigilRateGroup {
			continue
		}
		pool := binary.LittleEndian.Uint32(rates[offset+4:])
		if !naturalSigilPoolSet[pool] {
			continue
		}
		binary.LittleEndian.PutUint32(rates[offset+8:], uint32(activeByPool[pool]*5000))
		binary.LittleEndian.PutUint32(rates[offset+12:], 0)
		foundPools[pool] = true
	}
	for _, pool := range naturalSigilPools {
		if !foundPools[pool] {
			return nil, nil, 0, fmt.Errorf("gacha_rate_group.tbl 缺少因子池 0x%08X", pool)
		}
	}
	if sigilOnly {
		offset, ok := naturalSigilTransmarvelGachaOffset(gacha)
		if !ok {
			return nil, nil, 0, errors.New("gacha.tbl 缺少 Transmarvel 记录")
		}
		binary.LittleEndian.PutUint32(gacha[offset:], 100)
		binary.LittleEndian.PutUint32(gacha[offset+4:], 0)
	}
	return &naturalWrightstoneTables{Items: items, Lots: lots, RateGroups: rates, Gacha: gacha},
		&naturalSigilTables{Gem: gem}, len(resolved), nil
}
