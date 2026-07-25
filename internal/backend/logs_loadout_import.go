package backend

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/klauspost/compress/zstd"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	_ "modernc.org/sqlite"
)

const (
	logsLoadoutMaximumRecords = 200
	logsLoadoutMaximumBlob    = 8 * 1024 * 1024
)

type LogsLoadoutShareCandidate struct {
	LogTime           int64                     `json:"logTime"`
	PlayerName        string                    `json:"playerName"`
	CharacterName     string                    `json:"characterName"`
	OwnerCode         string                    `json:"ownerCode"`
	CharacterHash     string                    `json:"characterHash"`
	SigilCount        int                       `json:"sigilCount"`
	WeaponName        string                    `json:"weaponName,omitempty"`
	OverLimitCount    int                       `json:"overLimitCount,omitempty"`
	CompatibilityCode string                    `json:"compatibilityCode"`
	Preview           *RuntimePatchPartyLoadout `json:"preview"`
}

type logsLoadoutEncounter struct {
	PlayerData [4]*logsLoadoutPlayer `cbor:"playerData"`
}

type logsLoadoutPlayer struct {
	DisplayName     string                      `cbor:"displayName"`
	CharacterName   string                      `cbor:"characterName"`
	CharacterType   string                      `cbor:"characterType"`
	Sigils          []logsLoadoutSigil          `cbor:"sigils"`
	WeaponInfo      *logsLoadoutWeaponInfo      `cbor:"weaponInfo"`
	OvermasteryInfo *logsLoadoutOvermasteryInfo `cbor:"overmasteryInfo"`
	PlayerStats     *logsLoadoutPlayerStats     `cbor:"playerStats"`
}

type logsLoadoutSigil struct {
	FirstTraitID     uint32 `cbor:"firstTraitId"`
	FirstTraitLevel  uint32 `cbor:"firstTraitLevel"`
	SecondTraitID    uint32 `cbor:"secondTraitId"`
	SecondTraitLevel uint32 `cbor:"secondTraitLevel"`
	SigilID          uint32 `cbor:"sigilId"`
	SigilLevel       uint32 `cbor:"sigilLevel"`
}

type logsLoadoutTrait struct {
	ID    uint32 `cbor:"id"`
	Level uint32 `cbor:"level"`
}

type logsLoadoutWeaponInfo struct {
	WeaponID          uint32             `cbor:"weaponId"`
	StarLevel         uint32             `cbor:"starLevel"`
	PlusMarks         uint32             `cbor:"plusMarks"`
	AwakeningLevel    uint32             `cbor:"awakeningLevel"`
	LegacyTrait1ID    uint32             `cbor:"trait1Id"`
	LegacyTrait1Level uint32             `cbor:"trait1Level"`
	LegacyTrait2ID    uint32             `cbor:"trait2Id"`
	LegacyTrait2Level uint32             `cbor:"trait2Level"`
	LegacyTrait3ID    uint32             `cbor:"trait3Id"`
	LegacyTrait3Level uint32             `cbor:"trait3Level"`
	WrightstoneTraits []logsLoadoutTrait `cbor:"wrightstoneTraits"`
	InnateTraits      []logsLoadoutTrait `cbor:"innateTraits"`
	WrightstoneID     uint32             `cbor:"wrightstoneId"`
	WeaponLevel       uint32             `cbor:"weaponLevel"`
	WeaponHP          uint32             `cbor:"weaponHp"`
	WeaponAttack      uint32             `cbor:"weaponAttack"`
}

func (source *logsLoadoutWeaponInfo) effectiveWrightstoneTraits() []logsLoadoutTrait {
	if source == nil || len(source.WrightstoneTraits) > 0 {
		return source.WrightstoneTraits
	}
	legacy := []logsLoadoutTrait{
		{ID: source.LegacyTrait1ID, Level: source.LegacyTrait1Level},
		{ID: source.LegacyTrait2ID, Level: source.LegacyTrait2Level},
		{ID: source.LegacyTrait3ID, Level: source.LegacyTrait3Level},
	}
	result := make([]logsLoadoutTrait, 0, len(legacy))
	for _, trait := range legacy {
		if !runtimePatchPartyEmptyHash(trait.ID) {
			result = append(result, trait)
		}
	}
	return result
}

type logsLoadoutOvermastery struct {
	ID    uint32  `cbor:"id"`
	Flags uint32  `cbor:"flags"`
	Value float32 `cbor:"value"`
}

type logsLoadoutOvermasteryInfo struct {
	Overmasteries []logsLoadoutOvermastery `cbor:"overmasteries"`
}

