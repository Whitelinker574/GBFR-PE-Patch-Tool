package main

import (
	"bytes"
	"debug/pe"
	"fmt"
	"os"
	"testing"
)

func readPEImageRVA(path string, rva uintptr, size int) ([]byte, error) {
	image, err := pe.Open(path)
	if err != nil {
		return nil, err
	}
	defer image.Close()
	for _, section := range image.Sections {
		start := uintptr(section.VirtualAddress)
		span := uintptr(section.VirtualSize)
		if raw := uintptr(section.Size); raw > span {
			span = raw
		}
		if rva < start || rva+uintptr(size) > start+span {
			continue
		}
		data, err := section.Data()
		if err != nil {
			return nil, err
		}
		offset := int(rva - start)
		if offset < 0 || offset+size > len(data) {
			return nil, fmt.Errorf("RVA 0x%X exceeds section data", rva)
		}
		return append([]byte(nil), data[offset:offset+size]...), nil
	}
	return nil, fmt.Errorf("RVA 0x%X not mapped by PE sections", rva)
}

func TestLocalGameV202RuntimeSignatures(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_TEST to verify the locally supplied game binary")
	}
	checks := []struct {
		name string
		rva  uintptr
		want []byte
	}{
		{name: "sigil hook", rva: sigilMemoryHookRVA, want: sigilMemoryOriginalBytes},
		{name: "wrightstone hook", rva: wrightstoneMemoryHookRVA, want: wrightstoneMemoryOriginalBytes},
		{name: "save function", rva: wrightstoneMemorySaveRVA, want: gameSaveFunctionPrologue},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			got, err := readPEImageRVA(path, check.rva, len(check.want))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, check.want) {
				t.Fatalf("RVA 0x%X = % X, want % X", check.rva, got, check.want)
			}
		})
	}
}
