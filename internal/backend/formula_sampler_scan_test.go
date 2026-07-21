package backend

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"testing"
)

type countingFormulaScanMemory struct {
	data   []byte
	reads  int
	cancel context.CancelFunc
}

func (memory *countingFormulaScanMemory) ReadAt(address uintptr, destination []byte) error {
	if int(address)+len(destination) > len(memory.data) {
		return fmt.Errorf("unmapped read")
	}
	copy(destination, memory.data[int(address):int(address)+len(destination)])
	memory.reads++
	if memory.cancel != nil {
		memory.cancel()
		memory.cancel = nil
	}
	return nil
}

func TestFormulaStatusDiffIsBoundedAndReportsI32AndF32Candidates(t *testing.T) {
	a1 := make([]byte, formulaStatusObjectScanSize+4)
	b1 := make([]byte, len(a1))
	a2 := make([]byte, len(a1))
	b2 := make([]byte, len(a1))

	put := func(buffer []byte, offset int, bits uint32) {
		binary.LittleEndian.PutUint32(buffer[offset:offset+4], bits)
	}
	put(a1, 4, 100)
	put(a2, 4, 100)
	put(b1, 4, 125)
	put(b2, 4, 125)
	put(a1, 16, math.Float32bits(1.25))
	put(a2, 16, math.Float32bits(1.25))
	put(b1, 16, math.Float32bits(1.5))
	put(b2, 16, math.Float32bits(1.5))
	// A reversible change just outside the status-object bound must never leak
	// into the candidate set.
	put(a1, formulaStatusObjectScanSize, 200)
	put(a2, formulaStatusObjectScanSize, 200)
	put(b1, formulaStatusObjectScanSize, 250)
	put(b2, formulaStatusObjectScanSize, 250)

	candidates, err := diffFormulaStatusABAB(a1, b1, a2, b2, func(uint64) (bool, error) { return false, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[FormulaScalarKind]bool{}
	for _, candidate := range candidates {
		if candidate.Offset >= formulaStatusObjectScanSize {
			t.Fatalf("candidate escaped 0..0x6000: %+v", candidate)
		}
		if candidate.Offset == 4 || candidate.Offset == 16 {
			seen[candidate.Kind] = true
			if candidate.Offset == 4 && (candidate.ABits != 100 || candidate.BBits != 125) {
				t.Fatalf("candidate bits were transformed: %+v", candidate)
			}
		}
	}
	if !seen[FormulaScalarI32] || !seen[FormulaScalarF32] {
		t.Fatalf("plausible scalar interpretations = %v, want i32 and f32", seen)
	}
}

func TestFormulaStatusDiffNeverExportsPointerWords(t *testing.T) {
	a1 := make([]byte, formulaStatusObjectScanSize)
	b1 := make([]byte, formulaStatusObjectScanSize)
	a2 := make([]byte, formulaStatusObjectScanSize)
	b2 := make([]byte, formulaStatusObjectScanSize)
	const pointerOffset = 0x104 // deliberately 4-byte aligned, not 8-byte aligned
	const aPointer = uint64(0x000001ABCDEF1000)
	const bPointer = uint64(0x000001ABCDEE2000)
	binary.LittleEndian.PutUint64(a1[pointerOffset:], aPointer)
	binary.LittleEndian.PutUint64(a2[pointerOffset:], aPointer)
	binary.LittleEndian.PutUint64(b1[pointerOffset:], bPointer)
	binary.LittleEndian.PutUint64(b2[pointerOffset:], bPointer)
	candidates, err := diffFormulaStatusABAB(a1, b1, a2, b2, func(value uint64) (bool, error) {
		return value == aPointer || value == bPointer, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range candidates {
		if candidate.Offset == pointerOffset || candidate.Offset == pointerOffset+4 {
			t.Fatalf("pointer word leaked as a scalar candidate: %+v", candidate)
		}
	}
}

func TestFormulaStatusDiffKeepsAdjacentScalarsThatAreNotMappedAddresses(t *testing.T) {
	a1 := make([]byte, formulaStatusObjectScanSize)
	b1 := make([]byte, formulaStatusObjectScanSize)
	a2 := make([]byte, formulaStatusObjectScanSize)
	b2 := make([]byte, formulaStatusObjectScanSize)
	for _, item := range []struct {
		buffer []byte
		first  uint32
		second uint32
	}{{a1, 10_000, 2_000}, {a2, 10_000, 2_000}, {b1, 11_000, 2_500}, {b2, 11_000, 2_500}} {
		binary.LittleEndian.PutUint32(item.buffer[0x200:], item.first)
		binary.LittleEndian.PutUint32(item.buffer[0x204:], item.second)
	}
	candidates, err := diffFormulaStatusABAB(a1, b1, a2, b2, func(uint64) (bool, error) { return false, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[uint32]bool{}
	for _, candidate := range candidates {
		if candidate.Kind == FormulaScalarI32 {
			seen[candidate.Offset] = true
		}
	}
	if !seen[0x200] || !seen[0x204] {
		t.Fatalf("adjacent ordinary scalar candidates were removed: %v", seen)
	}
}

func TestFormulaStatusCaptureRedactsOnlyActuallyMappedPointerWords(t *testing.T) {
	snapshot := make([]byte, formulaStatusObjectScanSize)
	const pointerOffset = 0x104
	const pointerValue = uint64(0x000001ABCDEF1000)
	binary.LittleEndian.PutUint64(snapshot[pointerOffset:], pointerValue)
	binary.LittleEndian.PutUint32(snapshot[0x200:], 10_000)
	binary.LittleEndian.PutUint32(snapshot[0x204:], 2_000)
	redacted, _, err := redactFormulaMappedPointerWords(snapshot, func(value uint64) (bool, error) { return value == pointerValue, nil })
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint64(redacted[pointerOffset:]) != 0 {
		t.Fatal("mapped pointer was retained in the stored status snapshot")
	}
	if binary.LittleEndian.Uint32(redacted[0x200:]) != 10_000 || binary.LittleEndian.Uint32(redacted[0x204:]) != 2_000 {
		t.Fatal("ordinary adjacent scalar values were redacted")
	}
}

func TestFormulaAddressRedactionFailsClosedWhenMemoryMapCannotBeQueried(t *testing.T) {
	snapshot := make([]byte, formulaStatusObjectScanSize)
	binary.LittleEndian.PutUint64(snapshot[0x100:], 0x000001ABCDEF1000)
	if _, _, err := redactFormulaMappedPointerWords(snapshot, func(uint64) (bool, error) {
		return false, errors.New("query failed")
	}); err == nil {
		t.Fatal("memory-map query failure retained a possibly absolute address")
	}
}

func TestFormulaPointerMaskUnionExcludesAddressThatBecomesUnmappedInLaterPhase(t *testing.T) {
	const offset = 0x300
	const mappedA = uint64(0x000001ABCDEF1000)
	const danglingB = uint64(0x000001ABCDEE2000)
	aRaw := make([]byte, formulaStatusObjectScanSize)
	bRaw := make([]byte, formulaStatusObjectScanSize)
	binary.LittleEndian.PutUint64(aRaw[offset:], mappedA)
	binary.LittleEndian.PutUint64(bRaw[offset:], danglingB)
	a, mask, err := redactFormulaMappedPointerWords(aRaw, func(value uint64) (bool, error) { return value == mappedA, nil })
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := redactFormulaMappedPointerWords(bRaw, func(uint64) (bool, error) { return false, nil })
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := diffFormulaStatusABAB(a, b, a, b, func(uint64) (bool, error) { return false, nil }, mask)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range candidates {
		if candidate.Offset == offset || candidate.Offset == offset+4 {
			t.Fatalf("unioned pointer mask retained a later dangling pointer half: %+v", candidate)
		}
	}
}

func TestFormulaStatusCaptureRejectsInsufficientBudgetBeforeReading(t *testing.T) {
	memory := &countingFormulaScanMemory{data: make([]byte, formulaStatusObjectScanSize)}
	_, err := captureFormulaStatusObject(context.Background(), memory, 0, FormulaScanBudget{
		MaxBytes: formulaStatusObjectScanSize - 1,
	})
	if err == nil {
		t.Fatal("status capture exceeded its byte budget")
	}
	if memory.reads != 0 {
		t.Fatalf("status capture read %d chunks after rejecting the budget", memory.reads)
	}
}

func TestFormulaStatusCaptureStopsAtTheNextChunkWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	memory := &countingFormulaScanMemory{
		data: make([]byte, formulaStatusObjectScanSize), cancel: cancel,
	}
	_, err := captureFormulaStatusObject(ctx, memory, 0, FormulaScanBudget{MaxBytes: formulaStatusObjectScanSize})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled status capture returned %v", err)
	}
	if memory.reads != 1 {
		t.Fatalf("cancelled capture read %d chunks, want exactly the in-flight chunk", memory.reads)
	}
}
