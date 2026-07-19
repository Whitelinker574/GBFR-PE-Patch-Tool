package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

func wrightstoneMemoryTraitHash(t *testing.T, catalog *WrightstoneCatalog, id string) uint32 {
	t.Helper()
	trait, err := catalog.RequireTrait(id)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := ParseHashHex(trait.Hash)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func validWrightstoneMemoryUpdate(t *testing.T) (*WrightstoneCatalog, WrightstoneMemoryUpdate) {
	t.Helper()
	catalog, err := LoadWrightstoneCatalog()
	if err != nil {
		t.Fatal(err)
	}
	return catalog, WrightstoneMemoryUpdate{
		FirstHash:   wrightstoneMemoryTraitHash(t, catalog, "SKILL_000_00"),
		FirstLevel:  20,
		SecondHash:  wrightstoneMemoryTraitHash(t, catalog, "SKILL_001_00"),
		SecondLevel: 15,
		ThirdHash:   wrightstoneMemoryTraitHash(t, catalog, "SKILL_003_00"),
		ThirdLevel:  10,
	}
}

func TestValidateWrightstoneMemoryUpdateAcceptsKnownNaturalSlotCaps(t *testing.T) {
	catalog, update := validWrightstoneMemoryUpdate(t)
	if err := validateWrightstoneMemoryUpdate(catalog, update); err != nil {
		t.Fatalf("valid three-trait update rejected: %v", err)
	}
}

func TestValidateWrightstoneMemoryUpdateRejectsUnknownAndDuplicateTraits(t *testing.T) {
	catalog, update := validWrightstoneMemoryUpdate(t)
	update.SecondHash = 0xDEADBEEF
	if err := validateWrightstoneMemoryUpdate(catalog, update); err == nil {
		t.Fatal("unknown trait must be rejected")
	}

	_, update = validWrightstoneMemoryUpdate(t)
	update.SecondHash = update.FirstHash
	if err := validateWrightstoneMemoryUpdate(catalog, update); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("duplicate traits must be rejected, got %v", err)
	}
}

func TestValidateWrightstoneMemoryUpdateRequiresFirstAndCouplesEmptyLevels(t *testing.T) {
	catalog, update := validWrightstoneMemoryUpdate(t)
	update.FirstHash, update.FirstLevel = EmptyHash, 0
	if err := validateWrightstoneMemoryUpdate(catalog, update); err == nil {
		t.Fatal("first trait must not be empty")
	}

	_, update = validWrightstoneMemoryUpdate(t)
	update.SecondHash, update.SecondLevel = EmptyHash, 1
	if err := validateWrightstoneMemoryUpdate(catalog, update); err == nil {
		t.Fatal("empty second trait with non-zero level must be rejected")
	}

	update.SecondHash, update.SecondLevel = EmptyHash, 0
	update.ThirdHash, update.ThirdLevel = 0, 0
	if err := validateWrightstoneMemoryUpdate(catalog, update); err != nil {
		t.Fatalf("empty optional traits should be accepted: %v", err)
	}
}

func TestValidateWrightstoneMemoryUpdateEnforcesPerSlotCaps(t *testing.T) {
	catalog, update := validWrightstoneMemoryUpdate(t)
	for label, mutate := range map[string]func(*WrightstoneMemoryUpdate){
		"first":  func(u *WrightstoneMemoryUpdate) { u.FirstLevel = 21 },
		"second": func(u *WrightstoneMemoryUpdate) { u.SecondLevel = 16 },
		"third":  func(u *WrightstoneMemoryUpdate) { u.ThirdLevel = 11 },
	} {
		candidate := update
		mutate(&candidate)
		if err := validateWrightstoneMemoryUpdate(catalog, candidate); err == nil {
			t.Fatalf("%s slot above natural cap must be rejected", label)
		}
	}
}

func TestValidateWrightstoneMemorySelectionRejectsEveryStaleAddress(t *testing.T) {
	const expected = uintptr(0x12345000)
	if _, err := validateWrightstoneMemorySelection(expected, expected, expected); err != nil {
		t.Fatalf("stable selected address rejected: %v", err)
	}
	for name, values := range map[string][3]uintptr{
		"missing expected token": {0, expected, expected},
		"status changed":         {expected, expected + 8, expected + 8},
		"cave changed":           {expected, expected, expected + 8},
		"selection cleared":      {expected, expected, 0},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := validateWrightstoneMemorySelection(values[0], values[1], values[2]); err == nil {
				t.Fatal("stale or missing selected address must be rejected")
			}
		})
	}
}

func TestEncodeWrightstoneMemoryRecordWritesAllFieldsAndNormalizesEmpty(t *testing.T) {
	_, update := validWrightstoneMemoryUpdate(t)
	update.SecondHash, update.SecondLevel = 0, 0
	original := bytes.Repeat([]byte{0xA5}, wrightstoneMemoryRecordSize)

	encoded, err := encodeWrightstoneMemoryRecord(original, update)
	if err != nil {
		t.Fatal(err)
	}
	wants := map[int]uint32{
		0x00: update.FirstHash,
		0x04: update.FirstLevel,
		0x08: EmptyHash,
		0x0C: 0,
		0x10: update.ThirdHash,
		0x14: update.ThirdLevel,
	}
	for offset, want := range wants {
		if got := binary.LittleEndian.Uint32(encoded[offset : offset+4]); got != want {
			t.Fatalf("field +0x%02X = 0x%08X, want 0x%08X", offset, got, want)
		}
	}
}

