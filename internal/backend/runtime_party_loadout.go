package backend

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const (
	runtimePatchPartySpecifiedInstanceOffset = uintptr(0x70)
	runtimePatchPartyStatsOffset             = uintptr(0x15030)
	runtimePatchPartyStatsSize               = 0x1C
	runtimePatchPartyWeaponOffset            = uintptr(0x15080)
	runtimePatchPartyWeaponSize              = 0xCC
	runtimePatchPartyOvermasteryOffset       = uintptr(0x1A8E8)
	runtimePatchPartyOvermasterySize         = 0x40
	runtimePatchPartySigilPointerOffset      = uintptr(0x1AE90)
	runtimePatchPartySigilEntrySize          = 0x24
	runtimePatchPartySigilCount              = 12
	runtimePatchPartySigilListSize           = 0x230
	runtimePatchPartyAbilityOffset           = uintptr(0x1AB24)
	runtimePatchPartyAbilityCount            = 4
	runtimePatchPartyCharacterHashOffset     = uintptr(0x1AB40)
	runtimePatchPartyMasterLevelOffset       = uintptr(0x1AB90)
	runtimePatchPartySummonOffset            = uintptr(0x1AE08)
	runtimePatchPartySummonCount             = 4
	runtimePatchPartySummonStride            = 0x1C
	runtimePatchPartySummonSize              = runtimePatchPartySummonCount * runtimePatchPartySummonStride
	runtimePatchPartyCharaPowerRVA           = uintptr(0x7C24A78)
	runtimePatchPartyMasteryUnlockOffset     = uintptr(0x15168)
	runtimePatchPartyMasteryUnlockCount      = 400
	runtimePatchPartyMasteryUnlockStride     = 0x38
	runtimePatchPartyMasteryUnlockSize       = runtimePatchPartyMasteryUnlockCount * runtimePatchPartyMasteryUnlockStride
	runtimePatchPartyMaximumMasteryNodes     = 128
	runtimePatchPartyMinimumMasteryHash      = uint32(0x0031DB23)
)

type runtimePatchPartyMapLayout struct {
	endOffset     uintptr
	bucketsOffset uintptr
	maskOffset    uintptr
}

var (
	runtimePatchPartySkillboardCharMap = runtimePatchPartyMapLayout{endOffset: 0x728, bucketsOffset: 0x738, maskOffset: 0x750}
	runtimePatchPartySkillboardNodeMap = runtimePatchPartyMapLayout{endOffset: 0x320, bucketsOffset: 0x330, maskOffset: 0x348}
)

type RuntimePatchPartyPanelStats struct {
	Level        uint32  `json:"level"`
	TotalHP      uint32  `json:"totalHp"`
	TotalAttack  uint32  `json:"totalAttack"`
	StunPower    float32 `json:"stunPower"`
	CriticalRate float32 `json:"criticalRate"`
	TotalPower   uint32  `json:"totalPower"`
}

type RuntimePatchPartyTrait struct {
	Hash    uint32 `json:"hash"`
	HashHex string `json:"hashHex"`
	Name    string `json:"name"`
	Level   uint32 `json:"level"`
}

type RuntimePatchPartyAbility struct {
	Hash    uint32 `json:"hash"`
	HashHex string `json:"hashHex"`
	Key     string `json:"key,omitempty"`
	Name    string `json:"name"`
}

type RuntimePatchPartySummon struct {
	Index          int    `json:"index"`
	TypeHash       uint32 `json:"typeHash"`
	TypeHashHex    string `json:"typeHashHex"`
	Name           string `json:"name"`
	MainTraitHash  uint32 `json:"mainTraitHash"`
	MainTraitHex   string `json:"mainTraitHashHex"`
	MainTraitName  string `json:"mainTraitName"`
	MainTraitLevel uint32 `json:"mainTraitLevel"`
	SubParamHash   uint32 `json:"subParamHash"`
	SubParamHex    string `json:"subParamHashHex"`
	SubParamName   string `json:"subParamName"`
	SubParamLevel  uint32 `json:"subParamLevel"`
}

type RuntimePatchPartyWeapon struct {
	Hash           uint32                   `json:"hash"`
	HashHex        string                   `json:"hashHex"`
	Name           string                   `json:"name"`
	XP             uint32                   `json:"xp"`
	Level          uint32                   `json:"level"`
	StarLevel      uint32                   `json:"starLevel"`
	PlusMarks      uint32                   `json:"plusMarks"`
	AwakeningLevel uint32                   `json:"awakeningLevel"`
	WrightstoneID  uint32                   `json:"wrightstoneId"`
	HP             uint32                   `json:"hp"`
	Attack         uint32                   `json:"attack"`
	Traits         []RuntimePatchPartyTrait `json:"traits"`
	Skills         []RuntimePatchPartyTrait `json:"skills"`
}

type RuntimePatchPartyOverLimit struct {
	Index         int     `json:"index"`
	AttributeHash uint32  `json:"attributeHash"`
	HashHex       string  `json:"hashHex,omitempty"`
	Name          string  `json:"name,omitempty"`
	Flags         uint32  `json:"flags"`
	Level         uint32  `json:"level"`
	Value         float32 `json:"value"`
}

