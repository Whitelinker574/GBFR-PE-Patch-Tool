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
	naturalWrightstoneItemRowSize         = 56
	naturalWrightstoneLotRowSize          = 28
	naturalWrightstoneRateRowSize         = 16
	naturalWrightstoneGachaRowSize        = 48
	naturalWrightstoneRateGroup    uint32 = 0x67716D8A
	naturalWrightstoneGachaKey     uint32 = 0xFA21E311
)

var naturalWrightstoneRequiredTables = []struct {
	Name   string
	Size   int
	SHA256 string
}{
	{"item_pendulum.tbl", 4152, "56C05A88416A14F5A1F7B4094533EE0EA2C03AE07BB0AB9BC8361F1E565C0F83"},
	{"gacha_lot.tbl", 27672, "C75FE09E3D9E997358B707BB9CA8686F6D4AFD332DDB7C3EFDA4BB94B3FC0C83"},
	{"gacha_rate_group.tbl", 776, "5D248F9F959813A5CF9B51D644A991600B7973A6150D878E1E772629DDB8DF4F"},
	{"gacha.tbl", 200, "0462597C2D609EE2CA493B98A35D7187299F474C7F26F618E4131CEE000EE7CE"},
}

var naturalWrightstonePools = []uint32{0xD2CCD4EC, 0xFB27D2E3, 0xBD1CBF1C}

type naturalWrightstoneFamily struct {
	NameEn   string
	MainHash uint32
	Slots    [3]uint32
}

var naturalWrightstoneFamilies = []naturalWrightstoneFamily{
	{NameEn: "Dread Wrightstone", MainHash: 0xCEB700EE, Slots: [3]uint32{0x09E6F629, 0xBCDBC4B6, 0x3CB3EB94}},
	{NameEn: "Vitality Wrightstone", MainHash: 0x8D78A19B, Slots: [3]uint32{0x71173866, 0x3EF6DEF5, 0x31291925}},
	{NameEn: "Fortification Wrightstone", MainHash: 0xF372F096, Slots: [3]uint32{0x667EE1D3, 0xBE6124AD, 0xB359B5B2}},
	{NameEn: "Sequestration Wrightstone", MainHash: 0x6B694D6D, Slots: [3]uint32{0x202A0DB9, 0x0BD373A4, 0xF6E684FA}},
}

var naturalWrightstoneTraitHashes = []uint32{
	0x0053599E, 0x05F2ECDC, 0x09AA7DB5, 0x0AA20846, 0x1470F860, 0x1568E0E4,
	0x1C360C63, 0x1DC9D7E7, 0x2242921F, 0x24883AF3, 0x29B292A8, 0x2FC8FBFF,
	0x318D12E9, 0x3759A5B9, 0x3C2B57B0, 0x3F488339, 0x3FEC5F80, 0x48A95B8D,
	0x4BF2E191, 0x4F1A3683, 0x50079A1C, 0x50B453DD, 0x57AB5B10, 0x5E422AE5,
	0x6018372B, 0x6085DA25, 0x66DE60B1, 0x6B694D6D, 0x70395731, 0x71F11A9B,
	0x74AA75D6, 0x7C2E4D64, 0x7C84A6B3, 0x7CCFF74F, 0x7EDD69D0, 0x82CE278D,
	0x84078CB0, 0x8D078597, 0x8D78A19B, 0x8F502F0D, 0x9389CC06, 0x95F3FA86,
	0x9702860F, 0x973B49AF, 0xA2FA9685, 0xA7A45F28, 0xA8A3163B, 0xA9D17F55,
	0xAC9674C1, 0xB360801D, 0xB5FF9FD3, 0xB6E31F76, 0xC0979A17, 0xC35B111B,
	0xC86F3082, 0xCEB700EE, 0xCFB48782, 0xD2C8E10A, 0xD54F8CA7, 0xDC225C96,
	0xDC584F60, 0xDC607D75, 0xDD4A701E, 0xE0ABFDFE, 0xE69A4694, 0xE6CDBA9C,
	0xEAE321EB, 0xF17850B9, 0xF372F096, 0xF687C5EF, 0xFB572681,
}

