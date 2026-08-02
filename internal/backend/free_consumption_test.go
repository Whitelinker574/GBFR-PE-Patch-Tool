package backend

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestFreeConsumptionSiteContract(t *testing.T) {
	if len(freeConsumptionSites) != 11 {
		t.Fatalf("site count=%d, want 11", len(freeConsumptionSites))
	}
	seenRVA := make(map[uintptr]struct{}, len(freeConsumptionSites))
	for index, spec := range freeConsumptionSites {
		if spec.Label == "" || spec.RVA == 0 || len(spec.AOB) == 0 || len(spec.Original) == 0 {
			t.Fatalf("site %c is incomplete: %+v", 'A'+rune(index), spec)
		}
		if index == 0 {
			if len(spec.Original) != 5 || len(spec.Patch) != 0 {
				t.Fatalf("site A must reserve a five-byte rel32 cave entry: %+v", spec)
			}
		} else if len(spec.Original) != len(spec.Patch) || bytes.Equal(spec.Original, spec.Patch) {
			t.Fatalf("site %c has invalid direct patch bytes", 'A'+rune(index))
		}
		if _, duplicate := seenRVA[spec.RVA]; duplicate {
			t.Fatalf("duplicate RVA 0x%X", spec.RVA)
		}
		seenRVA[spec.RVA] = struct{}{}
		pattern, err := parseRuntimePatchPattern(spec.AOB)
		if err != nil {
			t.Fatalf("site %c pattern: %v", 'A'+rune(index), err)
		}
		if !matchRuntimePatchPattern(spec.Original, runtimePatchPattern{
			Values: pattern.Values[:len(spec.Original)], Mask: pattern.Mask[:len(spec.Original)],
		}) {
			t.Fatalf("site %c AOB does not begin with its locked original", 'A'+rune(index))
		}
	}
	if len(freeConsumptionCaveMarker) != 8 {
		t.Fatalf("cave marker length=%d, want 8", len(freeConsumptionCaveMarker))
	}
}

func TestFreeConsumptionCaveReplaysSiteAAndReturns(t *testing.T) {
	const cave = uintptr(0x180000000)
	const entry = uintptr(0x180001000)
	code, err := buildFreeConsumptionCave(cave, entry+uintptr(len(freeConsumptionSites[0].Original)))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateFreeConsumptionCaveBytes(cave, entry, code); err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(code, freeConsumptionSites[0].Original); got != 1 {
		t.Fatalf("site A original count=%d, want 1", got)
	}
	corrupt := append([]byte(nil), code...)
	corrupt[freeConsumptionCaveMarkerOffset] ^= 0xFF
	if err := validateFreeConsumptionCaveBytes(cave, entry, corrupt); err == nil {
		t.Fatal("corrupt ownership marker was accepted")
	}
}

func TestFreeConsumptionElevenSitesInstallAndRestoreAtomically(t *testing.T) {
	sites := make([]runtimePatchPatchSiteLease, len(freeConsumptionSites))
	values := make(map[uintptr][]byte, len(sites))
	for index, spec := range freeConsumptionSites {
		patch := append([]byte(nil), spec.Patch...)
		if index == 0 {
			patch = []byte{0xE9, 1, 2, 3, 4}
		}
		sites[index] = runtimePatchPatchSiteLease{
			Address: 0x10000 + uintptr(index)*0x100, RVA: uint64(spec.RVA),
			Original: append([]byte(nil), spec.Original...), Patch: patch,
		}
		values[sites[index].Address] = append([]byte(nil), spec.Original...)
	}
	memory := newRuntimePatchFakeMemory(values)
	if err := installRuntimePatchSites(memory, sites); err != nil {
		t.Fatal(err)
	}
	for index, site := range sites {
		if !bytes.Equal(memory.data[site.Address], site.Patch) {
			t.Fatalf("site %c was not installed", 'A'+rune(index))
		}
	}
	if err := restoreRuntimePatchSites(memory, sites); err != nil {
		t.Fatal(err)
	}
	for index, site := range sites {
		if !bytes.Equal(memory.data[site.Address], site.Original) {
			t.Fatalf("site %c was not restored", 'A'+rune(index))
		}
	}
	for index := range sites {
		wantAddress := sites[len(sites)-1-index].Address
		write := memory.writes[len(sites)+index]
		if write.addr != wantAddress {
			t.Fatalf("restore write[%d]=%#x, want reverse-order %#x", index, write.addr, wantAddress)
		}
	}
}

