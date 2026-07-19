package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestShutdownPersistsDetachFailure(t *testing.T) {
	body := appMethodBodyInFile(t, "app.go", "shutdown")
	if body == nil {
		t.Fatal("shutdown not found")
	}
	if got := countCallsIdent(body, "appendDiagnosticError"); got == 0 {
		t.Fatal("shutdown discards CharaDetach failures instead of persisting recovery diagnostics")
	}
}

func TestBeforeCloseFailsClosedOnDetachError(t *testing.T) {
	body := appMethodBodyInFile(t, "app.go", "beforeClose")
	if body == nil {
		t.Fatal("beforeClose not found")
	}
	if got := countCallsSelector(body, "a", "CharaDetach"); got == 0 {
		t.Fatal("beforeClose does not attempt to restore runtime hooks")
	}
	if got := countCallsIdent(body, "handleDetachBeforeClose"); got == 0 {
		t.Fatal("beforeClose does not route detach failures through the fail-closed close guard")
	}
	if detachPosition, savePosition := firstSelectorCallPosition(body, "a", "CharaDetach"), firstSelectorCallPosition(body, "a", "saveWindowSize"); detachPosition > savePosition {
		t.Fatal("beforeClose performs ordinary shutdown work before restoring runtime hooks")
	}
}

func TestDetachWithoutConnectionIsNotAnError(t *testing.T) {
	if err := NewApp().CharaDetach(); err != nil {
		t.Fatalf("ordinary disconnected state must remain safe to close: %v", err)
	}
}

func TestHandleDetachBeforeCloseBlocksAndPersistsFailure(t *testing.T) {
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	previousDialog := closeMessageDialog
	dialogCalls := 0
	closeMessageDialog = func(_ context.Context, options runtime.MessageDialogOptions) (string, error) {
		dialogCalls++
		if options.Type != runtime.ErrorDialog || !strings.Contains(options.Message, "诊断日志") {
			t.Errorf("unexpected close guard dialog: %+v", options)
		}
		return "确定", nil
	}
	t.Cleanup(func() { closeMessageDialog = previousDialog })

	detachErr := errors.New("injected restoration failure")
	if prevent := handleDetachBeforeClose(context.Background(), detachErr); !prevent {
		t.Fatal("hook restoration failure did not block close")
	}
	if dialogCalls != 1 {
		t.Fatalf("close guard dialogs = %d, want 1", dialogCalls)
	}
	contents, err := os.ReadFile(filepath.Join(localAppData, "GBFR-PE-Patch-Tool", "startup.log"))
	if err != nil {
		t.Fatal(err)
	}
	if text := string(contents); !strings.Contains(text, "before-close hook restoration") || !strings.Contains(text, detachErr.Error()) {
		t.Fatalf("persisted close diagnostic = %q", text)
	}

	if prevent := handleDetachBeforeClose(context.Background(), nil); prevent {
		t.Fatal("ordinary disconnected/successful detach state must not block close")
	}
	if dialogCalls != 1 {
		t.Fatalf("nil detach error unexpectedly opened another dialog: %d", dialogCalls)
	}
}
