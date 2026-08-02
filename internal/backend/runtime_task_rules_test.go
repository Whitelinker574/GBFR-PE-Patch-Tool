package backend

import (
	"bytes"
	"math"
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestTaskScoreMultiplierRequestBounds(t *testing.T) {
	for _, test := range []struct {
		name    string
		request TaskScoreMultiplierRequest
		ok      bool
	}{
		{name: "disabled ignores NaN", request: TaskScoreMultiplierRequest{Multiplier: math.NaN()}, ok: true},
		{name: "minimum", request: TaskScoreMultiplierRequest{Enabled: true, Multiplier: 0.1}, ok: true},
		{name: "maximum", request: TaskScoreMultiplierRequest{Enabled: true, Multiplier: 16}, ok: true},
		{name: "zero", request: TaskScoreMultiplierRequest{Enabled: true, Multiplier: 0}},
		{name: "below minimum", request: TaskScoreMultiplierRequest{Enabled: true, Multiplier: math.Nextafter(0.1, 0)}},
		{name: "above maximum", request: TaskScoreMultiplierRequest{Enabled: true, Multiplier: math.Nextafter(16, 17)}},
		{name: "NaN", request: TaskScoreMultiplierRequest{Enabled: true, Multiplier: math.NaN()}},
		{name: "infinity", request: TaskScoreMultiplierRequest{Enabled: true, Multiplier: math.Inf(1)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateTaskScoreMultiplierRequest(test.request)
			if (err == nil) != test.ok {
				t.Fatalf("error=%v, want ok=%v", err, test.ok)
			}
		})
	}
}

func TestTaskRuleSiteContracts(t *testing.T) {
	if len(taskScoreOriginal) != 7 || len(taskScoreMarker) != 8 {
		t.Fatalf("score contract is incomplete: original=% X marker=%q", taskScoreOriginal, taskScoreMarker)
	}
	scorePattern, err := parseRuntimePatchPattern(taskScoreAOB)
	if err != nil {
		t.Fatal(err)
	}
	if !matchRuntimePatchPattern(taskScoreOriginal, runtimePatchPattern{
		Values: scorePattern.Values[:len(taskScoreOriginal)], Mask: scorePattern.Mask[:len(taskScoreOriginal)],
	}) {
		t.Fatal("task score AOB does not begin with the locked original bytes")
	}
	if len(taskSideQuestSpecs) != 2 {
		t.Fatalf("side quest site count=%d, want 2", len(taskSideQuestSpecs))
	}
	for index, spec := range taskSideQuestSpecs {
		if spec.RVA == 0 || len(spec.Original) < 5 || len(spec.Marker) != 8 {
			t.Fatalf("side quest site %c incomplete: %+v", 'A'+rune(index), spec)
		}
		pattern, err := parseRuntimePatchPattern(spec.AOB)
		if err != nil {
			t.Fatal(err)
		}
		if !matchRuntimePatchPattern(spec.Original, runtimePatchPattern{
			Values: pattern.Values[:len(spec.Original)], Mask: pattern.Mask[:len(spec.Original)],
		}) {
			t.Fatalf("side quest site %c AOB does not begin with locked original bytes", 'A'+rune(index))
		}
	}
}

func TestTaskScoreCaveScalesReplaysAndReturns(t *testing.T) {
	const cave = uintptr(0x180000000)
	const entry = uintptr(0x180001000)
	code, err := buildTaskScoreCave(cave, entry+uintptr(len(taskScoreOriginal)), 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateTaskScoreCaveBytes(cave, entry, code, 4); err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(code, taskScoreOriginal); got != 1 {
		t.Fatalf("original count=%d, want 1", got)
	}
	corrupt := append([]byte(nil), code...)
	marker := bytes.Index(corrupt, taskScoreMarker)
	corrupt[marker+8] ^= 0xFF
	if err := validateTaskScoreCaveBytes(cave, entry, corrupt, 4); err == nil {
		t.Fatal("corrupt task score factor was accepted")
	}
}

func TestTaskSideQuestCavesReplayAndReturn(t *testing.T) {
	for index, spec := range taskSideQuestSpecs {
		t.Run(string(rune('A'+index)), func(t *testing.T) {
			cave := uintptr(0x180000000 + index*0x1000)
			entry := uintptr(0x180010000 + index*0x1000)
			code, err := buildTaskSideQuestCave(index, cave, entry+uintptr(len(spec.Original)))
			if err != nil {
				t.Fatal(err)
			}
			if err := validateTaskSideQuestCaveBytes(index, cave, entry, code); err != nil {
				t.Fatal(err)
			}
			if got := bytes.Count(code, spec.Original); got != 1 {
				t.Fatalf("original count=%d, want 1", got)
			}
		})
	}
}

func TestTaskSideQuestTwoSiteInstallFailureRollsBack(t *testing.T) {
	sites := []runtimePatchPatchSiteLease{
		{Address: 0x1000, Original: append([]byte(nil), taskSideQuestSpecs[0].Original...), Patch: []byte{0xE9, 1, 2, 3, 4, 0x90}},
		{Address: 0x2000, Original: append([]byte(nil), taskSideQuestSpecs[1].Original...), Patch: []byte{0xE9, 5, 6, 7, 8}},
	}
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{
		sites[0].Address: sites[0].Original,
		sites[1].Address: sites[1].Original,
	})
	memory.writeErrAt[2] = errRuntimeHookRollbackUnproven
	if err := installRuntimePatchSites(memory, sites); err == nil {
		t.Fatal("install error=nil, want injected failure")
	}
	for index, site := range sites {
		if !bytes.Equal(memory.data[site.Address], site.Original) {
			t.Fatalf("site %c remained patched after rollback: % X", 'A'+rune(index), memory.data[site.Address])
		}
	}
}

func TestCharaReleaseRestoresTaskRulesOwnedByPage(t *testing.T) {
	handle := windows.CurrentProcess()
	page, err := virtualAllocRemote(handle, 0x5000, windows.PAGE_EXECUTE_READWRITE)
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
		charaOwnerToken: "task-owner", runtimePatchVerifiedDigest: game203ExecutableSHA256,
	}
	scoreEntry, scoreCave := page+0x100, page+0x1000
	scoreCode, err := buildTaskScoreCave(scoreCave, scoreEntry+uintptr(len(taskScoreOriginal)), 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeCodeMemory(handle, scoreCave, scoreCode); err != nil {
		t.Fatal(err)
	}
	scorePatch, err := makeRelJump(scoreEntry, scoreCave, len(taskScoreOriginal))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeCodeMemory(handle, scoreEntry, scorePatch); err != nil {
		t.Fatal(err)
	}
	app.taskScoreMultiplierLease = &taskRuleLease{
		OwnerToken: "task-owner", Process: app.currentProcessInstance(), State: runtimePatchPatchEnabled, Multiplier: 2,
		Sites: []runtimePatchPatchSiteLease{{Address: scoreEntry, Original: taskScoreOriginal, Patch: scorePatch}},
		Caves: []uintptr{scoreCave},
	}

	sideSites := make([]runtimePatchPatchSiteLease, len(taskSideQuestSpecs))
	sideCaves := make([]uintptr, len(taskSideQuestSpecs))
	for index, spec := range taskSideQuestSpecs {
		entry := page + 0x200 + uintptr(index)*0x100
		cave := page + 0x2000 + uintptr(index)*0x800
		code, buildErr := buildTaskSideQuestCave(index, cave, entry+uintptr(len(spec.Original)))
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if err := writeCodeMemory(handle, cave, code); err != nil {
			t.Fatal(err)
		}
		patch, jumpErr := makeRelJump(entry, cave, len(spec.Original))
		if jumpErr != nil {
			t.Fatal(jumpErr)
		}
		if err := writeCodeMemory(handle, entry, patch); err != nil {
			t.Fatal(err)
		}
		sideSites[index] = runtimePatchPatchSiteLease{Address: entry, Original: spec.Original, Patch: patch}
		sideCaves[index] = cave
	}
	app.taskSideQuestAutoCompleteLease = &taskRuleLease{
		OwnerToken: "task-owner", Process: app.currentProcessInstance(), State: runtimePatchPatchEnabled,
		Sites: sideSites, Caves: sideCaves,
	}
	if err := app.CharaRelease("task-owner"); err != nil {
		t.Fatal(err)
	}
	checks := append([]runtimePatchPatchSiteLease{appendedTaskRuleSite(scoreEntry, taskScoreOriginal)}, sideSites...)
	for index, site := range checks {
		got := make([]byte, len(site.Original))
		if err := readProcessMemory(handle, site.Address, unsafe.Pointer(&got[0]), uintptr(len(got))); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, site.Original) {
			t.Fatalf("restored site[%d]=% X, want % X", index, got, site.Original)
		}
	}
	if app.taskScoreMultiplierLease != nil || app.taskSideQuestAutoCompleteLease != nil || app.hProcess != 0 {
		t.Fatalf("release retained task-rule state: score=%+v side=%+v handle=%v", app.taskScoreMultiplierLease, app.taskSideQuestAutoCompleteLease, app.hProcess)
	}
}

