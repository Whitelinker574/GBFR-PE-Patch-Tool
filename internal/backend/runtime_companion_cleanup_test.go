package backend

import (
	"errors"
	"os"
	"path/filepath"
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
	if !app.runtimeCompanionOwned(feature, process) {
		t.Fatal("ownership must remain visible while an error-state runtime still needs restoration")
	}
}

func TestRuntimeCompanionStartDecisionFailsClosedAfterUnknownStartup(t *testing.T) {
	process := processInstanceID{PID: 4242, Created: 777}
	if _, err := runtimeCompanionStartDecision(runtimeCompanionStatus{}, process, true); err == nil {
		t.Fatal("an existing owner without a matching status must not inject a second DLL")
	}
	alreadyActive, err := runtimeCompanionStartDecision(runtimeCompanionStatus{PID: 4242, Created: 777, State: "active"}, process, true)
	if err != nil || !alreadyActive {
		t.Fatalf("owned active companion decision = (%v, %v), want already active", alreadyActive, err)
	}
	if _, err := runtimeCompanionStartDecision(runtimeCompanionStatus{PID: 4242, Created: 777, State: "active"}, process, false); err == nil {
		t.Fatal("an active companion owned by another app must be rejected")
	}
	if _, err := runtimeCompanionStartDecision(runtimeCompanionStatus{PID: 4242, Created: 777, State: "error", Detail: "hook failed"}, process, true); err == nil {
		t.Fatal("an error-state companion must be restored or the game restarted before reinjection")
	}
	if already, err := runtimeCompanionStartDecision(runtimeCompanionStatus{}, process, false); err != nil || already {
		t.Fatalf("fresh startup decision = (%v, %v), want inject", already, err)
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
}

func TestWaitRuntimeCompanionStoppedDoesNotAcceptErrorAsStopped(t *testing.T) {
	feature := "error-state-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	path, err := runtimeCompanionPath(feature + ".status")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	if err := os.WriteFile(path, []byte("pid=4242\ncreated=777\nstate=error\ndetail=restore failed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = waitRuntimeCompanionStopped(feature, processInstanceID{PID: 4242, Created: 777})
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
