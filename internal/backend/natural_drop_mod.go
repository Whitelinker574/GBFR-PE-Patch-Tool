package backend

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

const (
	naturalDropGameVersion                  = "2.0.2"
	naturalDropGame203Version               = "2.0.3"
	naturalDropGame204Version               = "2.0.4"
	naturalDropGame202ExecutableSize        = int64(123522016)
	naturalDropGame203ExecutableSize        = int64(123506656)
	naturalDropGame204ExecutableSize        = int64(123512288)
	naturalDropModID                        = "gbfr.codex.natural-drop-lab"
	naturalDropForcedWeight          uint32 = 1_000_000
	naturalDropLowWeight             uint32 = 1
	summonTableRowSize                      = 36
	summonLotRowSize                        = 20
	rewardSummonLotRowSize                  = 16
	rewardTableRowSize                      = 52
	rewardLotTableRowSize                   = 60
	endlessPackageTableRowSize              = 108
	naturalDropItemMaxQuantity              = 999
	naturalDropItemMaxWeight                = 1_000_000
	naturalDropDefaultPackageIndex          = 0
	naturalDropDefaultPackageKey            = 0xDCFBA295
	naturalDropDefaultRewardKey             = 0xE48A5279
	naturalDropDefaultItemPool              = 0xDF1DB99C
)

var naturalDropRequiredTables = []struct {
	Name   string
	Size   int
	SHA256 string
}{
	{"summon.tbl", 6812, "80BBDF59F5F56FC10DF7CDDC57ACDEC1FFC389941D3C5162D287D4DC56EE07BE"},
	{"summon_lot.tbl", 14528, "25FAA86FB94A076B8DF02AF20232C878FD517E0DE325A497CAA6BF5644040F97"},
	{"reward_summon_lot.tbl", 9400, "20A8EAA16BDF6413EB57D39952B6313E758DF1B9F85C634400997047407DC4A1"},
}

var naturalDropItemRequiredTables = []struct {
	Name   string
	Size   int
	SHA256 string
}{
	{"reward.tbl", 327192, "52A90D18D1BECF2C4F610F98EEB44DE55FA225A314D6E7E9878CA0444C3FFF11"},
	{"reward_lot.tbl", 1473668, "0E35B52BA171F59C1CCB4E6CC389F5B5AB0BBE3A52109DBD3066A023927BE8A0"},
	{"endlessmode_package.tbl", 4760, "DB6067C20910F40B481360AEC28EDACA331C440A68DC5F9BC50A5BCFA5A2C75E"},
}

type NaturalDropTableStatus struct {
	Name     string `json:"name"`
	Size     int    `json:"size"`
	SHA256   string `json:"sha256"`
	Expected string `json:"expected"`
	Valid    bool   `json:"valid"`
}

type NaturalDropTraitOption struct {
	InternalID string `json:"internalId,omitempty"`
	Hash       string `json:"hash"`
	NameZh     string `json:"nameZh"`
	NameEn     string `json:"nameEn"`
}

type NaturalDropSummonOption struct {
	TypeHash    string                   `json:"typeHash"`
	NameZh      string                   `json:"nameZh"`
	NameEn      string                   `json:"nameEn"`
	Tier        string                   `json:"tier"`
	TypeNameZh  string                   `json:"typeNameZh"`
	TypeNameEn  string                   `json:"typeNameEn"`
	RewardPools int                      `json:"rewardPools"`
	MainTraits  []NaturalDropTraitOption `json:"mainTraits"`
	SubParams   []NaturalDropTraitOption `json:"subParams"`
}

type NaturalDropItemOption struct {
	Hash      string `json:"hash"`
	NameZh    string `json:"nameZh"`
	NameEn    string `json:"nameEn"`
	Category  string `json:"category"`
	Dangerous bool   `json:"dangerous"`
}

type NaturalDropConflict struct {
	ModID string `json:"modId"`
	Path  string `json:"path"`
	File  string `json:"file"`
	Scope string `json:"scope"`
}

type NaturalDropWorkspace struct {
	GameVersion            string                         `json:"gameVersion"`
	SourceDir              string                         `json:"sourceDir"`
	GameExePath            string                         `json:"gameExePath"`
	GameDir                string                         `json:"gameDir"`
	IndexPath              string                         `json:"indexPath"`
	Installed              bool                           `json:"installed"`
	Owned                  bool                           `json:"owned"`
	BackupReady            bool                           `json:"backupReady"`
	IndexValid             bool                           `json:"indexValid"`
	IndexSummary           string                         `json:"indexSummary"`
	Tables                 []NaturalDropTableStatus       `json:"tables"`
	Summons                []NaturalDropSummonOption      `json:"summons"`
	Sigils                 []NaturalDropSigilOption       `json:"sigils"`
	Wrightstones           []NaturalDropWrightstoneOption `json:"wrightstones"`
	Items                  []NaturalDropItemOption        `json:"items"`
	ItemRewardTargetZh     string                         `json:"itemRewardTargetZh"`
	ItemRewardTargetEn     string                         `json:"itemRewardTargetEn"`
	SummonTablesReady      bool                           `json:"summonTablesReady"`
	SigilTablesReady       bool                           `json:"sigilTablesReady"`
	WrightstoneTablesReady bool                           `json:"wrightstoneTablesReady"`
	ItemTablesReady        bool                           `json:"itemTablesReady"`
	Conflicts              []NaturalDropConflict          `json:"conflicts"`
}

type NaturalDropSelection struct {
	TypeHash  string `json:"typeHash"`
	MainTrait string `json:"mainTrait"`
	SubParam  string `json:"subParam"`
}

type NaturalDropItemSelection struct {
	ItemHash string `json:"itemHash"`
	Quantity int    `json:"quantity"`
	Weight   uint32 `json:"weight"`
}

type NaturalDropDeployRequest struct {
	SourceDir       string                            `json:"sourceDir"`
	GameExePath     string                            `json:"gameExePath"`
	Selections      []NaturalDropSelection            `json:"selections"`
	Sigils          []NaturalDropSigilSelection       `json:"sigils"`
	Wrightstones    []NaturalDropWrightstoneSelection `json:"wrightstones"`
	Items           []NaturalDropItemSelection        `json:"items"`
	ItemMultiplier  int                               `json:"itemMultiplier"`
	SigilOnly       bool                              `json:"sigilOnly"`
	WrightstoneOnly bool                              `json:"wrightstoneOnly"`
}

type NaturalDropRestoreRequest struct {
	GameExePath string `json:"gameExePath"`
}

type NaturalDropDeployResult struct {
	ModDir               string   `json:"modDir"`
	GeneratedFiles       []string `json:"generatedFiles"`
	SelectedSummons      int      `json:"selectedSummons"`
	SelectedSigils       int      `json:"selectedSigils"`
	SelectedWrightstones int      `json:"selectedWrightstones"`
	SelectedItems        int      `json:"selectedItems"`
	AffectedRewardPools  int      `json:"affectedRewardPools"`
	SourceDigest         string   `json:"sourceDigest"`
}

type NaturalDropStartupRecoveryStatus struct {
	Blocked bool   `json:"blocked"`
	Detail  string `json:"detail"`
}

type naturalDropManifest struct {
	SchemaVersion       int                               `json:"schemaVersion"`
	Owner               string                            `json:"owner"`
	GameVersion         string                            `json:"gameVersion"`
	GameExecutableSHA   string                            `json:"gameExecutableSha256"`
	GeneratedAt         string                            `json:"generatedAt"`
	OriginalIndexSHA    string                            `json:"originalIndexSha256"`
	DeployedIndexSHA    string                            `json:"deployedIndexSha256"`
	SourceFiles         map[string]string                 `json:"sourceFiles"`
	GeneratedFiles      map[string]string                 `json:"generatedFiles"`
	Selections          []NaturalDropSelection            `json:"selections"`
	Sigils              []NaturalDropSigilSelection       `json:"sigils,omitempty"`
	Wrightstones        []NaturalDropWrightstoneSelection `json:"wrightstones,omitempty"`
	Items               []NaturalDropItemSelection        `json:"items,omitempty"`
	ItemMultiplier      int                               `json:"itemMultiplier,omitempty"`
	SigilOnly           bool                              `json:"sigilOnly,omitempty"`
	WrightstoneOnly     bool                              `json:"wrightstoneOnly,omitempty"`
	AffectedRewardPools int                               `json:"affectedRewardPools"`
}

type naturalDropPreparedFile struct {
	BeforePresent bool   `json:"beforePresent"`
	BeforeSHA     string `json:"beforeSha256,omitempty"`
	AfterPresent  bool   `json:"afterPresent"`
	AfterSHA      string `json:"afterSha256,omitempty"`
	Snapshot      string `json:"snapshot,omitempty"`
}

type naturalDropPrepareJournal struct {
	SchemaVersion          int                                `json:"schemaVersion"`
	Owner                  string                             `json:"owner"`
	GameVersion            string                             `json:"gameVersion"`
	GameExecutableSHA      string                             `json:"gameExecutableSha256"`
	GameDirectory          string                             `json:"gameDirectory"`
	TargetIndexPath        string                             `json:"targetIndexPath"`
	BeforeIndexSHA         string                             `json:"beforeIndexSha256"`
	AfterIndexSHA          string                             `json:"afterIndexSha256"`
	BeforeIndexSnapshot    string                             `json:"beforeIndexSnapshot"`
	BeforeManifestPresent  bool                               `json:"beforeManifestPresent"`
	BeforeManifestSHA      string                             `json:"beforeManifestSha256,omitempty"`
	BeforeManifestSnapshot string                             `json:"beforeManifestSnapshot,omitempty"`
	AfterManifestSHA       string                             `json:"afterManifestSha256,omitempty"`
	RemoveBackupOnRecovery bool                               `json:"removeBackupOnRecovery"`
	BeforeBackupPresent    bool                               `json:"beforeBackupPresent,omitempty"`
	BeforeBackupSHA        string                             `json:"beforeBackupSha256,omitempty"`
	BeforeBackupSnapshot   string                             `json:"beforeBackupSnapshot,omitempty"`
	AfterBackupPresent     bool                               `json:"afterBackupPresent,omitempty"`
	AfterBackupSHA         string                             `json:"afterBackupSha256,omitempty"`
	GeneratedFiles         map[string]naturalDropPreparedFile `json:"generatedFiles"`
}

type naturalDropGameIdentityRecord struct {
	Version string
	SHA256  string
}

type naturalDropTables struct {
	Summon          []byte
	SummonLot       []byte
	RewardSummonLot []byte
}

type naturalDropItemTables struct {
	Rewards         []byte
	RewardLots      []byte
	EndlessPackages []byte
}

type naturalDropSummonRow struct {
	Offset    int
	SkillPool uint32
	EquipPool uint32
	TypeHash  uint32
}

func tableRowCount(data []byte, rowSize int) (int, error) {
	if len(data) < 8 {
		return 0, errors.New("table header is truncated")
	}
	count := int(binary.LittleEndian.Uint32(data[:4]))
	if count < 0 || 8+count*rowSize != len(data) {
		return 0, fmt.Errorf("table shape mismatch: rows=%d rowSize=%d bytes=%d", count, rowSize, len(data))
	}
	if binary.LittleEndian.Uint32(data[4:8]) != 0 {
		return 0, errors.New("table reserved header is not zero")
	}
	return count, nil
}

func fileSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func loadNaturalDropTables(sourceDir string, strictHash bool) (*naturalDropTables, []NaturalDropTableStatus, error) {
	bundled := naturalDropUsesBundledSource(sourceDir)
	if !bundled {
		sourceDir = filepath.Clean(strings.TrimSpace(sourceDir))
	}
	values := make(map[string][]byte, len(naturalDropRequiredTables))
	statuses := make([]NaturalDropTableStatus, 0, len(naturalDropRequiredTables))
	for _, required := range naturalDropRequiredTables {
		var data []byte
		var err error
		if bundled {
			data, err = naturalDropBundledTable(required.Name)
		} else {
			data, err = os.ReadFile(filepath.Join(sourceDir, required.Name))
		}
		if err != nil {
			return nil, statuses, fmt.Errorf("读取 %s 失败: %w", required.Name, err)
		}
		hash := fileSHA256(data)
		valid := len(data) == required.Size && strings.EqualFold(hash, required.SHA256)
		statuses = append(statuses, NaturalDropTableStatus{Name: required.Name, Size: len(data), SHA256: hash, Expected: required.SHA256, Valid: valid})
		if strictHash && !valid {
			return nil, statuses, fmt.Errorf("%s 不是已验证的 DLC 2.0.2 原表（大小 %d，SHA-256 %s）", required.Name, len(data), hash)
		}
		values[required.Name] = data
	}
	if _, err := tableRowCount(values["summon.tbl"], summonTableRowSize); err != nil {
		return nil, statuses, fmt.Errorf("summon.tbl: %w", err)
	}
	if _, err := tableRowCount(values["summon_lot.tbl"], summonLotRowSize); err != nil {
		return nil, statuses, fmt.Errorf("summon_lot.tbl: %w", err)
	}
	if _, err := tableRowCount(values["reward_summon_lot.tbl"], rewardSummonLotRowSize); err != nil {
		return nil, statuses, fmt.Errorf("reward_summon_lot.tbl: %w", err)
	}
	return &naturalDropTables{Summon: values["summon.tbl"], SummonLot: values["summon_lot.tbl"], RewardSummonLot: values["reward_summon_lot.tbl"]}, statuses, nil
}

func loadNaturalDropItemTables(sourceDir string, strictHash bool) (*naturalDropItemTables, []NaturalDropTableStatus, error) {
	bundled := naturalDropUsesBundledSource(sourceDir)
	if !bundled {
		sourceDir = filepath.Clean(strings.TrimSpace(sourceDir))
	}
	values := make(map[string][]byte, len(naturalDropItemRequiredTables))
	statuses := make([]NaturalDropTableStatus, 0, len(naturalDropItemRequiredTables))
	for _, required := range naturalDropItemRequiredTables {
		var data []byte
		var err error
		if bundled {
			data, err = naturalDropBundledTable(required.Name)
		} else {
			data, err = os.ReadFile(filepath.Join(sourceDir, required.Name))
		}
		if err != nil {
			return nil, statuses, fmt.Errorf("读取 %s 失败: %w", required.Name, err)
		}
		hash := fileSHA256(data)
		valid := len(data) == required.Size && strings.EqualFold(hash, required.SHA256)
		statuses = append(statuses, NaturalDropTableStatus{
			Name: required.Name, Size: len(data), SHA256: hash, Expected: required.SHA256, Valid: valid,
		})
		if strictHash && !valid {
			return nil, statuses, fmt.Errorf("%s 不是已验证的 DLC 2.0.2 原表（大小 %d，SHA-256 %s）", required.Name, len(data), hash)
		}
		values[required.Name] = data
	}
	if _, err := tableRowCount(values["reward.tbl"], rewardTableRowSize); err != nil {
		return nil, statuses, fmt.Errorf("reward.tbl: %w", err)
	}
	if _, err := tableRowCount(values["reward_lot.tbl"], rewardLotTableRowSize); err != nil {
		return nil, statuses, fmt.Errorf("reward_lot.tbl: %w", err)
	}
	if _, err := tableRowCount(values["endlessmode_package.tbl"], endlessPackageTableRowSize); err != nil {
		return nil, statuses, fmt.Errorf("endlessmode_package.tbl: %w", err)
	}
	return &naturalDropItemTables{
		Rewards:         values["reward.tbl"],
		RewardLots:      values["reward_lot.tbl"],
		EndlessPackages: values["endlessmode_package.tbl"],
	}, statuses, nil
}

func buildNaturalDropItemCatalog() ([]NaturalDropItemOption, error) {
	catalog, err := loadProgressionCatalog()
	if err != nil {
		return nil, err
	}
	result := make([]NaturalDropItemOption, 0, len(catalog.Items))
	for _, item := range catalog.Items {
		if item.Dangerous {
			continue
		}
		hash, err := ParseHashHex(item.Hash)
		if err != nil || hash == 0 || hash == summonInvalidTypeHash {
			continue
		}
		nameZh := strings.TrimSpace(item.NameCN)
		nameEn := strings.TrimSpace(item.NameEN)
		if nameZh == "" || nameEn == "" {
			continue
		}
		result = append(result, NaturalDropItemOption{
			Hash: fmt.Sprintf("0x%08X", hash), NameZh: nameZh, NameEn: nameEn, Category: item.Category,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Category != result[j].Category {
			return result[i].Category < result[j].Category
		}
		return result[i].NameZh < result[j].NameZh
	})
	return result, nil
}

func naturalDropSummonRows(data []byte) (map[uint32]naturalDropSummonRow, error) {
	count, err := tableRowCount(data, summonTableRowSize)
	if err != nil {
		return nil, err
	}
	rows := make(map[uint32]naturalDropSummonRow, count)
	for i := 0; i < count; i++ {
		offset := 8 + i*summonTableRowSize
		row := naturalDropSummonRow{
			Offset:    offset,
			SkillPool: binary.LittleEndian.Uint32(data[offset:]),
			EquipPool: binary.LittleEndian.Uint32(data[offset+8:]),
			TypeHash:  binary.LittleEndian.Uint32(data[offset+16:]),
		}
		if row.TypeHash == 0 || row.TypeHash == summonInvalidTypeHash {
			continue
		}
		if _, duplicate := rows[row.TypeHash]; duplicate {
			return nil, fmt.Errorf("summon.tbl contains duplicate type 0x%08X", row.TypeHash)
		}
		rows[row.TypeHash] = row
	}
	return rows, nil
}

func naturalDropLotRows(data []byte, pool uint32) ([][]byte, error) {
	count, err := tableRowCount(data, summonLotRowSize)
	if err != nil {
		return nil, err
	}
	rows := make([][]byte, 0, 12)
	for i := 0; i < count; i++ {
		offset := 8 + i*summonLotRowSize
		if binary.LittleEndian.Uint32(data[offset:]) == pool {
			rows = append(rows, data[offset:offset+summonLotRowSize])
		}
	}
	return rows, nil
}

func naturalDropRewardPoolCounts(data []byte) (map[uint32]int, map[uint32]int, error) {
	count, err := tableRowCount(data, rewardSummonLotRowSize)
	if err != nil {
		return nil, nil, err
	}
	typeCounts := make(map[uint32]int)
	poolCounts := make(map[uint32]int)
	for i := 0; i < count; i++ {
		offset := 8 + i*rewardSummonLotRowSize
		pool := binary.LittleEndian.Uint32(data[offset:])
		typeHash := binary.LittleEndian.Uint32(data[offset+4:])
		typeCounts[typeHash]++
		poolCounts[pool]++
	}
	return typeCounts, poolCounts, nil
}

func parseNaturalDropNameCatalog() (map[uint32][2]string, map[uint32][2]string, map[uint32][2]string, map[uint32][2]string, error) {
	var types summonTypeFile
	var skills summonSkillFile
	var subs summonSubParamFile
	if err := json.Unmarshal(summonTypesJSON, &types); err != nil {
		return nil, nil, nil, nil, err
	}
	if err := json.Unmarshal(summonSkillsJSON, &skills); err != nil {
		return nil, nil, nil, nil, err
	}
	if err := json.Unmarshal(summonSubParamsJSON, &subs); err != nil {
		return nil, nil, nil, nil, err
	}
	typeNames, typeKinds := make(map[uint32][2]string), make(map[uint32][2]string)
	mainNames, subNames := make(map[uint32][2]string), make(map[uint32][2]string)
	for _, item := range types.Summons {
		hash, err := ParseHashHex(item.Hash)
		if err != nil {
			continue
		}
		typeNames[hash] = [2]string{item.DisplayName, item.DisplayNameEN}
		typeKinds[hash] = [2]string{item.TypeName, item.TypeNameEN}
	}
	for _, item := range skills.Skills {
		hash, err := ParseHashHex(item.Hash)
		if err != nil {
			continue
		}
		mainNames[hash] = [2]string{item.DisplayName, item.DisplayNameEN}
	}
	for _, item := range subs.SubParams {
		hash, err := ParseHashHex(item.Hash)
		if err != nil {
			continue
		}
		subNames[hash] = [2]string{item.DisplayName, item.DisplayNameEN}
	}
	return typeNames, typeKinds, mainNames, subNames, nil
}

func naturalDropTraitOptions(hashes []string, rows [][]byte, names map[uint32][2]string) []NaturalDropTraitOption {
	available := make(map[uint32]bool, len(rows))
	for _, row := range rows {
		available[binary.LittleEndian.Uint32(row[4:])] = true
	}
	result := make([]NaturalDropTraitOption, 0, len(hashes))
	seen := make(map[uint32]bool)
	for _, raw := range hashes {
		hash, err := ParseHashHex(raw)
		if err != nil || !available[hash] || seen[hash] {
			continue
		}
		seen[hash] = true
		name := names[hash]
		zh, en := strings.TrimSpace(name[0]), strings.TrimSpace(name[1])
		if zh == "" {
			zh = fmt.Sprintf("0x%08X", hash)
		}
		if en == "" {
			en = fmt.Sprintf("0x%08X", hash)
		}
		result = append(result, NaturalDropTraitOption{Hash: fmt.Sprintf("0x%08X", hash), NameZh: zh, NameEn: en})
	}
	return result
}

func buildNaturalDropCatalog(tables *naturalDropTables) ([]NaturalDropSummonOption, error) {
	rules, err := loadSummonNaturalRules()
	if err != nil {
		return nil, err
	}
	rows, err := naturalDropSummonRows(tables.Summon)
	if err != nil {
		return nil, err
	}
	rewardTypes, _, err := naturalDropRewardPoolCounts(tables.RewardSummonLot)
	if err != nil {
		return nil, err
	}
	typeNames, typeKinds, mainNames, subNames, err := parseNaturalDropNameCatalog()
	if err != nil {
		return nil, err
	}
	result := make([]NaturalDropSummonOption, 0, len(rules))
	for _, rule := range rules {
		if rule.Mode == "固定" {
			continue
		}
		typeHash, err := ParseHashHex(rule.TypeHash)
		if err != nil {
			continue
		}
		row, exists := rows[typeHash]
		if !exists || rewardTypes[typeHash] == 0 {
			continue
		}
		mainRows, err := naturalDropLotRows(tables.SummonLot, row.SkillPool)
		if err != nil {
			return nil, err
		}
		subRows, err := naturalDropLotRows(tables.SummonLot, row.EquipPool)
		if err != nil {
			return nil, err
		}
		mainOptions := naturalDropTraitOptions(rule.MainTraitHashes, mainRows, mainNames)
		subOptions := naturalDropTraitOptions(rule.SubParamHashes, subRows, subNames)
		if len(mainOptions) == 0 || len(subOptions) == 0 {
			continue
		}
		name, kind := typeNames[typeHash], typeKinds[typeHash]
		nameZh, nameEn := strings.TrimSpace(name[0]), strings.TrimSpace(name[1])
		if nameZh == "" {
			nameZh = rule.Name + " · " + rule.Tier
		}
		if nameEn == "" {
			nameEn = fmt.Sprintf("Summon 0x%08X", typeHash)
		}
		result = append(result, NaturalDropSummonOption{
			TypeHash: fmt.Sprintf("0x%08X", typeHash), NameZh: nameZh, NameEn: nameEn,
			Tier: rule.Tier, TypeNameZh: kind[0], TypeNameEn: kind[1], RewardPools: rewardTypes[typeHash],
			MainTraits: mainOptions, SubParams: subOptions,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Tier != result[j].Tier {
			return result[i].Tier > result[j].Tier
		}
		return result[i].NameZh < result[j].NameZh
	})
	return result, nil
}

const (
	naturalDropManifestName         = "data.i.gbfr-codex-natural-drop.json"
	naturalDropBackupName           = "data.i.gbfr-codex-natural-drop.bak"
	naturalDropBackupPartialName    = "data.i.gbfr-codex-natural-drop.bak.partial"
	naturalDropPrepareJournalName   = "data.i.gbfr-codex-natural-drop.prepare.json"
	naturalDropCompletedJournalName = "data.i.gbfr-codex-natural-drop.completed.json"
	naturalDropPrepareDirectoryName = ".gbfr-codex-natural-drop.prepare"
	naturalDropTransactionLockName  = ".gbfr-codex-natural-drop.lock"
)

var naturalDropSummonTablePaths = []string{
	"system/table/summon.tbl",
	"system/table/summon_lot.tbl",
	"system/table/reward_summon_lot.tbl",
}

var naturalDropWrightstoneTablePaths = []string{
	"system/table/item_pendulum.tbl",
	"system/table/gacha_lot.tbl",
	"system/table/gacha_rate_group.tbl",
	"system/table/gacha.tbl",
}

var naturalDropItemTablePaths = []string{
	"system/table/reward_lot.tbl",
}

func naturalDropSourceFiles(includeSummons, includeWrightstones, includeSigils, includeItems bool) map[string]string {
	result := make(map[string]string, len(naturalDropRequiredTables)+len(naturalWrightstoneRequiredTables)+len(naturalSigilRequiredTables)+len(naturalDropItemRequiredTables))
	if includeSummons {
		for _, required := range naturalDropRequiredTables {
			result[required.Name] = required.SHA256
		}
	}
	if includeWrightstones {
		for _, required := range naturalWrightstoneRequiredTables {
			result[required.Name] = required.SHA256
		}
	}
	if includeSigils {
		for _, required := range naturalWrightstoneRequiredTables[1:] {
			result[required.Name] = required.SHA256
		}
		for _, required := range naturalSigilRequiredTables {
			result[required.Name] = required.SHA256
		}
	}
	if includeItems {
		for _, required := range naturalDropItemRequiredTables {
			result[required.Name] = required.SHA256
		}
	}
	return result
}

func naturalDropSourceDigest(sourceFiles map[string]string) string {
	parts := make([]string, 0, len(sourceFiles))
	for name, hash := range sourceFiles {
		parts = append(parts, name+":"+hash)
	}
	sort.Strings(parts)
	digest := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return strings.ToUpper(hex.EncodeToString(digest[:]))
}

func naturalDropFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(hasher.Sum(nil))), nil
}