type logsLoadoutPlayerStats struct {
	Level        uint32  `cbor:"level"`
	TotalHP      uint32  `cbor:"totalHp"`
	TotalAttack  uint32  `cbor:"totalAttack"`
	StunPower    float32 `cbor:"stunPower"`
	CriticalRate float32 `cbor:"criticalRate"`
	TotalPower   uint32  `cbor:"totalPower"`
}

func normalizeLogsOwnerCode(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 6 && strings.EqualFold(value[:2], "PL") {
		return "PL" + value[2:]
	}
	return ""
}

func decodeLogsLoadoutEncounter(blob []byte) (logsLoadoutEncounter, error) {
	if len(blob) == 0 || len(blob) > logsLoadoutMaximumBlob {
		return logsLoadoutEncounter{}, fmt.Errorf("日志记录大小无效")
	}
	decoder, err := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(logsLoadoutMaximumBlob), zstd.WithDecoderMaxWindow(logsLoadoutMaximumBlob))
	if err != nil {
		return logsLoadoutEncounter{}, err
	}
	decompressed, err := decoder.DecodeAll(blob, nil)
	decoder.Close()
	if err != nil || len(decompressed) == 0 || len(decompressed) > logsLoadoutMaximumBlob {
		return logsLoadoutEncounter{}, fmt.Errorf("解压日志记录失败或内容超限")
	}
	var encounter logsLoadoutEncounter
	if err := cbor.Unmarshal(decompressed, &encounter); err != nil {
		return logsLoadoutEncounter{}, err
	}
	return encounter, nil
}