type NaturalDropWrightstoneOption struct {
	MainTrait   NaturalDropTraitOption   `json:"mainTrait"`
	NameZh      string                   `json:"nameZh"`
	NameEn      string                   `json:"nameEn"`
	MaxVariants int                      `json:"maxVariants"`
	SubTraits   []NaturalDropTraitOption `json:"subTraits"`
}

type NaturalDropWrightstoneSelection struct {
	MainTrait string `json:"mainTrait"`
	SubTrait1 string `json:"subTrait1"`
	SubTrait2 string `json:"subTrait2"`
}

type naturalWrightstoneTables struct {
	Items      []byte
	Lots       []byte
	RateGroups []byte
	Gacha      []byte
}

func naturalWrightstoneChineseTrait(name string) string {
	if localized := strings.TrimSpace(wrightstoneTraitCN[name]); localized != "" {
		return localized
	}
	if localized := strings.TrimSpace(traitCN[name]); localized != "" {
		return localized
	}
	return name
}

func loadNaturalWrightstoneTables(sourceDir string, strict bool) (*naturalWrightstoneTables, []NaturalDropTableStatus, error) {
	values := make(map[string][]byte, len(naturalWrightstoneRequiredTables))
	statuses := make([]NaturalDropTableStatus, 0, len(naturalWrightstoneRequiredTables))
	missing := false
	for _, required := range naturalWrightstoneRequiredTables {
		path := filepath.Join(sourceDir, required.Name)
		data, err := os.ReadFile(path)
		if err != nil {
			if !strict && os.IsNotExist(err) {
				missing = true
				statuses = append(statuses, NaturalDropTableStatus{Name: required.Name, Expected: required.SHA256})
				continue
			}
			return nil, statuses, fmt.Errorf("读取 %s 失败: %w", required.Name, err)
		}
		hash := fileSHA256(data)
		valid := len(data) == required.Size && strings.EqualFold(hash, required.SHA256)
		statuses = append(statuses, NaturalDropTableStatus{Name: required.Name, Size: len(data), SHA256: hash, Expected: required.SHA256, Valid: valid})
		if strict && !valid {
			return nil, statuses, fmt.Errorf("%s 不是已验证的 DLC 2.0.2 原表（大小 %d，SHA-256 %s）", required.Name, len(data), hash)
		}
		if !valid {
			missing = true
			continue
		}
		values[required.Name] = data
	}
	if missing {
		return nil, statuses, nil
	}
	checks := []struct {
		name string
		size int
	}{
		{"item_pendulum.tbl", naturalWrightstoneItemRowSize},
		{"gacha_lot.tbl", naturalWrightstoneLotRowSize},
		{"gacha_rate_group.tbl", naturalWrightstoneRateRowSize},
		{"gacha.tbl", naturalWrightstoneGachaRowSize},
	}
	for _, check := range checks {
		if _, err := tableRowCount(values[check.name], check.size); err != nil {
			return nil, statuses, fmt.Errorf("%s: %w", check.name, err)
		}
	}
	return &naturalWrightstoneTables{Items: values["item_pendulum.tbl"], Lots: values["gacha_lot.tbl"], RateGroups: values["gacha_rate_group.tbl"], Gacha: values["gacha.tbl"]}, statuses, nil
}