func naturalDropCleanPath(path string) string {
	if naturalDropUsesBundledSource(path) {
		return naturalDropBundledSourceID
	}
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return filepath.Clean(path)
}

func naturalDropInstallPaths(gameExePath string) (gameDir, indexPath, backupPath, manifestPath string) {
	gameDir = filepath.Dir(gameExePath)
	indexPath = filepath.Join(gameDir, "data.i")
	backupPath = filepath.Join(gameDir, naturalDropBackupName)
	manifestPath = filepath.Join(gameDir, naturalDropManifestName)
	return
}

func naturalDropGamePathHash(path string) uint64 {
	normalized := strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
	return xxhash.Sum64String(normalized)
}

func naturalDropGameIdentity(path string) (version, digest string, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", "", fmt.Errorf("读取游戏程序失败: %w", err)
	}
	if info.IsDir() {
		return "", "", errors.New("所选游戏程序不是文件")
	}
	digest, err = naturalDropFileSHA256(path)
	if err != nil {
		return "", "", fmt.Errorf("校验游戏程序失败: %w", err)
	}
	switch {
	case info.Size() == naturalDropGame202ExecutableSize && strings.EqualFold(digest, runtimePatchCatalogGameSHA256):
		return naturalDropGameVersion, digest, nil
	case info.Size() == naturalDropGame203ExecutableSize && strings.EqualFold(digest, game203ExecutableSHA256):
		return naturalDropGame203Version, digest, nil
	case info.Size() == naturalDropGame204ExecutableSize && strings.EqualFold(digest, game204ExecutableSHA256):
		return naturalDropGame204Version, digest, nil
	default:
		return "", digest, fmt.Errorf("游戏程序版本不匹配（大小 %d，SHA-256 %s）", info.Size(), digest)
	}
}

func validateNaturalDropGameExecutable(path string) (string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return "", errors.New("请选择 granblue_fantasy_relink.exe")
	}
	if !strings.EqualFold(filepath.Base(path), gameExeName) {
		return "", fmt.Errorf("所选文件不是 %s", gameExeName)
	}
	if _, _, err := naturalDropGameIdentity(path); err != nil {
		return "", err
	}
	_, indexPath, _, _ := naturalDropInstallPaths(path)
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return "", fmt.Errorf("读取 data.i 失败: %w", err)
	}
	if _, err := parseGBFRDataIndex(data); err != nil {
		return "", err
	}
	return path, nil
}

func naturalDropManifestIdentitySupported(version, digest string) bool {
	switch {
	case version == naturalDropGameVersion && strings.EqualFold(digest, runtimePatchCatalogGameSHA256):
		return true
	case version == naturalDropGame203Version && strings.EqualFold(digest, game203ExecutableSHA256):
		return true
	case version == naturalDropGame204Version && strings.EqualFold(digest, game204ExecutableSHA256):
		return true
	default:
		return false
	}
}

func naturalDropRelativeTarget(gamePath string) string {
	return filepath.ToSlash(filepath.Join("data", filepath.FromSlash(gamePath)))
}

func naturalDropTargetPath(gameDir, gamePath string) string {
	return filepath.Join(gameDir, "data", filepath.FromSlash(gamePath))
}

func naturalDropReadManifest(gameDir string) (*naturalDropManifest, []byte, error) {
	path := filepath.Join(gameDir, naturalDropManifestName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var manifest naturalDropManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, data, err
	}
	if (manifest.SchemaVersion != 2 && manifest.SchemaVersion != 3 && manifest.SchemaVersion != 4) || manifest.Owner != naturalDropModID ||
		!naturalDropManifestIdentitySupported(manifest.GameVersion, manifest.GameExecutableSHA) {
		return nil, data, errors.New("天然掉落清单不属于当前版本的本工具")
	}
	for relative := range manifest.GeneratedFiles {
		clean := filepath.Clean(filepath.FromSlash(relative))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, data, fmt.Errorf("天然掉落清单包含越界路径: %q", relative)
		}
	}
	return &manifest, data, nil
}

func naturalDropOwnedInstall(gameDir string) (*naturalDropManifest, bool) {
	manifest, _, err := naturalDropReadManifest(gameDir)
	if err != nil {
		return nil, false
	}
	backupPath := filepath.Join(gameDir, naturalDropBackupName)
	digest, err := naturalDropFileSHA256(backupPath)
	if err != nil || !strings.EqualFold(digest, manifest.OriginalIndexSHA) {
		return nil, false
	}
	if data, err := os.ReadFile(backupPath); err != nil {
		return nil, false
	} else if index, err := parseGBFRDataIndex(data); err != nil || !strings.EqualFold(index.Codename, "relink") {
		return nil, false
	}
	return manifest, true
}

func naturalDropGeneratedFilesMatch(gameDir string, manifest *naturalDropManifest) bool {
	for relative, expected := range manifest.GeneratedFiles {
		digest, err := naturalDropFileSHA256(filepath.Join(gameDir, filepath.FromSlash(relative)))
		if err != nil || !strings.EqualFold(digest, expected) {
			return false
		}
	}
	return true
}

func naturalDropIndexSummary(index *gbfrDataIndex) string {
	if index == nil {
		return ""
	}
	return fmt.Sprintf("%s · %d 个归档文件 · %d 个外部文件", index.Codename, len(index.ArchiveFileHashes), len(index.ExternalFileHashes))
}

func naturalDropConflicts(gameDir string, index *gbfrDataIndex, owned bool) []NaturalDropConflict {
	if index == nil || owned {
		return nil
	}
	type scopedPath struct{ path, scope string }
	paths := make([]scopedPath, 0, len(naturalDropSummonTablePaths)+len(naturalDropWrightstoneTablePaths)+len(naturalDropSigilTablePaths)+len(naturalDropItemTablePaths))
	for _, path := range naturalDropSummonTablePaths {
		paths = append(paths, scopedPath{path: path, scope: "summon"})
	}
	paths = append(paths, scopedPath{path: naturalDropWrightstoneTablePaths[0], scope: "wrightstone"})
	for _, path := range naturalDropWrightstoneTablePaths[1:] {
		paths = append(paths, scopedPath{path: path, scope: "transmarvel"})
	}
	for _, path := range naturalDropSigilTablePaths {
		paths = append(paths, scopedPath{path: path, scope: "sigil"})
	}
	for _, path := range naturalDropItemTablePaths {
		paths = append(paths, scopedPath{path: path, scope: "item"})
	}
	result := make([]NaturalDropConflict, 0, len(paths))
	for _, candidate := range paths {
		gamePath := candidate.path
		hash := naturalDropGamePathHash(gamePath)
		externalAt := sort.Search(len(index.ExternalFileHashes), func(i int) bool { return index.ExternalFileHashes[i] >= hash })
		target := naturalDropTargetPath(gameDir, gamePath)
		_, fileErr := os.Stat(target)
		if (externalAt < len(index.ExternalFileHashes) && index.ExternalFileHashes[externalAt] == hash) || fileErr == nil {
			result = append(result, NaturalDropConflict{
				ModID: "external-file",
				Path:  target,
				File:  naturalDropRelativeTarget(gamePath),
				Scope: candidate.scope,
			})
		}
	}
	return result
}

func (a *App) SelectNaturalDropTableDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择 DLC 2.0.2 解包表目录（system/table）"})
}

func (a *App) SelectNaturalDropGameExecutable() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "选择 granblue_fantasy_relink.exe",
		Filters: []runtime.FileFilter{{DisplayName: "Granblue Fantasy: Relink", Pattern: "granblue_fantasy_relink.exe"}},
	})
}

func (a *App) rememberNaturalDropGameExecutable(path string) {
	_ = a.updateConfig(func(config *AppConfig) {
		config.NaturalDropGameExePath = path
	})
}

func (a *App) setNaturalDropStartupRecoveryError(err error) {
	a.naturalDropRecoveryStatusMu.Lock()
	defer a.naturalDropRecoveryStatusMu.Unlock()
	a.naturalDropRecoveryStatus = NaturalDropStartupRecoveryStatus{
		Blocked: true,
		Detail: fmt.Sprintf(
			"上次掉落与锻造规则部署尚未安全恢复。请完全退出游戏和其他工具实例，再打开“掉落与锻造规则”重试；在恢复完成前不要启动游戏。详情：%v",
			err,
		),
	}
}

func (a *App) clearNaturalDropStartupRecoveryError() {
	a.naturalDropRecoveryStatusMu.Lock()
	a.naturalDropRecoveryStatus = NaturalDropStartupRecoveryStatus{}
	a.naturalDropRecoveryStatusMu.Unlock()
}

func (a *App) GetNaturalDropStartupRecoveryStatus() NaturalDropStartupRecoveryStatus {
	a.naturalDropRecoveryStatusMu.Lock()
	defer a.naturalDropRecoveryStatusMu.Unlock()
	return a.naturalDropRecoveryStatus
}

