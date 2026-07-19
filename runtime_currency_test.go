package main

import (
	"encoding/binary"
	"testing"
)

func TestCurrencyCapturePatternMatchesDLC202Shape(t *testing.T) {
	if len(currencyCapturePattern) != len(currencyCaptureMask) || len(currencyCapturePattern) != 27 {
		t.Fatalf("unexpected currency pattern shape: pattern=%d mask=%d", len(currencyCapturePattern), len(currencyCaptureMask))
	}
	buf := append([]byte{0xCC, 0xCC}, currencyCapturePattern...)
	for i, fixed := range currencyCaptureMask {
		if !fixed {
			buf[2+i] = byte(0x40 + i)
		}
	}
	matches := findPatternMatches(buf, 0x140000000, currencyCapturePattern, currencyCaptureMask)
	if len(matches) != 1 || matches[0] != 0x140000002 {
		t.Fatalf("unexpected pattern matches: %v", matches)
	}
}

func TestBuildCurrencyCaptureCave(t *testing.T) {
	const cave = uintptr(0x10000000)
	const returnAddr = uintptr(0x10001000)
	original := []byte{0xBA, 0x78, 0x56, 0x34, 0x12}
	code, err := buildCurrencyCaptureCave(cave, returnAddr, original)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != int(currencyCaveDataOffset)+8 {
		t.Fatalf("unexpected cave size: %d", len(code))
	}
	if code[0] != 0x48 || code[1] != 0xBA || binary.LittleEndian.Uint64(code[2:10]) != uint64(cave+currencyCaveDataOffset) {
		t.Fatalf("unexpected data-address prologue: % X", code[:10])
	}
	if code[10] != 0x48 || code[11] != 0x89 || code[12] != 0x0A {
		t.Fatalf("cave does not capture RCX: % X", code[10:13])
	}
	for i := range original {
		if code[13+i] != original[i] {
			t.Fatalf("original instruction mismatch at %d", i)
		}
	}
	if target := relJumpTarget(cave+18, code[18:23]); target != returnAddr {
		t.Fatalf("return jump target = 0x%X, want 0x%X", target, returnAddr)
	}
	if string(code[currencyCaveMarkerOffset:currencyCaveMarkerOffset+len(currencyCaveMarker)]) != string(currencyCaveMarker[:]) {
		t.Fatalf("currency cave ownership marker is missing: % X", code[currencyCaveMarkerOffset:currencyCaveMarkerOffset+len(currencyCaveMarker)])
	}
}

func TestCurrencyFieldsMatchDLC202CT(t *testing.T) {
	want := map[string]uintptr{"rupies": 0x30, "transmarvel": 0x34, "msp": 0x98, "cp": 0x9C}
	if len(currencyDefs) != len(want) {
		t.Fatalf("currency definitions = %d, want %d", len(currencyDefs), len(want))
	}
	for _, def := range currencyDefs {
		if want[def.ID] != def.Offset {
			t.Fatalf("%s offset = 0x%X, want 0x%X", def.ID, def.Offset, want[def.ID])
		}
	}
}
