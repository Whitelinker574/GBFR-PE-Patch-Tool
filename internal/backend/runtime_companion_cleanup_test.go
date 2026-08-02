package backend

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestRestoreRuntimeCompanionsAttemptsEveryRuntimeAndJoinsFailures(t *testing.T) {
	var calls []string
	first := errors.New("first failed")
	third := errors.New("third failed")
	err := restoreRuntimeCompanions(
		struct {
			name   string
			remove func(string) error
		}{"first", func(string) error { calls = append(calls, "first"); return first }},
		struct {
			name   string
			remove func(string) error
		}{"second", func(string) error { calls = append(calls, "second"); return nil }},
		struct {
			name   string
			remove func(string) error
		}{"third", func(string) error { calls = append(calls, "third"); return third }},
	)
	if got := strings.Join(calls, ","); got != "first,second,third" {
		t.Fatalf("restore order = %s", got)
	}
	if !errors.Is(err, first) || !errors.Is(err, third) {
		t.Fatalf("joined restore error = %v", err)
	}
}

func TestStopOwnedRuntimeCompanionAcceptsNilDisableCallback(t *testing.T) {
	source, err := os.ReadFile("runtime_companion.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	start := strings.Index(body, "func (a *App) stopOwnedRuntimeCompanion")
	end := strings.Index(body[start:], "\nfunc (a *App) runtimeCompanionActive")
	if start < 0 || end < 0 {
		t.Fatal("stopOwnedRuntimeCompanion source block was not found")
	}
	block := body[start : start+end]
	if !strings.Contains(block, "if disable != nil") {
		t.Fatal("stopOwnedRuntimeCompanion still calls a nil disable callback during empty-config cleanup")
	}
}

func TestRuntimeCompanionStatusMatchesCompleteProcessIdentity(t *testing.T) {
	current := processInstanceID{PID: 42, Created: 100}
	cases := []struct {
		name   string
		status runtimeCompanionStatus
		want   bool
	}{
		{name: "same process", status: runtimeCompanionStatus{PID: 42, Created: 100}, want: true},
		{name: "reused pid", status: runtimeCompanionStatus{PID: 42, Created: 99}},
		{name: "legacy status without creation", status: runtimeCompanionStatus{PID: 42}},
		{name: "different pid", status: runtimeCompanionStatus{PID: 43, Created: 100}},
		{name: "empty status", status: runtimeCompanionStatus{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := runtimeCompanionMatchesProcess(tc.status, current); got != tc.want {
				t.Fatalf("match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRuntimeCompanionVerifiesExecutableBeforeOwnershipOrInjection(t *testing.T) {
	source, err := os.ReadFile("runtime_companion.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	start := strings.Index(body, "func (a *App) startRuntimeCompanion")
	end := strings.Index(body[start:], "\nfunc waitRuntimeCompanionStopped")
	if start < 0 || end < 0 {
		t.Fatal("startRuntimeCompanion source block was not found")
	}
	block := body[start : start+end]
	verifyAt := strings.Index(block, "verifyRuntimePatchExecutableLocked")
	claimAt := strings.Index(block, "claimRuntimeCompanionOwnership")
	injectAt := strings.Index(block, "extractAndInjectPatchCore")
	if verifyAt < 0 || claimAt < 0 || injectAt < 0 || verifyAt > claimAt || verifyAt > injectAt {
		t.Fatal("runtime companion must verify the 2.0.2 executable before ownership or DLL injection")
	}
}

func TestRuntimeCompanionCommandBindsLiveToolProcessIdentity(t *testing.T) {
	generation := "0123456789abcdef0123456789abcdef"
	command, err := runtimeCompanionCommand("runtime_camera", generation)
	if err != nil {
		t.Fatal(err)
	}
	created, err := processCreationTime(windows.CurrentProcess())
	if err != nil {
		t.Fatal(err)
	}
	want := "runtime_camera\nowner_pid=" + strconv.Itoa(os.Getpid()) +
		"\nowner_created=" + strconv.FormatUint(created, 10) +
		"\ngeneration=" + generation + "\n"
	if command != want {
		t.Fatalf("runtime companion command = %q, want %q", command, want)
	}
}

func TestNativeRuntimeCompanionsRestoreWhenToolOwnerExits(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "src_dll", "patch_core", "dllmain.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	required := []string{
		"class RuntimeOwnerGuard",
		"OpenProcess(SYNCHRONIZE | PROCESS_QUERY_LIMITED_INFORMATION",
		"actual.QuadPart != ownerCreated",
		"WaitForSingleObject(process_, 0) == WAIT_TIMEOUT",
		"RuntimeOwnerMatchesGeneration",
		`ReadCommandValue(content, "generation"`,
		"generation=%s",
		"GENERIC_READ | DELETE",
		"FILE_SHARE_READ | FILE_SHARE_WRITE",
		"SetFileInformationByHandle(handle_, FileDispositionInfo",
		"if (!published) DeleteFileW(temporary.c_str())",
		"ReleaseRuntimeOwnerAfterVerifiedStop",
		"WriteRuntimeInactiveAndReleaseOwner",
		`RuntimePath((std::wstring(feature) + L".owner").c_str())`,
		"while (owner.Alive())",
		"while (owner.Alive() && GetPrivateProfileIntW(L\"damage\"",
		"while (owner.Alive() && GetPrivateProfileIntW(L\"qol\"",
	}
	for _, text := range required {
		if !strings.Contains(body, text) {
			t.Fatalf("native runtime owner watchdog is missing %q", text)
		}
	}
	releaseStart := strings.Index(body, "static void ReleaseRuntimeOwnerAfterVerifiedStop")
	if releaseStart < 0 {
		t.Fatal("generation-scoped native owner release block was not found")
	}
	releaseEnd := strings.Index(body[releaseStart:], "\nstatic void WriteRuntimeInactiveAndReleaseOwner")
	if releaseEnd < 0 {
		t.Fatal("generation-scoped native owner release block was not found")
	}
	releaseBlock := body[releaseStart : releaseStart+releaseEnd]
	if strings.Contains(releaseBlock, "DeleteFileW(") {
		t.Fatal("native owner release must delete the validated file handle, not a path that may now name a successor")
	}
	for _, runtime := range []string{"camera", "virtual-sigils", "weapon-skills", "audio", "damage", "qol"} {
		start := strings.Index(body, "static DWORD Run"+map[string]string{
			"camera":         "Camera",
			"virtual-sigils": "VirtualSigil",
			"weapon-skills":  "WeaponSkills",
			"audio":          "Audio",
			"damage":         "Damage",
			"qol":            "QOL",
		}[runtime]+"Runtime()")
		if start < 0 {
			t.Fatalf("%s runtime entry was not found", runtime)
		}
		block := body[start:]
		next := strings.Index(block[1:], "\nstatic ")
		if next >= 0 {
			block = block[:next+1]
		}
		if !strings.Contains(block, "RuntimeOwnerGuard owner;") ||
			!strings.Contains(block, `owner.OpenFromCommand(L"`+runtime+`")`) {
			t.Fatalf("%s runtime does not fail closed without a live tool owner", runtime)
		}
	}
}

func TestCleanupRuntimeCompanionStatusTemporariesIsStrictlyScoped(t *testing.T) {
	feature := "status-temp-cleanup-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	dir, err := runtimeCompanionDirectory()
	if err != nil {
		t.Fatal(err)
	}
	generation := "0123456789abcdef0123456789abcdef"
	ownedTemporary := filepath.Join(dir, feature+".status."+generation+".tmp")
	keep := []string{
		filepath.Join(dir, feature+".status"),
		filepath.Join(dir, feature+".owner"),
		filepath.Join(dir, feature+".lease"),
		filepath.Join(dir, feature+".status."+generation[:31]+".tmp"),
		filepath.Join(dir, "another-"+feature+".status."+generation+".tmp"),
	}
	all := append([]string{ownedTemporary}, keep...)
	for _, path := range all {
		if err := os.WriteFile(path, []byte("sentinel"), 0o600); err != nil {
			t.Fatal(err)
		}
		path := path
		t.Cleanup(func() { _ = os.Remove(path) })
	}
	if err := cleanupRuntimeCompanionStatusTemporaries(feature); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ownedTemporary); !os.IsNotExist(err) {
		t.Fatalf("exact stale status temporary survived cleanup: %v", err)
	}
	for _, path := range keep {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("scoped cleanup touched %s: %v", filepath.Base(path), err)
		}
	}
}

func TestNativeOwnerHandleBlocksConcurrentPathDeletion(t *testing.T) {
	feature := "native-owner-handle-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	path, err := runtimeCompanionOwnerPath(feature)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	if err := os.WriteFile(path, []byte(
		"owner=test\ngeneration=0123456789abcdef0123456789abcdef\npid=1\ncreated=1\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	pathUTF16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(
		pathUTF16,
		windows.GENERIC_READ|windows.DELETE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = windows.CloseHandle(handle)
		}
	}()

	removeDone := make(chan error, 1)
	go func() { removeDone <- os.Remove(path) }()
	if err := <-removeDone; err == nil {
		t.Fatal("desktop path deletion succeeded while native held the validated owner handle")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("owner path disappeared while its validated handle was held: %v", err)
	}
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	closed = true
	if err := os.Remove(path); err != nil {
		t.Fatalf("owner path could not be removed after native-style handle close: %v", err)
	}
}

func TestRuntimeCompanionReleaseDoesNotDeleteSuccessorGeneration(t *testing.T) {
	feature := "release-generation-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	ownerPath, err := runtimeCompanionOwnerPath(feature)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(ownerPath)
	process := processInstanceID{PID: 4242, Created: 777}
	app := &App{}
	if err := app.claimRuntimeCompanionOwnership(feature, process); err != nil {
		t.Fatal(err)
	}
	lease, ok := app.runtimeCompanionLease(feature, process)
	if !ok {
		t.Fatal("claim did not retain a generation-scoped lease")
	}
	successorGeneration := "ffffffffffffffffffffffffffffffff"
	if successorGeneration == lease.Generation {
		t.Fatal("test successor generation unexpectedly matches the random lease")
	}
	if err := os.WriteFile(ownerPath, []byte(fmt.Sprintf(
		"owner=%s\ngeneration=%s\npid=%d\ncreated=%d\n",
		app.runtimeCompanionOwnerID(),
		successorGeneration,
		process.PID,
		process.Created,
	)), 0o600); err != nil {
		t.Fatal(err)
	}
	app.releaseRuntimeCompanionOwnership(feature)
	owner := readRuntimeCompanionOwner(feature)
	if owner.Generation != successorGeneration {
		t.Fatalf("release of generation %q deleted successor owner %#v", lease.Generation, owner)
	}
	if _, ok := app.runtimeCompanionLease(feature, process); ok {
		t.Fatal("release retained its old kernel lease after preserving the successor owner")
	}
}

func TestRuntimeCompanionRejectsDifferentOwnerForSameGameEvenWhenInactive(t *testing.T) {
	feature := "inactive-owner-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	ownerPath, err := runtimeCompanionOwnerPath(feature)
	if err != nil {
		t.Fatal(err)
	}
	statusPath, err := runtimeCompanionPath(feature + ".status")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(ownerPath)
	defer os.Remove(statusPath)

	process := processInstanceID{PID: uint32(os.Getpid()), Created: 777}
	otherGeneration := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	ownerData := fmt.Sprintf(
		"owner=another-tool\ngeneration=%s\npid=%d\ncreated=%d\n",
		otherGeneration,
		process.PID,
		process.Created,
	)
	if err := os.WriteFile(ownerPath, []byte(ownerData), 0o600); err != nil {
		t.Fatal(err)
	}
	statusData := fmt.Sprintf(
		"pid=%d\ncreated=%d\ngeneration=%s\nstate=inactive\ndetail=restored\n",
		process.PID,
		process.Created,
		otherGeneration,
	)
	if err := os.WriteFile(statusPath, []byte(statusData), 0o600); err != nil {
		t.Fatal(err)
	}

	app := &App{}
	if err := app.claimRuntimeCompanionOwnership(feature, process); err == nil {
		t.Fatal("a different owner for the same game instance was replaced after publishing inactive")
	}
	owner := readRuntimeCompanionOwner(feature)
	if owner.ID != "another-tool" || owner.Generation != otherGeneration {
		t.Fatalf("different owner was changed: %#v", owner)
	}
	if _, ok := app.runtimeCompanionLease(feature, process); ok {
		t.Fatal("failed ownership claim retained its kernel lease")
	}
}

func TestRuntimeCompanionLeaseMovesToRestartedGameAfterOldIdentityDies(t *testing.T) {
	feature := "restarted-game-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	ownerPath, err := runtimeCompanionOwnerPath(feature)
	if err != nil {
		t.Fatal(err)
	}
	statusPath, err := runtimeCompanionPath(feature + ".status")
	if err != nil {
		t.Fatal(err)
	}
	leasePath, err := runtimeCompanionLeasePath(feature)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(ownerPath)
	defer os.Remove(statusPath)
	defer os.Remove(leasePath)

	currentCreated, err := processCreationTime(windows.CurrentProcess())
	if err != nil {
		t.Fatal(err)
	}
	current := processInstanceID{PID: uint32(os.Getpid()), Created: currentCreated}
	old := processInstanceID{PID: current.PID, Created: current.Created - 1}
	oldGeneration := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	oldHandle, err := createExclusiveDeleteOnCloseFile(leasePath)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{
		runtimeCompanionLeases: map[string]runtimeCompanionLease{
			feature: {Handle: oldHandle, Process: old, Generation: oldGeneration},
		},
	}
	appID := app.runtimeCompanionOwnerID()
	if err := os.WriteFile(ownerPath, []byte(fmt.Sprintf(
		"owner=%s\ngeneration=%s\npid=%d\ncreated=%d\n",
		appID,
		oldGeneration,
		old.PID,
		old.Created,
	)), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := app.claimRuntimeCompanionOwnership(feature, current); err != nil {
		t.Fatalf("live tool could not move its companion lease to the restarted game: %v", err)
	}
	t.Cleanup(func() { app.releaseRuntimeCompanionOwnership(feature) })
	lease, ok := app.runtimeCompanionLease(feature, current)
	if !ok {
		t.Fatal("new game process does not own the replacement lease")
	}
	if lease.Generation == "" || lease.Generation == oldGeneration {
		t.Fatalf("replacement lease generation = %q", lease.Generation)
	}
	owner := readRuntimeCompanionOwner(feature)
	if owner.PID != current.PID || owner.Created != current.Created || owner.Generation != lease.Generation {
		t.Fatalf("replacement owner = %#v, lease = %#v", owner, lease)
	}
}

func TestRuntimeCompanionNeedsStopKeepsErrorStateOwned(t *testing.T) {
	process := processInstanceID{PID: 42, Created: 100}
	if !runtimeCompanionNeedsStop(runtimeCompanionStatus{PID: 42, Created: 100, State: "active"}, process) {
		t.Fatal("active companion must require restoration")
	}
	if !runtimeCompanionNeedsStop(runtimeCompanionStatus{PID: 42, Created: 100, State: "error"}, process) {
		t.Fatal("legacy error companion must remain a restoration obligation")
	}
	if !runtimeCompanionNeedsStop(runtimeCompanionStatus{PID: 42, Created: 100, State: "restore_failed"}, process) {
		t.Fatal("restore_failed companion must remain a restoration obligation")
	}
	if runtimeCompanionNeedsStop(runtimeCompanionStatus{PID: 42, Created: 100, State: "inactive"}, process) {
		t.Fatal("inactive companion must not require restoration")
	}
}

func TestRuntimeCompanionWorkspaceStateExposesRestorationErrors(t *testing.T) {
	process := processInstanceID{PID: 42, Created: 100}
	errorStatus := runtimeCompanionStatus{PID: 42, Created: 100, State: "error", Detail: "restore failed"}
	if !runtimeCompanionInstalled(errorStatus, process) {
		t.Fatal("an error-state companion must remain visible as installed until restoration succeeds")
	}
	if runtimeCompanionInstalled(runtimeCompanionStatus{PID: 42, Created: 100, State: "inactive"}, process) {
		t.Fatal("an inactive companion must not be shown as installed")
	}
	if runtimeCompanionInstalled(errorStatus, processInstanceID{PID: 42, Created: 99}) {
		t.Fatal("a stale status from a reused PID must not be shown as installed")
	}
}

func TestRuntimeCompanionOwnedIncludesErrorState(t *testing.T) {
	feature := "owned-error-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	path, err := runtimeCompanionOwnerPath(feature)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	process := processInstanceID{PID: 4242, Created: 777}
	app := &App{}
	if err := app.claimRuntimeCompanionOwnership(feature, process); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.releaseRuntimeCompanionOwnership(feature) })
	if !app.runtimeCompanionOwned(feature, process) {
		t.Fatal("ownership must remain visible while an error-state runtime still needs restoration")
	}
}

func TestRuntimeCompanionStartDecisionFailsClosedAfterUnknownStartup(t *testing.T) {
	process := processInstanceID{PID: 4242, Created: 777}
	generation := "cccccccccccccccccccccccccccccccc"
	if _, err := runtimeCompanionStartDecision(runtimeCompanionStatus{}, process, generation); err == nil {
		t.Fatal("an existing owner without a matching status must not inject a second DLL")
	}
	alreadyActive, err := runtimeCompanionStartDecision(runtimeCompanionStatus{
		PID: 4242, Created: 777, Generation: generation, State: "active",
	}, process, generation)
	if err != nil || !alreadyActive {
		t.Fatalf("owned active companion decision = (%v, %v), want already active", alreadyActive, err)
	}
	if _, err := runtimeCompanionStartDecision(runtimeCompanionStatus{
		PID: 4242, Created: 777, Generation: generation, State: "active",
	}, process, ""); err == nil {
		t.Fatal("an active companion owned by another app must be rejected")
	}
	if _, err := runtimeCompanionStartDecision(runtimeCompanionStatus{
		PID: 4242, Created: 777, Generation: generation, State: "error", Detail: "hook failed",
	}, process, generation); err == nil {
		t.Fatal("an error-state companion must be restored or the game restarted before reinjection")
	}
	if already, err := runtimeCompanionStartDecision(runtimeCompanionStatus{}, process, ""); err != nil || already {
		t.Fatalf("fresh startup decision = (%v, %v), want inject", already, err)
	}
	if _, err := runtimeCompanionStartDecision(runtimeCompanionStatus{
		PID: 4242, Created: 777, Generation: "dddddddddddddddddddddddddddddddd", State: "active",
	}, process, generation); err == nil {
		t.Fatal("an owned status from another generation was accepted")
	}
}

func TestClearStaleInactiveRuntimeCompanionStatusWaitsForFreshInjectionStatus(t *testing.T) {
	feature := "stale-inactive-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	path, err := runtimeCompanionPath(feature + ".status")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	process := processInstanceID{PID: 4242, Created: 777}
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := clearStaleInactiveRuntimeCompanionStatus(feature, runtimeCompanionStatus{PID: 4242, Created: 777, State: "inactive"}, process); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stale inactive status still exists: %v", err)
	}
	if err := os.WriteFile(path, []byte("active"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := clearStaleInactiveRuntimeCompanionStatus(feature, runtimeCompanionStatus{PID: 4242, Created: 777, State: "active"}, process); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("active status was removed: %v", err)
	}
}

func TestRuntimeCompanionOwnershipIsExclusivePerGameInstance(t *testing.T) {
	feature := "ownership-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	path, err := runtimeCompanionOwnerPath(feature)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	process := processInstanceID{PID: 4242, Created: 777}
	first := &App{}
	second := &App{}
	if err := first.claimRuntimeCompanionOwnership(feature, process); err != nil {
		t.Fatal(err)
	}
	if err := second.claimRuntimeCompanionOwnership(feature, process); err == nil {
		t.Fatal("second app instance acquired the same runtime owner")
	}
	if !first.ownsRuntimeCompanion(feature, process) || second.ownsRuntimeCompanion(feature, process) {
		t.Fatal("runtime owner identity was not isolated")
	}
	first.releaseRuntimeCompanionOwnership(feature)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("runtime owner file survived verified release: %v", err)
	}
	leasePath, err := runtimeCompanionLeasePath(feature)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(leasePath); !os.IsNotExist(err) {
		t.Fatalf("kernel lease file survived handle close: %v", err)
	}
	if err := second.claimRuntimeCompanionOwnership(feature, process); err != nil {
		t.Fatalf("second app could not acquire ownership after verified release: %v", err)
	}
	second.releaseRuntimeCompanionOwnership(feature)
}

