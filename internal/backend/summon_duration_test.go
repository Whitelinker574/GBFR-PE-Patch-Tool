package backend

import (
	"bytes"
	"math"
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestSummonDurationRequestBounds(t *testing.T) {
	for _, test := range []struct {
		name    string
		request SummonDurationRequest
		ok      bool
	}{
		{name: "disabled ignores NaN", request: SummonDurationRequest{DurationMultiplier: math.NaN()}, ok: true},
		{name: "minimum", request: SummonDurationRequest{Enabled: true, DurationMultiplier: 0.1}, ok: true},
		{name: "maximum", request: SummonDurationRequest{Enabled: true, DurationMultiplier: 16}, ok: true},
		{name: "infinite still requires finite factor", request: SummonDurationRequest{Enabled: true, Infinite: true, DurationMultiplier: 2}, ok: true},
		{name: "zero", request: SummonDurationRequest{Enabled: true, DurationMultiplier: 0}},
		{name: "above maximum", request: SummonDurationRequest{Enabled: true, DurationMultiplier: math.Nextafter(16, 17)}},
		{name: "infinite NaN", request: SummonDurationRequest{Enabled: true, Infinite: true, DurationMultiplier: math.NaN()}},
		{name: "infinity", request: SummonDurationRequest{Enabled: true, DurationMultiplier: math.Inf(1)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateSummonDurationRequest(test.request)
			if (err == nil) != test.ok {
				t.Fatalf("error=%v, want ok=%v", err, test.ok)
			}
		})
	}
}

func TestSummonDurationCavesRelocateOriginalAndReturn(t *testing.T) {
	const cave = uintptr(0x180000000)
	const entry = uintptr(0x180010000)
	for _, request := range []SummonDurationRequest{
		{Enabled: true, DurationMultiplier: 4},
		{Enabled: true, Infinite: true, DurationMultiplier: 2},
	} {
		name := "multiplier"
		if request.Infinite {
			name = "infinite"
		}
		t.Run(name, func(t *testing.T) {
			code, err := buildSummonDurationCave(cave, entry, request)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateSummonDurationCaveBytes(cave, entry, code, request); err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(code[:8], summonDurationOriginal) {
				t.Fatal("RIP-relative original instruction was copied without relocation")
			}
			corrupt := append([]byte(nil), code...)
			marker := bytes.Index(corrupt, summonDurationMarker)
			corrupt[marker] ^= 0xFF
			if err := validateSummonDurationCaveBytes(cave, entry, corrupt, request); err == nil {
				t.Fatal("corrupt ownership marker was accepted")
			}
		})
	}
}

func TestSummonDurationSingleSiteInstallAndRestore(t *testing.T) {
	site := runtimePatchPatchSiteLease{
		Address: 0x1000, RVA: uint64(summonDurationRVA),
		Original: append([]byte(nil), summonDurationOriginal...),
		Patch:    []byte{0xE9, 1, 2, 3, 4, 0x90, 0x90, 0x90},
	}
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{site.Address: site.Original})
	if err := installRuntimePatchSites(memory, []runtimePatchPatchSiteLease{site}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(memory.data[site.Address], site.Patch) {
		t.Fatal("summon duration entry was not installed")
	}
	if err := restoreRuntimePatchSites(memory, []runtimePatchPatchSiteLease{site}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(memory.data[site.Address], site.Original) {
		t.Fatal("summon duration entry was not restored")
	}
}

func TestCharaReleaseRestoresSummonDurationOwnedByPage(t *testing.T) {
	handle := windows.CurrentProcess()
	page, err := virtualAllocRemote(handle, 0x3000, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = virtualFreeRemote(handle, page) })
	created, err := processCreationTime(handle)
	if err != nil {
		t.Fatal(err)
	}
	entry, cave := page+0x100, page+0x1000
	request := SummonDurationRequest{Enabled: true, DurationMultiplier: 2}
	code, err := buildSummonDurationCave(cave, entry, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeCodeMemory(handle, cave, code); err != nil {
		t.Fatal(err)
	}
	patch, err := makeRelJump(entry, cave, len(summonDurationOriginal))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeCodeMemory(handle, entry, patch); err != nil {
		t.Fatal(err)
	}
	app := &App{
		hProcess: handle, moduleBase: page, charaPID: uint32(os.Getpid()), charaCreated: created,
		charaOwnerToken: "summon-owner", runtimePatchVerifiedDigest: game203ExecutableSHA256,
	}
	app.summonDurationLease = &summonDurationLease{
		OwnerToken: "summon-owner", Process: app.currentProcessInstance(), State: runtimePatchPatchEnabled,
		Request: request, CaveAddr: cave,
		Site: runtimePatchPatchSiteLease{Address: entry, RVA: uint64(summonDurationRVA), Original: summonDurationOriginal, Patch: patch},
	}
	if err := app.CharaRelease("summon-owner"); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(summonDurationOriginal))
	if err := readProcessMemory(handle, entry, unsafe.Pointer(&got[0]), uintptr(len(got))); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, summonDurationOriginal) {
		t.Fatalf("entry after release=% X, want % X", got, summonDurationOriginal)
	}
	if app.summonDurationLease != nil || app.hProcess != 0 || app.charaOwnerToken != "" {
		t.Fatalf("release retained state: lease=%+v handle=%v owner=%q", app.summonDurationLease, app.hProcess, app.charaOwnerToken)
	}
}

func TestSummonDurationPatternMatchesLocalGame203(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_203_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_203_TEST to verify local game 2.0.3")
	}
	if err := verifyRuntimePatchLocalGameIdentityExact(path, runtimePatchLocalGame203Size, runtimePatchLocalGame203SHA256); err != nil {
		t.Fatal(err)
	}
	sections, err := readRuntimePatchLocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	pattern, err := parseRuntimePatchPattern(summonDurationAOB)
	if err != nil {
		t.Fatal(err)
	}
	matches := findRuntimePatchLocalPatternMatches(sections, pattern)
	if len(matches) != 1 || uintptr(matches[0].rva) != summonDurationRVA {
		t.Fatalf("matches=%s, want one at RVA 0x%X", formatRuntimePatchLocalMatchLocations(matches), summonDurationRVA)
	}
	got, err := readPEImageRVA(path, summonDurationRVA, len(summonDurationOriginal))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, summonDurationOriginal) {
		t.Fatalf("original=% X, want % X", got, summonDurationOriginal)
	}
}
