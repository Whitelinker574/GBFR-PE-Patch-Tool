//go:build windows

package backend

import (
	"os"
	"strings"
	"testing"
)

func TestMonsterEnhanceVerifiesExecutableBeforePreparingOrInjecting(t *testing.T) {
	source, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	start := strings.Index(body, "func (a *App) monsterEnhanceSetPatchValueEnabledLocked")
	end := strings.Index(body[start:], "\nfunc (a *App) readMonsterEnhanceStatus")
	if start < 0 || end < 0 {
		t.Fatal("monsterEnhanceSetPatchValueEnabledLocked source block was not found")
	}
	block := body[start : start+end]
	verifyAt := strings.Index(block, "verifyRuntimePatchExecutableLocked")
	prepareAt := strings.Index(block, "prepareMonsterEnhanceEnable")
	injectAt := strings.Index(block, "extractAndInjectPatchCore")
	if verifyAt < 0 || prepareAt < 0 || injectAt < 0 || verifyAt > prepareAt || verifyAt > injectAt {
		t.Fatal("monster enhancement must verify the exact 2.0.2 executable before preparing writes or injecting patch_core")
	}
}
