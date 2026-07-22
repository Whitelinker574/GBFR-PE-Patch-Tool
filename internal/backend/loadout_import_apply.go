package backend

import (
	"fmt"
	"strings"
)

type preparedLoadoutImport struct {
	characterUnitID   uint32
	character         *LoadoutShareCharacterProgression
	weaponUnitID      uint32
	weapon            *LoadoutShareWeaponState
	wrightstoneHash   uint32
	wrightstoneTraits [3]struct {
		hash  uint32
		level int
	}
	characterWeaponChanges  []ProgressionWeaponChange
	characterWeaponExpected []progressionWeaponExpected
}

func loadoutCharacterUnitForHash(save *SaveData, charaHash uint32) (uint32, error) {
	var result uint32
	for _, entry := range save.findAllUnitsByType(1301) {
		if entry.ValueCnt != 1 || entry.Uint32() != charaHash {
			continue
		}
		if result != 0 && result != entry.UnitID {
			return 0, fmt.Errorf("角色 %08X 对应多个存档实例", charaHash)
		}
		result = entry.UnitID
	}
	if result == 0 {
		return 0, fmt.Errorf("存档里找不到角色 %08X", charaHash)
	}
	return result, nil
}

func prepareLoadoutImport(save *SaveData, changes []LoadoutWrite, payload *LoadoutImportApplyPayload) (*preparedLoadoutImport, error) {
	if payload == nil {
		return nil, nil
	}
	if len(changes) != 1 || changes[0].Op != "write" {
		return nil, fmt.Errorf("完整配装导入一次只能写入一个目标槽")
	}
	charaHash, err := ParseHashHex(changes[0].ExpectCharaHash)
	if err != nil {
		return nil, fmt.Errorf("导入目标角色无效: %w", err)
	}
	prepared := &preparedLoadoutImport{}
	if payload.Character != nil {
		if payload.Character.MasterTotalMSP < 0 || payload.Character.MasterTotalMSP > 0x7FFFFFFF ||
			payload.Character.LegacyProgress < 0 || uint64(payload.Character.LegacyProgress) > uint64(^uint32(0)) {
			return nil, fmt.Errorf("角色强化进度超出存档字段范围")
		}
		unitID, unitErr := loadoutCharacterUnitForHash(save, charaHash)
		if unitErr != nil {
			return nil, unitErr
		}
		for _, field := range []struct {
			id   uint32
			name string
		}{{1323, "专精总 MSP"}, {1321, "角色强化进度"}} {
			entry, ok := save.findUnitExact(field.id, unitID)
			if !ok || entry.ValueCnt != 1 {
				return nil, fmt.Errorf("目标角色尚未建立%s字段，无法安全补写系统结构", field.name)
			}
		}
		prepared.characterUnitID = unitID
		copyValue := *payload.Character
		copyValue.Weapons = append([]LoadoutShareProgressionWeapon(nil), payload.Character.Weapons...)
		prepared.character = &copyValue
		inventory := progressionInventory(save, "")
		byInternalID := make(map[string]OwnedProgressionWeapon, len(inventory.Weapons))
		for _, weapon := range inventory.Weapons {
			if weapon.InternalID != "" {
				if _, exists := byInternalID[weapon.InternalID]; !exists {
					byInternalID[weapon.InternalID] = weapon
				}
			}
		}
		seenWeapons := make(map[string]bool, len(payload.Character.Weapons))
		missingWeapons := 0
		for _, weapon := range payload.Character.Weapons {
			if weapon.InternalID == "" || seenWeapons[weapon.InternalID] {
				return nil, fmt.Errorf("角色强化武器目录含空值或重复项 %q", weapon.InternalID)
			}
			seenWeapons[weapon.InternalID] = true
			if weapon.Level < 1 || weapon.Level > 150 || weapon.Uncap < 0 || weapon.Uncap > 6 || weapon.Mirage < 0 || weapon.Mirage > 99 ||
				weapon.Awakening < 0 || weapon.Awakening > 10 || weapon.Transcendence < 0 || weapon.Transcendence > 7 {
				return nil, fmt.Errorf("角色强化武器 %s 的等级字段越界", weapon.InternalID)
			}
			baseHash := strings.TrimSpace(weapon.BaseHash)
			if baseHash == "" {
				baseHash = strings.TrimSpace(weapon.Hash)
			}
			hash, hashErr := ParseHashHex(baseHash)
			if hashErr != nil {
				return nil, fmt.Errorf("角色强化武器 %s 哈希无效: %w", weapon.InternalID, hashErr)
			}
			definition, known := progressionWeaponDefForHash(hash)
			if !known || definition.InternalID != weapon.InternalID {
				return nil, fmt.Errorf("角色强化武器 %s 不在当前 2.0.2 目录", weapon.InternalID)
			}
			change := ProgressionWeaponChange{
				Action: "add", Hash: baseHash, Level: weapon.Level, Uncap: weapon.Uncap, Mirage: weapon.Mirage,
				Awakening: weapon.Awakening, Transcendence: weapon.Transcendence, TranscendenceSkill: weapon.TranscendenceSkill,
			}
			if existing, ok := byInternalID[weapon.InternalID]; ok {
				change.Action = "update"
				change.UnitID = existing.UnitID
			} else {
				missingWeapons++
			}
			prepared.characterWeaponChanges = append(prepared.characterWeaponChanges, change)
		}
		if missingWeapons > inventory.EmptyWeapons {
			return nil, fmt.Errorf("复制武器收集强化需要 %d 个空武器槽，目标存档只有 %d 个", missingWeapons, inventory.EmptyWeapons)
		}
	}
	if payload.Weapon != nil {
		if changes[0].WeaponSlotID == 0 {
			return nil, fmt.Errorf("导入包含武器强化数据，但目标配装没有武器")
		}
		unitID, unitErr := exactWeaponUnitForSlot(save, changes[0].WeaponSlotID)
		if unitErr != nil {
			return nil, unitErr
		}
		if payload.Weapon.Uncap < 0 || payload.Weapon.Uncap > 6 || payload.Weapon.Mirage < 0 || payload.Weapon.Mirage > 99 ||
			payload.Weapon.Awakening < 0 || payload.Weapon.Awakening > 10 || payload.Weapon.Transcendence < 0 || payload.Weapon.Transcendence > 7 {
			return nil, fmt.Errorf("武器强化等级超出游戏字段范围")
		}
		_, parseErr := ParseHashHex(payload.Weapon.StoredHash)
		if parseErr != nil {
			return nil, fmt.Errorf("导入武器哈希无效: %w", parseErr)
		}
		currentHash, ok := save.findUnitExact(weaponIDType, unitID)
		if !ok || currentHash.ValueCnt != 1 {
			return nil, fmt.Errorf("目标武器缺少 2803 类型字段")
		}
		for _, field := range []struct {
			id   uint32
			name string
		}{{weaponXPIDType, "经验"}, {weaponUncapIDType, "上限突破"}, {weaponMirageIDType, "幻晶"},
			{weaponAwakeIDType, "觉醒"}, {weaponStoneIDType, "祝福引用"}, {weaponStoneSubType, "祝福类型"},
			{weaponTranscendenceIDType, "超凡"}, {weaponExtraIDType, "超凡技能"}} {
			entry, exists := save.findUnitExact(field.id, unitID)
			if !exists || (field.id == weaponExtraIDType && entry.ValueCnt < 5) || (field.id != weaponExtraIDType && entry.ValueCnt != 1) {
				return nil, fmt.Errorf("目标武器缺少完整的%s字段", field.name)
			}
		}
		if len(payload.Weapon.SkillHashes) != 5 {
			return nil, fmt.Errorf("导入武器必须携带完整的五技能向量")
		}
		for index, value := range payload.Weapon.SkillHashes {
			if _, parseErr := ParseHashHex(value); parseErr != nil {
				return nil, fmt.Errorf("导入武器技能槽 %d 无效: %w", index+1, parseErr)
			}
		}
		prepared.weaponUnitID = unitID
		copyValue := *payload.Weapon
		copyValue.SkillHashes = append([]string(nil), payload.Weapon.SkillHashes...)
		prepared.weapon = &copyValue
		if payload.Weapon.Wrightstone != nil {
			stoneHash, stoneErr := ParseHashHex(payload.Weapon.Wrightstone.Hash)
			if stoneErr != nil {
				return nil, fmt.Errorf("导入武器祝福哈希无效: %w", stoneErr)
			}
			prepared.wrightstoneHash = stoneHash
			seen := map[int]bool{}
			for _, trait := range payload.Weapon.Wrightstone.Traits {
				if trait.Index < 0 || trait.Index >= 3 || seen[trait.Index] || trait.Level < 0 {
					return nil, fmt.Errorf("导入武器祝福词条槽位无效")
				}
				traitHash, traitErr := ParseHashHex(trait.Hash)
				if traitErr != nil {
					return nil, fmt.Errorf("导入武器祝福词条 %d 无效: %w", trait.Index+1, traitErr)
				}
				seen[trait.Index] = true
				prepared.wrightstoneTraits[trait.Index].hash = traitHash
				prepared.wrightstoneTraits[trait.Index].level = trait.Level
			}
			if _, err := save.FindEmptyWrightstoneSlots(1); err != nil {
				return nil, fmt.Errorf("复制武器祝福需要一个空祝福槽: %w", err)
			}
			if _, err := save.GetMaxWrightstoneSlotID(); err != nil {
				return nil, err
			}
		}
	}
	return prepared, nil
}

