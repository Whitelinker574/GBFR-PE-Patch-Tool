package backend

import (
	"errors"
	"testing"
)

func TestRuntimeHookEnableFinalReadUsesAtomicFinalizer(t *testing.T) {
	for _, check := range []struct {
		file           string
		name           string
		implementation string
	}{
		{file: "sigil_memory.go", name: "SigilMemoryEnable", implementation: "sigilMemoryEnableLocked"},
		{file: "wrightstone_memory.go", name: "WrightstoneMemoryEnable", implementation: "wrightstoneMemoryEnableLocked"},
		{file: "overlimit.go", name: "OverLimitEnable", implementation: "overLimitEnableLocked"},
	} {
		t.Run(check.name, func(t *testing.T) {
			body := appMethodBodyInFile(t, check.file, check.implementation)
			if body == nil {
				t.Fatalf("%s implementation %s not found", check.name, check.implementation)
			}
			if got := countCallsIdent(body, "finalizeRuntimeHookEnable"); got == 0 {
				t.Fatalf("%s returns its post-install status read directly instead of rolling the hook back on failure", check.name)
			}
			if got := countCallsIdent(body, "runtimeHookInstallFailure"); got == 0 {
				t.Fatalf("%s discards the cave recovery lease when an entry install cannot prove rollback", check.name)
			}
		})
	}
}

func TestFinalizeRuntimeHookEnableRollsBackPostInstallReadFailure(t *testing.T) {
	readErr := errors.New("injected post-install read failure")
	rollbackCalls := 0
	poisonCalls := 0
	_, err := finalizeRuntimeHookEnable(
		"test Hook",
		func() (int, error) { return 0, readErr },
		func() error { rollbackCalls++; return nil },
		func() { poisonCalls++ },
	)
	if !errors.Is(err, readErr) {
		t.Fatalf("finalize error = %v, want post-install read failure", err)
	}
	if errors.Is(err, errRuntimeHookRollbackUnproven) {
		t.Fatalf("proven rollback was reported as indeterminate: %v", err)
	}
	if rollbackCalls != 1 || poisonCalls != 0 {
		t.Fatalf("rollback calls=%d poison calls=%d, want 1 and 0", rollbackCalls, poisonCalls)
	}
}

func TestFinalizeRuntimeHookEnablePoisonsAndKeepsUnprovenRollback(t *testing.T) {
	readErr := errors.New("injected post-install read failure")
	rollbackErr := errors.New("injected restore readback failure")
	poisonCalls := 0
	_, err := finalizeRuntimeHookEnable(
		"test Hook",
		func() (int, error) { return 0, readErr },
		func() error { return rollbackErr },
		func() { poisonCalls++ },
	)
	if !errors.Is(err, readErr) || !errors.Is(err, rollbackErr) || !errors.Is(err, errRuntimeHookRollbackUnproven) {
		t.Fatalf("unproven rollback error lost its causes: %v", err)
	}
	if poisonCalls != 1 {
		t.Fatalf("poison calls=%d, want 1", poisonCalls)
	}
}

func TestRuntimeHookInstallFailureRetainsLeaseOnlyWhenRollbackIsUnproven(t *testing.T) {
	cause := errors.New("injected partial entry write")
	for _, test := range []struct {
		name   string
		result codeHookInstallResult
	}{
		{name: "entry never published", result: codeHookInstallResult{State: codeHookEntryNeverPublished}},
		{name: "entry restored after publish attempt", result: codeHookInstallResult{State: codeHookEntryRestoredAfterPublishAttempt}},
		{name: "entry indeterminate", result: codeHookInstallResult{State: codeHookEntryRecoveryRequired}},
	} {
		t.Run(test.name, func(t *testing.T) {
			freeCalls, retireCalls, retainCalls, poisonCalls := 0, 0, 0, 0
			err := runtimeHookInstallFailure(
				"test Hook", test.result, cause,
				func() { freeCalls++ },
				func() { retireCalls++ },
				func() { retainCalls++ },
				func() { poisonCalls++ },
			)
			if !errors.Is(err, cause) {
				t.Fatalf("install error lost its cause: %v", err)
			}
			switch test.result.State {
			case codeHookEntryNeverPublished:
				if freeCalls != 1 || retireCalls != 0 || retainCalls != 0 || poisonCalls != 0 || errors.Is(err, errRuntimeHookRollbackUnproven) {
					t.Fatalf("never-published handling: free=%d retire=%d retain=%d poison=%d err=%v", freeCalls, retireCalls, retainCalls, poisonCalls, err)
				}
			case codeHookEntryRestoredAfterPublishAttempt:
				if freeCalls != 0 || retireCalls != 1 || retainCalls != 0 || poisonCalls != 0 || errors.Is(err, errRuntimeHookRollbackUnproven) {
					t.Fatalf("retired handling: free=%d retire=%d retain=%d poison=%d err=%v", freeCalls, retireCalls, retainCalls, poisonCalls, err)
				}
			case codeHookEntryRecoveryRequired:
				if freeCalls != 0 || retireCalls != 0 || retainCalls != 1 || poisonCalls != 1 || !errors.Is(err, errRuntimeHookRollbackUnproven) {
					t.Fatalf("unproven restore handling: free=%d retire=%d retain=%d poison=%d err=%v", freeCalls, retireCalls, retainCalls, poisonCalls, err)
				}
			}
		})
	}
}

func TestRuntimeHookInstallFailureRetiresPublishedCaveUntilDetach(t *testing.T) {
	app := &App{charaPID: 42, charaCreated: 100}
	const cave = uintptr(0x12345000)
	freeCalls := 0
	cause := errors.New("injected post-publish verification failure")

	err := runtimeHookInstallFailure(
		"test Hook",
		codeHookInstallResult{State: codeHookEntryRestoredAfterPublishAttempt},
		cause,
		func() { freeCalls++ },
		func() { app.retireRuntimeCaveLocked(cave, "test retired cave") },
		func() { t.Fatal("restored entry must not retain an active recovery lease") },
		func() { t.Fatal("restored entry must not poison the process") },
	)
	if !errors.Is(err, cause) || freeCalls != 0 {
		t.Fatalf("error=%v freeCalls=%d, want original error and no cave free", err, freeCalls)
	}
	if got := app.retiredRuntimeCaves; len(got) != 1 || got[0].Address != cave || !sameProcessInstance(got[0].Process, processInstanceID{PID: 42, Created: 100}) {
		t.Fatalf("retired caves=%+v, want cave 0x%X bound to PID/Created", got, cave)
	}
	if err := app.charaDetachLocked(); err != nil {
		t.Fatal(err)
	}
	if len(app.retiredRuntimeCaves) != 0 {
		t.Fatalf("detach retained retired-cave metadata: %+v", app.retiredRuntimeCaves)
	}
}
