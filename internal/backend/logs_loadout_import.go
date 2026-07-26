package backend

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
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
	ProtocolLabel     string                    `json:"protocolLabel"`
	CapturedFields    []string                  `json:"capturedFields"`
	MissingFields     []string                  `json:"missingFields,omitempty"`
	Warnings          []string                  `json:"warnings,omitempty"`
	CompatibilityCode string                    `json:"compatibilityCode"`
	Preview           *RuntimePatchPartyLoadout `json:"preview"`
}

type logsLoadoutEncounter struct {
	PlayerData [4]*logsLoadoutPlayer `cbor:"playerData"`
}

type logsLoadoutPlayer struct {
	ActorIndex      uint32                      `cbor:"actorIndex"`
	DisplayName     string                      `cbor:"displayName"`
	CharacterName   string                      `cbor:"characterName"`
	CharacterType   logsCharacterType           `cbor:"characterType"`
	Sigils          []logsLoadoutSigil          `cbor:"sigils"`
	Summons         []logsLoadoutSummon         `cbor:"summons"`
	Abilities       []uint32                    `cbor:"abilities"`
	WeaponKey       string                      `cbor:"weaponKey"`
	MasterLevel     uint32                      `cbor:"masterLevel"`
	Skillboard      []uint32                    `cbor:"skillboard"`
	Stats           *logsLoadoutRecordStats     `cbor:"stats"`
	WeaponState     *logsLoadoutWeaponState     `cbor:"weaponState"`
	IsOnline        bool                        `cbor:"isOnline"`
	WeaponInfo      *logsLoadoutWeaponInfo      `cbor:"weaponInfo"`
	OvermasteryInfo *logsLoadoutOvermasteryInfo `cbor:"overmasteryInfo"`
	PlayerStats     *logsLoadoutPlayerStats     `cbor:"playerStats"`
}

type logsCharacterType struct {
	Code        string
	UnknownHash uint32
}

func (value *logsCharacterType) UnmarshalCBOR(data []byte) error {
	var code string
	if err := cbor.Unmarshal(data, &code); err == nil {
		value.Code = code
		value.UnknownHash = 0
		return nil
	}
	var unknown map[string]uint32
	if err := cbor.Unmarshal(data, &unknown); err != nil {
		return fmt.Errorf("decode characterType: %w", err)
	}
	for key, hash := range unknown {
		if strings.EqualFold(key, "Unknown") {
			value.Code = ""
			value.UnknownHash = hash
			return nil
		}
	}
	return fmt.Errorf("decode characterType: unsupported enum variant")
}

func (value logsCharacterType) MarshalCBOR() ([]byte, error) {
	if strings.TrimSpace(value.Code) != "" {
		return cbor.Marshal(value.Code)
	}
	return cbor.Marshal(map[string]uint32{"Unknown": value.UnknownHash})
}

func (value logsCharacterType) String() string {
	if strings.TrimSpace(value.Code) != "" {
		return value.Code
	}
	if value.UnknownHash != 0 {
		return fmt.Sprintf("Unknown(%08X)", value.UnknownHash)
	}
	return ""
}