func appendedTaskRuleSite(address uintptr, original []byte) runtimePatchPatchSiteLease {
	return runtimePatchPatchSiteLease{Address: address, Original: append([]byte(nil), original...)}
}

func TestTaskRulePatternsMatchLocalGame203(t *testing.T) {
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
	checks := []struct {
		name     string
		aob      string
		rva      uintptr
		original []byte
	}{{name: "score", aob: taskScoreAOB, rva: taskScoreMultiplierRVA, original: taskScoreOriginal}}
	for index, spec := range taskSideQuestSpecs {
		checks = append(checks, struct {
			name     string
			aob      string
			rva      uintptr
			original []byte
		}{name: "side-" + string(rune('A'+index)), aob: spec.AOB, rva: spec.RVA, original: spec.Original})
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			pattern, err := parseRuntimePatchPattern(check.aob)
			if err != nil {
				t.Fatal(err)
			}
			matches := findRuntimePatchLocalPatternMatches(sections, pattern)
			if len(matches) != 1 || uintptr(matches[0].rva) != check.rva {
				t.Fatalf("matches=%s, want one at RVA 0x%X", formatRuntimePatchLocalMatchLocations(matches), check.rva)
			}
			got, err := readPEImageRVA(path, check.rva, len(check.original))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, check.original) {
				t.Fatalf("original=% X, want % X", got, check.original)
			}
		})
	}
}
