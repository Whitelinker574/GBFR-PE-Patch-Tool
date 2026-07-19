package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func validSummonMemoryUpdate(t *testing.T) (*summonStatCatalog, SummonUpdate) {
	t.Helper()
	catalog, err := loadSummonStatCatalog()
	if err != nil {
		t.Fatal(err)
	}
	return catalog, SummonUpdate{
		Index:          34,
		TypeHash:       0xF2BE819E,
		MainTraitHash:  0x318D12E9,
		SubParamHash:   0xA66241C9,
		MainTraitLevel: 15,
		SubParamLevel:  9,
		Rank:           2,
	}
}

func TestValidateSummonMemoryUpdateAcceptsAuditedNaturalValues(t *testing.T) {
	catalog, update := validSummonMemoryUpdate(t)
	if err := validateSummonMemoryUpdate(catalog, update); err != nil {
		t.Fatalf("audited summon update rejected: %v", err)
	}
}

func TestValidateSummonMemoryUpdateRejectsUnknownCatalogHashes(t *testing.T) {
	catalog, valid := validSummonMemoryUpdate(t)
	for name, mutate := range map[string]func(*SummonUpdate){
		"type":       func(v *SummonUpdate) { v.TypeHash = 0xDEADBEEF },
		"main trait": func(v *SummonUpdate) { v.MainTraitHash = 0xDEADBEEF },
		"sub param":  func(v *SummonUpdate) { v.SubParamHash = 0xDEADBEEF },
	} {
		t.Run(name, func(t *testing.T) {
			update := valid
			mutate(&update)
			if err := validateSummonMemoryUpdate(catalog, update); err == nil || !strings.Contains(err.Error(), "DEADBEEF") {
				t.Fatalf("unknown %s must fail closed, got %v", name, err)
			}
		})
	}
}

func TestValidateSummonMemoryUpdateEnforcesNaturalAndSafetyCaps(t *testing.T) {
	catalog, valid := validSummonMemoryUpdate(t)
	mainMax := catalog.main[valid.MainTraitHash].MaxLevel
	for name, mutate := range map[string]func(*SummonUpdate){
		"main natural cap": func(v *SummonUpdate) { v.MainTraitLevel = uint32(mainMax + 1) },
		"sub natural cap":  func(v *SummonUpdate) { v.SubParamLevel = 10 },
		"rank safety cap":  func(v *SummonUpdate) { v.Rank = 4 },
	} {
		t.Run(name, func(t *testing.T) {
			update := valid
			mutate(&update)
			if err := validateSummonMemoryUpdate(catalog, update); err == nil {
				t.Fatalf("%s must be rejected", name)
			}
		})
	}

	unsafeCatalog := &summonStatCatalog{
		types: map[uint32]SummonOption{valid.TypeHash: {Hash: valid.TypeHash}},
		main: map[uint32]SummonOption{valid.MainTraitHash: {
			Hash: valid.MainTraitHash, MaxLevel: int(summonMainTraitSafetyMaxLevel) + 100,
		}},
		sub: map[uint32]SummonOption{valid.SubParamHash: {
			Hash: valid.SubParamHash, MaxLevel: int(summonSubParamSafetyMaxLevel) + 100,
			Values: make([]float64, summonSubParamSafetyMaxLevel+101),
		}},
	}
	tooHigh := valid
	tooHigh.MainTraitLevel = summonMainTraitSafetyMaxLevel + 1
	if err := validateSummonMemoryUpdate(unsafeCatalog, tooHigh); err == nil {
		t.Fatal("malformed catalog must not raise the main-trait safety cap")
	}
	tooHigh = valid
	tooHigh.SubParamLevel = summonSubParamSafetyMaxLevel + 1
	if err := validateSummonMemoryUpdate(unsafeCatalog, tooHigh); err == nil {
		t.Fatal("malformed catalog must not raise the sub-param safety cap")
	}
}

