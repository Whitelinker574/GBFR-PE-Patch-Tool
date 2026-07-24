package backend

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestParseFloatUnitsDecodesIEEE754Value(t *testing.T) {
	data := make([]byte, 72)
	putU16 := func(offset int, value uint16) {
		binary.LittleEndian.PutUint16(data[offset:offset+2], value)
	}
	putU32 := func(offset int, value uint32) {
		binary.LittleEndian.PutUint32(data[offset:offset+4], value)
	}

	// A minimal FlatBuffers vector containing one FloatSaveDataUnit.
	// The value vector stores the binary32 bit pattern for 8.0, not the
	// integer 0x41000000 converted numerically to a float.
	putU32(4, 4)           // parent field -> vector at 8
	putU32(8, 1)           // one table
	putU32(12, 36)         // vector element -> table at 48
	putU16(32, 10)         // child vtable size
	putU16(34, 16)         // child object size
	putU16(36, 4)          // IDType field
	putU16(38, 8)          // UnitID field
	putU16(40, 12)         // ValueData field
	putU32(48, 16)         // child table -> vtable at 32
	putU32(52, 1312)       // base stun save ID
	putU32(56, 10004)      // Io UnitID
	putU32(60, 4)          // ValueData -> vector at 64
	putU32(64, 1)          // one float
	putU32(68, 0x41000000) // IEEE754 8.0

	units := parseFloatUnits(&fbReader{data: data}, 0, 4)
	if len(units) != 1 || len(units[0].ValueData) != 1 {
		t.Fatalf("float unit shape = %+v, want one scalar", units)
	}
	if got := units[0].ValueData[0]; got != 8 {
		t.Fatalf("0x41000000 decoded as %g, want IEEE754 value 8", got)
	}
}

func TestReadVectorAtRejectsTruncatedAndHugeVectors(t *testing.T) {
	data := make([]byte, 20)
	binary.LittleEndian.PutUint32(data[4:8], 4)
	binary.LittleEndian.PutUint32(data[8:12], 3)
	r := &fbReader{data: data}
	if count, start := r.readVectorAt(0, 4, 4); count != 0 || start != 0 {
		t.Fatalf("truncated vector accepted: count=%d start=%d", count, start)
	}

	binary.LittleEndian.PutUint32(data[8:12], math.MaxUint32)
	if count, start := r.readVectorAt(0, 4, 1); count != 0 || start != 0 {
		t.Fatalf("huge vector accepted: count=%d start=%d", count, start)
	}
}

func TestParseSaveDataRejectsOverflowingHeaderSpan(t *testing.T) {
	data := make([]byte, 52)
	binary.LittleEndian.PutUint64(data[20:28], uint64(math.MaxInt64-0x1000))
	binary.LittleEndian.PutUint64(data[36:44], 0x2000)
	if _, err := ParseSaveData(data); err == nil {
		t.Fatal("overflowing binary1 span was accepted")
	}
}

func TestLoadSaveRejectsOverflowingSlotSpan(t *testing.T) {
	data := make([]byte, 52)
	binary.LittleEndian.PutUint64(data[0x1C:0x24], uint64(math.MaxInt64-0x1000))
	binary.LittleEndian.PutUint64(data[0x2C:0x34], 0x2000)
	path := writeTestSave(t, t.TempDir(), 1, string(data))
	if _, err := LoadSave(path); err == nil {
		t.Fatal("overflowing slot span was accepted")
	}
}

func FuzzParseSaveDataNeverPanics(f *testing.F) {
	f.Add([]byte{})
	overflow := make([]byte, 52)
	binary.LittleEndian.PutUint64(overflow[20:28], uint64(math.MaxInt64-0x1000))
	binary.LittleEndian.PutUint64(overflow[36:44], 0x2000)
	f.Add(overflow)
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParseSaveData(data)
	})
}

func FuzzParseSaveDataBinaryNeverPanics(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{4, 0, 0, 0, 4, 0, 4, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseSaveDataBinary(data)
	})
}