func buildNaturalWrightstoneCatalog() ([]NaturalDropWrightstoneOption, error) {
	catalog, err := LoadWrightstoneCatalog()
	if err != nil {
		return nil, err
	}
	sharedCatalog, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	traitNames := func(hash uint32) (string, string, bool) {
		if definition := catalog.LookupTraitByHash(hash); definition != nil {
			return naturalWrightstoneChineseTrait(definition.DisplayName), definition.DisplayName, true
		}
		if definition := sharedCatalog.LookupTraitByHash(hash); definition != nil {
			nameEn := strings.TrimSpace(definition.DisplayName)
			nameZh := strings.TrimSpace(runtimeNameCN[hash])
			if nameZh == "" {
				nameZh = cnTrait(nameEn)
			}
			return nameZh, nameEn, nameZh != "" && nameEn != ""
		}
		nameZh, nameEn := strings.TrimSpace(runtimeNameCN[hash]), strings.TrimSpace(runtimeNameEN[hash])
		return nameZh, nameEn, nameZh != "" && nameEn != ""
	}
	traits := make([]NaturalDropTraitOption, 0, len(naturalWrightstoneTraitHashes))
	for _, hash := range naturalWrightstoneTraitHashes {
		nameZh, nameEn, ok := traitNames(hash)
		if !ok {
			return nil, fmt.Errorf("天然祝福词条 0x%08X 不在共享目录", hash)
		}
		traits = append(traits, NaturalDropTraitOption{Hash: fmt.Sprintf("0x%08X", hash), NameZh: nameZh, NameEn: nameEn})
	}
	sort.Slice(traits, func(i, j int) bool { return traits[i].NameZh < traits[j].NameZh })
	result := make([]NaturalDropWrightstoneOption, 0, len(naturalWrightstoneFamilies))
	for _, family := range naturalWrightstoneFamilies {
		nameZh, nameEn, ok := traitNames(family.MainHash)
		if !ok {
			return nil, fmt.Errorf("天然祝福主词条 0x%08X 不在共享目录", family.MainHash)
		}
		familyTraits := make([]NaturalDropTraitOption, 0, len(traits)-1)
		for _, trait := range traits {
			hash, _ := ParseHashHex(trait.Hash)
			if hash != family.MainHash {
				familyTraits = append(familyTraits, trait)
			}
		}
		result = append(result, NaturalDropWrightstoneOption{
			MainTrait: NaturalDropTraitOption{Hash: fmt.Sprintf("0x%08X", family.MainHash), NameZh: nameZh, NameEn: nameEn},
			NameZh:    previewChineseWrightstoneName(family.NameEn), NameEn: family.NameEn, MaxVariants: len(family.Slots), SubTraits: familyTraits,
		})
	}
	return result, nil
}

func naturalWrightstoneFamilyByMain(hash uint32) (*naturalWrightstoneFamily, bool) {
	for index := range naturalWrightstoneFamilies {
		if naturalWrightstoneFamilies[index].MainHash == hash {
			return &naturalWrightstoneFamilies[index], true
		}
	}
	return nil, false
}

func naturalWrightstonePinItem(data []byte, key, main, sub1, sub2 uint32) error {
	count, err := tableRowCount(data, naturalWrightstoneItemRowSize)
	if err != nil {
		return err
	}
	for index := 0; index < count; index++ {
		offset := 8 + index*naturalWrightstoneItemRowSize
		if binary.LittleEndian.Uint32(data[offset+32:]) != key {
			continue
		}
		binary.LittleEndian.PutUint32(data[offset:], sub1)
		binary.LittleEndian.PutUint32(data[offset+4:], sub2)
		binary.LittleEndian.PutUint32(data[offset+8:], 15)
		binary.LittleEndian.PutUint32(data[offset+12:], 10)
		for _, field := range []int{16, 20, 24, 28, 44} {
			binary.LittleEndian.PutUint32(data[offset+field:], 0)
		}
		binary.LittleEndian.PutUint32(data[offset+36:], main)
		binary.LittleEndian.PutUint32(data[offset+40:], 20)
		return nil
	}
	return fmt.Errorf("item_pendulum.tbl 缺少祝福石槽 0x%08X", key)
}

func naturalWrightstoneSetRate(data []byte, pool uint32, weight uint32) error {
	count, err := tableRowCount(data, naturalWrightstoneRateRowSize)
	if err != nil {
		return err
	}
	for index := 0; index < count; index++ {
		offset := 8 + index*naturalWrightstoneRateRowSize
		if binary.LittleEndian.Uint32(data[offset:]) == naturalWrightstoneRateGroup && binary.LittleEndian.Uint32(data[offset+4:]) == pool {
			binary.LittleEndian.PutUint32(data[offset+8:], weight)
			binary.LittleEndian.PutUint32(data[offset+12:], 0)
			return nil
		}
	}
	return fmt.Errorf("gacha_rate_group.tbl 缺少祝福石池 0x%08X", pool)
}