func (a *App) GetNaturalDropWorkspace(sourceDir, gameExePath string) (*NaturalDropWorkspace, error) {
	a.naturalDropMu.Lock()
	defer a.naturalDropMu.Unlock()

	workspace := &NaturalDropWorkspace{
		GameVersion: naturalDropGameVersion,
		SourceDir:   naturalDropCleanPath(sourceDir),
		GameExePath: naturalDropCleanPath(gameExePath),
	}
	if strings.TrimSpace(sourceDir) == "" {
		sourceDir = naturalDropBundledSourceID
		workspace.SourceDir = sourceDir
	}
	tables, statuses, err := loadNaturalDropTables(sourceDir, true)
	workspace.Tables = statuses
	if err != nil {
		return nil, err
	}
	workspace.SummonTablesReady = true
	workspace.Summons, err = buildNaturalDropCatalog(tables)
	if err != nil {
		return nil, err
	}
	wrightstoneTables, wrightstoneStatuses, wrightstoneErr := loadNaturalWrightstoneTables(sourceDir, true)
	workspace.Tables = append(workspace.Tables, wrightstoneStatuses...)
	if wrightstoneErr != nil {
		return nil, wrightstoneErr
	}
	if wrightstoneTables != nil {
		workspace.WrightstoneTablesReady = true
		workspace.Wrightstones, err = buildNaturalWrightstoneCatalog()
		if err != nil {
			return nil, err
		}
	}
	sigilTables, sigilStatuses, sigilErr := loadNaturalSigilTables(sourceDir, true)
	workspace.Tables = append(workspace.Tables, sigilStatuses...)
	if sigilErr != nil {
		return nil, sigilErr
	}
	if sigilTables != nil && wrightstoneTables != nil {
		workspace.SigilTablesReady = true
		workspace.Sigils, err = buildNaturalSigilCatalog(wrightstoneTables.Lots, sigilTables.Gem)
		if err != nil {
			return nil, err
		}
	}
	itemTables, itemStatuses, itemErr := loadNaturalDropItemTables(sourceDir, true)
	workspace.Tables = append(workspace.Tables, itemStatuses...)
	if itemErr != nil {
		return nil, itemErr
	}
	if _, err := resolveNaturalDropItemRewardPool(itemTables); err != nil {
		return nil, err
	}
	workspace.Items, err = buildNaturalDropItemCatalog()
	if err != nil {
		return nil, err
	}
	workspace.ItemTablesReady = true
	workspace.ItemRewardTargetZh = "无尽模式 · 锻造师奖励"
	workspace.ItemRewardTargetEn = "Endless Mode · Forger's Bounty"
	if strings.TrimSpace(gameExePath) == "" {
		return workspace, nil
	}
	validated, err := validateNaturalDropGameExecutable(gameExePath)
	if err != nil {
		return nil, err
	}
	workspace.GameExePath = validated
	a.rememberNaturalDropGameExecutable(validated)
	gameDir, indexPath, backupPath, _ := naturalDropInstallPaths(validated)
	releaseLease, err := acquireNaturalDropTransactionLease(gameDir)
	if err != nil {
		return nil, err
	}
	defer releaseLease()
	if err := naturalDropRecoverPreparedTransactionIfSafe(gameDir); err != nil {
		a.setNaturalDropStartupRecoveryError(err)
		return nil, fmt.Errorf("恢复上次未完成的天然掉落部署失败: %w", err)
	}
	a.clearNaturalDropStartupRecoveryError()
	workspace.GameDir, workspace.IndexPath = gameDir, indexPath
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}
	index, err := parseGBFRDataIndex(indexData)
	if err != nil {
		return nil, err
	}
	workspace.IndexValid = true
	workspace.IndexSummary = naturalDropIndexSummary(index)
	manifest, owned := naturalDropOwnedInstall(gameDir)
	workspace.Owned = owned
	workspace.BackupReady = owned
	if owned {
		currentDigest := fileSHA256(indexData)
		workspace.Installed = strings.EqualFold(currentDigest, manifest.DeployedIndexSHA) && naturalDropGeneratedFilesMatch(gameDir, manifest)
	} else if _, err := os.Stat(backupPath); err == nil {
		workspace.BackupReady = true
	}
	workspace.Conflicts = naturalDropConflicts(gameDir, index, owned)
	return workspace, nil
}

func parseNaturalDropSelection(value string, label string) (uint32, error) {
	hash, err := ParseHashHex(strings.TrimSpace(value))
	if err != nil || hash == 0 || hash == summonInvalidTypeHash {
		return 0, fmt.Errorf("%s哈希无效: %q", label, value)
	}
	return hash, nil
}

func resolveNaturalDropItemRewardPool(tables *naturalDropItemTables) (uint32, error) {
	if tables == nil {
		return 0, errors.New("通用物品掉落原表为空")
	}
	packageCount, err := tableRowCount(tables.EndlessPackages, endlessPackageTableRowSize)
	if err != nil {
		return 0, fmt.Errorf("endlessmode_package.tbl: %w", err)
	}
	if packageCount <= naturalDropDefaultPackageIndex {
		return 0, errors.New("内置锻造师奖励包记录缺失")
	}
	packageOffset := 8 + naturalDropDefaultPackageIndex*endlessPackageTableRowSize
	packageKey := binary.LittleEndian.Uint32(tables.EndlessPackages[packageOffset+64:])
	rewardKey := binary.LittleEndian.Uint32(tables.EndlessPackages[packageOffset+68:])
	if packageKey != naturalDropDefaultPackageKey || rewardKey != naturalDropDefaultRewardKey {
		return 0, fmt.Errorf("内置锻造师奖励包关系不匹配: package=0x%08X reward=0x%08X", packageKey, rewardKey)
	}

	rewardCount, err := tableRowCount(tables.Rewards, rewardTableRowSize)
	if err != nil {
		return 0, fmt.Errorf("reward.tbl: %w", err)
	}
	var pool uint32
	for i := 0; i < rewardCount; i++ {
		offset := 8 + i*rewardTableRowSize
		if binary.LittleEndian.Uint32(tables.Rewards[offset+24:]) == rewardKey {
			pool = binary.LittleEndian.Uint32(tables.Rewards[offset:])
			break
		}
	}
	if pool != naturalDropDefaultItemPool {
		return 0, fmt.Errorf("内置锻造师奖励池关系不匹配: 0x%08X", pool)
	}

	lotCount, err := tableRowCount(tables.RewardLots, rewardLotTableRowSize)
	if err != nil {
		return 0, fmt.Errorf("reward_lot.tbl: %w", err)
	}
	for i := 0; i < lotCount; i++ {
		offset := 8 + i*rewardLotTableRowSize
		if binary.LittleEndian.Uint32(tables.RewardLots[offset+8:]) == pool {
			return pool, nil
		}
	}
	return 0, fmt.Errorf("内置锻造师奖励池 0x%08X 没有可复制的物品行", pool)
}

func validateNaturalDropItemMultiplier(multiplier int) (int, error) {
	switch multiplier {
	case 1, 2, 4, 8, 16:
		return multiplier, nil
	default:
		return 0, errors.New("物品奖励倍率必须是 1、2、4、8 或 16")
	}
}

func patchNaturalDropItemTable(tables *naturalDropItemTables, selections []NaturalDropItemSelection, multiplier int) ([]byte, error) {
	multiplier, err := validateNaturalDropItemMultiplier(multiplier)
	if err != nil {
		return nil, err
	}
	pool, err := resolveNaturalDropItemRewardPool(tables)
	if err != nil {
		return nil, err
	}
	if _, err := loadProgressionCatalog(); err != nil {
		return nil, err
	}
	result := append([]byte(nil), tables.RewardLots...)
	count, err := tableRowCount(result, rewardLotTableRowSize)
	if err != nil {
		return nil, err
	}
	template := []byte(nil)
	existing := make(map[uint32]int)
	for i := 0; i < count; i++ {
		offset := 8 + i*rewardLotTableRowSize
		if binary.LittleEndian.Uint32(result[offset+8:]) != pool {
			continue
		}
		if template == nil {
			template = append([]byte(nil), result[offset:offset+rewardLotTableRowSize]...)
		}
		itemHash := binary.LittleEndian.Uint32(result[offset+12:])
		weaponHash := binary.LittleEndian.Uint32(result[offset+16:])
		gemHash := binary.LittleEndian.Uint32(result[offset+20:])
		if itemHash != 0 && itemHash != summonInvalidTypeHash &&
			weaponHash == summonInvalidTypeHash && gemHash == summonInvalidTypeHash {
			existing[itemHash] = offset
		}
	}
	if template == nil {
		return nil, errors.New("内置锻造师奖励池没有模板行")
	}

	seen := make(map[uint32]bool, len(selections))
	for _, selection := range selections {
		itemHash, err := parseNaturalDropSelection(selection.ItemHash, "物品")
		if err != nil {
			return nil, err
		}
		def, allowed := progressionItemByHash[itemHash]
		if !allowed || def.Dangerous {
			return nil, fmt.Errorf("物品 0x%08X 不在可掉落目录中", itemHash)
		}
		if seen[itemHash] {
			return nil, fmt.Errorf("物品 %s 被重复添加", progressionItemName(def))
		}
		seen[itemHash] = true
		if selection.Quantity < 1 || selection.Quantity > naturalDropItemMaxQuantity {
			return nil, fmt.Errorf("%s 数量必须为 1–%d", progressionItemName(def), naturalDropItemMaxQuantity)
		}
		quantity := selection.Quantity * multiplier
		if quantity > naturalDropItemMaxQuantity {
			return nil, fmt.Errorf("%s 的数量 %d × %d 超过表字段上限 %d", progressionItemName(def), selection.Quantity, multiplier, naturalDropItemMaxQuantity)
		}
		if selection.Weight < 1 || selection.Weight > naturalDropItemMaxWeight {
			return nil, fmt.Errorf("%s 权重必须为 1–%d", progressionItemName(def), naturalDropItemMaxWeight)
		}

		offset, exists := existing[itemHash]
		if !exists {
			row := append([]byte(nil), template...)
			binary.LittleEndian.PutUint32(row[12:16], itemHash)
			result = append(result, row...)
			offset = len(result) - rewardLotTableRowSize
			existing[itemHash] = offset
		}
		binary.LittleEndian.PutUint32(result[offset:offset+4], uint32(quantity))
		binary.LittleEndian.PutUint32(result[offset+48:offset+52], selection.Weight)
	}
	binary.LittleEndian.PutUint32(result[:4], uint32((len(result)-8)/rewardLotTableRowSize))
	if _, err := tableRowCount(result, rewardLotTableRowSize); err != nil {
		return nil, fmt.Errorf("生成 reward_lot.tbl 回读失败: %w", err)
	}
	return result, nil
}