type RuntimePatchPartySigil struct {
	Index                 int    `json:"index"`
	Hash                  uint32 `json:"hash"`
	HashHex               string `json:"hashHex"`
	Name                  string `json:"name"`
	Level                 uint32 `json:"level"`
	PrimaryTraitHash      uint32 `json:"primaryTraitHash"`
	PrimaryTraitHashHex   string `json:"primaryTraitHashHex"`
	PrimaryTraitName      string `json:"primaryTraitName"`
	PrimaryTraitLevel     uint32 `json:"primaryTraitLevel"`
	SecondaryTraitHash    uint32 `json:"secondaryTraitHash,omitempty"`
	SecondaryTraitHashHex string `json:"secondaryTraitHashHex,omitempty"`
	SecondaryTraitName    string `json:"secondaryTraitName,omitempty"`
	SecondaryTraitLevel   uint32 `json:"secondaryTraitLevel,omitempty"`
}

type RuntimePatchPartyLoadout struct {
	Available                bool                         `json:"available"`
	Stable                   bool                         `json:"stable"`
	SnapshotCount            int                          `json:"snapshotCount"`
	Verification             string                       `json:"verification"`
	Evidence                 string                       `json:"evidence"`
	UnavailableReason        string                       `json:"unavailableReason,omitempty"`
	Layout                   string                       `json:"layout,omitempty"`
	CharacterCode            string                       `json:"characterCode,omitempty"`
	CharacterHash            string                       `json:"characterHash,omitempty"`
	CharacterName            string                       `json:"characterName,omitempty"`
	RuntimeLabel             string                       `json:"runtimeLabel,omitempty"`
	Online                   bool                         `json:"online"`
	PartyIndex               uint32                       `json:"partyIndex"`
	Stats                    RuntimePatchPartyPanelStats  `json:"stats"`
	Weapon                   RuntimePatchPartyWeapon      `json:"weapon"`
	Abilities                []RuntimePatchPartyAbility   `json:"abilities"`
	Summons                  []RuntimePatchPartySummon    `json:"summons"`
	MasterLevel              uint32                       `json:"masterLevel"`
	MasteryAvailable         bool                         `json:"masteryAvailable"`
	MasteryUnavailableReason string                       `json:"masteryUnavailableReason,omitempty"`
	Mastery                  []LoadoutMasteryNode         `json:"mastery"`
	MasterySummary           *MasteryBuildSummary         `json:"masterySummary,omitempty"`
	CombinedSkills           []TraitBonus                 `json:"combinedSkills"`
	Sigils                   []RuntimePatchPartySigil     `json:"sigils"`
	OverLimit                []RuntimePatchPartyOverLimit `json:"overLimit"`
}

type runtimePatchPartyLoadoutCandidate struct {
	loadout     RuntimePatchPartyLoadout
	base        uintptr
	fingerprint [32]byte
}

var runtimePatchPartyCharacterNames = map[string][2]string{
	"PL0000": {"古兰", "Gran"}, "PL0100": {"姬塔", "Djeeta"}, "PL0200": {"卡塔莉娜", "Katalina"},
	"PL0300": {"拉卡姆", "Rackam"}, "PL0400": {"伊欧", "Io"}, "PL0500": {"欧根", "Eugen"},
	"PL0600": {"萝赛塔", "Rosetta"}, "PL0700": {"菲莉", "Ferry"}, "PL0800": {"兰斯洛特", "Lancelot"},
	"PL0900": {"巴恩", "Vane"}, "PL1000": {"珀西瓦尔", "Percival"}, "PL1100": {"齐格飞", "Siegfried"},
	"PL1200": {"夏洛特", "Charlotta"}, "PL1300": {"尤达拉哈", "Yodarha"}, "PL1400": {"娜露梅", "Narmaya"},
	"PL1500": {"冈达葛萨", "Ghandagoza"}, "PL1600": {"泽塔", "Zeta"}, "PL1700": {"巴萨拉卡", "Vaseraga"},
	"PL1800": {"卡莉奥丝特罗", "Cagliostro"}, "PL1900": {"伊德", "Id"}, "PL2100": {"圣德芬", "Sandalphon"},
	"PL2200": {"希耶提", "Seofon"}, "PL2300": {"索恩", "Tweyen"}, "PL2400": {"伽兰查", "Gallanza"},
	"PL2500": {"玛琪拉菲菈", "Maglielle"}, "PL2600": {"贝阿朵丽丝", "Beatrix"}, "PL2700": {"尤斯提斯", "Eustace"},
	"PL2800": {"芙劳", "Fraux"}, "PL2900": {"菲迪埃尔", "Fediel"},
}

func readRuntimePatchPartyLoadoutCandidates(memory runtimePatchPartyMemory, entity uintptr) (RuntimePatchPartyLoadout, uintptr, [32]byte, error) {
	var zero [32]byte
	if memory == nil || !plausibleRuntimePatchPartyPointer(entity) {
		return RuntimePatchPartyLoadout{}, 0, zero, fmt.Errorf("runtime entity pointer is invalid")
	}

	bases := []struct {
		address uintptr
		layout  string
	}{{entity, "entity+{0x15030,0x15080,0x1AE90}"}}
	if slot, ok := checkedRuntimePatchMonitorAddress(entity, runtimePatchPartySpecifiedInstanceOffset); ok {
		if specified, err := readRuntimePatchPointer(memory, slot); err == nil && plausibleRuntimePatchPartyPointer(specified) && specified != entity {
			bases = append(bases, struct {
				address uintptr
				layout  string
			}{specified, "entity+0x70 -> instance+{0x15030,0x15080,0x1AE90}"})
		}
	}

	valid := make([]runtimePatchPartyLoadoutCandidate, 0, len(bases))
	errorsFound := make([]string, 0, len(bases))
	for _, base := range bases {
		loadout, fingerprint, err := readRuntimePatchPartyLoadoutAt(memory, base.address, base.layout)
		if err != nil {
			errorsFound = append(errorsFound, err.Error())
			continue
		}
		valid = append(valid, runtimePatchPartyLoadoutCandidate{loadout: loadout, base: base.address, fingerprint: fingerprint})
	}
	if len(valid) == 0 {
		return RuntimePatchPartyLoadout{}, 0, zero, fmt.Errorf("no bounded loadout candidate passed: %s", strings.Join(errorsFound, "; "))
	}
	if len(valid) != 1 {
		return RuntimePatchPartyLoadout{}, 0, zero, fmt.Errorf("multiple loadout candidates passed; layout is ambiguous")
	}
	return valid[0].loadout, valid[0].base, valid[0].fingerprint, nil
}

