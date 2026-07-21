package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

func validCatalogSigilMemoryUpdate(t *testing.T) (*Catalog, SigilMemoryUpdate) {
	t.Helper()
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sigil := catalog.LookupSigilByHash(0x2D7F2E70) // Attack Power V+
	if sigil == nil {
		t.Fatal("test catalog is missing Attack Power V+")
	}
	primary, err := catalog.RequireTrait(sigil.PrimaryTraitID)
	if err != nil {
		t.Fatal(err)
	}
	primaryHash, err := ParseHashHex(primary.Hash)
	if err != nil {
		t.Fatal(err)
	}
	return catalog, SigilMemoryUpdate{
		SigilHash:         0x2D7F2E70,
		SigilLevel:        15,
		PrimaryTraitHash:  primaryHash,
		PrimaryTraitLevel: 15,
	}
}

func TestValidateSigilMemoryUpdatesPreflightsEveryEntry(t *testing.T) {
	catalog, valid := validCatalogSigilMemoryUpdate(t)
	invalid := valid
	invalid.SigilHash = 0xDEADBEEF

	err := validateSigilMemoryUpdates(catalog, []SigilMemoryUpdate{valid, invalid})
	if err == nil || !strings.Contains(err.Error(), "2") || !strings.Contains(err.Error(), "DEADBEEF") {
		t.Fatalf("expected the second unknown sigil to fail full-batch preflight, got %v", err)
	}
}

func TestValidateSigilMemoryUpdateRejectsMismatchedPrimaryTrait(t *testing.T) {
	catalog, update := validCatalogSigilMemoryUpdate(t)
	update.PrimaryTraitHash = 0xF372F096 // HP, not Attack Power V+'s primary trait

	if err := validateSigilMemoryUpdate(catalog, update); err == nil || !strings.Contains(err.Error(), "主词条") {
		t.Fatalf("expected mismatched primary trait rejection, got %v", err)
	}
}

func TestSigilMemoryWriteDefaultsToAdvisoryRulesButKeepsRequiredEncoding(t *testing.T) {
	catalog, update := validCatalogSigilMemoryUpdate(t)
	update.SigilHash = 0xDEADBEEF
	update.PrimaryTraitHash = 0xCAFEBABE
	update.SecondaryTraitHash = update.PrimaryTraitHash
	update.SigilLevel = ^uint32(0)
	update.PrimaryTraitLevel = ^uint32(0)
	update.SecondaryTraitLevel = ^uint32(0)
	if err := validateSigilMemoryWriteRequest(catalog, update); err != nil {
		t.Fatalf("write request was blocked by advisory rules: %v", err)
	}
	update.SigilHash = 0
	if err := validateSigilMemoryWriteRequest(catalog, update); err == nil {
		t.Fatal("write must still reject a missing required encoding hash")
	}
}

func TestValidateSigilMemoryUpdateUsesVerifiedDiscreteLevels(t *testing.T) {
	catalog, update := validCatalogSigilMemoryUpdate(t)
	update.SigilLevel = 14 // Attack Power V+ is verified at item level 15 only.

	if err := validateSigilMemoryUpdate(catalog, update); err == nil || !strings.Contains(err.Error(), "等级") {
		t.Fatalf("expected non-catalog sigil level rejection, got %v", err)
	}
}

func TestValidateSigilMemoryUpdateAcceptsDLCSupplementRowsNowInsideUnifiedCatalog(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	update := SigilMemoryUpdate{
		SigilHash:         0x9300FADB, // Celestial Ventus, observed in live memory
		SigilLevel:        15,
		PrimaryTraitHash:  0x73220725,
		PrimaryTraitLevel: 15,
	}
	if err := validateSigilMemoryUpdate(catalog, update); err != nil {
		t.Fatalf("DLC 2.0.2 runtime catalog supplemental row should be accepted by the unified catalog: %v", err)
	}
}

func TestValidateSigilMemoryUpdateRejectsUnknownTraits(t *testing.T) {
	catalog, update := validCatalogSigilMemoryUpdate(t)
	update.SecondaryTraitHash = 0xDEADBEEF
	update.SecondaryTraitLevel = 15

	if err := validateSigilMemoryUpdate(catalog, update); err == nil || !strings.Contains(err.Error(), "副词条") {
		t.Fatalf("expected unknown secondary trait rejection, got %v", err)
	}
}

