package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
)

const (
	weaponMemorySkillWindowOffset = uintptr(0xA4)
	weaponMemorySkillWindowSize   = 0x28
	weaponMemoryPhysicalSlotCount = 5
)

var weaponMemoryWriteMu sync.Mutex

type WeaponMemorySkillUpdate struct {
	Hash  uint32 `json:"hash"`
	Level uint32 `json:"level"`
}

type WeaponMemoryUpdate struct {
	ExpectedSelectedAddr uint64                    `json:"expectedSelectedAddr"`
	Slots                []WeaponMemorySkillUpdate `json:"slots"`
}

func isEmptyWeaponMemorySkill(hash uint32) bool {
	return hash == 0 || hash == EmptyHash
}

// validateWeaponMemoryUpdate deliberately checks only the physical record
// shape and hash/level coupling. A skill that is absent from the audited
// catalog may still be accepted by the game's own synthesis/runtime logic;
// the UI can warn about that without turning a writable field into a false
// hard prohibition.
func validateWeaponMemoryUpdate(update WeaponMemoryUpdate) error {
	if len(update.Slots) != weaponMemoryPhysicalSlotCount {
		return fmt.Errorf("武器记录固定包含 %d 个技能槽，当前收到 %d 个", weaponMemoryPhysicalSlotCount, len(update.Slots))
	}
	for index, slot := range update.Slots {
		if isEmptyWeaponMemorySkill(slot.Hash) {
			if slot.Level != 0 {
				return fmt.Errorf("武器技能槽 %d 为空时等级必须为 0", index+1)
			}
			continue
		}
		if slot.Level == 0 {
			return fmt.Errorf("武器技能槽 %d 非空时等级不能为 0", index+1)
		}
	}
	return nil
}

func validateWeaponMemorySelection(expected, statusSelected, caveSelected uintptr) (uintptr, error) {
	if expected == 0 {
		return 0, fmt.Errorf("缺少写入前捕获的武器记录地址")
	}
	if statusSelected != expected {
		return 0, fmt.Errorf("武器记录已从 0x%X 切换到 0x%X，请重新确认", expected, statusSelected)
	}
	if caveSelected == 0 || caveSelected != expected {
		return 0, fmt.Errorf("写入前武器记录地址已变化，请重新选择目标武器")
	}
	return expected, nil
}

func validateWeaponMemorySnapshot(expected, statusSelected, caveSelected uintptr, original, current []byte) error {
	if _, err := validateWeaponMemorySelection(expected, statusSelected, caveSelected); err != nil {
		return err
	}
	if len(original) != weaponMemorySkillWindowSize || len(current) != weaponMemorySkillWindowSize {
		return fmt.Errorf("备份后武器技能窗口长度异常")
	}
	if !bytes.Equal(original, current) {
		return fmt.Errorf("自动备份期间目标武器技能已变化，请重新选择后确认")
	}
	return nil
}

func encodeWeaponMemorySkillWindow(original []byte, update WeaponMemoryUpdate) ([]byte, error) {
	if len(original) != weaponMemorySkillWindowSize {
		return nil, fmt.Errorf("武器技能窗口长度 %d，预期 %d", len(original), weaponMemorySkillWindowSize)
	}
	if err := validateWeaponMemoryUpdate(update); err != nil {
		return nil, err
	}
	encoded := append([]byte(nil), original...)
	for index, slot := range update.Slots {
		offset := index * 8
		hash := slot.Hash
		if isEmptyWeaponMemorySkill(hash) {
			hash = EmptyHash
		}
		binary.LittleEndian.PutUint32(encoded[offset:offset+4], hash)
		binary.LittleEndian.PutUint32(encoded[offset+4:offset+8], slot.Level)
	}
	return encoded, nil
}

type weaponMemoryWindowWriter func([]byte) error
type weaponMemoryWindowCommitter func() error
type weaponMemoryWindowReader func() ([]byte, error)

func verifyWeaponMemorySkillWindow(want []byte, reader weaponMemoryWindowReader) error {
	got, err := reader()
	if err != nil {
		return fmt.Errorf("武器技能回读失败: %w", err)
	}
	if len(got) != weaponMemorySkillWindowSize {
		return fmt.Errorf("武器技能回读长度 %d，预期 %d", len(got), weaponMemorySkillWindowSize)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("武器技能回读不一致")
	}
	return nil
}

func rollbackWeaponMemorySkillWindow(original []byte, persist bool, writer weaponMemoryWindowWriter, committer weaponMemoryWindowCommitter, reader weaponMemoryWindowReader) error {
	if err := writer(original); err != nil {
		return fmt.Errorf("恢复原武器技能失败: %w", err)
	}
	if persist {
		if err := committer(); err != nil {
			return fmt.Errorf("重新保存原武器技能失败: %w", err)
		}
	}
	if err := verifyWeaponMemorySkillWindow(original, reader); err != nil {
		return fmt.Errorf("恢复原武器技能后验证失败: %w", err)
	}
	return nil
}

func weaponMemoryTransactionError(cause, rollback error) error {
	if rollback == nil {
		return cause
	}
	return errors.Join(cause, errLiveMemoryRollbackUnproven, fmt.Errorf("武器技能回滚失败: %w", rollback))
}

func writeWeaponMemorySkillWindowAtomic(original, desired []byte, writer weaponMemoryWindowWriter, committer weaponMemoryWindowCommitter, reader weaponMemoryWindowReader) error {
	if len(original) != weaponMemorySkillWindowSize || len(desired) != weaponMemorySkillWindowSize || writer == nil || committer == nil || reader == nil {
		return fmt.Errorf("武器技能原子写入参数无效")
	}
	if err := writer(desired); err != nil {
		return weaponMemoryTransactionError(err, rollbackWeaponMemorySkillWindow(original, false, writer, committer, reader))
	}
	if err := verifyWeaponMemorySkillWindow(desired, reader); err != nil {
		return weaponMemoryTransactionError(err, rollbackWeaponMemorySkillWindow(original, false, writer, committer, reader))
	}
	if err := committer(); err != nil {
		if isRemoteCallIndeterminate(err) {
			return err
		}
		return weaponMemoryTransactionError(err, rollbackWeaponMemorySkillWindow(original, true, writer, committer, reader))
	}
	if err := verifyWeaponMemorySkillWindow(desired, reader); err != nil {
		return weaponMemoryTransactionError(err, rollbackWeaponMemorySkillWindow(original, true, writer, committer, reader))
	}
	return nil
}