func readRuntimePatchPartyLoadoutAt(memory runtimePatchPartyMemory, base uintptr, layout string) (RuntimePatchPartyLoadout, [32]byte, error) {
	return readRuntimePatchPartyLoadoutAtWithModule(memory, 0, base, layout)
}

func readRuntimePatchPartyLoadoutAtWithModule(memory runtimePatchPartyMemory, moduleBase, base uintptr, layout string) (RuntimePatchPartyLoadout, [32]byte, error) {
	var zero [32]byte
	statsRaw, err := readRuntimePatchPartyBlock(memory, base, runtimePatchPartyStatsOffset, runtimePatchPartyStatsSize)
	if err != nil {
		return RuntimePatchPartyLoadout{}, zero, fmt.Errorf("stats: %w", err)
	}
	weaponRaw, err := readRuntimePatchPartyBlock(memory, base, runtimePatchPartyWeaponOffset, runtimePatchPartyWeaponSize)
	if err != nil {
		return RuntimePatchPartyLoadout{}, zero, fmt.Errorf("weapon: %w", err)
	}
	overmasteryRaw, err := readRuntimePatchPartyBlock(memory, base, runtimePatchPartyOvermasteryOffset, runtimePatchPartyOvermasterySize)
	if err != nil {
		return RuntimePatchPartyLoadout{}, zero, fmt.Errorf("overmastery: %w", err)
	}
	sigilSlot, ok := checkedRuntimePatchMonitorAddress(base, runtimePatchPartySigilPointerOffset)
	if !ok {
		return RuntimePatchPartyLoadout{}, zero, fmt.Errorf("sigil pointer address overflow")
	}
	sigilPointer, err := readRuntimePatchPointer(memory, sigilSlot)
	if err != nil || !plausibleRuntimePatchPartyPointer(sigilPointer) {
		return RuntimePatchPartyLoadout{}, zero, fmt.Errorf("sigil pointer is unavailable or invalid")
	}
	sigilRaw := make([]byte, runtimePatchPartySigilListSize)
	if err := memory.ReadAt(sigilPointer, sigilRaw); err != nil {
		return RuntimePatchPartyLoadout{}, zero, fmt.Errorf("sigil list: %w", err)
	}

	loadout, err := decodeRuntimePatchPartyLoadout(statsRaw, weaponRaw, sigilRaw, overmasteryRaw, layout)
	if err != nil {
		return RuntimePatchPartyLoadout{}, zero, err
	}
	fingerprintInput := make([]byte, 0, len(statsRaw)+len(weaponRaw)+len(sigilRaw)+len(overmasteryRaw)+runtimePatchPartyAbilityCount*4+runtimePatchPartySummonSize+8)
	fingerprintInput = append(fingerprintInput, statsRaw...)
	fingerprintInput = append(fingerprintInput, weaponRaw...)
	fingerprintInput = append(fingerprintInput, sigilRaw...)
	fingerprintInput = append(fingerprintInput, overmasteryRaw...)
	if moduleBase != 0 {
		expansionFingerprint, expansionErr := readRuntimePatchPartyExpansionLoadout(memory, moduleBase, base, &loadout)
		if expansionErr != nil {
			return RuntimePatchPartyLoadout{}, zero, expansionErr
		}
		fingerprintInput = append(fingerprintInput, expansionFingerprint...)
	}
	return loadout, sha256.Sum256(fingerprintInput), nil
}