func uniqueNaturalDropPoolHash(typeHash uint32, occupied map[uint32]bool) (uint32, error) {
	for attempt := 0; attempt < 128; attempt++ {
		candidate := gbfrHash32(fmt.Sprintf("CODEX_NATURAL_DROP_%08X_%02d", typeHash, attempt))
		if candidate != 0 && candidate != summonInvalidTypeHash && !occupied[candidate] {
			occupied[candidate] = true
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("无法为召唤石 0x%08X 分配独立掉落池", typeHash)
}

func cloneAndPinNaturalDropPool(sourceRows [][]byte, newPool, chosen uint32) ([]byte, error) {
	if len(sourceRows) == 0 {
		return nil, errors.New("天然附加词条池为空")
	}
	found := false
	result := make([]byte, 0, len(sourceRows)*summonLotRowSize)
	for _, source := range sourceRows {
		if len(source) != summonLotRowSize {
			return nil, errors.New("天然附加词条池行宽异常")
		}
		row := append([]byte(nil), source...)
		binary.LittleEndian.PutUint32(row[0:4], newPool)
		value := binary.LittleEndian.Uint32(row[4:8])
		weight := naturalDropLowWeight
		if value == chosen {
			weight = naturalDropForcedWeight
			found = true
		}
		binary.LittleEndian.PutUint32(row[12:16], weight)
		result = append(result, row...)
	}
	if !found {
		return nil, fmt.Errorf("所选词条 0x%08X 不在对应原始掉落池", chosen)
	}
	return result, nil
}

func patchNaturalDropTables(source *naturalDropTables, selections []NaturalDropSelection) (*naturalDropTables, int, error) {
	if source == nil {
		return nil, 0, errors.New("天然掉落原表为空")
	}
	summon := append([]byte(nil), source.Summon...)
	lot := append([]byte(nil), source.SummonLot...)
	reward := append([]byte(nil), source.RewardSummonLot...)
	rows, err := naturalDropSummonRows(summon)
	if err != nil {
		return nil, 0, err
	}
	_, poolCounts, err := naturalDropRewardPoolCounts(reward)
	if err != nil {
		return nil, 0, err
	}
	occupied := make(map[uint32]bool)
	for pool := range poolCounts {
		occupied[pool] = true
	}
	seenTypes := make(map[uint32]bool)
	selectedRows := make(map[uint32]naturalDropSummonRow)
	for _, selection := range selections {
		typeHash, err := parseNaturalDropSelection(selection.TypeHash, "召唤石种类")
		if err != nil {
			return nil, 0, err
		}
		mainHash, err := parseNaturalDropSelection(selection.MainTrait, "主加护")
		if err != nil {
			return nil, 0, err
		}
		subHash, err := parseNaturalDropSelection(selection.SubParam, "附加词条")
		if err != nil {
			return nil, 0, err
		}
		if seenTypes[typeHash] {
			return nil, 0, fmt.Errorf("召唤石 0x%08X 被重复配置", typeHash)
		}
		seenTypes[typeHash] = true
		row, exists := rows[typeHash]
		if !exists {
			return nil, 0, fmt.Errorf("召唤石 0x%08X 不在 summon.tbl", typeHash)
		}
		mainRows, err := naturalDropLotRows(lot, row.SkillPool)
		if err != nil {
			return nil, 0, err
		}
		newSkillPool, err := uniqueNaturalDropPoolHash(typeHash, occupied)
		if err != nil {
			return nil, 0, err
		}
		appendedMain, err := cloneAndPinNaturalDropPool(mainRows, newSkillPool, mainHash)
		if err != nil {
			return nil, 0, fmt.Errorf("召唤石 0x%08X 主加护: %w", typeHash, err)
		}
		lot = append(lot, appendedMain...)
		binary.LittleEndian.PutUint32(lot[:4], uint32((len(lot)-8)/summonLotRowSize))
		binary.LittleEndian.PutUint32(summon[row.Offset:row.Offset+4], newSkillPool)

		subRows, err := naturalDropLotRows(lot, row.EquipPool)
		if err != nil {
			return nil, 0, err
		}
		newPool, err := uniqueNaturalDropPoolHash(typeHash, occupied)
		if err != nil {
			return nil, 0, err
		}
		appended, err := cloneAndPinNaturalDropPool(subRows, newPool, subHash)
		if err != nil {
			return nil, 0, fmt.Errorf("召唤石 0x%08X 附加词条: %w", typeHash, err)
		}
		lot = append(lot, appended...)
		binary.LittleEndian.PutUint32(lot[:4], uint32((len(lot)-8)/summonLotRowSize))
		binary.LittleEndian.PutUint32(summon[row.Offset+8:row.Offset+12], newPool)
		row.SkillPool, row.EquipPool = newSkillPool, newPool
		selectedRows[typeHash] = row
	}
	binary.LittleEndian.PutUint32(lot[:4], uint32((len(lot)-8)/summonLotRowSize))

	rewardCount, err := tableRowCount(reward, rewardSummonLotRowSize)
	if err != nil {
		return nil, 0, err
	}
	affectedPools := make(map[uint32]bool)
	for i := 0; i < rewardCount; i++ {
		offset := 8 + i*rewardSummonLotRowSize
		pool := binary.LittleEndian.Uint32(reward[offset:])
		typeHash := binary.LittleEndian.Uint32(reward[offset+4:])
		if _, selected := selectedRows[typeHash]; selected {
			affectedPools[pool] = true
		}
	}
	for i := 0; i < rewardCount; i++ {
		offset := 8 + i*rewardSummonLotRowSize
		pool := binary.LittleEndian.Uint32(reward[offset:])
		if !affectedPools[pool] {
			continue
		}
		typeHash := binary.LittleEndian.Uint32(reward[offset+4:])
		weight := naturalDropLowWeight
		if _, selected := selectedRows[typeHash]; selected {
			weight = naturalDropForcedWeight
		}
		binary.LittleEndian.PutUint32(reward[offset+8:offset+12], weight)
	}
	if _, err := tableRowCount(summon, summonTableRowSize); err != nil {
		return nil, 0, fmt.Errorf("生成后的 summon.tbl: %w", err)
	}
	if _, err := tableRowCount(lot, summonLotRowSize); err != nil {
		return nil, 0, fmt.Errorf("生成后的 summon_lot.tbl: %w", err)
	}
	return &naturalDropTables{Summon: summon, SummonLot: lot, RewardSummonLot: reward}, len(affectedPools), nil
}

func naturalDropRequireStoppedProcesses() error {
	if _, err := findProcessByName(charaProcessName); err == nil {
		return errors.New("部署或恢复天然掉落前请先完全退出游戏")
	}
	return nil
}

func cloneGBFRDataIndex(index *gbfrDataIndex) *gbfrDataIndex {
	if index == nil {
		return nil
	}
	clone := *index
	clone.ArchiveFileHashes = append([]uint64(nil), index.ArchiveFileHashes...)
	clone.FileToChunkIndexers = append([]gbfrFileToChunkIndexer(nil), index.FileToChunkIndexers...)
	clone.Chunks = append([]gbfrDataChunk(nil), index.Chunks...)
	clone.ExternalFileHashes = append([]uint64(nil), index.ExternalFileHashes...)
	clone.ExternalFileSizes = append([]uint64(nil), index.ExternalFileSizes...)
	clone.CachedChunkIndices = append([]uint32(nil), index.CachedChunkIndices...)
	return &clone
}

func naturalDropBuildIndex(base []byte, files map[string][]byte) ([]byte, error) {
	index, err := parseGBFRDataIndex(base)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(index.Codename, "relink") {
		return nil, fmt.Errorf("data.i 标识为 %q；请先恢复游戏原始索引，避免覆盖其他加载器的临时索引", index.Codename)
	}
	patched := cloneGBFRDataIndex(index)
	paths := make([]string, 0, len(files))
	for gamePath := range files {
		paths = append(paths, gamePath)
	}
	sort.Strings(paths)
	for _, gamePath := range paths {
		data := files[gamePath]
		registerGBFRExternalFile(patched, naturalDropGamePathHash(gamePath), uint64(len(data)))
	}
	result, err := buildGBFRDataIndex(patched)
	if err != nil {
		return nil, err
	}
	readback, err := parseGBFRDataIndex(result)
	if err != nil {
		return nil, err
	}
	for _, gamePath := range paths {
		hash := naturalDropGamePathHash(gamePath)
		at := sort.Search(len(readback.ExternalFileHashes), func(i int) bool { return readback.ExternalFileHashes[i] >= hash })
		if at >= len(readback.ExternalFileHashes) || readback.ExternalFileHashes[at] != hash || readback.ExternalFileSizes[at] != uint64(len(files[gamePath])) {
			return nil, fmt.Errorf("生成索引未正确登记 %s", gamePath)
		}
	}
	return result, nil
}

func naturalDropWriteAtomic(path string, data []byte) error {
	return writeFileAtomicVerified(path, data)
}

func naturalDropCanonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	return absolute, nil
}

func acquireNaturalDropTransactionLease(gameDir string) (func(), error) {
	canonicalGameDir, err := naturalDropCanonicalPath(gameDir)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(canonicalGameDir, naturalDropTransactionLockName)
	handle, err := createExclusiveDeleteOnCloseFile(path)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return nil, errors.New("另一个工具实例正在检查、部署或恢复天然掉落，请等待其完成")
		}
		return nil, fmt.Errorf("创建天然掉落跨进程事务锁失败: %w", err)
	}
	return func() {
		_ = windows.CloseHandle(handle)
	}, nil
}

func naturalDropSamePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if filepath.Separator == '\\' {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func naturalDropAllowedGeneratedFiles() map[string]bool {
	result := make(map[string]bool)
	groups := [][]string{
		naturalDropSummonTablePaths,
		naturalDropWrightstoneTablePaths,
		naturalDropSigilTablePaths,
		naturalDropItemTablePaths,
	}
	for _, group := range groups {
		for _, gamePath := range group {
			result[naturalDropRelativeTarget(gamePath)] = true
		}
	}
	return result
}

func naturalDropValidatePreparedRelative(relative string) error {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relative)))
	if clean != relative || !naturalDropAllowedGeneratedFiles()[relative] {
		return fmt.Errorf("天然掉落事务包含非本工具目标文件: %q", relative)
	}
	return nil
}

func naturalDropPrepareSnapshotPath(gameDir, name string) (string, error) {
	if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return "", fmt.Errorf("天然掉落事务快照名称无效: %q", name)
	}
	return filepath.Join(gameDir, naturalDropPrepareDirectoryName, name), nil
}