func logsPlayerLoadoutShare(logTime int64, player *logsLoadoutPlayer) (*LogsLoadoutShareCandidate, error) {
	if player == nil || len(player.Sigils) == 0 || len(player.Sigils) > loadoutMaxSigils {
		return nil, fmt.Errorf("日志玩家没有有效的 1..12 格因子")
	}
	ownerCode := normalizeLogsOwnerCode(player.CharacterType)
	characterHash, known := runtimeOwnerCharacterHash[ownerCode]
	if !known {
		return nil, fmt.Errorf("日志角色类型 %q 无法映射到存档角色", player.CharacterType)
	}
	catalog, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	if _, err := loadProgressionCatalog(); err != nil {
		return nil, err
	}
	characterName := strings.TrimSpace(player.CharacterName)
	if names, ok := runtimePatchPartyCharacterNames[ownerCode]; ok {
		characterName = names[0]
		if !useChinese() {
			characterName = names[1]
		}
	}
	playerName := strings.TrimSpace(player.DisplayName)
	share := &LoadoutShare{
		Format: loadoutShareFormat, Version: loadoutShareVersion,
		CharaHash: hashText(characterHash), CharaName: characterName, OwnerCode: ownerCode,
		Name:       characterName + runtimePatchMonitorText(" · Logs 因子配装", " · Logs Sigils"),
		SourceKind: loadoutShareSourceLogsDB, ProgressionPolicy: loadoutProgressionEndgame,
		CapturedFields: []string{"sigils"},
	}
	preview := &RuntimePatchPartyLoadout{
		Available: true, Stable: true, SnapshotCount: 1,
		Verification: "logs-db-v1", Evidence: runtimePatchMonitorText("GBFR Logs v1 记录", "GBFR Logs v1 record"),
		CharacterCode: ownerCode, CharacterHash: share.CharaHash, CharacterName: characterName,
		Sigils: make([]RuntimePatchPartySigil, 0, len(player.Sigils)), OverLimit: make([]RuntimePatchPartyOverLimit, 4),
	}
	preview.Stats.Level = 100
	if player.PlayerStats != nil {
		preview.Stats = RuntimePatchPartyPanelStats{
			Level: 100, TotalHP: player.PlayerStats.TotalHP, TotalAttack: player.PlayerStats.TotalAttack,
			StunPower: player.PlayerStats.StunPower, CriticalRate: player.PlayerStats.CriticalRate, TotalPower: player.PlayerStats.TotalPower,
		}
	}
	for index, source := range player.Sigils {
		if runtimePatchPartyEmptyHash(source.SigilID) {
			continue
		}
		sigilDef := catalog.LookupSigilByHash(source.SigilID)
		primaryName, primaryKnown := runtimePatchPartyTraitName(catalog, source.FirstTraitID)
		if sigilDef == nil || !primaryKnown {
			return nil, fmt.Errorf("日志因子槽 %d 不在当前 2.0.2 目录", index+1)
		}
		runtimeSigil := maximizeCapturedSigil(catalog, RuntimePatchPartySigil{
			Index: index, Hash: source.SigilID, Name: displaySigilName(sigilDef), Level: source.SigilLevel,
			PrimaryTraitHash: source.FirstTraitID, PrimaryTraitName: primaryName, PrimaryTraitLevel: source.FirstTraitLevel,
			SecondaryTraitHash: source.SecondTraitID, SecondaryTraitLevel: source.SecondTraitLevel,
		})
		if !runtimePatchPartyEmptyHash(source.SecondTraitID) {
			secondaryName, secondaryKnown := runtimePatchPartyTraitName(catalog, source.SecondTraitID)
			if !secondaryKnown {
				return nil, fmt.Errorf("日志因子槽 %d 的副词条不在当前目录", index+1)
			}
			runtimeSigil.SecondaryTraitName = secondaryName
		}
		itemIndex := index
		share.Sigils = append(share.Sigils, LoadoutShareSigil{
			Index: &itemIndex, Hash: hashText(runtimeSigil.Hash), Name: runtimeSigil.Name, Level: int(runtimeSigil.Level),
			PrimaryTraitHash: hashText(runtimeSigil.PrimaryTraitHash), PrimaryTraitLevel: int(runtimeSigil.PrimaryTraitLevel),
			SecondaryTraitHash: shareHex(runtimeSigil.SecondaryTraitHash), SecondaryTraitLevel: int(runtimeSigil.SecondaryTraitLevel),
		})
		preview.Sigils = append(preview.Sigils, runtimeSigil)
	}
	if len(share.Sigils) == 0 {
		return nil, fmt.Errorf("日志玩家没有可导入因子")
	}
	if player.PlayerStats != nil {
		share.CapturedFields = append(share.CapturedFields, "stats")
	}
	if source := player.WeaponInfo; source != nil && !runtimePatchPartyEmptyHash(source.WeaponID) {
		definition, weaponKnown := progressionWeaponDefForHash(source.WeaponID)
		if !weaponKnown || definition.OwnerCode != ownerCode {
			return nil, fmt.Errorf("日志武器 %08X 与角色 %s 不匹配", source.WeaponID, ownerCode)
		}
		weapon := RuntimePatchPartyWeapon{
			Hash: source.WeaponID, HashHex: hashText(source.WeaponID), Name: progressionWeaponName(definition),
			Level: source.WeaponLevel, StarLevel: source.StarLevel, PlusMarks: source.PlusMarks,
			AwakeningLevel: source.AwakeningLevel, WrightstoneID: source.WrightstoneID,
			HP: source.WeaponHP, Attack: source.WeaponAttack,
		}
		for _, trait := range source.effectiveWrightstoneTraits() {
			if runtimePatchPartyEmptyHash(trait.ID) {
				continue
			}
			name, knownTrait := runtimePatchPartyTraitName(catalog, trait.ID)
			if !knownTrait {
				return nil, fmt.Errorf("日志武器祝福词条 %08X 不在当前目录", trait.ID)
			}
			weapon.Traits = append(weapon.Traits, RuntimePatchPartyTrait{Hash: trait.ID, HashHex: hashText(trait.ID), Name: name, Level: trait.Level})
		}
		for _, trait := range source.InnateTraits {
			if runtimePatchPartyEmptyHash(trait.ID) {
				continue
			}
			name, knownTrait := runtimePatchPartyTraitName(catalog, trait.ID)
			if !knownTrait {
				return nil, fmt.Errorf("日志武器技能 %08X 不在当前目录", trait.ID)
			}
			weapon.Skills = append(weapon.Skills, RuntimePatchPartyTrait{Hash: trait.ID, HashHex: hashText(trait.ID), Name: name, Level: trait.Level})
		}
		share.WeaponHash = hashText(source.WeaponID)
		share.WeaponName = weapon.Name
		share.Weapon, err = endgameCapturedWeapon(weapon, definition)
		if err != nil {
			return nil, err
		}
		share.WeaponSkillHashes = append([]string(nil), share.Weapon.SkillHashes...)
		share.CapturedFields = append(share.CapturedFields, "weapon", "weaponSkills")
		if share.Weapon.Wrightstone != nil {
			share.CapturedFields = append(share.CapturedFields, "wrightstone")
		}
		share.Name = characterName + runtimePatchMonitorText(" · Logs 队友配装", " · Logs Party Loadout")
		preview.Weapon = weapon
		preview.Weapon.Level = uint32(definition.MaxLevel)
		preview.Weapon.StarLevel = uint32(share.Weapon.Uncap)
		preview.Weapon.PlusMarks = uint32(share.Weapon.Mirage)
		preview.Weapon.AwakeningLevel = uint32(share.Weapon.Awakening)
		if share.Weapon.Wrightstone != nil {
			preview.Weapon.Traits = preview.Weapon.Traits[:0]
			for _, trait := range share.Weapon.Wrightstone.Traits {
				hash, parseErr := ParseHashHex(trait.Hash)
				if parseErr != nil {
					return nil, parseErr
				}
				preview.Weapon.Traits = append(preview.Weapon.Traits, RuntimePatchPartyTrait{Hash: hash, HashHex: hashText(hash), Name: trait.Name, Level: uint32(trait.Level)})
			}
		}
	}
	overLimitCount := 0
	if player.OvermasteryInfo != nil && len(player.OvermasteryInfo.Overmasteries) > 0 {
		if len(player.OvermasteryInfo.Overmasteries) > 4 {
			return nil, fmt.Errorf("日志上限突破槽位超过 4 个")
		}
		share.OverLimit = make([]LoadoutShareOverLimit, 4)
		for index := range share.OverLimit {
			share.OverLimit[index].Index = index
		}
		for index, source := range player.OvermasteryInfo.Overmasteries {
			if runtimePatchPartyEmptyHash(source.ID) {
				continue
			}
			if _, known := overLimitCatalog[source.ID]; !known || source.Flags == 0 || source.Flags > 0x200 || source.Flags&(source.Flags-1) != 0 {
				return nil, fmt.Errorf("日志上限突破槽 %d 无效", index+1)
			}
			definition := overLimitCatalog[source.ID]
			share.OverLimit[index] = LoadoutShareOverLimit{Index: index, AttributeHash: hashText(source.ID), Level: 10}
			preview.OverLimit[index] = RuntimePatchPartyOverLimit{Index: index, AttributeHash: source.ID, HashHex: hashText(source.ID), Name: definition.name, Flags: 0x200, Level: 10, Value: definition.maxValue}
			overLimitCount++
		}
		share.CapturedFields = append(share.CapturedFields, "overLimit")
	}
	preview.CombinedSkills = runtimePatchPartyCombinedSkills(*preview)
	encoded, err := encodeLoadoutShareCode(share)
	if err != nil {
		return nil, err
	}
	return &LogsLoadoutShareCandidate{
		LogTime: logTime, PlayerName: playerName, CharacterName: characterName, OwnerCode: ownerCode,
		CharacterHash: share.CharaHash, SigilCount: len(share.Sigils), WeaponName: share.WeaponName,
		OverLimitCount: overLimitCount, CompatibilityCode: encoded.CompatibilityCode, Preview: preview,
	}, nil
}