func readRuntimePatchPartyExpansionLoadout(memory runtimePatchPartyMemory, moduleBase, base uintptr, loadout *RuntimePatchPartyLoadout) ([]byte, error) {
	if memory == nil || moduleBase == 0 || loadout == nil {
		return nil, fmt.Errorf("runtime expansion loadout parameters are invalid")
	}
	abilityRaw, err := readRuntimePatchPartyBlock(memory, base, runtimePatchPartyAbilityOffset, runtimePatchPartyAbilityCount*4)
	if err != nil {
		return nil, fmt.Errorf("abilities: %w", err)
	}
	characterRaw, err := readRuntimePatchPartyBlock(memory, base, runtimePatchPartyCharacterHashOffset, 4)
	if err != nil {
		return nil, fmt.Errorf("character hash: %w", err)
	}
	masterLevelRaw, err := readRuntimePatchPartyBlock(memory, base, runtimePatchPartyMasterLevelOffset, 4)
	if err != nil {
		return nil, fmt.Errorf("master level: %w", err)
	}
	summonRaw, err := readRuntimePatchPartyBlock(memory, base, runtimePatchPartySummonOffset, runtimePatchPartySummonSize)
	if err != nil {
		return nil, fmt.Errorf("summons: %w", err)
	}

	expectedCharacterHash, ok := runtimeOwnerCharacterHash[loadout.CharacterCode]
	if !ok {
		return nil, fmt.Errorf("character %s has no runtime hash mapping", loadout.CharacterCode)
	}
	characterHash := binary.LittleEndian.Uint32(characterRaw)
	if characterHash != expectedCharacterHash {
		return nil, fmt.Errorf("runtime character hash %08X does not match %s (%08X)", characterHash, loadout.CharacterCode, expectedCharacterHash)
	}
	loadout.CharacterHash = fmt.Sprintf("%08X", characterHash)

	loadSkillNameCatalog()
	loadout.Abilities = make([]RuntimePatchPartyAbility, 0, runtimePatchPartyAbilityCount)
	for index := 0; index < runtimePatchPartyAbilityCount; index++ {
		hash := binary.LittleEndian.Uint32(abilityRaw[index*4 : index*4+4])
		if runtimePatchPartyEmptyHash(hash) {
			continue
		}
		if !skillBelongsToOwner(hash, loadout.CharacterCode) {
			return nil, fmt.Errorf("ability %d hash %08X does not belong to %s", index+1, hash, loadout.CharacterCode)
		}
		name := strings.TrimSpace(skillNameForHash(hash))
		if name == "" {
			return nil, fmt.Errorf("ability %d hash %08X is missing from the 2.0.2 catalog", index+1, hash)
		}
		loadout.Abilities = append(loadout.Abilities, RuntimePatchPartyAbility{
			Hash: hash, HashHex: fmt.Sprintf("%08X", hash), Key: skillKeyForHash(hash), Name: name,
		})
	}

	loadout.MasterLevel = binary.LittleEndian.Uint32(masterLevelRaw)
	if loadout.MasterLevel > 55 {
		return nil, fmt.Errorf("master level %d is outside the verified 0..55 range", loadout.MasterLevel)
	}
	summonCatalog, err := loadSummonStatCatalog()
	if err != nil {
		return nil, err
	}
	loadout.Summons = make([]RuntimePatchPartySummon, 0, runtimePatchPartySummonCount)
	for index := 0; index < runtimePatchPartySummonCount; index++ {
		offset := index * runtimePatchPartySummonStride
		typeHash := binary.LittleEndian.Uint32(summonRaw[offset : offset+4])
		if runtimePatchPartyEmptyHash(typeHash) {
			continue
		}
		mainHash := binary.LittleEndian.Uint32(summonRaw[offset+0x08 : offset+0x0C])
		subHash := binary.LittleEndian.Uint32(summonRaw[offset+0x0C : offset+0x10])
		mainLevel := binary.LittleEndian.Uint32(summonRaw[offset+0x10 : offset+0x14])
		subLevel := binary.LittleEndian.Uint32(summonRaw[offset+0x14 : offset+0x18])
		typeDef, typeKnown := summonCatalog.types[typeHash]
		mainDef, mainKnown := summonCatalog.main[mainHash]
		subDef, subKnown := summonCatalog.sub[subHash]
		if !typeKnown || !mainKnown || !subKnown {
			return nil, fmt.Errorf("summon %d failed catalog validation: type=%08X main=%08X sub=%08X", index+1, typeHash, mainHash, subHash)
		}
		if mainLevel > uint32(max(mainDef.MaxLevel, 0)) || subLevel > uint32(max(subDef.MaxLevel, 0)) {
			return nil, fmt.Errorf("summon %d levels %d/%d exceed catalog maximums %d/%d", index+1, mainLevel, subLevel, mainDef.MaxLevel, subDef.MaxLevel)
		}
		loadout.Summons = append(loadout.Summons, RuntimePatchPartySummon{
			Index: index, TypeHash: typeHash, TypeHashHex: fmt.Sprintf("%08X", typeHash), Name: typeDef.Name,
			MainTraitHash: mainHash, MainTraitHex: fmt.Sprintf("%08X", mainHash), MainTraitName: mainDef.Name, MainTraitLevel: mainLevel,
			SubParamHash: subHash, SubParamHex: fmt.Sprintf("%08X", subHash), SubParamName: subDef.Name, SubParamLevel: subLevel,
		})
	}

	mastery, masteryFingerprint, masteryErr := readRuntimePatchPartySkillboard(memory, moduleBase, base, loadout.CharacterCode, characterHash)
	if masteryErr != nil {
		loadout.MasteryAvailable = false
		loadout.MasteryUnavailableReason = masteryErr.Error()
		loadout.Mastery = nil
	} else {
		loadout.MasteryAvailable = true
		loadout.MasteryUnavailableReason = ""
		loadout.Mastery = mastery
		hashes := make([]uint32, 0, len(mastery))
		for _, node := range mastery {
			if hash, parseErr := ParseHashHex(node.Hash); parseErr == nil {
				hashes = append(hashes, hash)
			}
		}
		loadout.MasterySummary, _ = summarizeMasteryHashes(loadout.CharacterCode, hashes)
	}
	loadout.CombinedSkills = runtimePatchPartyCombinedSkills(*loadout)
	fingerprint := make([]byte, 0, len(abilityRaw)+len(characterRaw)+len(masterLevelRaw)+len(summonRaw)+len(masteryFingerprint))
	fingerprint = append(fingerprint, abilityRaw...)
	fingerprint = append(fingerprint, characterRaw...)
	fingerprint = append(fingerprint, masterLevelRaw...)
	fingerprint = append(fingerprint, summonRaw...)
	fingerprint = append(fingerprint, masteryFingerprint...)
	return fingerprint, nil
}

