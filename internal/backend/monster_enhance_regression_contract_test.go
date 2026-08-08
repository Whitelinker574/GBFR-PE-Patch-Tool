package backend

import (
	"os"
	"strings"
	"testing"
)

func TestMonsterDamageNewUsesTrueDamageAndPreservesGameEdgeWrites(t *testing.T) {
	source := readPatchCoreSource(t)
	body := cppFunctionBody(t, source, "static bool PatchMonsterDamageNewHook")
	for _, required := range []string{
		"mov r11,r13",
		"cmp rdx,1",
		"jbe restore (0 or 1: game edge value)",
		"xor r9,r9",
		"R13<=0",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("PatchMonsterDamageNewHook is missing %q", required)
		}
	}
}

func TestOdRateAlsoHooksInlineBossPath(t *testing.T) {
	source := readPatchCoreSource(t)
	for _, required := range []string{
		"kOdRateInlineExpected",
		"PatchOdRateHookInline",
		"0x2B3F77E",
		"od gauge rate (inline)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("inline OD path is missing %q", required)
		}
	}
}

func readPatchCoreSource(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../src_dll/patch_core/dllmain.cpp")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func cppFunctionBody(t *testing.T, source, signature string) string {
	t.Helper()
	start := strings.Index(source, signature)
	if start < 0 {
		t.Fatalf("missing C++ function %q", signature)
	}
	next := strings.Index(source[start+len(signature):], "\nstatic bool ")
	if next < 0 {
		return source[start:]
	}
	return source[start : start+len(signature)+next]
}