func TestValidateSummonMemoryUpdateRejectsMalformedCatalogRanges(t *testing.T) {
	catalog, valid := validSummonMemoryUpdate(t)
	broken := *catalog
	broken.sub = make(map[uint32]SummonOption, len(catalog.sub))
	for hash, option := range catalog.sub {
		broken.sub[hash] = option
	}
	option := broken.sub[valid.SubParamHash]
	option.Values = option.Values[:3]
	broken.sub[valid.SubParamHash] = option
	if err := validateSummonMemoryUpdate(&broken, valid); err == nil {
		t.Fatal("catalog max level beyond its value table must fail closed")
	}
}

func TestEncodeSummonMemoryRecordWritesOneRecordAndPreservesSlot(t *testing.T) {
	_, update := validSummonMemoryUpdate(t)
	original := bytes.Repeat([]byte{0xA5}, summonRecordSize)
	binary.LittleEndian.PutUint32(original[0x04:0x08], 0x12345678)

	desired, err := encodeSummonMemoryRecord(original, update)
	if err != nil {
		t.Fatal(err)
	}
	wants := map[int]uint32{
		0x00: update.TypeHash,
		0x04: 0x12345678,
		0x08: update.MainTraitHash,
		0x0C: update.SubParamHash,
		0x10: update.MainTraitLevel,
		0x14: update.SubParamLevel,
		0x18: update.Rank,
	}
	for offset, want := range wants {
		if got := binary.LittleEndian.Uint32(desired[offset : offset+4]); got != want {
			t.Fatalf("field +0x%02X = 0x%08X, want 0x%08X", offset, got, want)
		}
	}
	if bytes.Equal(original, desired) {
		t.Fatal("encoded summon record did not change")
	}
}

func TestValidateSummonMemorySnapshotRejectsStaleRootTypeAndRecord(t *testing.T) {
	const inventory = uintptr(0x12345000)
	const typeHash = uint32(0xF2BE819E)
	original := bytes.Repeat([]byte{0x41}, summonRecordSize)
	binary.LittleEndian.PutUint32(original[0:4], typeHash)
	if err := validateSummonMemorySnapshot(inventory, inventory, typeHash, original, append([]byte(nil), original...)); err != nil {
		t.Fatalf("stable target rejected: %v", err)
	}
	if err := validateSummonMemorySnapshot(inventory, inventory+0x1000, typeHash, original, original); err == nil {
		t.Fatal("rebuilt inventory root was accepted")
	}
	changedType := append([]byte(nil), original...)
	binary.LittleEndian.PutUint32(changedType[0:4], typeHash+1)
	if err := validateSummonMemorySnapshot(inventory, inventory, typeHash, original, changedType); err == nil {
		t.Fatal("replaced summon type at the target index was accepted")
	}
	changedRecord := append([]byte(nil), original...)
	changedRecord[0x04] ^= 0xFF
	if err := validateSummonMemorySnapshot(inventory, inventory, typeHash, original, changedRecord); err == nil {
		t.Fatal("changed full 0x1C target record was accepted")
	}
}