func materializeLoadoutImportSummons(save *SaveData, changes []LoadoutWrite, payload *LoadoutImportApplyPayload) ([]SummonSaveRecord, error) {
	if payload == nil || len(payload.ConstructedSummons) == 0 {
		return nil, nil
	}
	if len(changes) != 1 || len(changes[0].SummonSlotIDs) != 4 {
		return nil, fmt.Errorf("缺失召唤石只能随一套完整四槽配装生成")
	}
	seen := map[int]bool{}
	for _, draft := range payload.ConstructedSummons {
		if draft.Index < 0 || draft.Index >= 4 || seen[draft.Index] || changes[0].SummonSlotIDs[draft.Index] != 0 {
			return nil, fmt.Errorf("待生成召唤石槽位 %d 无效或已被占用", draft.Index+1)
		}
		if _, err := save.summonRegistrationFlags(draft.State.TypeHash); err != nil {
			return nil, err
		}
		seen[draft.Index] = true
	}
	inventory, err := save.InspectSummonInventory()
	if err != nil {
		return nil, err
	}
	if inventory.Occupied+len(payload.ConstructedSummons) > inventory.Capacity {
		return nil, fmt.Errorf("召唤石空槽不足：需要 %d，剩余 %d", len(payload.ConstructedSummons), inventory.Capacity-inventory.Occupied)
	}
	created := make([]SummonSaveRecord, 0, len(payload.ConstructedSummons))
	for _, draft := range payload.ConstructedSummons {
		record, createErr := save.CreateSummonRecord(draft.State)
		if createErr != nil {
			return nil, fmt.Errorf("生成第 %d 槽召唤石失败: %w", draft.Index+1, createErr)
		}
		changes[0].SummonSlotIDs[draft.Index] = record.SlotID
		created = append(created, record)
	}
	return created, nil
}