func readLogsLoadoutShares(path string) ([]LogsLoadoutShareCandidate, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Clean(path))+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("打开 GBFR Logs 数据库失败: %w", err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT time, data FROM logs WHERE version = 1 ORDER BY time DESC LIMIT ?`, logsLoadoutMaximumRecords)
	if err != nil {
		return nil, fmt.Errorf("读取 GBFR Logs 的 logs 表失败: %w", err)
	}
	defer rows.Close()
	result := make([]LogsLoadoutShareCandidate, 0)
	seen := make(map[string]bool)
	for rows.Next() {
		var logTime int64
		var blob []byte
		if err := rows.Scan(&logTime, &blob); err != nil {
			return nil, fmt.Errorf("读取 GBFR Logs 记录失败: %w", err)
		}
		encounter, decodeErr := decodeLogsLoadoutEncounter(blob)
		if decodeErr != nil {
			continue
		}
		for _, player := range encounter.PlayerData {
			candidate, convertErr := logsPlayerLoadoutShare(logTime, player)
			if convertErr != nil || seen[candidate.CompatibilityCode] {
				continue
			}
			seen[candidate.CompatibilityCode] = true
			result = append(result, *candidate)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 GBFR Logs 记录失败: %w", err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("没有找到可映射到 DLC 2.0.2 目录的 v1 因子配装")
	}
	return result, nil
}

func (a *App) SelectLogsLoadoutShares() ([]LogsLoadoutShareCandidate, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择 GBFR Logs 数据库",
		Filters: []runtime.FileFilter{
			{DisplayName: "GBFR Logs (logs.dbw;*.db;*.sqlite)", Pattern: "logs.dbw;*.db;*.sqlite;*.sqlite3"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil || path == "" {
		return nil, err
	}
	return readLogsLoadoutShares(path)
}