func TestValidateSigilMemoryUpdateRejectsDuplicatePrimaryAsSecondary(t *testing.T) {
	catalog, update := validCatalogSigilMemoryUpdate(t)
	update.SecondaryTraitHash = update.PrimaryTraitHash
	update.SecondaryTraitLevel = 15

	if err := validateSigilMemoryUpdate(catalog, update); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("expected duplicate primary/secondary rejection, got %v", err)
	}
}

func TestPreflightSigilMemoryLoadoutReturnsValidatedCount(t *testing.T) {
	catalog, update := validCatalogSigilMemoryUpdate(t)
	result, err := preflightSigilMemoryLoadout(catalog, []SigilMemoryUpdate{update, update})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid || result.Count != 2 {
		t.Fatalf("unexpected preflight result: %#v", result)
	}
}

func TestEncodeSigilMemoryRecordPreservesUnownedBytes(t *testing.T) {
	original := make([]byte, sigilMemoryRecordSize)
	for i := range original {
		original[i] = byte(i + 1)
	}
	update := SigilMemoryUpdate{
		PrimaryTraitHash: 0x11111111, PrimaryTraitLevel: 12,
		SecondaryTraitHash: 0x22222222, SecondaryTraitLevel: 13,
		SigilHash: 0x33333333, SigilLevel: 14,
	}

	encoded, err := encodeSigilMemoryRecord(original, update)
	if err != nil {
		t.Fatal(err)
	}
	checks := map[int]uint32{
		0x00: update.PrimaryTraitHash, 0x04: update.PrimaryTraitLevel,
		0x08: update.SecondaryTraitHash, 0x0C: update.SecondaryTraitLevel,
		0x10: update.SigilHash, 0x18: update.SigilLevel,
	}
	for offset, want := range checks {
		if got := binary.LittleEndian.Uint32(encoded[offset : offset+4]); got != want {
			t.Fatalf("field +0x%02X = 0x%08X, want 0x%08X", offset, got, want)
		}
	}
	if !bytes.Equal(encoded[0x14:0x18], original[0x14:0x18]) {
		t.Fatalf("unowned bytes at +0x14 were changed: got % X want % X", encoded[0x14:0x18], original[0x14:0x18])
	}
}

func TestEncodeSigilMemoryRecordCanonicalizesEmptySecondaryTrait(t *testing.T) {
	for _, hash := range []uint32{0, EmptyHash} {
		original := bytes.Repeat([]byte{0xA5}, sigilMemoryRecordSize)
		update := SigilMemoryUpdate{
			PrimaryTraitHash:    0x11111111,
			PrimaryTraitLevel:   15,
			SecondaryTraitHash:  hash,
			SecondaryTraitLevel: 15,
			SigilHash:           0x22222222,
			SigilLevel:          15,
		}

		encoded, err := encodeSigilMemoryRecord(original, update)
		if err != nil {
			t.Fatal(err)
		}
		if got := binary.LittleEndian.Uint32(encoded[0x08:0x0C]); got != EmptyHash {
			t.Fatalf("empty secondary hash 0x%08X encoded as 0x%08X, want EmptyHash", hash, got)
		}
		if got := binary.LittleEndian.Uint32(encoded[0x0C:0x10]); got != 0 {
			t.Fatalf("empty secondary level encoded as %d, want 0", got)
		}
	}
}

func TestWriteSigilMemoryRecordAtomicRollsBackPartialWrite(t *testing.T) {
	forced := errors.New("forced partial write")
	original := bytes.Repeat([]byte{0x11}, sigilMemoryRecordSize)
	desired := bytes.Repeat([]byte{0x22}, sigilMemoryRecordSize)
	memory := append([]byte(nil), original...)
	writes := 0
	writer := func(data []byte) error {
		writes++
		if writes == 1 {
			copy(memory[:8], data[:8])
			return forced
		}
		copy(memory, data)
		return nil
	}
	reader := func() ([]byte, error) { return append([]byte(nil), memory...), nil }

	err := writeSigilMemoryRecordAtomic(original, desired, writer, func() error { return nil }, reader)
	if !errors.Is(err, forced) {
		t.Fatalf("expected original write error, got %v", err)
	}
	if !bytes.Equal(memory, original) {
		t.Fatalf("partial write was not rolled back\ngot  % X\nwant % X", memory, original)
	}
}