func applyPreparedLoadoutImport(save *SaveData, prepared *preparedLoadoutImport) (int, error) {
	if prepared == nil {
		return 0, nil
	}
	verified := 0
	for _, change := range prepared.characterWeaponChanges {
		_, expected, err := applyProgressionWeaponChange(save, change)
		if err != nil {
			return verified, fmt.Errorf("同步角色强化武器失败: %w", err)
		}
		prepared.characterWeaponExpected = append(prepared.characterWeaponExpected, expected)
	}
	if prepared.character != nil {
		if err := save.patchInt(1323, prepared.characterUnitID, prepared.character.MasterTotalMSP); err != nil {
			return verified, err
		}
		if err := save.patchUint(1321, prepared.characterUnitID, uint32(prepared.character.LegacyProgress)); err != nil {
			return verified, err
		}
		verified++
	}
	if prepared.weapon != nil {
		storedHash, _ := ParseHashHex(prepared.weapon.StoredHash)
		if err := save.patchUint(weaponIDType, prepared.weaponUnitID, storedHash); err != nil {
			return verified, err
		}
		if err := save.patchUint(weaponXPIDType, prepared.weaponUnitID, prepared.weapon.XP); err != nil {
			return verified, err
		}
		for _, field := range []struct {
			id    uint32
			value int
		}{
			{weaponUncapIDType, prepared.weapon.Uncap}, {weaponMirageIDType, prepared.weapon.Mirage},
			{weaponAwakeIDType, prepared.weapon.Awakening}, {weaponTranscendenceIDType, prepared.weapon.Transcendence},
		} {
			if err := save.patchInt(field.id, prepared.weaponUnitID, field.value); err != nil {
				return verified, err
			}
		}
		extra, _ := save.findUnitExact(weaponExtraIDType, prepared.weaponUnitID)
		for index, value := range prepared.weapon.SkillHashes {
			hash, _ := ParseHashHex(strings.TrimSpace(value))
			if err := extra.SetUint32At(index, hash); err != nil {
				return verified, err
			}
		}
		traitBase := weaponTraitBase + (int(prepared.weaponUnitID)-weaponSlotBase)*weaponTraitStride
		if prepared.weapon.Wrightstone == nil {
			if err := save.patchUint(weaponStoneIDType, prepared.weaponUnitID, EmptyHash); err != nil {
				return verified, err
			}
			if err := save.patchUint(weaponStoneSubType, prepared.weaponUnitID, EmptyHash); err != nil {
				return verified, err
			}
			for index := 0; index < 3; index++ {
				if err := save.patchUint(TraitHashIDType, uint32(traitBase+index), EmptyHash); err != nil {
					return verified, err
				}
				if err := save.patchInt(TraitLevelIDType, uint32(traitBase+index), 0); err != nil {
					return verified, err
				}
			}
		} else {
			empty, err := save.FindEmptyWrightstoneSlots(1)
			if err != nil {
				return verified, err
			}
			maxSlot, err := save.GetMaxWrightstoneSlotID()
			if err != nil {
				return verified, err
			}
			newSlotID := maxSlot + 1
			if newSlotID <= 0 {
				return verified, fmt.Errorf("祝福 SlotID 溢出")
			}
			args := prepared.wrightstoneTraits
			if err := save.SetMaxWrightstoneSlotID(newSlotID); err != nil {
				return verified, err
			}
			if err := save.PatchWrightstone(empty[0], newSlotID, prepared.wrightstoneHash,
				args[0].hash, args[0].level, args[1].hash, args[1].level, args[2].hash, args[2].level); err != nil {
				return verified, err
			}
			if err := save.patchUint(weaponStoneIDType, prepared.weaponUnitID, uint32(newSlotID)); err != nil {
				return verified, err
			}
			if err := save.patchUint(weaponStoneSubType, prepared.weaponUnitID, prepared.wrightstoneHash); err != nil {
				return verified, err
			}
			for index := 0; index < 3; index++ {
				hash := args[index].hash
				if hash == 0 {
					hash = EmptyHash
				}
				if err := save.patchUint(TraitHashIDType, uint32(traitBase+index), hash); err != nil {
					return verified, err
				}
				if err := save.patchInt(TraitLevelIDType, uint32(traitBase+index), args[index].level); err != nil {
					return verified, err
				}
			}
		}
		verified++
	}
	return verified, nil
}