func naturalDropBeginPreparedTransaction(
	gameDir string,
	beforeIndex []byte,
	afterIndex []byte,
	beforeFiles map[string][]byte,
	beforeManifest []byte,
	afterFiles map[string][]byte,
	removeBackupOnRecovery bool,
	afterManifest []byte,
	identities ...naturalDropGameIdentityRecord,
) (resultErr error) {
	canonicalGameDir, err := naturalDropCanonicalPath(gameDir)
	if err != nil {
		return err
	}
	indexPath := filepath.Join(canonicalGameDir, "data.i")
	identity := naturalDropGameIdentityRecord{Version: naturalDropGameVersion, SHA256: runtimePatchCatalogGameSHA256}
	if len(identities) > 1 {
		return errors.New("天然掉落事务收到多个游戏版本身份")
	}
	if len(identities) == 1 {
		identity = identities[0]
	}
	if !naturalDropManifestIdentitySupported(identity.Version, identity.SHA256) {
		return errors.New("天然掉落事务的游戏版本身份无效")
	}
	journalPath := filepath.Join(canonicalGameDir, naturalDropPrepareJournalName)
	prepareDir := filepath.Join(canonicalGameDir, naturalDropPrepareDirectoryName)
	if _, err := os.Stat(journalPath); err == nil {
		return errors.New("发现尚未恢复的天然掉落部署事务")
	} else if !os.IsNotExist(err) {
		return err
	}
	completedJournalPath := filepath.Join(canonicalGameDir, naturalDropCompletedJournalName)
	if err := os.Remove(completedJournalPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("清理已完成的天然掉落事务标记失败: %w", err)
	}
	if _, err := parseGBFRDataIndex(beforeIndex); err != nil {
		return fmt.Errorf("部署前 data.i 快照无效: %w", err)
	}
	if _, err := parseGBFRDataIndex(afterIndex); err != nil {
		return fmt.Errorf("待部署 data.i 快照无效: %w", err)
	}
	if err := os.RemoveAll(prepareDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("清理旧事务暂存目录失败: %w", err)
	}
	if err := os.MkdirAll(prepareDir, 0o700); err != nil {
		return err
	}
	journalWritten := false
	defer func() {
		if resultErr != nil && !journalWritten {
			resultErr = errors.Join(resultErr, os.RemoveAll(prepareDir))
		}
	}()

	indexSnapshot := "data.i.before"
	indexSnapshotPath, _ := naturalDropPrepareSnapshotPath(canonicalGameDir, indexSnapshot)
	if err := naturalDropWriteAtomic(indexSnapshotPath, beforeIndex); err != nil {
		return err
	}
	journal := naturalDropPrepareJournal{
		SchemaVersion:          1,
		Owner:                  naturalDropModID,
		GameVersion:            identity.Version,
		GameExecutableSHA:      identity.SHA256,
		GameDirectory:          canonicalGameDir,
		TargetIndexPath:        indexPath,
		BeforeIndexSHA:         fileSHA256(beforeIndex),
		AfterIndexSHA:          fileSHA256(afterIndex),
		BeforeIndexSnapshot:    indexSnapshot,
		RemoveBackupOnRecovery: removeBackupOnRecovery,
		GeneratedFiles:         make(map[string]naturalDropPreparedFile, len(beforeFiles)+len(afterFiles)),
	}
	if beforeManifest != nil {
		journal.BeforeManifestPresent = true
		journal.BeforeManifestSHA = fileSHA256(beforeManifest)
		journal.BeforeManifestSnapshot = "manifest.before"
		snapshotPath, _ := naturalDropPrepareSnapshotPath(canonicalGameDir, journal.BeforeManifestSnapshot)
		if err := naturalDropWriteAtomic(snapshotPath, beforeManifest); err != nil {
			return err
		}
	}
	if afterManifest != nil {
		journal.AfterManifestSHA = fileSHA256(afterManifest)
	}
	relativeSet := make(map[string]bool, len(beforeFiles)+len(afterFiles))
	for relative := range beforeFiles {
		relativeSet[relative] = true
	}
	for relative := range afterFiles {
		relativeSet[relative] = true
	}
	relatives := make([]string, 0, len(relativeSet))
	for relative := range relativeSet {
		if err := naturalDropValidatePreparedRelative(relative); err != nil {
			return err
		}
		if _, ok := beforeFiles[relative]; !ok {
			return fmt.Errorf("天然掉落事务缺少目标文件的部署前快照: %s", relative)
		}
		relatives = append(relatives, relative)
	}
	sort.Strings(relatives)
	for index, relative := range relatives {
		before := beforeFiles[relative]
		after, afterPresent := afterFiles[relative]
		entry := naturalDropPreparedFile{AfterPresent: afterPresent && after != nil}
		if entry.AfterPresent {
			entry.AfterSHA = fileSHA256(after)
		}
		if before != nil {
			entry.BeforePresent = true
			entry.BeforeSHA = fileSHA256(before)
			entry.Snapshot = fmt.Sprintf("file-%03d.before", index)
			snapshotPath, _ := naturalDropPrepareSnapshotPath(canonicalGameDir, entry.Snapshot)
			if err := naturalDropWriteAtomic(snapshotPath, before); err != nil {
				return err
			}
		}
		journal.GeneratedFiles[relative] = entry
	}
	journalData, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	journalData = append(journalData, '\n')
	if err := naturalDropWriteAtomic(journalPath, journalData); err != nil {
		return err
	}
	journalWritten = true
	return nil
}

func naturalDropReadPreparedTransaction(gameDir string) (*naturalDropPrepareJournal, error) {
	canonicalGameDir, err := naturalDropCanonicalPath(gameDir)
	if err != nil {
		return nil, err
	}
	journalPath := filepath.Join(canonicalGameDir, naturalDropPrepareJournalName)
	data, err := os.ReadFile(journalPath)
	if err != nil {
		return nil, err
	}
	var journal naturalDropPrepareJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return nil, fmt.Errorf("天然掉落事务日志损坏: %w", err)
	}
	if journal.SchemaVersion != 1 || journal.Owner != naturalDropModID ||
		!naturalDropManifestIdentitySupported(journal.GameVersion, journal.GameExecutableSHA) {
		return nil, errors.New("天然掉落事务日志不属于当前版本的本工具")
	}
	if !naturalDropSamePath(journal.GameDirectory, canonicalGameDir) ||
		!naturalDropSamePath(journal.TargetIndexPath, filepath.Join(canonicalGameDir, "data.i")) {
		return nil, errors.New("天然掉落事务日志绑定了其他游戏目录，拒绝恢复")
	}
	if journal.BeforeIndexSHA == "" || journal.AfterIndexSHA == "" {
		return nil, errors.New("天然掉落事务日志缺少 data.i 前后校验值")
	}
	if _, err := naturalDropPrepareSnapshotPath(canonicalGameDir, journal.BeforeIndexSnapshot); err != nil {
		return nil, err
	}
	if journal.BeforeManifestPresent {
		if journal.BeforeManifestSHA == "" {
			return nil, errors.New("天然掉落事务日志缺少原部署清单校验值")
		}
		if _, err := naturalDropPrepareSnapshotPath(canonicalGameDir, journal.BeforeManifestSnapshot); err != nil {
			return nil, err
		}
	}
	if journal.BeforeBackupPresent {
		if journal.BeforeBackupSHA == "" {
			return nil, errors.New("天然掉落事务缺少部署前备份校验值")
		}
		if _, err := naturalDropPrepareSnapshotPath(canonicalGameDir, journal.BeforeBackupSnapshot); err != nil {
			return nil, err
		}
	}
	if journal.AfterBackupPresent && journal.AfterBackupSHA == "" {
		return nil, errors.New("天然掉落事务缺少部署后备份校验值")
	}
	for relative, entry := range journal.GeneratedFiles {
		if err := naturalDropValidatePreparedRelative(relative); err != nil {
			return nil, err
		}
		if entry.BeforePresent {
			if entry.BeforeSHA == "" {
				return nil, fmt.Errorf("天然掉落事务缺少 %s 的原文件校验值", relative)
			}
			if _, err := naturalDropPrepareSnapshotPath(canonicalGameDir, entry.Snapshot); err != nil {
				return nil, err
			}
		} else if entry.BeforeSHA != "" || entry.Snapshot != "" {
			return nil, fmt.Errorf("天然掉落事务的缺失文件 %s 带有无效快照", relative)
		}
		if entry.AfterPresent && entry.AfterSHA == "" {
			return nil, fmt.Errorf("天然掉落事务缺少 %s 的待部署校验值", relative)
		}
	}
	return &journal, nil
}

func naturalDropAddPreparedBackupTransition(gameDir string, beforeBackup []byte, afterBackup []byte) error {
	canonicalGameDir, err := naturalDropCanonicalPath(gameDir)
	if err != nil {
		return err
	}
	journal, err := naturalDropReadPreparedTransaction(canonicalGameDir)
	if err != nil {
		return err
	}
	if beforeBackup != nil {
		journal.BeforeBackupPresent = true
		journal.BeforeBackupSHA = fileSHA256(beforeBackup)
		journal.BeforeBackupSnapshot = "backup.before"
		snapshotPath, _ := naturalDropPrepareSnapshotPath(canonicalGameDir, journal.BeforeBackupSnapshot)
		if err := naturalDropWriteAtomic(snapshotPath, beforeBackup); err != nil {
			return err
		}
	}
	if afterBackup != nil {
		journal.AfterBackupPresent = true
		journal.AfterBackupSHA = fileSHA256(afterBackup)
	}
	journalData, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	journalData = append(journalData, '\n')
	return naturalDropWriteAtomic(filepath.Join(canonicalGameDir, naturalDropPrepareJournalName), journalData)
}

func naturalDropPreparedCurrentAllowed(path string, beforePresent bool, beforeSHA string, afterPresent bool, afterSHA string) error {
	digest, err := naturalDropFileSHA256(path)
	if os.IsNotExist(err) {
		if !beforePresent || !afterPresent {
			return nil
		}
		return fmt.Errorf("事务目标意外缺失: %s", path)
	}
	if err != nil {
		return err
	}
	if (beforePresent && strings.EqualFold(digest, beforeSHA)) || (afterPresent && strings.EqualFold(digest, afterSHA)) {
		return nil
	}
	return fmt.Errorf("事务目标已被其他程序改动，拒绝覆盖: %s", path)
}

func naturalDropReadVerifiedSnapshot(gameDir, name, expectedSHA string) ([]byte, error) {
	path, err := naturalDropPrepareSnapshotPath(gameDir, name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(fileSHA256(data), expectedSHA) {
		return nil, fmt.Errorf("天然掉落事务快照校验失败: %s", name)
	}
	return data, nil
}

func naturalDropRecoverPreparedTransaction(gameDir string) error {
	canonicalGameDir, err := naturalDropCanonicalPath(gameDir)
	if err != nil {
		return err
	}
	journal, err := naturalDropReadPreparedTransaction(canonicalGameDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	indexSnapshot, err := naturalDropReadVerifiedSnapshot(canonicalGameDir, journal.BeforeIndexSnapshot, journal.BeforeIndexSHA)
	if err != nil {
		return fmt.Errorf("读取部署前 data.i 快照失败: %w", err)
	}
	if _, err := parseGBFRDataIndex(indexSnapshot); err != nil {
		return fmt.Errorf("部署前 data.i 快照无效: %w", err)
	}
	indexPath := filepath.Join(canonicalGameDir, "data.i")
	if err := naturalDropPreparedCurrentAllowed(indexPath, true, journal.BeforeIndexSHA, true, journal.AfterIndexSHA); err != nil {
		return err
	}
	type restoreFile struct {
		relative string
		entry    naturalDropPreparedFile
		before   []byte
	}
	restores := make([]restoreFile, 0, len(journal.GeneratedFiles))
	for relative, entry := range journal.GeneratedFiles {
		target := filepath.Join(canonicalGameDir, filepath.FromSlash(relative))
		if err := naturalDropPreparedCurrentAllowed(target, entry.BeforePresent, entry.BeforeSHA, entry.AfterPresent, entry.AfterSHA); err != nil {
			return err
		}
		var before []byte
		if entry.BeforePresent {
			before, err = naturalDropReadVerifiedSnapshot(canonicalGameDir, entry.Snapshot, entry.BeforeSHA)
			if err != nil {
				return fmt.Errorf("读取 %s 的部署前快照失败: %w", relative, err)
			}
		}
		restores = append(restores, restoreFile{relative: relative, entry: entry, before: before})
	}
	sort.Slice(restores, func(i, j int) bool { return restores[i].relative < restores[j].relative })
	manifestPath := filepath.Join(canonicalGameDir, naturalDropManifestName)
	if err := naturalDropPreparedCurrentAllowed(
		manifestPath,
		journal.BeforeManifestPresent,
		journal.BeforeManifestSHA,
		journal.AfterManifestSHA != "",
		journal.AfterManifestSHA,
	); err != nil {
		return err
	}
	var beforeManifest []byte
	if journal.BeforeManifestPresent {
		beforeManifest, err = naturalDropReadVerifiedSnapshot(canonicalGameDir, journal.BeforeManifestSnapshot, journal.BeforeManifestSHA)
		if err != nil {
			return fmt.Errorf("读取部署前清单快照失败: %w", err)
		}
	}
	backupPath := filepath.Join(canonicalGameDir, naturalDropBackupName)
	if journal.BeforeBackupPresent || journal.AfterBackupPresent {
		if err := naturalDropPreparedCurrentAllowed(
			backupPath,
			journal.BeforeBackupPresent,
			journal.BeforeBackupSHA,
			journal.AfterBackupPresent,
			journal.AfterBackupSHA,
		); err != nil {
			return err
		}
	}
	var beforeBackup []byte
	if journal.BeforeBackupPresent {
		beforeBackup, err = naturalDropReadVerifiedSnapshot(canonicalGameDir, journal.BeforeBackupSnapshot, journal.BeforeBackupSHA)
		if err != nil {
			return fmt.Errorf("读取部署前备份快照失败: %w", err)
		}
	}
	if journal.RemoveBackupOnRecovery {
		digest, digestErr := naturalDropFileSHA256(backupPath)
		if digestErr == nil && !strings.EqualFold(digest, journal.BeforeIndexSHA) {
			return errors.New("天然掉落 data.i 备份已被其他程序改动，拒绝删除")
		}
		if digestErr != nil && !os.IsNotExist(digestErr) {
			return digestErr
		}
	}

	if err := naturalDropWriteAtomic(indexPath, indexSnapshot); err != nil {
		return fmt.Errorf("恢复部署前 data.i 失败: %w", err)
	}
	for _, restore := range restores {
		target := filepath.Join(canonicalGameDir, filepath.FromSlash(restore.relative))
		if restore.entry.BeforePresent {
			if err := naturalDropWriteAtomic(target, restore.before); err != nil {
				return fmt.Errorf("恢复部署前文件 %s 失败: %w", restore.relative, err)
			}
		} else if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("清理事务生成文件 %s 失败: %w", restore.relative, err)
		}
	}
	if journal.BeforeManifestPresent {
		if err := naturalDropWriteAtomic(manifestPath, beforeManifest); err != nil {
			return fmt.Errorf("恢复部署前清单失败: %w", err)
		}
	} else if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("清理未完成部署清单失败: %w", err)
	}
	if journal.RemoveBackupOnRecovery {
		if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("清理未完成部署备份失败: %w", err)
		}
		partialBackupPath := filepath.Join(canonicalGameDir, naturalDropBackupPartialName)
		if err := os.Remove(partialBackupPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("清理未完成部署的半写备份失败: %w", err)
		}
	} else if journal.BeforeBackupPresent {
		if err := naturalDropWriteAtomic(backupPath, beforeBackup); err != nil {
			return fmt.Errorf("恢复部署前备份失败: %w", err)
		}
	} else if journal.AfterBackupPresent {
		if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("恢复部署前无备份状态失败: %w", err)
		}
	}
	if err := naturalDropCompletePreparedTransaction(canonicalGameDir); err != nil {
		return fmt.Errorf("完成天然掉落恢复事务失败: %w", err)
	}
	return nil
}