type logsLoadoutSummon struct {
	SummonID       uint32 `cbor:"summonId"`
	MainTraitID    uint32 `cbor:"mainTraitId"`
	MainTraitLevel uint32 `cbor:"mainTraitLevel"`
	BonusID        uint32 `cbor:"bonusId"`
	BonusLevel     uint32 `cbor:"bonusLevel"`
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

type logsLoadoutWeaponState struct {
	WeaponID          uint32             `cbor:"weaponId"`
	EXP               uint32             `cbor:"exp"`
	StarLevel         uint32             `cbor:"starLevel"`
	PlusMarks         uint32             `cbor:"plusMarks"`
	AwakeningLevel    uint32             `cbor:"awakeningLevel"`
	WrightstoneID     uint32             `cbor:"wrightstoneId"`
	WrightstoneTraits []logsLoadoutTrait `cbor:"wrightstoneTraits"`
	InnateTraits      []logsLoadoutTrait `cbor:"innateTraits"`
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
	if source == nil {
		return nil
	}
	if len(source.WrightstoneTraits) > 0 {
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

type logsLoadoutRecordStats struct {
	Level        uint32  `cbor:"level"`
	HP           uint32  `cbor:"hp"`
	Attack       uint32  `cbor:"attack"`
	Unknown50    uint32  `cbor:"unk50"`
	StunPower    float32 `cbor:"stunPower"`
	CriticalRate float32 `cbor:"criticalRate"`
	Power        uint32  `cbor:"power"`
}

func normalizeLogsOwnerCode(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 6 && strings.EqualFold(value[:2], "PL") {
		return "PL" + value[2:]
	}
	return ""
}

func logsProtocolLabel(player *logsLoadoutPlayer) string {
	if player != nil && (player.WeaponState != nil || len(player.Summons) > 0 || len(player.Abilities) > 0 ||
		player.MasterLevel > 0 || len(player.Skillboard) > 0 || player.Stats != nil || strings.TrimSpace(player.WeaponKey) != "") {
		return runtimePatchMonitorText("GBFR Logs 扩展协议 v1", "GBFR Logs expanded protocol v1")
	}
	if player != nil && player.WeaponInfo != nil && (len(player.WeaponInfo.WrightstoneTraits) > 0 || len(player.WeaponInfo.InnateTraits) > 0) {
		return runtimePatchMonitorText("GBFR Logs 2.0.2 协议 v1", "GBFR Logs 2.0.2 protocol v1")
	}
	return runtimePatchMonitorText("GBFR Logs 传统协议 v1", "GBFR Logs legacy protocol v1")
}

var logsLoadoutCapabilityFields = []string{
	"stats", "sigils", "skills", "summons", "weapon", "weaponSkills", "wrightstone", "mastery", "overLimit",
}

func logsMissingCapturedFields(captured []string) []string {
	present := make(map[string]bool, len(captured))
	for _, field := range captured {
		present[field] = true
	}
	missing := make([]string, 0, len(logsLoadoutCapabilityFields))
	for _, field := range logsLoadoutCapabilityFields {
		if !present[field] {
			missing = append(missing, field)
		}
	}
	return missing
}

func logsPlayerPanelStats(player *logsLoadoutPlayer) (RuntimePatchPartyPanelStats, bool) {
	if player == nil {
		return RuntimePatchPartyPanelStats{}, false
	}
	if source := player.Stats; source != nil {
		return RuntimePatchPartyPanelStats{
			Level: source.Level, TotalHP: source.HP, TotalAttack: source.Attack,
			StunPower: source.StunPower, CriticalRate: source.CriticalRate, TotalPower: source.Power,
		}, true
	}
	if source := player.PlayerStats; source != nil {
		return RuntimePatchPartyPanelStats{
			Level: source.Level, TotalHP: source.TotalHP, TotalAttack: source.TotalAttack,
			StunPower: source.StunPower, CriticalRate: source.CriticalRate, TotalPower: source.TotalPower,
		}, true
	}
	return RuntimePatchPartyPanelStats{}, false
}

func progressionWeaponDefForInternalID(value string) (ProgressionWeaponDef, uint32, bool) {
	catalog, err := loadProgressionCatalog()
	if err != nil || catalog == nil {
		return ProgressionWeaponDef{}, 0, false
	}
	want := strings.TrimSpace(value)
	for _, definition := range catalog.Weapons {
		if !strings.EqualFold(strings.TrimSpace(definition.InternalID), want) {
			continue
		}
		hash, parseErr := ParseHashHex(definition.Hash)
		if parseErr != nil {
			return ProgressionWeaponDef{}, 0, false
		}
		return definition, hash, true
	}
	return ProgressionWeaponDef{}, 0, false
}

type logsEffectiveWeapon struct {
	WeaponID          uint32
	EXP               uint32
	StarLevel         uint32
	PlusMarks         uint32
	AwakeningLevel    uint32
	WrightstoneID     uint32
	WeaponLevel       uint32
	WeaponHP          uint32
	WeaponAttack      uint32
	WrightstoneTraits []logsLoadoutTrait
	InnateTraits      []logsLoadoutTrait
}

func effectiveLogsWeapon(player *logsLoadoutPlayer) logsEffectiveWeapon {
	var result logsEffectiveWeapon
	if player == nil {
		return result
	}
	if source := player.WeaponInfo; source != nil {
		result = logsEffectiveWeapon{
			WeaponID: source.WeaponID, StarLevel: source.StarLevel, PlusMarks: source.PlusMarks,
			AwakeningLevel: source.AwakeningLevel, WrightstoneID: source.WrightstoneID,
			WeaponLevel: source.WeaponLevel, WeaponHP: source.WeaponHP, WeaponAttack: source.WeaponAttack,
			WrightstoneTraits: append([]logsLoadoutTrait(nil), source.effectiveWrightstoneTraits()...),
			InnateTraits:      append([]logsLoadoutTrait(nil), source.InnateTraits...),
		}
	}
	if source := player.WeaponState; source != nil {
		if !runtimePatchPartyEmptyHash(source.WeaponID) {
			result.WeaponID = source.WeaponID
		}
		result.EXP = source.EXP
		result.StarLevel = max(result.StarLevel, source.StarLevel)
		result.PlusMarks = max(result.PlusMarks, source.PlusMarks)
		result.AwakeningLevel = max(result.AwakeningLevel, source.AwakeningLevel)
		if !runtimePatchPartyEmptyHash(source.WrightstoneID) {
			result.WrightstoneID = source.WrightstoneID
		}
		if len(source.WrightstoneTraits) > 0 {
			result.WrightstoneTraits = append([]logsLoadoutTrait(nil), source.WrightstoneTraits...)
		}
		if len(source.InnateTraits) > 0 {
			result.InnateTraits = append([]logsLoadoutTrait(nil), source.InnateTraits...)
		}
	}
	if runtimePatchPartyEmptyHash(result.WeaponID) && strings.TrimSpace(player.WeaponKey) != "" {
		if _, hash, ok := progressionWeaponDefForInternalID(player.WeaponKey); ok {
			result.WeaponID = hash
		}
	}
	return result
}

func logsVersionList(db *sql.DB) ([]int, error) {
	rows, err := db.Query(`SELECT DISTINCT version FROM logs ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var versions []int
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

func validateLogsDatabaseSchema(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(logs)`)
	if err != nil {
		return fmt.Errorf("读取 logs 表结构失败: %w", err)
	}
	defer rows.Close()
	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("解析 logs 表结构失败: %w", err)
		}
		columns[strings.ToLower(strings.TrimSpace(name))] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, required := range []string{"time", "data", "version"} {
		if !columns[required] {
			return fmt.Errorf("不是兼容的 Logs 数据库：logs 表缺少 %s 字段", required)
		}
	}
	return nil
}

func formatLogsVersions(versions []int) string {
	values := make([]string, len(versions))
	for index, version := range versions {
		values[index] = strconv.Itoa(version)
	}
	return strings.Join(values, ", ")
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
	if player == nil || len(player.Sigils) > loadoutMaxSigils {
		return nil, fmt.Errorf("日志玩家因子超过 %d 格", loadoutMaxSigils)
	}
	characterType := player.CharacterType.String()
	ownerCode := normalizeLogsOwnerCode(characterType)
	characterHash, known := runtimeOwnerCharacterHash[ownerCode]
	if !known {
		return nil, fmt.Errorf("日志角色类型 %q 无法映射到存档角色", characterType)
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
		CapturedFields: nil,
	}
	preview := &RuntimePatchPartyLoadout{
		Available: true, Stable: true, SnapshotCount: 1,
		Verification: "logs-db-protocol-v1", Evidence: logsProtocolLabel(player),
		CharacterCode: ownerCode, CharacterHash: share.CharaHash, CharacterName: characterName,
		Sigils: make([]RuntimePatchPartySigil, 0, len(player.Sigils)), OverLimit: make([]RuntimePatchPartyOverLimit, 4),
	}
	warnings := make([]string, 0)
	preview.Stats.Level = 100
	if stats, captured := logsPlayerPanelStats(player); captured {
		preview.Stats = stats
		preview.Stats.Level = 100
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
	if len(share.Sigils) > 0 {
		share.CapturedFields = append(share.CapturedFields, "sigils")
	} else if len(player.Sigils) > 0 {
		return nil, fmt.Errorf("日志玩家没有可导入因子")
	}
	if _, captured := logsPlayerPanelStats(player); captured {
		share.CapturedFields = append(share.CapturedFields, "stats")
	}
	weaponSource := effectiveLogsWeapon(player)
	if !runtimePatchPartyEmptyHash(weaponSource.WeaponID) {
		definition, weaponKnown := progressionWeaponDefForHash(weaponSource.WeaponID)
		if !weaponKnown || definition.OwnerCode != ownerCode {
			return nil, fmt.Errorf("日志武器 %08X 与角色 %s 不匹配", weaponSource.WeaponID, ownerCode)
		}
		weapon := RuntimePatchPartyWeapon{
			Hash: weaponSource.WeaponID, HashHex: hashText(weaponSource.WeaponID), Name: progressionWeaponName(definition), XP: weaponSource.EXP,
			Level: weaponSource.WeaponLevel, StarLevel: weaponSource.StarLevel, PlusMarks: weaponSource.PlusMarks,
			AwakeningLevel: weaponSource.AwakeningLevel, WrightstoneID: weaponSource.WrightstoneID,
			HP: weaponSource.WeaponHP, Attack: weaponSource.WeaponAttack,
		}
		wrightstoneComplete := len(weaponSource.WrightstoneTraits) <= 3
		if !wrightstoneComplete {
			warnings = append(warnings, runtimePatchMonitorText("武器祝福词条超过 3 槽，祝福仅预览且不开放写入", "Wrightstone traits exceed three slots; the wrightstone is preview-only"))
		}
		if !runtimePatchPartyEmptyHash(weaponSource.WrightstoneID) && len(weaponSource.WrightstoneTraits) > 0 {
			wrightstoneCatalog, catalogErr := LoadWrightstoneCatalog()
			if catalogErr != nil {
				return nil, catalogErr
			}
			if wrightstoneCatalog.LookupWrightstoneByHash(weaponSource.WrightstoneID) == nil {
				wrightstoneComplete = false
				warnings = append(warnings, fmt.Sprintf(runtimePatchMonitorText("武器祝福 %08X 未收录，祝福仅预览且不开放写入", "Wrightstone %08X is unknown; the wrightstone is preview-only"), weaponSource.WrightstoneID))
			}
		}
		for _, trait := range weaponSource.WrightstoneTraits {
			if runtimePatchPartyEmptyHash(trait.ID) {
				continue
			}
			name, knownTrait := runtimePatchPartyTraitName(catalog, trait.ID)
			if !knownTrait {
				wrightstoneComplete = false
				warnings = append(warnings, fmt.Sprintf(runtimePatchMonitorText("武器祝福词条 %08X 未收录，祝福仅预览且不开放写入", "Wrightstone trait %08X is unknown; the wrightstone is preview-only"), trait.ID))
				continue
			}
			weapon.Traits = append(weapon.Traits, RuntimePatchPartyTrait{Hash: trait.ID, HashHex: hashText(trait.ID), Name: name, Level: trait.Level})
		}
		weaponSkillsPresent := len(weaponSource.InnateTraits) > 0
		weaponSkillsComplete := weaponSkillsPresent && len(weaponSource.InnateTraits) <= 5
		if weaponSkillsPresent && !weaponSkillsComplete {
			warnings = append(warnings, runtimePatchMonitorText("武器技能超过 5 槽，仅预览且不开放写入", "Weapon skills exceed five slots and are preview-only"))
		}
		for _, trait := range weaponSource.InnateTraits {
			if runtimePatchPartyEmptyHash(trait.ID) {
				continue
			}
			name, knownTrait := runtimePatchPartyTraitName(catalog, trait.ID)
			if !knownTrait {
				weaponSkillsComplete = false
				warnings = append(warnings, fmt.Sprintf(runtimePatchMonitorText("武器技能 %08X 未收录，武器技能仅预览且不开放写入", "Weapon skill %08X is unknown; weapon skills are preview-only"), trait.ID))
				continue
			}
			weapon.Skills = append(weapon.Skills, RuntimePatchPartyTrait{Hash: trait.ID, HashHex: hashText(trait.ID), Name: name, Level: trait.Level})
		}
		share.WeaponHash = hashText(weaponSource.WeaponID)
		share.WeaponName = weapon.Name
		share.Weapon, err = endgameCapturedWeapon(weapon, definition)
		if err != nil {
			return nil, err
		}
		share.CapturedFields = append(share.CapturedFields, "weapon")
		if weaponSkillsComplete {
			share.WeaponSkillHashes = append([]string(nil), share.Weapon.SkillHashes...)
			share.CapturedFields = append(share.CapturedFields, "weaponSkills")
		} else {
			share.Weapon.SkillHashes = nil
		}
		if !wrightstoneComplete {
			share.Weapon.Wrightstone = nil
			share.Weapon.WrightstoneReference = ""
		}
		if wrightstoneComplete && share.Weapon.Wrightstone != nil {
			share.CapturedFields = append(share.CapturedFields, "wrightstone")
		}
		if len(weapon.Traits) > 0 && share.Weapon.Wrightstone == nil {
			warnings = append(warnings, runtimePatchMonitorText("日志记录了武器祝福词条，但没有可部署的祝福石 ID", "Wrightstone traits were recorded without a deployable wrightstone ID"))
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

	if len(player.Abilities) > 0 {
		loadSkillNameCatalog()
		abilities := make([]RuntimePatchPartyAbility, 0, len(player.Abilities))
		abilitiesComplete := len(player.Abilities) == runtimePatchPartyAbilityCount
		for _, hash := range player.Abilities {
			if runtimePatchPartyEmptyHash(hash) {
				abilitiesComplete = false
				continue
			}
			name := strings.TrimSpace(skillNameForHash(hash))
			if name == "" || !skillBelongsToOwner(hash, ownerCode) {
				abilitiesComplete = false
				continue
			}
			abilities = append(abilities, RuntimePatchPartyAbility{Hash: hash, HashHex: hashText(hash), Key: skillKeyForHash(hash), Name: name})
		}
		preview.Abilities = abilities
		if abilitiesComplete && len(abilities) == runtimePatchPartyAbilityCount {
			for _, ability := range abilities {
				share.Skills = append(share.Skills, LoadoutSkill{Hash: ability.HashHex, Key: ability.Key, Name: ability.Name})
			}
			share.CapturedFields = append(share.CapturedFields, "skills")
		} else {
			warnings = append(warnings, runtimePatchMonitorText("角色技能记录不完整，仅用于预览，不开放写入", "Ability capture was incomplete and is preview-only"))
		}
	}

	if len(player.Summons) > 0 {
		summonCatalog, catalogErr := loadSummonStatCatalog()
		if catalogErr != nil {
			return nil, catalogErr
		}
		summons := make([]RuntimePatchPartySummon, 0, len(player.Summons))
		summonsComplete := len(player.Summons) == runtimePatchPartySummonCount
		for index, source := range player.Summons {
			typeDef, typeKnown := summonCatalog.types[source.SummonID]
			mainDef, mainKnown := summonCatalog.main[source.MainTraitID]
			subDef, subKnown := summonCatalog.sub[source.BonusID]
			if !typeKnown || !mainKnown || !subKnown || source.MainTraitLevel > uint32(max(mainDef.MaxLevel, 0)) || source.BonusLevel >= uint32(max(subDef.MaxLevel, 0)) {
				summonsComplete = false
				continue
			}
			summons = append(summons, RuntimePatchPartySummon{
				Index: index, TypeHash: source.SummonID, TypeHashHex: hashText(source.SummonID), Name: typeDef.Name,
				MainTraitHash: source.MainTraitID, MainTraitHex: hashText(source.MainTraitID), MainTraitName: mainDef.Name, MainTraitLevel: source.MainTraitLevel,
				SubParamHash: source.BonusID, SubParamHex: hashText(source.BonusID), SubParamName: subDef.Name, SubParamLevel: source.BonusLevel + 1,
			})
		}
		preview.Summons = summons
		if summonsComplete && len(summons) == runtimePatchPartySummonCount {
			for _, summon := range summons {
				mainLevel, subLevel := int(summon.MainTraitLevel), int(summon.SubParamLevel)
				if definition := summonCatalog.main[summon.MainTraitHash]; definition.MaxLevel > mainLevel {
					mainLevel = definition.MaxLevel
				}
				if definition := summonCatalog.sub[summon.SubParamHash]; definition.MaxLevel > subLevel {
					subLevel = definition.MaxLevel
				}
				share.Summons = append(share.Summons, LoadoutShareSummon{
					TypeHash: summon.TypeHashHex, Name: summon.Name,
					MainTraitHash: summon.MainTraitHex, MainTraitLevel: mainLevel,
					SubParamHash: summon.SubParamHex, SubParamLevel: subLevel, Rank: 3,
				})
			}
			share.CapturedFields = append(share.CapturedFields, "summons")
			for index := range preview.Summons {
				preview.Summons[index].MainTraitLevel = uint32(share.Summons[index].MainTraitLevel)
				preview.Summons[index].SubParamLevel = uint32(share.Summons[index].SubParamLevel)
			}
		} else {
			warnings = append(warnings, runtimePatchMonitorText("召唤石记录不完整，仅用于预览，不开放写入", "Summon capture was incomplete and is preview-only"))
		}
	}

	preview.MasterLevel = player.MasterLevel
	if player.MasterLevel > 55 {
		warnings = append(warnings, fmt.Sprintf(runtimePatchMonitorText("Master Level %d 超出 0..55，已忽略", "Master Level %d is outside 0..55 and was ignored"), player.MasterLevel))
		preview.MasterLevel = 0
	}
	if len(player.Skillboard) > 0 {
		mastery := make([]LoadoutMasteryNode, 0, min(len(player.Skillboard), loadoutMaxMastery))
		masteryHashes := make([]uint32, 0, min(len(player.Skillboard), loadoutMaxMastery))
		masteryComplete := len(player.Skillboard) <= loadoutMaxMastery
		seenMastery := make(map[uint32]bool)
		for _, hash := range player.Skillboard {
			if runtimePatchPartyEmptyHash(hash) {
				continue
			}
			if seenMastery[hash] {
				masteryComplete = false
				continue
			}
			seenMastery[hash] = true
			definition, knownDefinition := skillboardNodeForHash(hash)
			node, knownNode := loadoutMasteryNodeForHash(hash)
			if !knownDefinition || !knownNode || (definition.Char != "" && definition.Char != ownerCode) {
				masteryComplete = false
				continue
			}
			mastery = append(mastery, node)
			masteryHashes = append(masteryHashes, hash)
		}
		preview.Mastery = mastery
		if _, quotaErr := validateMasteryQuota(masteryHashes, ownerCode, false); quotaErr != nil {
			masteryComplete = false
		}
		if summary, summaryErr := summarizeMasteryHashes(ownerCode, masteryHashes); summaryErr == nil {
			preview.MasterySummary = summary
		} else {
			masteryComplete = false
		}
		if masteryComplete && len(mastery) > 0 {
			preview.MasteryAvailable = true
			share.MasteryHashes = make([]string, loadoutMaxMastery)
			for index := range share.MasteryHashes {
				share.MasteryHashes[index] = hashText(EmptyHash)
			}
			for index, node := range mastery {
				share.MasteryHashes[index] = node.Hash
			}
			share.CapturedFields = append(share.CapturedFields, "mastery")
		} else {
			preview.MasteryAvailable = false
			preview.MasteryUnavailableReason = runtimePatchMonitorText("专精节点记录包含未收录或超限内容，仅展示已识别节点", "Mastery contained unknown or excess nodes; only recognized nodes are shown")
			warnings = append(warnings, preview.MasteryUnavailableReason)
		}
	} else {
		preview.MasteryAvailable = false
		preview.MasteryUnavailableReason = runtimePatchMonitorText("该日志没有记录专精节点", "This log did not record mastery nodes")
	}
	overLimitCount := 0
	if player.OvermasteryInfo != nil && len(player.OvermasteryInfo.Overmasteries) > 0 {
		overLimitComplete := len(player.OvermasteryInfo.Overmasteries) <= 4
		if !overLimitComplete {
			warnings = append(warnings, runtimePatchMonitorText("上限突破记录超过 4 槽，仅预览已识别内容且不开放写入", "Overmastery exceeds four slots; recognized entries are preview-only"))
		}
		share.OverLimit = make([]LoadoutShareOverLimit, 4)
		for index := range share.OverLimit {
			share.OverLimit[index].Index = index
		}
		for index, source := range player.OvermasteryInfo.Overmasteries[:min(len(player.OvermasteryInfo.Overmasteries), 4)] {
			if runtimePatchPartyEmptyHash(source.ID) {
				continue
			}
			if _, known := overLimitCatalog[source.ID]; !known || source.Flags == 0 || source.Flags > 0x200 || source.Flags&(source.Flags-1) != 0 {
				overLimitComplete = false
				warnings = append(warnings, fmt.Sprintf(runtimePatchMonitorText("上限突破第 %d 槽无效，仅预览已识别内容且不开放写入", "Overmastery slot %d is invalid; recognized entries are preview-only"), index+1))
				continue
			}
			definition := overLimitCatalog[source.ID]
			share.OverLimit[index] = LoadoutShareOverLimit{Index: index, AttributeHash: hashText(source.ID), Level: 10}
			preview.OverLimit[index] = RuntimePatchPartyOverLimit{Index: index, AttributeHash: source.ID, HashHex: hashText(source.ID), Name: definition.name, Flags: 0x200, Level: 10, Value: definition.maxValue}
			overLimitCount++
		}
		if overLimitComplete {
			share.CapturedFields = append(share.CapturedFields, "overLimit")
		} else {
			share.OverLimit = nil
		}
	}
	if len(share.CapturedFields) == 0 || (len(share.CapturedFields) == 1 && share.CapturedFields[0] == "stats") {
		return nil, fmt.Errorf("日志玩家没有可部署的配装范围")
	}
	preview.CombinedSkills = runtimePatchPartyCombinedSkills(*preview)
	encoded, err := encodeLoadoutShareCode(share)
	if err != nil {
		return nil, err
	}
	return &LogsLoadoutShareCandidate{
		LogTime: logTime, PlayerName: playerName, CharacterName: characterName, OwnerCode: ownerCode,
		CharacterHash: share.CharaHash, SigilCount: len(share.Sigils), WeaponName: share.WeaponName,
		OverLimitCount: overLimitCount, ProtocolLabel: logsProtocolLabel(player),
		CapturedFields: append([]string(nil), share.CapturedFields...), MissingFields: logsMissingCapturedFields(share.CapturedFields),
		Warnings: warnings, CompatibilityCode: encoded.CompatibilityCode, Preview: preview,
	}, nil
}

func logsReadOnlyDatabaseDSN(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return "", err
	}
	slashPath := filepath.ToSlash(absolute)
	if !strings.HasPrefix(slashPath, "/") {
		slashPath = "/" + slashPath
	}
	return (&url.URL{Scheme: "file", Path: slashPath, RawQuery: "mode=ro"}).String(), nil
}

func readLogsLoadoutShares(path string) ([]LogsLoadoutShareCandidate, error) {
	dsn, err := logsReadOnlyDatabaseDSN(path)
	if err != nil {
		return nil, fmt.Errorf("解析 GBFR Logs 数据库路径失败: %w", err)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 GBFR Logs 数据库失败: %w", err)
	}
	defer db.Close()
	if err := validateLogsDatabaseSchema(db); err != nil {
		return nil, err
	}
	versions, err := logsVersionList(db)
	if err != nil {
		return nil, fmt.Errorf("读取 Logs 协议版本失败: %w", err)
	}
	hasV1 := false
	for _, version := range versions {
		if version == 1 {
			hasV1 = true
			break
		}
	}
	if !hasV1 {
		label := formatLogsVersions(versions)
		if label == "" {
			label = "无记录"
		}
		return nil, fmt.Errorf("数据库没有可导入的协议 v1 记录；实际版本：%s", label)
	}
	rows, err := db.Query(`SELECT time, data FROM logs WHERE version = 1 ORDER BY time DESC LIMIT ?`, logsLoadoutMaximumRecords)
	if err != nil {
		return nil, fmt.Errorf("读取 GBFR Logs 的 logs 表失败: %w", err)
	}
	defer rows.Close()
	result := make([]LogsLoadoutShareCandidate, 0)
	seen := make(map[string]bool)
	totalRows, decodedRows, skippedRows := 0, 0, 0
	lastReason := ""
	for rows.Next() {
		totalRows++
		var logTime int64
		var blob []byte
		if err := rows.Scan(&logTime, &blob); err != nil {
			return nil, fmt.Errorf("读取 GBFR Logs 记录失败: %w", err)
		}
		encounter, decodeErr := decodeLogsLoadoutEncounter(blob)
		if decodeErr != nil {
			skippedRows++
			lastReason = decodeErr.Error()
			continue
		}
		decodedRows++
		convertedInRow := 0
		for _, player := range encounter.PlayerData {
			candidate, convertErr := logsPlayerLoadoutShare(logTime, player)
			if convertErr != nil {
				lastReason = convertErr.Error()
				continue
			}
			identity := fmt.Sprintf("%d\x00%d\x00%s\x00%s\x00%s", logTime, player.ActorIndex, strings.TrimSpace(player.DisplayName), candidate.OwnerCode, candidate.CompatibilityCode)
			if seen[identity] {
				continue
			}
			seen[identity] = true
			result = append(result, *candidate)
			convertedInRow++
		}
		if convertedInRow == 0 {
			skippedRows++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 GBFR Logs 记录失败: %w", err)
	}
	if len(result) == 0 {
		if lastReason == "" {
			lastReason = "记录中没有包含可映射到 DLC 2.0.2 目录的角色因子"
		}
		return nil, fmt.Errorf("没有找到可导入的 Logs 配装：v1 记录 %d，成功解码 %d，跳过 %d；最后原因：%s", totalRows, decodedRows, skippedRows, lastReason)
	}
	return result, nil
}

func (a *App) SelectLogsLoadoutShares() ([]LogsLoadoutShareCandidate, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择 GBFR Logs 数据库",
		Filters: []runtime.FileFilter{
			{DisplayName: "GBFR Logs (logs.db;*.db;*.sqlite)", Pattern: "logs.db;*.db;*.sqlite;*.sqlite3"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil || path == "" {
		return nil, err
	}
	return readLogsLoadoutShares(path)
}
