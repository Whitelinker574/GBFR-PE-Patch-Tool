package backend

import (
	"debug/pe"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestPersistentRuntimeCompanionReconnectDecision(t *testing.T) {
	oldProcess := processInstanceID{PID: 19156, Created: 100}
	newProcess := processInstanceID{PID: 18140, Created: 200}
	oldInactive := runtimeCompanionStatus{PID: oldProcess.PID, Created: oldProcess.Created, State: "inactive"}

	if !shouldReconnectPersistentRuntimeCompanion(true, oldInactive, newProcess) {
		t.Fatal("an enabled long-running feature did not reconnect after the game process generation changed")
	}
	if shouldReconnectPersistentRuntimeCompanion(false, oldInactive, newProcess) {
		t.Fatal("a feature explicitly disabled by the user must not reconnect")
	}
	for _, state := range []string{"active", "error", "restore_failed", "starting", ""} {
		status := runtimeCompanionStatus{PID: newProcess.PID, Created: newProcess.Created, State: state}
		if shouldReconnectPersistentRuntimeCompanion(true, status, newProcess) {
			t.Fatalf("same-process state %q must not be injected over", state)
		}
	}
	inactive := runtimeCompanionStatus{PID: newProcess.PID, Created: newProcess.Created, State: "inactive"}
	if !shouldReconnectPersistentRuntimeCompanion(true, inactive, newProcess) {
		t.Fatal("an enabled feature with a proven inactive status should restart")
	}
}

func TestWeaponRuntimePendingRefreshIsNotAnInstalledHookButRemainsVisible(t *testing.T) {
	process := processInstanceID{PID: 42, Created: 84}
	status := runtimeCompanionStatus{PID: process.PID, Created: process.Created, State: "inactive_pending_refresh"}
	if runtimeCompanionNeedsStop(status, process) || runtimeCompanionInstalled(status, process) {
		t.Fatal("restored weapon Hook was still reported as installed")
	}
	if !runtimeCompanionRecoveryRequired(status, process) {
		t.Fatal("pending native status refresh disappeared from recovery UI")
	}
	if shouldReconnectPersistentRuntimeCompanion(true, status, process) {
		t.Fatal("supervisor reinjected weapon skills before pending cached state was acknowledged")
	}
}

func TestRuntimeCompanionSummaryIncludesAuthoritativeRuntimePatchState(t *testing.T) {
	summaries := (&App{}).GetRuntimeCompanionSummary()
	foundReward := false
	foundPatches := false
	for _, summary := range summaries {
		if summary.ID == "taskRewardMultiplier" {
			foundReward = true
			if summary.Active || summary.Owned || summary.Multiplier != 1 || summary.State != "inactive" {
				t.Fatalf("empty app published a stale task-reward session: %+v", summary)
			}
		}
		if summary.ID != "runtimePatches" {
			continue
		}
		foundPatches = true
		if summary.Active || summary.ActiveCount != 0 || summary.RecoveryCount != 0 || summary.State != "inactive" {
			t.Fatalf("empty app published a stale runtime-patch session: %+v", summary)
		}
	}
	if !foundReward {
		t.Fatal("task-reward authority was omitted from the shell summary")
	}
	if !foundPatches {
		t.Fatal("runtime-patch authority was omitted from the shell summary")
	}
}

func parseRuntimeSignature(t *testing.T, value string) []int {
	t.Helper()
	parts := strings.Fields(value)
	result := make([]int, len(parts))
	for index, part := range parts {
		if part == "??" {
			result[index] = -1
			continue
		}
		parsed, err := strconv.ParseUint(part, 16, 8)
		if err != nil {
			t.Fatalf("parse signature byte %q: %v", part, err)
		}
		result[index] = int(parsed)
	}
	return result
}

func countRuntimeSignature(data []byte, pattern []int) int {
	count := 0
	for start := 0; start+len(pattern) <= len(data); start++ {
		matched := true
		for index, expected := range pattern {
			if expected >= 0 && data[start+index] != byte(expected) {
				matched = false
				break
			}
		}
		if matched {
			count++
		}
	}
	return count
}

func TestCameraRuntimeSignaturesMatchExternal203Executable(t *testing.T) {
	path := strings.TrimSpace(os.Getenv("GBFR_GAME_EXE_TEST"))
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_TEST to a local 2.0.3 executable")
	}
	image, err := pe.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer image.Close()
	text := image.Section(".text")
	if text == nil {
		t.Fatal("2.0.3 executable has no .text section")
	}
	data, err := text.Data()
	if err != nil {
		t.Fatal(err)
	}
	signatures := map[string]string{
		"camera initializer":    "56 48 83 EC ?? 8B 05 ?? ?? ?? ?? 65 48 8B 0C 25 ?? ?? ?? ?? 48 8B 04 C1 48 8B 88 ?? ?? ?? ?? 48 8B 81 ?? ?? ?? ?? 48 8B 70 ?? 48 85 F6 0F 84 ?? ?? ?? ?? 89 F2 83 E2 ?? 0F 85 ?? ?? ?? ?? FF 40 ?? 48 8B 0E 48 89 48 ?? C5 F8 57 C0 C5 F8 29 46 ?? C5 F8 29 46",
		"current camera global": "48 89 35 ?? ?? ?? ?? C5 F8 29 06 48 C7 46 10 00 00 00 00 48 8D 05",
		"zoom decrease":         "C5 FA 10 05 ?? ?? ?? ?? EB ?? C5 FA 10 05 ?? ?? ?? ?? C5 FA 58 05",
		"zoom increase":         "C5 FA 10 05 ?? ?? ?? ?? C5 FA 58 05 ?? ?? ?? ?? C5 FA 5D 05",
	}
	for name, signature := range signatures {
		if count := countRuntimeSignature(data, parseRuntimeSignature(t, signature)); count != 1 {
			t.Errorf("%s signature matched %d times; want exactly one 2.0.3 location", name, count)
		}
	}
}