func TestWriteSigilMemoryRecordAtomicRollsBackCommittedFailure(t *testing.T) {
	forced := errors.New("forced commit failure")
	original := bytes.Repeat([]byte{0x31}, sigilMemoryRecordSize)
	desired := bytes.Repeat([]byte{0x42}, sigilMemoryRecordSize)
	memory := append([]byte(nil), original...)
	commitCalls := 0
	writer := func(data []byte) error { copy(memory, data); return nil }
	committer := func() error {
		commitCalls++
		if commitCalls == 1 {
			return forced
		}
		return nil
	}
	reader := func() ([]byte, error) { return append([]byte(nil), memory...), nil }

	err := writeSigilMemoryRecordAtomic(original, desired, writer, committer, reader)
	if !errors.Is(err, forced) {
		t.Fatalf("expected commit error, got %v", err)
	}
	if !bytes.Equal(memory, original) || commitCalls != 2 {
		t.Fatalf("commit failure did not restore and re-commit original record: calls=%d memory=% X", commitCalls, memory)
	}
}

func TestWriteSigilMemoryRecordAtomicVerifiesBeforeCommit(t *testing.T) {
	original := bytes.Repeat([]byte{0x51}, sigilMemoryRecordSize)
	desired := bytes.Repeat([]byte{0x62}, sigilMemoryRecordSize)
	memory := append([]byte(nil), original...)
	readCalls := 0
	commitCalls := 0
	writer := func(data []byte) error { copy(memory, data); return nil }
	reader := func() ([]byte, error) {
		readCalls++
		got := append([]byte(nil), memory...)
		if readCalls == 1 {
			got[3] ^= 0xFF
		}
		return got, nil
	}

	err := writeSigilMemoryRecordAtomic(original, desired, writer, func() error { commitCalls++; return nil }, reader)
	if err == nil || !strings.Contains(err.Error(), "回读") {
		t.Fatalf("expected pre-commit readback mismatch, got %v", err)
	}
	if commitCalls != 0 || !bytes.Equal(memory, original) {
		t.Fatalf("mismatched data was committed or not rolled back: commits=%d memory=% X", commitCalls, memory)
	}
}

func TestWriteSigilMemoryRecordAtomicCommitsVerifiedRecord(t *testing.T) {
	original := bytes.Repeat([]byte{0x71}, sigilMemoryRecordSize)
	desired := bytes.Repeat([]byte{0x82}, sigilMemoryRecordSize)
	memory := append([]byte(nil), original...)
	commitCalls := 0
	writer := func(data []byte) error { copy(memory, data); return nil }
	reader := func() ([]byte, error) { return append([]byte(nil), memory...), nil }

	if err := writeSigilMemoryRecordAtomic(original, desired, writer, func() error { commitCalls++; return nil }, reader); err != nil {
		t.Fatal(err)
	}
	if commitCalls != 1 || !bytes.Equal(memory, desired) {
		t.Fatalf("verified record was not committed exactly once: calls=%d memory=% X", commitCalls, memory)
	}
}

func TestWriteSigilMemoryRecordAtomicDoesNotRaceIndeterminateCommit(t *testing.T) {
	original := bytes.Repeat([]byte{0x51}, sigilMemoryRecordSize)
	desired := bytes.Repeat([]byte{0x62}, sigilMemoryRecordSize)
	memory := append([]byte(nil), original...)
	writes := 0

	err := writeSigilMemoryRecordAtomic(
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

func TestWriteSigilMemoryRecordAtomicMarksUnprovenRollback(t *testing.T) {
	cause := errors.New("forced desired write failure")
	rollbackFailure := errors.New("forced rollback failure")
	original := bytes.Repeat([]byte{0x31}, sigilMemoryRecordSize)
	desired := bytes.Repeat([]byte{0x42}, sigilMemoryRecordSize)
	writes := 0

	err := writeSigilMemoryRecordAtomic(
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
		t.Fatalf("unproven sigil rollback must preserve the cause and quarantine marker: %v", err)
	}
}

func TestValidateSigilMemorySelectionAndSnapshot(t *testing.T) {
	original := bytes.Repeat([]byte{0x41}, sigilMemoryRecordSize)
	if _, err := validateSigilMemorySelection(0, 0x1000, 0x1000); err == nil {
		t.Fatal("missing expected address was accepted")
	}
	if _, err := validateSigilMemorySelection(0x1000, 0x2000, 0x2000); err == nil {
		t.Fatal("selection switch was accepted")
	}
	if err := validateSigilMemorySnapshot(0x1000, 0x1000, 0x1000, original, append([]byte(nil), original...)); err != nil {
		t.Fatalf("unchanged snapshot rejected: %v", err)
	}
	changed := append([]byte(nil), original...)
	changed[5] ^= 0xFF
	if err := validateSigilMemorySnapshot(0x1000, 0x1000, 0x1000, original, changed); err == nil {
		t.Fatal("changed record after backup was accepted")
	}
}