func TestWriteWrightstoneMemoryRecordAtomicRollsBackEveryFailureStage(t *testing.T) {
	stages := []string{"write", "verify-before-save", "save", "verify-after-save"}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			forced := errors.New("forced " + stage)
			original := bytes.Repeat([]byte{0x31}, wrightstoneMemoryRecordSize)
			desired := bytes.Repeat([]byte{0x42}, wrightstoneMemoryRecordSize)
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

			err := writeWrightstoneMemoryRecordAtomic(original, desired, writer, committer, reader)
			if !errors.Is(err, forced) {
				t.Fatalf("expected injected error, got %v", err)
			}
			if !bytes.Equal(memory, original) {
				t.Fatalf("failed stage left a partial record: % X", memory)
			}
			if stage == "save" && commits != 2 {
				t.Fatalf("save failure must restore and persist the original, commits=%d", commits)
			}
		})
	}
}

func TestWriteWrightstoneMemoryRecordAtomicCommitsVerifiedRecord(t *testing.T) {
	original := bytes.Repeat([]byte{0x61}, wrightstoneMemoryRecordSize)
	desired := bytes.Repeat([]byte{0x72}, wrightstoneMemoryRecordSize)
	memory := append([]byte(nil), original...)
	commits := 0

	err := writeWrightstoneMemoryRecordAtomic(
		original,
		desired,
		func(data []byte) error { copy(memory, data); return nil },
		func() error { commits++; return nil },
		func() ([]byte, error) { return append([]byte(nil), memory...), nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if commits != 1 || !bytes.Equal(memory, desired) {
		t.Fatalf("verified record was not committed exactly once: commits=%d memory=% X", commits, memory)
	}
}

func TestWriteWrightstoneMemoryRecordAtomicDoesNotRaceIndeterminateCommit(t *testing.T) {
	original := bytes.Repeat([]byte{0x31}, wrightstoneMemoryRecordSize)
	desired := bytes.Repeat([]byte{0x42}, wrightstoneMemoryRecordSize)
	memory := append([]byte(nil), original...)
	writes := 0

	err := writeWrightstoneMemoryRecordAtomic(
		original,
		desired,
		func(data []byte) error { writes++; copy(memory, data); return nil },
		func() error { return newRemoteCallIndeterminateError("test timeout") },
		func() ([]byte, error) { return append([]byte(nil), memory...), nil },
	)
	if !isRemoteCallIndeterminate(err) {
		t.Fatalf("error = %v, want indeterminate remote call", err)
	}
	if writes != 1 {
		t.Fatalf("indeterminate commit must not trigger a racing rollback, writes=%d", writes)
	}
	if !bytes.Equal(memory, desired) {
		t.Fatalf("record changed during indeterminate commit: % X", memory)
	}
}

func TestWriteWrightstoneMemoryRecordAtomicMarksUnprovenRollback(t *testing.T) {
	cause := errors.New("forced desired write failure")
	rollbackFailure := errors.New("forced rollback failure")
	original := bytes.Repeat([]byte{0x31}, wrightstoneMemoryRecordSize)
	desired := bytes.Repeat([]byte{0x42}, wrightstoneMemoryRecordSize)
	writes := 0

	err := writeWrightstoneMemoryRecordAtomic(
		original,
		desired,
		func([]byte) error {
			writes++
			if writes == 1 {
				return cause
			}
			return rollbackFailure
		},
		func() error { return nil },
		func() ([]byte, error) { return append([]byte(nil), original...), nil },
	)
	if !errors.Is(err, cause) || !errors.Is(err, errSummonMemoryRollbackUnproven) {
		t.Fatalf("unproven wrightstone rollback must preserve the cause and quarantine marker: %v", err)
	}
}

func TestValidateWrightstoneMemorySnapshotAfterBackup(t *testing.T) {
	original := bytes.Repeat([]byte{0x21}, wrightstoneMemoryRecordSize)
	if err := validateWrightstoneMemorySnapshot(0x1000, 0x1000, 0x1000, original, append([]byte(nil), original...)); err != nil {
		t.Fatalf("unchanged target rejected: %v", err)
	}
	if err := validateWrightstoneMemorySnapshot(0x1000, 0x2000, 0x2000, original, original); err == nil {
		t.Fatal("selection switch during backup was accepted")
	}
	changed := append([]byte(nil), original...)
	changed[7] ^= 0xFF
	if err := validateWrightstoneMemorySnapshot(0x1000, 0x1000, 0x1000, original, changed); err == nil {
		t.Fatal("record replacement during backup was accepted")
	}
}