func TestWriteSummonMemoryRecordAtomicRollsBackEveryDeterminateFailureStage(t *testing.T) {
	for _, stage := range []string{"write", "verify-before-save", "save", "verify-after-save"} {
		t.Run(stage, func(t *testing.T) {
			forced := errors.New("forced " + stage)
			original := bytes.Repeat([]byte{0x31}, summonRecordSize)
			desired := bytes.Repeat([]byte{0x42}, summonRecordSize)
			memory := append([]byte(nil), original...)
			writes, reads, saves := 0, 0, 0
			writer := func(record []byte) error {
				writes++
				if stage == "write" && writes == 1 {
					copy(memory[:8], record[:8])
					return forced
				}
				copy(memory, record)
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
			saver := func() error {
				saves++
				if stage == "save" && saves == 1 {
					return forced
				}
				return nil
			}

			err := writeSummonMemoryRecordAtomic(original, desired, writer, saver, reader)
			if !errors.Is(err, forced) {
				t.Fatalf("expected injected error, got %v", err)
			}
			if !bytes.Equal(memory, original) {
				t.Fatalf("failed stage left a partial record: % X", memory)
			}
			if (stage == "save" || stage == "verify-after-save") && saves != 2 {
				t.Fatalf("post-save failure must restore and persist original, saves=%d", saves)
			}
		})
	}
}

func TestWriteSummonMemoryRecordAtomicReturnsRollbackReadbackFailure(t *testing.T) {
	forced := errors.New("forced save failure")
	rollbackRead := errors.New("forced rollback readback failure")
	original := bytes.Repeat([]byte{0x51}, summonRecordSize)
	desired := bytes.Repeat([]byte{0x62}, summonRecordSize)
	memory := append([]byte(nil), original...)
	reads, saves := 0, 0
	err := writeSummonMemoryRecordAtomic(
		original,
		desired,
		func(record []byte) error { copy(memory, record); return nil },
		func() error {
			saves++
			if saves == 1 {
				return forced
			}
			return nil
		},
		func() ([]byte, error) {
			reads++
			if reads == 2 {
				return nil, rollbackRead
			}
			return append([]byte(nil), memory...), nil
		},
	)
	if !errors.Is(err, forced) || !errors.Is(err, rollbackRead) {
		t.Fatalf("transaction must report cause and unproven rollback: %v", err)
	}
	if !errors.Is(err, errSummonMemoryRollbackUnproven) {
		t.Fatalf("unproven rollback must carry the fail-closed marker: %v", err)
	}
}

func TestWriteSummonMemoryRecordAtomicDoesNotRaceIndeterminateSave(t *testing.T) {
	original := bytes.Repeat([]byte{0x71}, summonRecordSize)
	desired := bytes.Repeat([]byte{0x82}, summonRecordSize)
	memory := append([]byte(nil), original...)
	writes := 0
	err := writeSummonMemoryRecordAtomic(
		original,
		desired,
		func(record []byte) error { writes++; copy(memory, record); return nil },
		func() error { return newRemoteCallIndeterminateError("test timeout") },
		func() ([]byte, error) { return append([]byte(nil), memory...), nil },
	)
	if !isRemoteCallIndeterminate(err) {
		t.Fatalf("error = %v, want indeterminate remote save", err)
	}
	if writes != 1 || !bytes.Equal(memory, desired) {
		t.Fatalf("indeterminate save raced a rollback: writes=%d memory=% X", writes, memory)
	}
}

func TestWriteSummonMemoryRecordAtomicCommitsVerifiedRecord(t *testing.T) {
	original := bytes.Repeat([]byte{0x91}, summonRecordSize)
	desired := bytes.Repeat([]byte{0xA2}, summonRecordSize)
	memory := append([]byte(nil), original...)
	saves := 0
	err := writeSummonMemoryRecordAtomic(
		original,
		desired,
		func(record []byte) error { copy(memory, record); return nil },
		func() error { saves++; return nil },
		func() ([]byte, error) { return append([]byte(nil), memory...), nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if saves != 1 || !bytes.Equal(memory, desired) {
		t.Fatalf("verified record was not saved exactly once: saves=%d memory=% X", saves, memory)
	}
}

func TestSummonUpdatePinsProcessAndSerializesLiveWrites(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "summon_memory.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body *ast.BlockStmt
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "summonUpdate" {
			body = fn.Body
			break
		}
	}
	if body == nil {
		t.Fatal("missing SummonUpdate implementation summonUpdate")
	}
	if !blockCallsSelector(body, "a", "acquireGameProcessLease") {
		t.Fatal("SummonUpdate must pin PID, process handle, and module base")
	}
	if !blockCallsSelector(body, "a", "acquireOwnedRuntimeWriteLease") {
		t.Fatal("SummonUpdateOwned must validate its owner inside the stable process lease")
	}
	if !blockCallsSelector(body, "liveMemoryWriteMu", "Lock") {
		t.Fatal("SummonUpdate must serialize with all live-memory transactions")
	}
}