func patchNaturalWrightstoneTables(source *naturalWrightstoneTables, selections []NaturalDropWrightstoneSelection, wrightstoneOnly bool) (*naturalWrightstoneTables, int, error) {
	if source == nil {
		return nil, 0, errors.New("天然祝福石原表为空")
	}
	if len(selections) == 0 || len(selections) > 12 {
		return nil, 0, fmt.Errorf("天然祝福石配置数量必须为 1 到 12，当前为 %d", len(selections))
	}
	items := append([]byte(nil), source.Items...)
	lots := append([]byte(nil), source.Lots...)
	rates := append([]byte(nil), source.RateGroups...)
	gacha := append([]byte(nil), source.Gacha...)
	allowed := make(map[uint32]bool, len(naturalWrightstoneTraitHashes))
	for _, hash := range naturalWrightstoneTraitHashes {
		allowed[hash] = true
	}
	usedSlots := make(map[uint32]bool)
	activeByPool := make(map[uint32]int)
	usedByFamily := make(map[uint32]int)
	for _, selection := range selections {
		main, err := parseNaturalDropSelection(selection.MainTrait, "祝福石主词条")
		if err != nil {
			return nil, 0, err
		}
		family, ok := naturalWrightstoneFamilyByMain(main)
		if !ok {
			return nil, 0, fmt.Errorf("0x%08X 不是天然祝福石主词条", main)
		}
		sub1, err := parseNaturalDropSelection(selection.SubTrait1, "祝福石第二词条")
		if err != nil {
			return nil, 0, err
		}
		sub2, err := parseNaturalDropSelection(selection.SubTrait2, "祝福石第三词条")
		if err != nil {
			return nil, 0, err
		}
		if !allowed[sub1] || !allowed[sub2] || sub1 == main || sub2 == main {
			return nil, 0, fmt.Errorf("%s 包含不属于该原始掉落池的副词条", family.NameEn)
		}
		variant := usedByFamily[main]
		if variant >= len(family.Slots) {
			return nil, 0, fmt.Errorf("%s 最多配置 %d 个天然变体", family.NameEn, len(family.Slots))
		}
		usedByFamily[main] = variant + 1
		key := family.Slots[variant]
		pool := naturalWrightstonePools[variant]
		if err := naturalWrightstonePinItem(items, key, main, sub1, sub2); err != nil {
			return nil, 0, err
		}
		usedSlots[key] = true
		activeByPool[pool]++
	}
	lotCount, err := tableRowCount(lots, naturalWrightstoneLotRowSize)
	if err != nil {
		return nil, 0, err
	}
	foundSlots := make(map[uint32]bool)
	for index := 0; index < lotCount; index++ {
		offset := 8 + index*naturalWrightstoneLotRowSize
		pool := binary.LittleEndian.Uint32(lots[offset+8:])
		if pool != naturalWrightstonePools[0] && pool != naturalWrightstonePools[1] && pool != naturalWrightstonePools[2] {
			continue
		}
		key := binary.LittleEndian.Uint32(lots[offset+12:])
		weight := uint32(0)
		if usedSlots[key] {
			weight = 50
			foundSlots[key] = true
		}
		binary.LittleEndian.PutUint32(lots[offset+16:], weight)
		binary.LittleEndian.PutUint32(lots[offset+24:], 0)
	}
	for key := range usedSlots {
		if !foundSlots[key] {
			return nil, 0, fmt.Errorf("gacha_lot.tbl 缺少祝福石槽 0x%08X", key)
		}
	}
	for _, pool := range naturalWrightstonePools {
		if err := naturalWrightstoneSetRate(rates, pool, uint32(activeByPool[pool]*5000)); err != nil {
			return nil, 0, err
		}
	}
	if wrightstoneOnly {
		count, err := tableRowCount(gacha, naturalWrightstoneGachaRowSize)
		if err != nil {
			return nil, 0, err
		}
		found := false
		for index := 0; index < count; index++ {
			offset := 8 + index*naturalWrightstoneGachaRowSize
			if binary.LittleEndian.Uint32(gacha[offset+8:]) == naturalWrightstoneGachaKey {
				binary.LittleEndian.PutUint32(gacha[offset:], 0)
				binary.LittleEndian.PutUint32(gacha[offset+4:], 100)
				found = true
				break
			}
		}
		if !found {
			return nil, 0, errors.New("gacha.tbl 缺少 Transmarvel 记录")
		}
	}
	return &naturalWrightstoneTables{Items: items, Lots: lots, RateGroups: rates, Gacha: gacha}, len(selections), nil
}
