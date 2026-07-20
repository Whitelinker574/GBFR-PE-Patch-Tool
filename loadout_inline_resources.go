package main

import (
	"fmt"
	"strings"
)

// LoadoutApplyRequest keeps edits to global owned resource instances in the
// same save transaction as the preset references that selected them.
type LoadoutApplyRequest struct {
	Changes     []LoadoutWrite            `json:"changes"`
	WeaponEdits []LoadoutWeaponInlineEdit `json:"weaponEdits,omitempty"`
	SummonEdits []LoadoutSummonInlineEdit `json:"summonEdits,omitempty"`
}

// LoadoutWeaponInlineEdit deliberately exposes only the audited seventh-stage
// option stored in 2818[4]. The first four 2818 values are never writable here.
type LoadoutWeaponInlineEdit struct {
	SlotID                   uint32 `json:"slotId"`
	ExpectUnitID             uint32 `json:"expectUnitId"`
	ExpectStoredHash         string `json:"expectStoredHash"`
	ExpectTranscendence      int    `json:"expectTranscendence"`
	ExpectTranscendenceSkill string `json:"expectTranscendenceSkill"`
	TranscendenceSkill       string `json:"transcendenceSkill"`
}

// LoadoutSummonInlineEdit changes only 1458/1459/1460. TypeHash is a stale
// snapshot guard for 1457 and cannot be changed through this interface.
type LoadoutSummonInlineEdit struct {
	SlotID               uint32 `json:"slotId"`
	ExpectUnitID         uint32 `json:"expectUnitId"`
	ExpectTypeHash       string `json:"expectTypeHash"`
	ExpectMainTraitHash  string `json:"expectMainTraitHash"`
	ExpectMainTraitLevel int    `json:"expectMainTraitLevel"`
	ExpectSubParamHash   string `json:"expectSubParamHash"`
	ExpectSubParamLevel  int    `json:"expectSubParamLevel"`
	ExpectRank           int    `json:"expectRank"`
	MainTraitHash        string `json:"mainTraitHash"`
	MainTraitLevel       int    `json:"mainTraitLevel"`
	SubParamHash         string `json:"subParamHash"`
	SubParamLevel        int    `json:"subParamLevel"`
	Rank                 int    `json:"rank"`
}

type preparedLoadoutWeaponEdit struct {
	slotID uint32
	unitID uint32
	skill  uint32
}

type preparedLoadoutSummonEdit struct {
	slotID uint32
	unitID uint32
	state  SummonTraitState
}

type preparedLoadoutInlineResources struct {
	weapons []preparedLoadoutWeaponEdit
	summons []preparedLoadoutSummonEdit
}

func selectedWeaponSlotIDs(resolved []*resolvedWrite) map[uint32]bool {
	selected := make(map[uint32]bool, len(resolved))
	for _, write := range resolved {
		if write != nil && write.op != "clear" && write.weaponSID != 0 {
			selected[write.weaponSID] = true
		}
	}
	return selected
}

func selectedSummonSlotIDSet(slotIDs []uint32) map[uint32]bool {
	selected := make(map[uint32]bool, len(slotIDs))
	for _, slotID := range slotIDs {
		selected[slotID] = true
	}
	return selected
}

func exactWeaponUnitForSlot(save *SaveData, slotID uint32) (uint32, error) {
	var unitID uint32
	for _, entry := range save.findAllUnitsByType(weaponSlotIDType) {
		if entry.Uint32() != slotID {
			continue
		}
		if unitID != 0 && unitID != entry.UnitID {
			return 0, fmt.Errorf("weapon SlotID %d maps to multiple owned instances", slotID)
		}
		unitID = entry.UnitID
	}
	if unitID == 0 {
		return 0, fmt.Errorf("weapon SlotID %d does not exist", slotID)
	}
	return unitID, nil
}