func readRuntimePatchPartySkillboard(memory runtimePatchPartyMemory, moduleBase, base uintptr, ownerCode string, characterHash uint32) ([]LoadoutMasteryNode, []byte, error) {
	managerSlot, ok := checkedRuntimePatchMonitorAddress(moduleBase, runtimePatchPartyCharaPowerRVA)
	if !ok {
		return nil, nil, fmt.Errorf("CharaPower manager address overflow")
	}
	manager, err := readRuntimePatchPointer(memory, managerSlot)
	if err != nil || !plausibleRuntimePatchPartyPointer(manager) {
		return nil, nil, fmt.Errorf("CharaPower manager pointer is unavailable or invalid")
	}
	charNode, err := findRuntimePatchPartyMapNode(memory, manager, runtimePatchPartySkillboardCharMap, characterHash)
	if err != nil {
		return nil, nil, fmt.Errorf("character skillboard map: %w", err)
	}
	begin, err := readRuntimePatchPointer(memory, charNode+0x18)
	if err != nil || !plausibleRuntimePatchPartyPointer(begin) {
		return nil, nil, fmt.Errorf("character skillboard key vector begin is invalid")
	}
	end, err := readRuntimePatchPointer(memory, charNode+0x20)
	if err != nil || end < begin || (end-begin)%8 != 0 {
		return nil, nil, fmt.Errorf("character skillboard key vector bounds are invalid")
	}
	count := (end - begin) / 8
	if count == 0 || count > 512 {
		return nil, nil, fmt.Errorf("character skillboard key count %d is outside 1..512", count)
	}
	keyBytes := make([]byte, int(count)*8)
	if err := memory.ReadAt(begin, keyBytes); err != nil {
		return nil, nil, fmt.Errorf("character skillboard key vector: %w", err)
	}
	unlockRaw, err := readRuntimePatchPartyBlock(memory, base, runtimePatchPartyMasteryUnlockOffset, runtimePatchPartyMasteryUnlockSize)
	if err != nil {
		return nil, nil, fmt.Errorf("skillboard unlock array: %w", err)
	}
	unlocks := make(map[uint32]uint32)
	for index := 0; index < runtimePatchPartyMasteryUnlockCount; index++ {
		offset := index * runtimePatchPartyMasteryUnlockStride
		id := binary.LittleEndian.Uint32(unlockRaw[offset : offset+4])
		if runtimePatchPartyEmptyHash(id) {
			continue
		}
		unlocks[id] = binary.LittleEndian.Uint32(unlockRaw[offset+4 : offset+8])
	}
	if len(unlocks) == 0 {
		return []LoadoutMasteryNode{}, unlockRaw, nil
	}

	result := make([]LoadoutMasteryNode, 0, min(loadoutMaxMastery, int(count)))
	seen := make(map[uint32]bool)
	fingerprint := append([]byte(nil), unlockRaw...)
	for index := uintptr(0); index < count; index++ {
		key := binary.LittleEndian.Uint32(keyBytes[index*8 : index*8+4])
		if runtimePatchPartyEmptyHash(key) {
			continue
		}
		node, findErr := findRuntimePatchPartyMapNode(memory, manager, runtimePatchPartySkillboardNodeMap, key)
		if findErr != nil {
			return nil, nil, fmt.Errorf("skillboard node %08X: %w", key, findErr)
		}
		row, readErr := readRuntimePatchPointer(memory, node+0x18)
		if readErr != nil || !plausibleRuntimePatchPartyPointer(row) {
			return nil, nil, fmt.Errorf("skillboard node %08X row pointer is invalid", key)
		}
		rowRaw := make([]byte, 0x78)
		if readErr := memory.ReadAt(row, rowRaw); readErr != nil {
			return nil, nil, fmt.Errorf("skillboard node %08X row: %w", key, readErr)
		}
		effectID := binary.LittleEndian.Uint32(rowRaw[0x74:0x78])
		bit := binary.LittleEndian.Uint32(rowRaw[0x5C:0x60])
		if bit > 31 {
			return nil, nil, fmt.Errorf("skillboard node %08X unlock bit %d is invalid", key, bit)
		}
		nodeID := binary.LittleEndian.Uint32(rowRaw[0x48:0x4C])
		bits, exists := unlocks[nodeID]
		if !exists || ((bits>>bit)&1) == 0 {
			continue
		}
		if isMasterySpecializationHash(key) {
			continue
		}
		definition, known := skillboardNodeForHash(key)
		if !known && effectID < runtimePatchPartyMinimumMasteryHash {
			continue
		}
		if !known || definition.Char != ownerCode {
			return nil, nil, fmt.Errorf("unlocked skillboard key %08X (effect %08X) is unknown or belongs to %s instead of %s", key, effectID, definition.Char, ownerCode)
		}
		rank, _, rankKnown := masteryRankOfGrp(definition.Grp)
		if !rankKnown {
			return nil, nil, fmt.Errorf("unlocked skillboard key %08X has no mastery rank", key)
		}
		if rank == "R1" && strings.TrimSpace(definition.Name) != "" {
			continue
		}
		resolved, known := loadoutMasteryNodeForHash(key)
		if !known {
			return nil, nil, fmt.Errorf("unlocked skillboard key %08X has no mastery rank mapping", key)
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, resolved)
			fingerprint = binary.LittleEndian.AppendUint32(fingerprint, key)
			if len(result) >= runtimePatchPartyMaximumMasteryNodes {
				return nil, nil, fmt.Errorf("unlocked skillboard effects exceed %d", runtimePatchPartyMaximumMasteryNodes)
			}
		}
	}
	return result, fingerprint, nil
}

