package main

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
		name    string
		canFree bool
	}{
		{name: "entry restored", canFree: true},
		{name: "entry indeterminate", canFree: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			freeCalls, retainCalls, poisonCalls := 0, 0, 0
			err := runtimeHookInstallFailure(
				"test Hook", test.canFree, cause,
				func() { freeCalls++ },
				func() { retainCalls++ },
				func() { poisonCalls++ },
			)
			if !errors.Is(err, cause) {
				t.Fatalf("install error lost its cause: %v", err)
			}
			if test.canFree {
				if freeCalls != 1 || retainCalls != 0 || poisonCalls != 0 || errors.Is(err, errRuntimeHookRollbackUnproven) {
					t.Fatalf("proven restore handling: free=%d retain=%d poison=%d err=%v", freeCalls, retainCalls, poisonCalls, err)
				}
			} else if freeCalls != 0 || retainCalls != 1 || poisonCalls != 1 || !errors.Is(err, errRuntimeHookRollbackUnproven) {
				t.Fatalf("unproven restore handling: free=%d retain=%d poison=%d err=%v", freeCalls, retainCalls, poisonCalls, err)
			}
		})
	}
}