func exactSummonStateForSlot(save *SaveData, slotID uint32) (uint32, SummonTraitState, error) {
	var unitID uint32
	for _, entry := range save.findAllUnitsByType(1456) {
		if entry.ValueCnt != 1 || entry.Uint32() != slotID {
			continue
		}
		if unitID != 0 && unitID != entry.UnitID {
			return 0, SummonTraitState{}, fmt.Errorf("summon SlotID %d maps to multiple owned instances", slotID)
		}
		unitID = entry.UnitID
	}
	if unitID == 0 {
		return 0, SummonTraitState{}, fmt.Errorf("summon SlotID %d does not exist", slotID)
	}
	typeEntry, typeOK := save.findUnitExact(1457, unitID)
	traits, traitsOK := save.findUnitExact(1458, unitID)
	levels, levelsOK := save.findUnitExact(1459, unitID)
	rank, rankOK := save.findUnitExact(1460, unitID)
	if !typeOK || typeEntry.ValueCnt != 1 || !traitsOK || traits.ValueCnt != 2 ||
		!levelsOK || levels.ValueCnt != 2 || !rankOK || rank.ValueCnt != 1 {
		return 0, SummonTraitState{}, fmt.Errorf("summon SlotID %d has an incomplete 1457..1460 record", slotID)
	}
	mainHash, _ := traits.Uint32At(0)
	subHash, _ := traits.Uint32At(1)
	mainLevel, _ := levels.Uint32At(0)
	subLevel, _ := levels.Uint32At(1)
	return unitID, SummonTraitState{
		TypeHash: typeEntry.Uint32(), MainTraitHash: mainHash, SubParamHash: subHash,
		MainTraitLevel: mainLevel, SubParamLevel: subLevel, Rank: rank.Uint32(),
	}, nil
}

func parseInlineHash(label, text string) (uint32, error) {
	hash, err := ParseHashHex(strings.TrimSpace(text))
	if err != nil {
		return 0, fmt.Errorf("%s is invalid: %w", label, err)
	}
	return hash, nil
}

func nonNegativeInlineValue(label string, value int) (uint32, error) {
	if value < 0 {
		return 0, fmt.Errorf("%s cannot be negative", label)
	}
	if uint64(value) > uint64(^uint32(0)) {
		return 0, fmt.Errorf("%s is too large", label)
	}
	return uint32(value), nil
}

func prepareLoadoutWeaponEdits(save *SaveData, edits []LoadoutWeaponInlineEdit, selected map[uint32]bool) ([]preparedLoadoutWeaponEdit, error) {
	deduplicated := make(map[uint32]LoadoutWeaponInlineEdit, len(edits))
	order := make([]uint32, 0, len(edits))
	for _, edit := range edits {
		if previous, exists := deduplicated[edit.SlotID]; exists {
			if previous != edit {
				return nil, fmt.Errorf("conflicting weapon edits for SlotID %d", edit.SlotID)
			}
			continue
		}
		deduplicated[edit.SlotID] = edit
		order = append(order, edit.SlotID)
	}
	prepared := make([]preparedLoadoutWeaponEdit, 0, len(order))
	for _, slotID := range order {
		edit := deduplicated[slotID]
		if !selected[slotID] {
			return nil, fmt.Errorf("weapon SlotID %d is not selected by this loadout transaction", slotID)
		}
		unitID, err := exactWeaponUnitForSlot(save, slotID)
		if err != nil {
			return nil, err
		}
		hash, hashOK := save.findUnitExact(weaponIDType, unitID)
		transcendence, transOK := save.findUnitExact(weaponTranscendenceIDType, unitID)
		extra, extraOK := save.findUnitExact(weaponExtraIDType, unitID)
		if !hashOK || hash.ValueCnt != 1 || !transOK || transcendence.ValueCnt != 1 || !extraOK || extra.ValueCnt < 5 {
			return nil, fmt.Errorf("weapon SlotID %d lacks the audited 2803/2817/2818 fields", slotID)
		}
		expectHash, err := parseInlineHash("expected weapon hash", edit.ExpectStoredHash)
		if err != nil {
			return nil, err
		}
		expectSkill, err := parseWeaponTranscendenceSkill(edit.ExpectTranscendenceSkill)
		if err != nil {
			return nil, fmt.Errorf("expected weapon transcendence skill is invalid: %w", err)
		}
		currentSkill, readErr := extra.Uint32At(4)
		if readErr != nil || unitID != edit.ExpectUnitID || hash.Uint32() != expectHash ||
			int(transcendence.Int32()) != edit.ExpectTranscendence || currentSkill != expectSkill {
			return nil, fmt.Errorf("stale weapon snapshot for SlotID %d", slotID)
		}
		if transcendence.Int32() < 7 {
			return nil, fmt.Errorf("weapon SlotID %d has not reached transcendence stage 7", slotID)
		}
		skill, err := parseWeaponTranscendenceSkill(edit.TranscendenceSkill)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, preparedLoadoutWeaponEdit{slotID: slotID, unitID: unitID, skill: skill})
	}
	return prepared, nil
}