func findRuntimePatchPartyMapNode(memory runtimePatchPartyMemory, manager uintptr, layout runtimePatchPartyMapLayout, key uint32) (uintptr, error) {
	mask, err := readRuntimePatchU32At(memory, manager+layout.maskOffset)
	if err != nil || mask > 0xFFFF || mask&(mask+1) != 0 {
		return 0, fmt.Errorf("map mask %08X is invalid", mask)
	}
	buckets, err := readRuntimePatchPointer(memory, manager+layout.bucketsOffset)
	if err != nil || !plausibleRuntimePatchPartyPointer(buckets) {
		return 0, fmt.Errorf("map bucket pointer is invalid")
	}
	endNode, err := readRuntimePatchPointer(memory, manager+layout.endOffset)
	if err != nil || !plausibleRuntimePatchPartyPointer(endNode) {
		return 0, fmt.Errorf("map end pointer is invalid")
	}
	bucketIndex := uintptr(mask & key)
	if bucketIndex > ^uintptr(0)/0x10 {
		return 0, fmt.Errorf("map bucket offset overflow")
	}
	bucket, ok := checkedRuntimePatchMonitorAddress(buckets, bucketIndex*0x10)
	if !ok {
		return 0, fmt.Errorf("map bucket address overflow")
	}
	head, err := readRuntimePatchPointer(memory, bucket)
	if err != nil {
		return 0, fmt.Errorf("map bucket head is unreadable")
	}
	node, err := readRuntimePatchPointer(memory, bucket+8)
	if err != nil {
		return 0, fmt.Errorf("map bucket tail is unreadable")
	}
	visited := make(map[uintptr]bool)
	for step := 0; step < 64; step++ {
		if node == 0 || node == endNode || !plausibleRuntimePatchPartyPointer(node) {
			return 0, fmt.Errorf("key %08X was not found", key)
		}
		if visited[node] {
			return 0, fmt.Errorf("map chain contains a cycle")
		}
		visited[node] = true
		current, readErr := readRuntimePatchU32At(memory, node+0x10)
		if readErr != nil {
			return 0, fmt.Errorf("map node key is unreadable")
		}
		if current == key {
			return node, nil
		}
		if node == head {
			return 0, fmt.Errorf("key %08X was not found", key)
		}
		node, err = readRuntimePatchPointer(memory, node+8)
		if err != nil {
			return 0, fmt.Errorf("map node link is unreadable")
		}
	}
	return 0, fmt.Errorf("map chain exceeds 64 nodes")
}

