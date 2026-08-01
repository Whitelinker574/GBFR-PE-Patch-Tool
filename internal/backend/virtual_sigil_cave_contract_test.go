package backend

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func virtualSigilNativeFunction(t *testing.T, source, name string) string {
	t.Helper()
	pattern := regexp.MustCompile(`(?s)static\s+[^\n]+\s+` + regexp.QuoteMeta(name) + `\([^)]*\)\s*\{(.*?)\n\}`)
	match := pattern.FindStringSubmatch(source)
	if len(match) != 2 {
		t.Fatalf("native function %s was not found", name)
	}
	return match[1]
}

func TestVirtualSigilTraitFetchCaveDoesNotCopyRelativeBranch(t *testing.T) {
	raw, err := os.ReadFile("../../src_dll/patch_core/dllmain.cpp")
	if err != nil {
		t.Fatal(err)
	}
	body := virtualSigilNativeFunction(t, string(raw), "InstallVirtualTraitFetchHook")
	if regexp.MustCompile(`memcpy\s*\(\s*code\s*\+\s*i\s*,\s*kTraitFetchOriginal`).MatchString(body) {
		t.Fatal("virtual-sigil trait-fetch cave copies the original short relative JE verbatim; in the real 2.0.3 crash dump JE +0x3E landed in zero-filled cave memory and raised 0xC0000005")
	}
	if strings.Count(body, "callPath") < 2 {
		t.Fatal("virtual-sigil trait-fetch cave must route both virtual slots and the relocated native bl==0 branch to the original call path")
	}
}