func prepareLoadoutSummonEdits(save *SaveData, catalog *summonStatCatalog, edits []LoadoutSummonInlineEdit, selected map[uint32]bool) ([]preparedLoadoutSummonEdit, error) {
	deduplicated := make(map[uint32]LoadoutSummonInlineEdit, len(edits))
	order := make([]uint32, 0, len(edits))
	for _, edit := range edits {
		if previous, exists := deduplicated[edit.SlotID]; exists {
			if previous != edit {
				return nil, fmt.Errorf("conflicting summon edits for SlotID %d", edit.SlotID)
			}
			continue
		}
		deduplicated[edit.SlotID] = edit
		order = append(order, edit.SlotID)
	}
	prepared := make([]preparedLoadoutSummonEdit, 0, len(order))
	for _, slotID := range order {
		edit := deduplicated[slotID]
		if !selected[slotID] {
			return nil, fmt.Errorf("summon SlotID %d is not selected by this loadout transaction", slotID)
		}
		unitID, existing, err := exactSummonStateForSlot(save, slotID)
		if err != nil {
			return nil, err
		}
		expectType, err := parseInlineHash("expected summon type", edit.ExpectTypeHash)
		if err != nil {
			return nil, err
		}
		expectMain, err := parseInlineHash("expected summon main trait", edit.ExpectMainTraitHash)
		if err != nil {
			return nil, err
		}
		expectSub, err := parseInlineHash("expected summon sub parameter", edit.ExpectSubParamHash)
		if err != nil {
			return nil, err
		}
		expectMainLevel, err := nonNegativeInlineValue("expected summon main level", edit.ExpectMainTraitLevel)
		if err != nil {
			return nil, err
		}
		expectSubLevel, err := nonNegativeInlineValue("expected summon sub level", edit.ExpectSubParamLevel)
		if err != nil {
			return nil, err
		}
		expectRank, err := nonNegativeInlineValue("expected summon rank", edit.ExpectRank)
		if err != nil {
			return nil, err
		}
		if unitID != edit.ExpectUnitID || existing != (SummonTraitState{
			TypeHash: expectType, MainTraitHash: expectMain, SubParamHash: expectSub,
			MainTraitLevel: expectMainLevel, SubParamLevel: expectSubLevel, Rank: expectRank,
		}) {
			return nil, fmt.Errorf("stale summon snapshot for SlotID %d", slotID)
		}
		mainHash, err := parseInlineHash("summon main trait", edit.MainTraitHash)
		if err != nil {
			return nil, err
		}
		subHash, err := parseInlineHash("summon sub parameter", edit.SubParamHash)
		if err != nil {
			return nil, err
		}
		mainLevel, err := nonNegativeInlineValue("summon main level", edit.MainTraitLevel)
		if err != nil {
			return nil, err
		}
		subLevel, err := nonNegativeInlineValue("summon sub level", edit.SubParamLevel)
		if err != nil {
			return nil, err
		}
		rank, err := nonNegativeInlineValue("summon rank", edit.Rank)
		if err != nil {
			return nil, err
		}
		draft := SummonTraitState{
			TypeHash: existing.TypeHash, MainTraitHash: mainHash, SubParamHash: subHash,
			MainTraitLevel: mainLevel, SubParamLevel: subLevel, Rank: rank,
		}
		if err := validateSummonTraitChange(catalog, draft, existing); err != nil {
			return nil, fmt.Errorf("summon SlotID %d: %w", slotID, err)
		}
		prepared = append(prepared, preparedLoadoutSummonEdit{slotID: slotID, unitID: unitID, state: draft})
	}
	return prepared, nil
}