func decodeRuntimePatchPartyLoadout(statsRaw, weaponRaw, sigilRaw, overmasteryRaw []byte, layout string) (RuntimePatchPartyLoadout, error) {
	if len(statsRaw) != runtimePatchPartyStatsSize || len(weaponRaw) != runtimePatchPartyWeaponSize || len(sigilRaw) != runtimePatchPartySigilListSize || len(overmasteryRaw) != runtimePatchPartyOvermasterySize {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("runtime loadout block sizes are invalid")
	}
	if _, err := loadProgressionCatalog(); err != nil {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("weapon catalog: %w", err)
	}
	catalog, err := LoadCatalog()
	if err != nil {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("factor catalog: %w", err)
	}

	stats := RuntimePatchPartyPanelStats{
		Level: binary.LittleEndian.Uint32(statsRaw[0x00:0x04]), TotalHP: binary.LittleEndian.Uint32(statsRaw[0x04:0x08]),
		TotalAttack: binary.LittleEndian.Uint32(statsRaw[0x08:0x0C]), StunPower: math.Float32frombits(binary.LittleEndian.Uint32(statsRaw[0x10:0x14])),
		CriticalRate: math.Float32frombits(binary.LittleEndian.Uint32(statsRaw[0x14:0x18])), TotalPower: binary.LittleEndian.Uint32(statsRaw[0x18:0x1C]),
	}
	if stats.Level == 0 || stats.Level > 999 || stats.TotalHP == 0 || stats.TotalHP > 1_000_000_000 || stats.TotalAttack > 1_000_000_000 || stats.TotalPower == 0 || stats.TotalPower > 100_000_000 || !finiteRuntimePatchFloat(stats.StunPower) || stats.StunPower < 0 || stats.StunPower > 10_000_000 || !finiteRuntimePatchFloat(stats.CriticalRate) || stats.CriticalRate < 0 || stats.CriticalRate > 100_000 {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("player stats failed plausibility checks")
	}

	weaponHash := binary.LittleEndian.Uint32(weaponRaw[0x04:0x08])
	weaponDef, knownWeapon := progressionWeaponDefForHash(weaponHash)
	if !knownWeapon || weaponHash == 0 || weaponHash == EmptyHash {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("weapon hash %08X is not in the 2.0.2 catalog", weaponHash)
	}
	weapon := RuntimePatchPartyWeapon{
		Hash: weaponHash, HashHex: fmt.Sprintf("%08X", weaponHash), Name: progressionWeaponName(weaponDef),
		XP:        binary.LittleEndian.Uint32(weaponRaw[0x10:0x14]),
		StarLevel: binary.LittleEndian.Uint32(weaponRaw[0x14:0x18]), PlusMarks: binary.LittleEndian.Uint32(weaponRaw[0x18:0x1C]),
		AwakeningLevel: binary.LittleEndian.Uint32(weaponRaw[0x1C:0x20]), WrightstoneID: binary.LittleEndian.Uint32(weaponRaw[0x38:0x3C]),
		Level: binary.LittleEndian.Uint32(weaponRaw[0x58:0x5C]), HP: binary.LittleEndian.Uint32(weaponRaw[0x5C:0x60]),
		Attack: binary.LittleEndian.Uint32(weaponRaw[0x60:0x64]), Traits: make([]RuntimePatchPartyTrait, 0, 3), Skills: make([]RuntimePatchPartyTrait, 0, 5),
	}
	for index := 0; index < 5; index++ {
		hash := binary.LittleEndian.Uint32(weaponRaw[0x44+index*4 : 0x48+index*4])
		if runtimePatchPartyEmptyHash(hash) {
			break
		}
		name, ok := runtimePatchPartyWeaponSkillName(catalog, hash)
		if !ok {
			return RuntimePatchPartyLoadout{}, fmt.Errorf("weapon skill %d hash %08X is unknown", index+1, hash)
		}
		level := uint32(0)
		for pair := 0; pair < 5; pair++ {
			offset := 0xA4 + pair*8
			if binary.LittleEndian.Uint32(weaponRaw[offset:offset+4]) == hash {
				level = binary.LittleEndian.Uint32(weaponRaw[offset+4 : offset+8])
				break
			}
		}
		if level > 999 {
			return RuntimePatchPartyLoadout{}, fmt.Errorf("weapon skill %d level is invalid", index+1)
		}
		weapon.Skills = append(weapon.Skills, RuntimePatchPartyTrait{Hash: hash, HashHex: fmt.Sprintf("%08X", hash), Name: name, Level: level})
	}
	if weapon.Level == 0 || weapon.Level > 999 || weapon.StarLevel > 20 || weapon.PlusMarks > 9999 || weapon.AwakeningLevel > 100 || weapon.HP > 100_000_000 || weapon.Attack > 100_000_000 {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("weapon values failed plausibility checks")
	}
	for index := 0; index < 3; index++ {
		offset := 0x20 + index*8
		hash := binary.LittleEndian.Uint32(weaponRaw[offset : offset+4])
		level := binary.LittleEndian.Uint32(weaponRaw[offset+4 : offset+8])
		if runtimePatchPartyEmptyHash(hash) {
			if level != 0 {
				return RuntimePatchPartyLoadout{}, fmt.Errorf("weapon trait %d is empty with a non-zero level", index+1)
			}
			continue
		}
		name, ok := runtimePatchPartyTraitName(catalog, hash)
		if !ok || level > 999 {
			return RuntimePatchPartyLoadout{}, fmt.Errorf("weapon trait %d failed catalog or level validation", index+1)
		}
		weapon.Traits = append(weapon.Traits, RuntimePatchPartyTrait{Hash: hash, HashHex: fmt.Sprintf("%08X", hash), Name: name, Level: level})
	}

	sigils := make([]RuntimePatchPartySigil, 0, runtimePatchPartySigilCount)
	for index := 0; index < runtimePatchPartySigilCount; index++ {
		offset := index * runtimePatchPartySigilEntrySize
		primaryHash := binary.LittleEndian.Uint32(sigilRaw[offset : offset+4])
		primaryLevel := binary.LittleEndian.Uint32(sigilRaw[offset+4 : offset+8])
		secondaryHash := binary.LittleEndian.Uint32(sigilRaw[offset+8 : offset+12])
		secondaryLevel := binary.LittleEndian.Uint32(sigilRaw[offset+12 : offset+16])
		sigilHash := binary.LittleEndian.Uint32(sigilRaw[offset+16 : offset+20])
		sigilLevel := binary.LittleEndian.Uint32(sigilRaw[offset+24 : offset+28])
		if runtimePatchPartyEmptyHash(sigilHash) {
			continue
		}
		sigilDef := catalog.LookupSigilByHash(sigilHash)
		runtimeSigilName := strings.TrimSpace(localizedRuntimeName(sigilHash))
		if (sigilDef == nil && runtimeSigilName == "") || runtimePatchPartyEmptyHash(primaryHash) || sigilLevel > 999 || primaryLevel > 999 || secondaryLevel > 999 {
			return RuntimePatchPartyLoadout{}, fmt.Errorf(
				"factor slot %d failed catalog or level validation: factor=%08X level=%d primary=%08X/%d secondary=%08X/%d catalog=%t",
				index+1, sigilHash, sigilLevel, primaryHash, primaryLevel, secondaryHash, secondaryLevel, sigilDef != nil,
			)
		}
		primaryName, primaryKnown := runtimePatchPartyTraitName(catalog, primaryHash)
		if !primaryKnown {
			return RuntimePatchPartyLoadout{}, fmt.Errorf("factor slot %d primary trait %08X is unknown", index+1, primaryHash)
		}
		name := sigilDisplayName(sigilHash)
		if strings.TrimSpace(name) == "" && sigilDef != nil {
			name = displaySigilName(sigilDef)
		}
		sigil := RuntimePatchPartySigil{
			Index: index, Hash: sigilHash, HashHex: fmt.Sprintf("%08X", sigilHash), Name: name, Level: sigilLevel,
			PrimaryTraitHash: primaryHash, PrimaryTraitHashHex: fmt.Sprintf("%08X", primaryHash), PrimaryTraitName: primaryName, PrimaryTraitLevel: primaryLevel,
		}
		if !runtimePatchPartyEmptyHash(secondaryHash) {
			secondaryName, secondaryKnown := runtimePatchPartyTraitName(catalog, secondaryHash)
			if !secondaryKnown {
				return RuntimePatchPartyLoadout{}, fmt.Errorf("factor slot %d secondary trait %08X is unknown", index+1, secondaryHash)
			}
			sigil.SecondaryTraitHash = secondaryHash
			sigil.SecondaryTraitHashHex = fmt.Sprintf("%08X", secondaryHash)
			sigil.SecondaryTraitName = secondaryName
			sigil.SecondaryTraitLevel = secondaryLevel
		}
		sigils = append(sigils, sigil)
	}

	partyIndex := binary.LittleEndian.Uint32(sigilRaw[0x22C:0x230])
	if partyIndex > 3 && partyIndex != 0xFF {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("party index %d is invalid", partyIndex)
	}
	online := binary.LittleEndian.Uint32(sigilRaw[0x1C8:0x1CC])
	if online > 1 {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("online flag %d is invalid", online)
	}
	characterCode := weaponDef.OwnerCode
	runtimeLabel := runtimePatchPartyASCII(sigilRaw[0x1E8:0x1F8])
	if runtimePatchPartyCharacterCode(runtimeLabel) && !strings.EqualFold(runtimeLabel, characterCode) {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("runtime character %s does not match weapon owner %s", runtimeLabel, characterCode)
	}
	characterName := characterCode
	if names, ok := runtimePatchPartyCharacterNames[characterCode]; ok {
		characterName = names[0]
		if !useChinese() {
			characterName = names[1]
		}
	}
	characterHash := ""
	if hash, ok := runtimeOwnerCharacterHash[characterCode]; ok {
		characterHash = fmt.Sprintf("%08X", hash)
	}
	overLimit := make([]RuntimePatchPartyOverLimit, 0, 4)
	for index := 0; index < 4; index++ {
		offset := index * 0x10
		hash := binary.LittleEndian.Uint32(overmasteryRaw[offset : offset+4])
		flags := binary.LittleEndian.Uint32(overmasteryRaw[offset+4 : offset+8])
		value := math.Float32frombits(binary.LittleEndian.Uint32(overmasteryRaw[offset+0x0C : offset+0x10]))
		entry := RuntimePatchPartyOverLimit{Index: index, AttributeHash: hash, Flags: flags, Value: value}
		if runtimePatchPartyEmptyHash(hash) {
			if flags != 0 || value != 0 {
				return RuntimePatchPartyLoadout{}, fmt.Errorf("overmastery slot %d is empty with non-zero fields", index+1)
			}
			overLimit = append(overLimit, entry)
			continue
		}
		definition, known := overLimitCatalog[hash]
		if !known || flags == 0 || flags > 0x200 || flags&(flags-1) != 0 || !finiteRuntimePatchFloat(value) || math.Abs(float64(value)) > 100_000_000 {
			return RuntimePatchPartyLoadout{}, fmt.Errorf("overmastery slot %d failed catalog or value validation", index+1)
		}
		level := uint32(1)
		for bits := flags; bits > 1; bits >>= 1 {
			level++
		}
		entry.HashHex = fmt.Sprintf("%08X", hash)
		entry.Name = definition.name
		entry.Level = level
		overLimit = append(overLimit, entry)
	}
	return RuntimePatchPartyLoadout{
		Available: true, Verification: "candidate", Evidence: runtimePatchMonitorText("2.0.2 受限候选布局", "Bounded candidate layout for 2.0.2"),
		Layout: layout, CharacterCode: characterCode, CharacterHash: characterHash, CharacterName: characterName, RuntimeLabel: runtimeLabel,
		Online: online == 1, PartyIndex: partyIndex, Stats: stats, Weapon: weapon, Sigils: sigils, OverLimit: overLimit,
	}, nil
}

