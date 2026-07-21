package backend

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
	want := map[string]struct {
		name   string
		offset uintptr
	}{
		"rupies":      {name: "金币", offset: 0x30},
		"transmarvel": {name: "高级炼成点数", offset: 0x34},
		"msp":         {name: "MSP", offset: 0x98},
		"rp":          {name: "共鸣点数（RP）", offset: 0x9C},
	}
	if len(currencyDefs) != len(want) {
		t.Fatalf("currency definitions = %d, want %d", len(currencyDefs), len(want))
	}
	for _, def := range currencyDefs {
		expected, ok := want[def.ID]
		if !ok {
			t.Fatalf("unexpected published currency id %q", def.ID)
		}
		if def.Offset != expected.offset {
			t.Fatalf("%s offset = 0x%X, want 0x%X", def.ID, def.Offset, expected.offset)
		}
		if def.Name != expected.name {
			t.Fatalf("%s name = %q, want %q", def.ID, def.Name, expected.name)
		}
	}
}

func TestCurrencyInputLookupAcceptsLegacyCPAliasWithoutPublishingIt(t *testing.T) {
	for _, inputID := range []string{"rp", "cp", " cp "} {
		def, ok := lookupCurrencyDef(inputID)
		if !ok {
			t.Fatalf("lookupCurrencyDef(%q) did not resolve", inputID)
		}
		if def.ID != "rp" || def.Name != "共鸣点数（RP）" || def.Offset != 0x9C {
			t.Fatalf("lookupCurrencyDef(%q) = %+v, want canonical RP definition", inputID, def)
		}
	}
	if _, ok := lookupCurrencyDef("CP"); ok {
		t.Fatal("legacy alias must remain the exact backend input id cp")
	}
}
