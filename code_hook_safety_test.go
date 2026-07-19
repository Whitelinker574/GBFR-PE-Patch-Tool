package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestInstallCodeHookAtomicRestoresPartialPatchBeforeCaveCleanup(t *testing.T) {
	original := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	patch := []byte{9, 9, 9, 9, 9, 9, 9, 9}
	memory := append([]byte(nil), original...)
	first := true
	forced := errors.New("partial write")

	result, err := installCodeHookAtomic(
		original,
		patch,
		func(data []byte) error {
			if first {
				first = false
				copy(memory[:3], data[:3])
				return forced
			}
			copy(memory, data)
			return nil
		},
		func() ([]byte, error) { return append([]byte(nil), memory...), nil },
	)
	if !errors.Is(err, forced) {
		t.Fatalf("error = %v, want partial write", err)
	}
	if result.State != codeHookEntryRestoredAfterPublishAttempt || result.CanFreePreparedCave() || !result.OriginalEntryProven() || !bytes.Equal(memory, original) {
		t.Fatalf("result=%+v memory=% X, want restored published entry without cave cleanup", result, memory)
	}
}

func TestInstallCodeHookAtomicKeepsCaveWhenRestoreCannotBeProven(t *testing.T) {
	original := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	patch := []byte{9, 9, 9, 9, 9, 9, 9, 9}
	memory := append([]byte(nil), original...)
	writes := 0

	result, err := installCodeHookAtomic(
		original,
		patch,
		func(data []byte) error {
			writes++
			if writes == 1 {
				copy(memory[:4], data[:4])
				return errors.New("partial patch")
			}
			return errors.New("restore failed")
		},
		func() ([]byte, error) { return append([]byte(nil), memory...), nil },
	)
	if err == nil {
		t.Fatal("unrestored hook failure returned nil")
	}
	if result.State != codeHookEntryRecoveryRequired || result.CanFreePreparedCave() || result.OriginalEntryProven() {
		t.Fatalf("result=%+v, want retained recovery evidence for an unproven entry", result)
	}
}

func TestInstallCodeHookAtomicVerifiesSuccessfulPatch(t *testing.T) {
	original := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	patch := []byte{9, 9, 9, 9, 9, 9, 9, 9}
	memory := append([]byte(nil), original...)
	result, err := installCodeHookAtomic(
		original,
		patch,
		func(data []byte) error { copy(memory, data); return nil },
		func() ([]byte, error) { return append([]byte(nil), memory...), nil },
	)
	if err != nil || result.State != codeHookEntryInstalled || result.CanFreePreparedCave() || result.OriginalEntryProven() || !bytes.Equal(memory, patch) {
		t.Fatalf("result=%+v err=%v memory=% X", result, err, memory)
	}
}

func TestInstallCodeHookAtomicAllowsPreparedCaveCleanupOnlyBeforePublishAttempt(t *testing.T) {
	original := []byte{1, 2, 3, 4, 5}
	patch := []byte{9, 9, 9, 9, 9}
	preflightErr := errors.New("injected preflight read failure")
	writes := 0

	result, err := installCodeHookAtomic(
		original,
		patch,
		func([]byte) error { writes++; return nil },
		func() ([]byte, error) { return nil, preflightErr },
	)
	if !errors.Is(err, preflightErr) {
		t.Fatalf("error=%v, want preflight failure", err)
	}
	if writes != 0 || result.State != codeHookEntryNeverPublished || !result.CanFreePreparedCave() || result.OriginalEntryProven() {
		t.Fatalf("writes=%d result=%+v, want never-published prepared cave cleanup", writes, result)
	}
}

func TestInstallCodeHookAtomicRetiresCaveAfterPublishedPatchReadbackFailure(t *testing.T) {
	original := []byte{1, 2, 3, 4, 5}
	patch := []byte{9, 9, 9, 9, 9}
	memory := append([]byte(nil), original...)
	readCalls := 0
	readbackErr := errors.New("injected post-publish readback failure")

	result, err := installCodeHookAtomic(
		original,
		patch,
		func(data []byte) error { copy(memory, data); return nil },
		func() ([]byte, error) {
			readCalls++
			if readCalls == 2 {
				return nil, readbackErr
			}
			return append([]byte(nil), memory...), nil
		},
	)
	if !errors.Is(err, readbackErr) {
		t.Fatalf("error=%v, want post-publish readback failure", err)
	}
	if result.State != codeHookEntryRestoredAfterPublishAttempt || result.CanFreePreparedCave() || !result.OriginalEntryProven() || !bytes.Equal(memory, original) {
		t.Fatalf("result=%+v memory=% X, want restored entry and retained retired cave", result, memory)
	}
}

func TestInstallCodeHookAtomicRetiresCaveWhenCodeWriteReportsPostPublishFailure(t *testing.T) {
	for _, test := range []struct {
		name               string
		patchWriteError    error
		patchReadError     error
		restoreWriteError  error
		wantContainedError error
	}{
		{
			name:               "patch flush or protect failure after bytes became visible",
			patchWriteError:    errors.New("injected patch FlushInstructionCache failure"),
			wantContainedError: errors.New("injected patch FlushInstructionCache failure"),
		},
		{
			name:               "restore protect failure after original bytes became visible",
			patchReadError:     errors.New("injected patch readback failure"),
			restoreWriteError:  errors.New("injected restore VirtualProtectEx failure"),
			wantContainedError: errors.New("injected restore VirtualProtectEx failure"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := []byte{1, 2, 3, 4, 5}
			patch := []byte{9, 9, 9, 9, 9}
			memory := append([]byte(nil), original...)
			writeCalls, readCalls := 0, 0

			result, err := installCodeHookAtomic(
				original,
				patch,
				func(data []byte) error {
					writeCalls++
					copy(memory, data)
					if writeCalls == 1 {
						return test.patchWriteError
					}
					return test.restoreWriteError
				},
				func() ([]byte, error) {
					readCalls++
					if readCalls == 2 && test.patchReadError != nil {
						return nil, test.patchReadError
					}
					return append([]byte(nil), memory...), nil
				},
			)
			if err == nil || !strings.Contains(err.Error(), test.wantContainedError.Error()) {
				t.Fatalf("error=%v, want %q", err, test.wantContainedError)
			}
			if result.State != codeHookEntryRestoredAfterPublishAttempt || result.CanFreePreparedCave() || !result.OriginalEntryProven() || !bytes.Equal(memory, original) {
				t.Fatalf("result=%+v memory=% X, want restored entry with retired cave", result, memory)
			}
		})
	}
}

func TestCodeHookInstallResultZeroValueFailsClosed(t *testing.T) {
	var result codeHookInstallResult
	if result.CanFreePreparedCave() || result.OriginalEntryProven() || !result.RequiresRecoveryLease() {
		t.Fatalf("zero-value install result=%+v must require recovery and must never permit cave cleanup", result)
	}
}
