package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestEncodeWeaponMemorySkillWindowWritesExactlyFivePhysicalSlots(t *testing.T) {
	original := bytes.Repeat([]byte{0xA5}, weaponMemorySkillWindowSize)
	update := WeaponMemoryUpdate{Slots: []WeaponMemorySkillUpdate{
		{Hash: 0x11111111, Level: 11},
		{Hash: 0, Level: 0},
		{Hash: 0x33333333, Level: 33},
		{Hash: 0x44444444, Level: 44},
		{Hash: 0x55555555, Level: 55},
	}}

	encoded, err := encodeWeaponMemorySkillWindow(original, update)
	if err != nil {
		t.Fatal(err)
	}
	wants := [5][2]uint32{{0x11111111, 11}, {EmptyHash, 0}, {0x33333333, 33}, {0x44444444, 44}, {0x55555555, 55}}
	for index, want := range wants {
		offset := index * 8
		if got := binary.LittleEndian.Uint32(encoded[offset : offset+4]); got != want[0] {
			t.Fatalf("slot %d hash = 0x%08X, want 0x%08X", index+1, got, want[0])
		}
		if got := binary.LittleEndian.Uint32(encoded[offset+4 : offset+8]); got != want[1] {
			t.Fatalf("slot %d level = %d, want %d", index+1, got, want[1])
		}
	}
	if !bytes.Equal(original, bytes.Repeat([]byte{0xA5}, weaponMemorySkillWindowSize)) {
		t.Fatal("encoder mutated the captured original window")
	}
}

func TestValidateWeaponMemoryUpdateRequiresExactlyFiveCoupledSlots(t *testing.T) {
	valid := WeaponMemoryUpdate{Slots: []WeaponMemorySkillUpdate{
		{Hash: 1, Level: 1}, {Hash: 2, Level: 2}, {Hash: 3, Level: 3}, {Hash: 4, Level: 4}, {Hash: 5, Level: 5},
	}}
	if err := validateWeaponMemoryUpdate(valid); err != nil {
		t.Fatalf("five physical weapon slots rejected: %v", err)
	}
	invalidCount := valid
	invalidCount.Slots = invalidCount.Slots[:4]
	if err := validateWeaponMemoryUpdate(invalidCount); err == nil {
		t.Fatal("update with fewer than five physical slots was accepted")
	}
	invalidEmpty := valid
	invalidEmpty.Slots = append([]WeaponMemorySkillUpdate(nil), valid.Slots...)
	invalidEmpty.Slots[1] = WeaponMemorySkillUpdate{Hash: EmptyHash, Level: 1}
	if err := validateWeaponMemoryUpdate(invalidEmpty); err == nil {
		t.Fatal("empty weapon skill with non-zero level was accepted")
	}
}

func TestWriteWeaponMemorySkillWindowAtomicRollsBackEveryDeterminateFailure(t *testing.T) {
	for _, stage := range []string{"write", "verify-before-save", "save", "verify-after-save"} {
		t.Run(stage, func(t *testing.T) {
			forced := errors.New("forced " + stage)
			original := bytes.Repeat([]byte{0x31}, weaponMemorySkillWindowSize)
			desired := bytes.Repeat([]byte{0x42}, weaponMemorySkillWindowSize)
			memory := append([]byte(nil), original...)
			writes, reads, commits := 0, 0, 0
			writer := func(data []byte) error {
				writes++
				if stage == "write" && writes == 1 {
					copy(memory[:8], data[:8])
					return forced
				}
				copy(memory, data)
				return nil
			}
			reader := func() ([]byte, error) {
				reads++
				if stage == "verify-before-save" && reads == 1 {
					return nil, forced
				}
				if stage == "verify-after-save" && reads == 2 {
					return nil, forced
				}
				return append([]byte(nil), memory...), nil
			}
			committer := func() error {
				commits++
				if stage == "save" && commits == 1 {
					return forced
				}
				return nil
			}

			err := writeWeaponMemorySkillWindowAtomic(original, desired, writer, committer, reader)
			if !errors.Is(err, forced) {
				t.Fatalf("expected injected error, got %v", err)
			}
			if !bytes.Equal(memory, original) {
				t.Fatalf("failed stage left a partial weapon window: % X", memory)
			}
			if stage == "save" && commits != 2 {
				t.Fatalf("save failure must restore and persist original weapon window, commits=%d", commits)
			}
		})
	}
}

func TestWriteWeaponMemorySkillWindowAtomicDoesNotRaceIndeterminateSave(t *testing.T) {
	original := bytes.Repeat([]byte{0x31}, weaponMemorySkillWindowSize)
	desired := bytes.Repeat([]byte{0x42}, weaponMemorySkillWindowSize)
	memory := append([]byte(nil), original...)
	writes := 0
	err := writeWeaponMemorySkillWindowAtomic(
		original, desired,
		func(data []byte) error { writes++; copy(memory, data); return nil },
		func() error { return newRemoteCallIndeterminateError("weapon save timeout") },
		func() ([]byte, error) { return append([]byte(nil), memory...), nil },
	)
	if !isRemoteCallIndeterminate(err) {
		t.Fatalf("error = %v, want indeterminate remote call", err)
	}
	if writes != 1 || !bytes.Equal(memory, desired) {
		t.Fatalf("indeterminate save raced a rollback: writes=%d memory=% X", writes, memory)
	}
}
