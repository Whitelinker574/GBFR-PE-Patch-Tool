package backend

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func TestClassifyRemoteCallWait(t *testing.T) {
	if err := classifyRemoteCallWait(uint32(windows.WAIT_OBJECT_0), nil); err != nil {
		t.Fatalf("completed thread = %v", err)
	}
	for _, tc := range []struct {
		name    string
		wait    uint32
		waitErr error
	}{
		{name: "timeout", wait: uint32(windows.WAIT_TIMEOUT)},
		{name: "wait failure", wait: 0xFFFFFFFF, waitErr: errors.New("wait failed")},
		{name: "unexpected", wait: uint32(windows.WAIT_ABANDONED)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyRemoteCallWait(tc.wait, tc.waitErr)
			if !isRemoteCallIndeterminate(err) {
				t.Fatalf("result = %v, want indeterminate", err)
			}
		})
	}
}

func TestRemoteCallIndeterminateSurvivesWrapping(t *testing.T) {
	err := errors.Join(errors.New("save failed"), newRemoteCallIndeterminateError("thread still running"))
	if !isRemoteCallIndeterminate(err) {
		t.Fatalf("wrapped error was not classified: %v", err)
	}
}