func TestFreeConsumptionLaterWriteFailureRollsBackEarlierSites(t *testing.T) {
	sites := make([]runtimePatchPatchSiteLease, len(freeConsumptionSites))
	values := make(map[uintptr][]byte, len(sites))
	for index, spec := range freeConsumptionSites {
		patch := append([]byte(nil), spec.Patch...)
		if index == 0 {
			patch = []byte{0xE9, 1, 2, 3, 4}
		}
		sites[index] = runtimePatchPatchSiteLease{
			Address:  0x20000 + uintptr(index)*0x100,
			Original: append([]byte(nil), spec.Original...), Patch: patch,
		}
		values[sites[index].Address] = append([]byte(nil), spec.Original...)
	}
	memory := newRuntimePatchFakeMemory(values)
	memory.writeErrAt[6] = errors.New("injected site F write failure")
	if err := installRuntimePatchSites(memory, sites); err == nil {
		t.Fatal("install error=nil, want rollback")
	}
	for index, site := range sites {
		if !bytes.Equal(memory.data[site.Address], site.Original) {
			t.Fatalf("site %c remained partially patched: % X", 'A'+rune(index), memory.data[site.Address])
		}
	}
}

func TestFreeConsumptionLeaseRejectsWrongOwnerAndProcess(t *testing.T) {
	current := processInstanceID{PID: 10, Created: 20}
	lease := &freeConsumptionLease{OwnerToken: "owner", Process: current}
	if runtimeOwnerTokenMatches(lease.OwnerToken, "wrong") {
		t.Fatal("wrong owner unexpectedly matched")
	}
	if sameProcessInstance(lease.Process, processInstanceID{PID: 10, Created: 21}) {
		t.Fatal("replaced process instance unexpectedly matched")
	}
}

func TestCharaReleaseRestoresFreeConsumptionOwnedByPage(t *testing.T) {
	handle := windows.CurrentProcess()
	page, err := virtualAllocRemote(handle, 0x4000, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = virtualFreeRemote(handle, page) })
	created, err := processCreationTime(handle)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{
		hProcess: handle, moduleBase: page, charaPID: uint32(os.Getpid()), charaCreated: created,
		charaOwnerToken: "free-owner",
	}
	sites := make([]runtimePatchPatchSiteLease, len(freeConsumptionSites))
	for index, spec := range freeConsumptionSites {
		patch := append([]byte(nil), spec.Patch...)
		if index == 0 {
			patch = []byte{0xE9, 1, 2, 3, 4}
		}
		sites[index] = runtimePatchPatchSiteLease{
			Address: page + 0x100 + uintptr(index)*0x40, RVA: uint64(0x100 + index*0x40),
			Original: append([]byte(nil), spec.Original...), Patch: patch,
		}
		if err := writeCodeMemory(handle, sites[index].Address, patch); err != nil {
			t.Fatal(err)
		}
	}
	app.freeConsumptionLease = &freeConsumptionLease{
		OwnerToken: "free-owner", Process: app.currentProcessInstance(), State: runtimePatchPatchEnabled,
		Sites: sites, CaveAddr: page + 0x1000, CaveSize: freeConsumptionCaveSize,
	}
	if err := app.CharaRelease("free-owner"); err != nil {
		t.Fatal(err)
	}
	for index, site := range sites {
		got := make([]byte, len(site.Original))
		if err := readProcessMemory(handle, site.Address, unsafe.Pointer(&got[0]), uintptr(len(got))); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, site.Original) {
			t.Fatalf("site %c after CharaRelease=% X, want % X", 'A'+rune(index), got, site.Original)
		}
	}
	if app.freeConsumptionLease != nil || app.hProcess != 0 || app.charaOwnerToken != "" {
		t.Fatalf("release retained state: lease=%+v handle=%v owner=%q", app.freeConsumptionLease, app.hProcess, app.charaOwnerToken)
	}
}

func TestFreeConsumptionPatternsMatchLocalGame203(t *testing.T) {
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
	for index, spec := range freeConsumptionSites {
		t.Run(string(rune('A'+index)), func(t *testing.T) {
			pattern, err := parseRuntimePatchPattern(spec.AOB)
			if err != nil {
				t.Fatal(err)
			}
			matches := findRuntimePatchLocalPatternMatches(sections, pattern)
			if len(matches) != 1 || uintptr(matches[0].rva) != spec.RVA {
				t.Fatalf("matches=%s, want one at RVA 0x%X", formatRuntimePatchLocalMatchLocations(matches), spec.RVA)
			}
			got, err := readPEImageRVA(path, spec.RVA, len(spec.Original))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, spec.Original) {
				t.Fatalf("original=% X, want % X", got, spec.Original)
			}
		})
	}
}
