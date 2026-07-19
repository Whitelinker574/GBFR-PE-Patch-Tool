package main

import (
	"bytes"
	"errors"
	"testing"
)

func TestInstallCodeHookAtomicRestoresPartialPatchBeforeCaveCleanup(t *testing.T) {
	original := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	patch := []byte{9, 9, 9, 9, 9, 9, 9, 9}
	memory := append([]byte(nil), original...)
	first := true
	forced := errors.New("partial write")

	canFree, err := installCodeHookAtomic(
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
	if !canFree || !bytes.Equal(memory, original) {
		t.Fatalf("restored=%v memory=% X, want safe cleanup and original", canFree, memory)
	}
}

func TestInstallCodeHookAtomicKeepsCaveWhenRestoreCannotBeProven(t *testing.T) {
	original := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	patch := []byte{9, 9, 9, 9, 9, 9, 9, 9}
	memory := append([]byte(nil), original...)
	writes := 0

	canFree, err := installCodeHookAtomic(
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
	if canFree {
		t.Fatal("code cave was declared safe to free while a partial jump remained")
	}
}

func TestInstallCodeHookAtomicVerifiesSuccessfulPatch(t *testing.T) {
	original := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	patch := []byte{9, 9, 9, 9, 9, 9, 9, 9}
	memory := append([]byte(nil), original...)
	canFree, err := installCodeHookAtomic(
		original,
		patch,
		func(data []byte) error { copy(memory, data); return nil },
		func() ([]byte, error) { return append([]byte(nil), memory...), nil },
	)
	if err != nil || canFree || !bytes.Equal(memory, patch) {
		t.Fatalf("canFree=%v err=%v memory=% X", canFree, err, memory)
	}
}
