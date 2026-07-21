package backend

import (
	"encoding/binary"
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