func prepareLoadoutInlineResources(save *SaveData, request LoadoutApplyRequest, resolved []*resolvedWrite, summonSlotIDs []uint32) (*preparedLoadoutInlineResources, error) {
	weapons, err := prepareLoadoutWeaponEdits(save, request.WeaponEdits, selectedWeaponSlotIDs(resolved))
	if err != nil {
		return nil, err
	}
	var summons []preparedLoadoutSummonEdit
	if len(request.SummonEdits) > 0 {
		catalog, err := loadSummonStatCatalog()
		if err != nil {
			return nil, err
		}
		summons, err = prepareLoadoutSummonEdits(save, catalog, request.SummonEdits, selectedSummonSlotIDSet(summonSlotIDs))
		if err != nil {
			return nil, err
		}
	}
	return &preparedLoadoutInlineResources{weapons: weapons, summons: summons}, nil
}

func applyPreparedLoadoutInlineResources(save *SaveData, prepared *preparedLoadoutInlineResources) error {
	if prepared == nil {
		return nil
	}
	for _, edit := range prepared.weapons {
		extra, ok := save.findUnitExact(weaponExtraIDType, edit.unitID)
		if !ok || extra.ValueCnt < 5 {
			return fmt.Errorf("weapon SlotID %d lost its 2818 vector before apply", edit.slotID)
		}
		if err := extra.SetUint32At(4, edit.skill); err != nil {
			return err
		}
	}
	for _, edit := range prepared.summons {
		traits, traitsOK := save.findUnitExact(1458, edit.unitID)
		levels, levelsOK := save.findUnitExact(1459, edit.unitID)
		rank, rankOK := save.findUnitExact(1460, edit.unitID)
		if !traitsOK || traits.ValueCnt != 2 || !levelsOK || levels.ValueCnt != 2 || !rankOK || rank.ValueCnt != 1 {
			return fmt.Errorf("summon SlotID %d lost its 1458/1459/1460 fields before apply", edit.slotID)
		}
		if err := traits.SetUint32At(0, edit.state.MainTraitHash); err != nil {
			return err
		}
		if err := traits.SetUint32At(1, edit.state.SubParamHash); err != nil {
			return err
		}
		if err := levels.SetInt32At(0, int32(edit.state.MainTraitLevel)); err != nil {
			return err
		}
		if err := levels.SetInt32At(1, int32(edit.state.SubParamLevel)); err != nil {
			return err
		}
		rank.SetUint32(edit.state.Rank)
	}
	return nil
}

func verifyPreparedLoadoutInlineResources(save *SaveData, prepared *preparedLoadoutInlineResources) (int, error) {
	if prepared == nil {
		return 0, nil
	}
	verified := 0
	for _, edit := range prepared.weapons {
		extra, ok := save.findUnitExact(weaponExtraIDType, edit.unitID)
		if !ok || extra.ValueCnt < 5 {
			return verified, fmt.Errorf("weapon SlotID %d readback is missing 2818", edit.slotID)
		}
		got, err := extra.Uint32At(4)
		if err != nil || got != edit.skill {
			return verified, fmt.Errorf("weapon SlotID %d 2818[4] readback mismatch", edit.slotID)
		}
		verified++
	}
	for _, edit := range prepared.summons {
		unitID, state, err := exactSummonStateForSlot(save, edit.slotID)
		if err != nil {
			return verified, err
		}
		if unitID != edit.unitID || state != edit.state {
			return verified, fmt.Errorf("summon SlotID %d 1458/1459/1460 readback mismatch", edit.slotID)
		}
		verified++
	}
	return verified, nil
}
