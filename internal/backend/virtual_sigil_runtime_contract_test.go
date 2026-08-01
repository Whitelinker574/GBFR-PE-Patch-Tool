package backend

import (
	"bytes"
	"os"
	"testing"
)

func TestVirtualSigil203NativeSitesMatchLockedExecutable(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_203_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_203_TEST to verify the locally supplied game 2.0.3 executable")
	}
	if err := verifyRuntimePatchLocalGameIdentityExact(path, runtimePatchLocalGame203Size, runtimePatchLocalGame203SHA256); err != nil {
		t.Fatal(err)
	}
	sections, err := readRuntimePatchLocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		rva  uint32
		want []byte
	}{
		{name: "trait apply loop", rva: 0x00A1EBE4 - 4, want: []byte{0xFF, 0xC7, 0x83, 0xFF, 0x0D, 0x0F, 0x84, 0xB7, 0x00, 0x00, 0x00, 0xC5, 0xF8, 0x11, 0x75, 0xF0}},
		{name: "trait category loop", rva: 0x00A1F7F6 - 6, want: []byte{0x49, 0xFF, 0xC5, 0x49, 0x83, 0xFD, 0x0D, 0x0F, 0x84, 0xE4, 0x00, 0x00, 0x00}},
		{name: "trait fetch", rva: 0x00A1F80E, want: []byte{0x84, 0xDB, 0x74, 0x3E, 0x49, 0x8B, 0x87, 0x80, 0x5E, 0x00, 0x00}},
		{name: "gem getter", rva: 0x00A25D70, want: []byte{0x55, 0x41, 0x57, 0x41, 0x56, 0x56, 0x57, 0x53, 0x48, 0x83, 0xEC, 0x28}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := runtimePatchLocalBytesAtRVA(sections, test.rva, len(test.want))
			if !bytes.Equal(got, test.want) {
				t.Fatalf("2.0.3 RVA 0x%X bytes=% X, want % X", test.rva, got, test.want)
			}
		})
	}
}

func TestVirtualSigilLiveRecoverySitesAreReadable(t *testing.T) {
	if os.Getenv("GBFR_LIVE_VIRTUAL_RECOVERY_DIAGNOSTIC") != "1" {
		t.Skip("set GBFR_LIVE_VIRTUAL_RECOVERY_DIAGNOSTIC=1 for an explicit read-only live recovery diagnostic")
	}
	process, err := openReadOnlyGameProcessForLayouts(windowsReadOnlyProcessBackend{}, charaProcessName, runtimeCharacterPanelRuntimeLayouts)
	if err != nil {
		t.Fatal(err)
	}
	defer process.handle.Close()
	if process.version != "2.0.3" {
		t.Fatalf("live game version = %s, want 2.0.3", process.version)
	}
	tests := []struct {
		name string
		rva  uintptr
		want []byte
	}{
		{name: "trait apply limit", rva: 0x00A1EBE4, want: []byte{0x0D}},
		{name: "trait category limit", rva: 0x00A1F7F6, want: []byte{0x0D}},
		{name: "trait fetch", rva: 0x00A1F80E, want: []byte{0x84, 0xDB, 0x74, 0x3E, 0x49, 0x8B, 0x87, 0x80, 0x5E, 0x00, 0x00}},
		{name: "gem getter", rva: 0x00A25D70, want: []byte{0x55, 0x41, 0x57, 0x41, 0x56, 0x56, 0x57, 0x53, 0x48, 0x83, 0xEC, 0x28}},
	}
	for _, test := range tests {
		got := make([]byte, len(test.want))
		if err := process.handle.ReadAt(process.moduleBase+test.rva, got); err != nil {
			t.Errorf("%s read: %v", test.name, err)
			continue
		}
		if !bytes.Equal(got, test.want) {
			t.Errorf("%s bytes=% X, want restored % X", test.name, got, test.want)
		}
	}
}