func TestWaitRuntimeCompanionStoppedDoesNotAcceptErrorAsStopped(t *testing.T) {
	feature := "error-state-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	path, err := runtimeCompanionPath(feature + ".status")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	generation := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	if err := os.WriteFile(path, []byte("pid=4242\ncreated=777\ngeneration="+generation+"\nstate=error\ndetail=restore failed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = waitRuntimeCompanionStopped(feature, processInstanceID{PID: 4242, Created: 777}, generation)
	if err == nil || !strings.Contains(err.Error(), "restore failed") {
		t.Fatalf("error state was accepted as stopped: %v", err)
	}
}

func TestCleanupRuntimeCompanionDLLRemovesOwnedDLLAndCommand(t *testing.T) {
	dir := t.TempDir()
	dllPath := filepath.Join(dir, "patch_core_test.dll")
	if err := os.WriteFile(dllPath, []byte("dll"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dllPath+".command", []byte("runtime_audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	rememberRuntimeCompanionDLL("test", dllPath)
	cleanupRuntimeCompanionDLL("test")
	for _, path := range []string{dllPath, dllPath + ".command"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("temporary runtime file still exists: %s", path)
		}
	}
}

func TestWriteRuntimeCompanionFileRetriesTransientSharingViolation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "virtual-sigils.bin")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	pathUTF16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(pathUTF16, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan struct{})
	go func() {
		time.Sleep(75 * time.Millisecond)
		windows.CloseHandle(handle)
		close(closed)
	}()
	if err := writeRuntimeCompanionFile(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	<-closed
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("runtime companion contents = %q, want new", data)
	}
}