func naturalDropRecoverPreparedTransactionIfSafe(gameDir string) error {
	journalPath := filepath.Join(gameDir, naturalDropPrepareJournalName)
	if _, err := os.Stat(journalPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if err := naturalDropRequireStoppedProcesses(); err != nil {
		return fmt.Errorf("检测到上次未完成的天然掉落部署；请先完全退出游戏再恢复: %w", err)
	}
	return naturalDropRecoverPreparedTransaction(gameDir)
}

func recoverNaturalDropTransactionsAtStartup(configuredGameExePath string) error {
	candidates := make([]string, 0, len(findSteamLibraryFolders())+1)
	if configured := strings.TrimSpace(configuredGameExePath); configured != "" {
		candidates = append(candidates, configured)
	}
	for _, library := range findSteamLibraryFolders() {
		candidates = append(candidates, filepath.Join(library, "steamapps", "common", gameFolder, gameExeName))
	}
	seen := make(map[string]bool, len(candidates))
	var recoveryErr error
	for _, candidate := range candidates {
		canonicalCandidate, err := naturalDropCanonicalPath(candidate)
		if err != nil {
			recoveryErr = errors.Join(recoveryErr, err)
			continue
		}
		key := strings.ToLower(canonicalCandidate)
		if seen[key] {
			continue
		}
		seen[key] = true
		gameDir := filepath.Dir(canonicalCandidate)
		journalPath := filepath.Join(gameDir, naturalDropPrepareJournalName)
		if _, err := os.Stat(journalPath); os.IsNotExist(err) {
			continue
		} else if err != nil {
			recoveryErr = errors.Join(recoveryErr, fmt.Errorf("检查 %s 失败: %w", journalPath, err))
			continue
		}
		validated, err := validateNaturalDropGameExecutable(canonicalCandidate)
		if err != nil {
			recoveryErr = errors.Join(recoveryErr, fmt.Errorf("验证待恢复游戏目录失败: %w", err))
			continue
		}
		gameDir = filepath.Dir(validated)
		releaseLease, err := acquireNaturalDropTransactionLease(gameDir)
		if err != nil {
			recoveryErr = errors.Join(recoveryErr, err)
			continue
		}
		err = naturalDropRecoverPreparedTransactionIfSafe(gameDir)
		releaseLease()
		if err != nil {
			recoveryErr = errors.Join(recoveryErr, fmt.Errorf("恢复 %s 失败: %w", gameDir, err))
		}
	}
	return recoveryErr
}

func naturalDropCompletePreparedTransaction(gameDir string) error {
	canonicalGameDir, err := naturalDropCanonicalPath(gameDir)
	if err != nil {
		return err
	}
	journalPath := filepath.Join(canonicalGameDir, naturalDropPrepareJournalName)
	completedPath := filepath.Join(canonicalGameDir, naturalDropCompletedJournalName)
	if _, err := os.Stat(journalPath); err == nil {
		from, err := windows.UTF16PtrFromString(journalPath)
		if err != nil {
			return err
		}
		to, err := windows.UTF16PtrFromString(completedPath)
		if err != nil {
			return err
		}
		if err := windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
			return fmt.Errorf("持久化天然掉落事务完成标记失败: %w", err)
		}
		if _, err := os.Stat(journalPath); !os.IsNotExist(err) {
			return errors.New("天然掉落事务完成后仍存在活动日志")
		}
		if _, err := os.Stat(completedPath); err != nil {
			return fmt.Errorf("回读天然掉落事务完成标记失败: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	// The write-through rename is the durable terminal state. Once the active
	// journal is gone, a crash may leave snapshots or the completed marker, but
	// startup will never mistake that cleanup residue for an interrupted deploy.
	_ = os.RemoveAll(filepath.Join(canonicalGameDir, naturalDropPrepareDirectoryName))
	_ = os.Remove(completedPath)
	return nil
}

func naturalDropCleanupBackup(path string, cause error, removeFile func(string) error) error {
	cleanupErr := removeFile(path)
	if cleanupErr == nil || os.IsNotExist(cleanupErr) {
		return cause
	}
	cleanupErr = fmt.Errorf("清理未完成的 data.i 备份失败: %w", cleanupErr)
	if cause == nil {
		return cleanupErr
	}
	return errors.Join(cause, cleanupErr)
}

func naturalDropCreateBackup(path string, data []byte) (resultErr error) {
	partialPath := filepath.Join(filepath.Dir(path), naturalDropBackupPartialName)
	file, err := os.OpenFile(partialPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	complete := false
	closed := false
	defer func() {
		if !closed {
			if closeErr := file.Close(); closeErr != nil {
				resultErr = errors.Join(resultErr, closeErr)
			}
		}
		if !complete {
			resultErr = naturalDropCleanupBackup(partialPath, resultErr, os.Remove)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	closeErr := file.Close()
	closed = true
	if closeErr != nil {
		return closeErr
	}
	readback, err := os.ReadFile(partialPath)
	if err != nil {
		return err
	}
	if fileSHA256(readback) != fileSHA256(data) {
		return errors.New("data.i 备份写后校验失败")
	}
	from, err := windows.UTF16PtrFromString(partialPath)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if err := windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return fmt.Errorf("原子发布 data.i 备份失败: %w", err)
	}
	readback, err = os.ReadFile(path)
	if err != nil {
		return err
	}
	if fileSHA256(readback) != fileSHA256(data) {
		return errors.New("data.i 备份发布后校验失败")
	}
	complete = true
	return nil
}

func naturalDropVerifyOwnedCurrent(gameDir string, manifest *naturalDropManifest, indexData []byte) error {
	if !strings.EqualFold(fileSHA256(indexData), manifest.DeployedIndexSHA) {
		return errors.New("当前 data.i 已被其他程序改动；请先恢复或验证游戏文件，拒绝叠加覆盖")
	}
	if !naturalDropGeneratedFilesMatch(gameDir, manifest) {
		return errors.New("本工具部署的掉落表已被其他程序改动；拒绝覆盖未知内容")
	}
	return nil
}

func naturalDropSnapshotExistingFile(path string, readFile func(string) ([]byte, error)) ([]byte, error) {
	data, err := readFile(path)
	if err == nil {
		return data, nil
	}
	if os.IsNotExist(err) {
		return nil, nil
	}
	return nil, err
}

func (a *App) DeployNaturalDropMod(request NaturalDropDeployRequest) (*NaturalDropDeployResult, error) {
	a.naturalDropMu.Lock()
	defer a.naturalDropMu.Unlock()
	if err := naturalDropRequireStoppedProcesses(); err != nil {
		return nil, err
	}
	if len(request.Selections) == 0 && len(request.Sigils) == 0 && len(request.Wrightstones) == 0 && len(request.Items) == 0 {
		return nil, errors.New("请至少选择一颗召唤石、因子、祝福石或物品")
	}
	if request.SigilOnly && request.WrightstoneOnly {
		return nil, errors.New("Transmarvel 不能同时设为只出因子和只出祝福石")
	}
	gameExePath, err := validateNaturalDropGameExecutable(request.GameExePath)
	if err != nil {
		return nil, err
	}
	a.rememberNaturalDropGameExecutable(gameExePath)
	gameVersion, gameExecutableSHA, err := naturalDropGameIdentity(gameExePath)
	if err != nil {
		return nil, err
	}
	gameDir, indexPath, backupPath, manifestPath := naturalDropInstallPaths(gameExePath)
	releaseLease, err := acquireNaturalDropTransactionLease(gameDir)
	if err != nil {
		return nil, err
	}
	defer releaseLease()
	if err := naturalDropRecoverPreparedTransaction(gameDir); err != nil {
		a.setNaturalDropStartupRecoveryError(err)
		return nil, fmt.Errorf("恢复上次未完成的天然掉落部署失败: %w", err)
	}
	a.clearNaturalDropStartupRecoveryError()
	files := make(map[string][]byte, len(naturalDropSummonTablePaths)+len(naturalDropWrightstoneTablePaths)+len(naturalDropSigilTablePaths)+len(naturalDropItemTablePaths))
	affectedPools := 0
	if len(request.Selections) > 0 {
		source, _, loadErr := loadNaturalDropTables(request.SourceDir, true)
		if loadErr != nil {
			return nil, loadErr
		}
		patched, poolCount, patchErr := patchNaturalDropTables(source, request.Selections)
		if patchErr != nil {
			return nil, patchErr
		}
		affectedPools = poolCount
		files[naturalDropSummonTablePaths[0]] = patched.Summon
		files[naturalDropSummonTablePaths[1]] = patched.SummonLot
		files[naturalDropSummonTablePaths[2]] = patched.RewardSummonLot
	}
	if len(request.Sigils) > 0 || len(request.Wrightstones) > 0 {
		shared, _, loadErr := loadNaturalWrightstoneTables(request.SourceDir, true)
		if loadErr != nil {
			return nil, loadErr
		}
		var sigilTables *naturalSigilTables
		if len(request.Sigils) > 0 {
			sigilTables, _, loadErr = loadNaturalSigilTables(request.SourceDir, true)
			if loadErr != nil {
				return nil, loadErr
			}
			var patchErr error
			shared, sigilTables, _, patchErr = patchNaturalSigilTables(shared, sigilTables, request.Sigils, request.SigilOnly)
			if patchErr != nil {
				return nil, patchErr
			}
			files[naturalDropSigilTablePaths[0]] = sigilTables.Gem
		}
		if len(request.Wrightstones) > 0 {
			var patchErr error
			shared, _, patchErr = patchNaturalWrightstoneTables(shared, request.Wrightstones, request.WrightstoneOnly)
			if patchErr != nil {
				return nil, patchErr
			}
			files[naturalDropWrightstoneTablePaths[0]] = shared.Items
		}
		files[naturalDropWrightstoneTablePaths[1]] = shared.Lots
		files[naturalDropWrightstoneTablePaths[2]] = shared.RateGroups
		files[naturalDropWrightstoneTablePaths[3]] = shared.Gacha
	}
	if len(request.Items) > 0 {
		itemTables, _, loadErr := loadNaturalDropItemTables(request.SourceDir, true)
		if loadErr != nil {
			return nil, loadErr
		}
		patchedLots, patchErr := patchNaturalDropItemTable(itemTables, request.Items, request.ItemMultiplier)
		if patchErr != nil {
			return nil, patchErr
		}
		files[naturalDropItemTablePaths[0]] = patchedLots
		affectedPools++
	}
	currentIndex, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}
	currentParsed, err := parseGBFRDataIndex(currentIndex)
	if err != nil {
		return nil, err
	}
	manifest, oldManifest, manifestErr := naturalDropReadManifest(gameDir)
	owned := manifestErr == nil
	if manifestErr != nil && !os.IsNotExist(manifestErr) {
		return nil, fmt.Errorf("发现无法验证的天然掉落清单: %w", manifestErr)
	}
	var baseIndex []byte
	backupCreated := !owned
	if owned {
		if err := naturalDropVerifyOwnedCurrent(gameDir, manifest, currentIndex); err != nil {
			return nil, err
		}
		baseIndex, err = os.ReadFile(backupPath)
		if err != nil || !strings.EqualFold(fileSHA256(baseIndex), manifest.OriginalIndexSHA) {
			return nil, errors.New("天然掉落原始 data.i 备份缺失或校验失败")
		}
	} else {
		if _, err := os.Stat(backupPath); err == nil {
			return nil, errors.New("发现无有效清单对应的 data.i 备份；拒绝覆盖，请先人工核对")
		}
		selectedTargets := make(map[string]bool, len(files))
		for gamePath := range files {
			selectedTargets[naturalDropRelativeTarget(gamePath)] = true
		}
		conflictCount := 0
		for _, conflict := range naturalDropConflicts(gameDir, currentParsed, false) {
			if selectedTargets[conflict.File] {
				conflictCount++
			}
		}
		if conflictCount > 0 {
			return nil, fmt.Errorf("所选目标表已被其他外部文件方案占用（%d 项），拒绝覆盖", conflictCount)
		}
		baseIndex = currentIndex
	}
	deployedIndex, err := naturalDropBuildIndex(baseIndex, files)
	if err != nil {
		return nil, err
	}

	oldFiles := make(map[string][]byte, len(files))
	afterFiles := make(map[string][]byte, len(files))
	generated := make(map[string]string, len(files))
	for gamePath, data := range files {
		relative := naturalDropRelativeTarget(gamePath)
		target := filepath.Join(gameDir, filepath.FromSlash(relative))
		oldFiles[relative], err = naturalDropSnapshotExistingFile(target, os.ReadFile)
		if err != nil {
			return nil, fmt.Errorf("读取目标表 %s 的原始内容失败，部署已取消: %w", relative, err)
		}
		afterFiles[relative] = data
		generated[relative] = fileSHA256(data)
	}
	var staleGenerated []string
	if owned {
		for relative := range manifest.GeneratedFiles {
			if _, retained := generated[relative]; retained {
				continue
			}
			target := filepath.Join(gameDir, filepath.FromSlash(relative))
			oldFiles[relative], err = naturalDropSnapshotExistingFile(target, os.ReadFile)
			if err != nil {
				return nil, fmt.Errorf("读取旧生成表 %s 的原始内容失败，部署已取消: %w", relative, err)
			}
			afterFiles[relative] = nil
			staleGenerated = append(staleGenerated, relative)
		}
	}
	sort.Strings(staleGenerated)
	selectionCopy := append([]NaturalDropSelection(nil), request.Selections...)
	sort.Slice(selectionCopy, func(i, j int) bool { return selectionCopy[i].TypeHash < selectionCopy[j].TypeHash })
	itemCopy := append([]NaturalDropItemSelection(nil), request.Items...)
	sort.Slice(itemCopy, func(i, j int) bool { return itemCopy[i].ItemHash < itemCopy[j].ItemHash })
	newManifest := naturalDropManifest{
		SchemaVersion:       4,
		Owner:               naturalDropModID,
		GameVersion:         gameVersion,
		GameExecutableSHA:   gameExecutableSHA,
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
		OriginalIndexSHA:    fileSHA256(baseIndex),
		DeployedIndexSHA:    fileSHA256(deployedIndex),
		SourceFiles:         naturalDropSourceFiles(len(selectionCopy) > 0, len(request.Wrightstones) > 0, len(request.Sigils) > 0, len(itemCopy) > 0),
		GeneratedFiles:      generated,
		Selections:          selectionCopy,
		Sigils:              append([]NaturalDropSigilSelection(nil), request.Sigils...),
		Wrightstones:        append([]NaturalDropWrightstoneSelection(nil), request.Wrightstones...),
		Items:               itemCopy,
		ItemMultiplier:      request.ItemMultiplier,
		SigilOnly:           request.SigilOnly,
		WrightstoneOnly:     request.WrightstoneOnly,
		AffectedRewardPools: affectedPools,
	}
	manifestData, err := json.MarshalIndent(newManifest, "", "  ")
	if err != nil {
		return nil, err
	}
	manifestData = append(manifestData, '\n')
	if err := naturalDropBeginPreparedTransaction(
		gameDir,
		currentIndex,
		deployedIndex,
		oldFiles,
		oldManifest,
		afterFiles,
		backupCreated,
		manifestData,
		naturalDropGameIdentityRecord{Version: gameVersion, SHA256: gameExecutableSHA},
	); err != nil {
		return nil, fmt.Errorf("创建天然掉落部署事务失败: %w", err)
	}
	rollback := func(cause error) error {
		if rollbackErr := naturalDropRecoverPreparedTransaction(gameDir); rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("自动回滚未完全成功: %w", rollbackErr))
		}
		return cause
	}
	if backupCreated {
		if err := naturalDropCreateBackup(backupPath, baseIndex); err != nil {
			return nil, rollback(fmt.Errorf("创建原始 data.i 备份失败: %w", err))
		}
	}
	for gamePath, data := range files {
		if err := naturalDropWriteAtomic(naturalDropTargetPath(gameDir, gamePath), data); err != nil {
			return nil, rollback(fmt.Errorf("部署 %s 失败: %w", gamePath, err))
		}
	}
	if err := naturalDropWriteAtomic(indexPath, deployedIndex); err != nil {
		return nil, rollback(fmt.Errorf("部署 data.i 失败: %w", err))
	}
	for _, relative := range staleGenerated {
		if err := os.Remove(filepath.Join(gameDir, filepath.FromSlash(relative))); err != nil && !os.IsNotExist(err) {
			return nil, rollback(fmt.Errorf("清理旧生成表 %s 失败: %w", relative, err))
		}
	}
	if err := naturalDropWriteAtomic(manifestPath, manifestData); err != nil {
		return nil, rollback(fmt.Errorf("写入部署清单失败: %w", err))
	}
	if err := naturalDropCompletePreparedTransaction(gameDir); err != nil {
		return nil, rollback(fmt.Errorf("清理天然掉落部署事务失败: %w", err))
	}
	fileList := make([]string, 0, len(generated)+1)
	for path := range generated {
		fileList = append(fileList, path)
	}
	fileList = append(fileList, "data.i")
	sort.Strings(fileList)
	return &NaturalDropDeployResult{
		ModDir:               gameDir,
		GeneratedFiles:       fileList,
		SelectedSummons:      len(selectionCopy),
		SelectedSigils:       len(request.Sigils),
		SelectedWrightstones: len(request.Wrightstones),
		SelectedItems:        len(itemCopy),
		AffectedRewardPools:  affectedPools,
		SourceDigest:         naturalDropSourceDigest(newManifest.SourceFiles),
	}, nil
}

func (a *App) RestoreNaturalDropDefaults(request NaturalDropRestoreRequest) error {
	a.naturalDropMu.Lock()
	defer a.naturalDropMu.Unlock()
	if err := naturalDropRequireStoppedProcesses(); err != nil {
		return err
	}
	gameExePath, err := validateNaturalDropGameExecutable(request.GameExePath)
	if err != nil {
		return err
	}
	a.rememberNaturalDropGameExecutable(gameExePath)
	gameDir, indexPath, backupPath, manifestPath := naturalDropInstallPaths(gameExePath)
	releaseLease, err := acquireNaturalDropTransactionLease(gameDir)
	if err != nil {
		return err
	}
	defer releaseLease()
	if err := naturalDropRecoverPreparedTransaction(gameDir); err != nil {
		a.setNaturalDropStartupRecoveryError(err)
		return fmt.Errorf("恢复上次未完成的天然掉落部署失败: %w", err)
	}
	a.clearNaturalDropStartupRecoveryError()
	manifest, manifestData, err := naturalDropReadManifest(gameDir)
	if err != nil {
		return errors.New("未找到本工具拥有的天然掉落部署")
	}
	currentIndex, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}
	if err := naturalDropVerifyOwnedCurrent(gameDir, manifest, currentIndex); err != nil {
		return err
	}
	backup, err := os.ReadFile(backupPath)
	if err != nil || !strings.EqualFold(fileSHA256(backup), manifest.OriginalIndexSHA) {
		return errors.New("原始 data.i 备份缺失或校验失败，拒绝恢复")
	}
	if _, err := parseGBFRDataIndex(backup); err != nil {
		return fmt.Errorf("原始 data.i 备份无效: %w", err)
	}
	beforeFiles := make(map[string][]byte, len(manifest.GeneratedFiles))
	afterFiles := make(map[string][]byte, len(manifest.GeneratedFiles))
	relatives := make([]string, 0, len(manifest.GeneratedFiles))
	for relative := range manifest.GeneratedFiles {
		if err := naturalDropValidatePreparedRelative(relative); err != nil {
			return err
		}
		target := filepath.Join(gameDir, filepath.FromSlash(relative))
		data, err := os.ReadFile(target)
		if err != nil {
			return fmt.Errorf("读取待恢复表 %s 失败: %w", relative, err)
		}
		beforeFiles[relative] = data
		afterFiles[relative] = nil
		relatives = append(relatives, relative)
	}
	sort.Strings(relatives)
	if err := naturalDropBeginPreparedTransaction(
		gameDir,
		currentIndex,
		backup,
		beforeFiles,
		manifestData,
		afterFiles,
		false,
		nil,
		naturalDropGameIdentityRecord{Version: manifest.GameVersion, SHA256: manifest.GameExecutableSHA},
	); err != nil {
		return fmt.Errorf("创建天然掉落恢复事务失败: %w", err)
	}
	if err := naturalDropAddPreparedBackupTransition(gameDir, backup, nil); err != nil {
		_ = naturalDropRecoverPreparedTransaction(gameDir)
		return fmt.Errorf("记录天然掉落恢复备份失败: %w", err)
	}
	rollback := func(cause error) error {
		if rollbackErr := naturalDropRecoverPreparedTransaction(gameDir); rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("自动回滚未完全成功: %w", rollbackErr))
		}
		return cause
	}
	if err := naturalDropWriteAtomic(indexPath, backup); err != nil {
		return rollback(fmt.Errorf("恢复原始 data.i 失败: %w", err))
	}
	for _, relative := range relatives {
		target := filepath.Join(gameDir, filepath.FromSlash(relative))
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return rollback(fmt.Errorf("清理部署表 %s 失败: %w", relative, err))
		}
	}
	if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
		return rollback(fmt.Errorf("清理部署清单失败: %w", err))
	}
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return rollback(fmt.Errorf("清理备份副本失败: %w", err))
	}
	if err := naturalDropCompletePreparedTransaction(gameDir); err != nil {
		return rollback(fmt.Errorf("提交天然掉落恢复事务失败: %w", err))
	}
	for _, path := range []string{
		filepath.Join(gameDir, "data", "system", "table"),
		filepath.Join(gameDir, "data", "system"),
	} {
		_ = os.Remove(path)
	}
	return nil
}