func readRuntimePatchPartyBlock(memory runtimePatchPartyMemory, base, offset uintptr, size int) ([]byte, error) {
	address, ok := checkedRuntimePatchMonitorAddress(base, offset)
	if !ok || size <= 0 {
		return nil, fmt.Errorf("address overflow")
	}
	block := make([]byte, size)
	if err := memory.ReadAt(address, block); err != nil {
		return nil, err
	}
	return block, nil
}

func plausibleRuntimePatchPartyPointer(value uintptr) bool {
	return value >= 0x10000 && uint64(value) <= 0x00007FFFFFFFFFFF
}

func runtimePatchPartyEmptyHash(value uint32) bool {
	return value == 0 || value == EmptyHash || value == ^uint32(0)
}

func runtimePatchPartyTraitName(catalog *Catalog, hash uint32) (string, bool) {
	if catalog == nil || runtimePatchPartyEmptyHash(hash) {
		return "", false
	}
	if trait := catalog.LookupTraitByHash(hash); trait != nil {
		return loadoutTraitDisplayName(catalog, hash), true
	}
	if name := strings.TrimSpace(localizedRuntimeName(hash)); name != "" {
		return name, true
	}
	return "", false
}

func runtimePatchPartyWeaponSkillName(catalog *Catalog, hash uint32) (string, bool) {
	if name, ok := runtimePatchPartyTraitName(catalog, hash); ok {
		return name, true
	}
	if catalog == nil || runtimePatchPartyEmptyHash(hash) {
		return "", false
	}
	data, err := loadLoadoutWeaponStats()
	if err != nil || data == nil {
		return "", false
	}
	id := data.TraitIDs[hashText(hash)]
	definition := loadTraitValues()[id]
	if definition == nil {
		return "", false
	}
	name := strings.TrimSpace(definition.Name)
	if name == "" {
		return "", false
	}
	return name, true
}

func runtimePatchPartyASCII(value []byte) string {
	if end := strings.IndexByte(string(value), 0); end >= 0 {
		value = value[:end]
	}
	for _, item := range value {
		if item < 0x20 || item > 0x7E {
			return ""
		}
	}
	return strings.TrimSpace(string(value))
}

func runtimePatchPartyCharacterCode(value string) bool {
	if len(value) != 6 || value[0] != 'P' || value[1] != 'L' {
		return false
	}
	for _, digit := range value[2:] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