func verifyPreparedLoadoutImport(save *SaveData, prepared *preparedLoadoutImport) (int, error) {
	if prepared == nil {
		return 0, nil
	}
	verified := 0
	if len(prepared.characterWeaponExpected) > 0 {
		n, err := verifyProgressionChanges(save, nil, nil, prepared.characterWeaponExpected)
		if err != nil {
			return verified, err
		}
		verified += n
	}
	if prepared.character != nil {
		msp, mok := save.findUnitExact(1323, prepared.characterUnitID)
		legacy, lok := save.findUnitExact(1321, prepared.characterUnitID)
		if !mok || !lok || int(msp.Int32()) != prepared.character.MasterTotalMSP || int(legacy.Uint32()) != prepared.character.LegacyProgress {
			return verified, fmt.Errorf("角色强化/专精等级回读不一致")
		}
		verified++
	}
	if prepared.weapon != nil {
		context, err := readLoadoutWeaponContext(save, func() uint32 {
			slot, _ := save.findUnitExact(weaponSlotIDType, prepared.weaponUnitID)
			if slot == nil {
				return 0
			}
			return slot.Uint32()
		}())
		if err != nil {
			return verified, err
		}
		if context.XP != prepared.weapon.XP || context.Uncap != prepared.weapon.Uncap || context.Mirage != prepared.weapon.Mirage ||
			context.Awakening != prepared.weapon.Awakening || context.Transcendence != prepared.weapon.Transcendence {
			return verified, fmt.Errorf("武器强化回读不一致")
		}
		if prepared.weapon.Wrightstone == nil {
			if context.Wrightstone != nil {
				return verified, fmt.Errorf("武器祝福清空回读不一致")
			}
		} else if context.Wrightstone == nil || !strings.EqualFold(context.Wrightstone.Hash, prepared.weapon.Wrightstone.Hash) {
			return verified, fmt.Errorf("武器祝福回读不一致")
		}
		verified++
	}
	return verified, nil
}
