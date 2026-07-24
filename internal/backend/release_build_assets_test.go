package backend

import (
	"bytes"
	"debug/pe"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const patchCoreResourcePath = "resources/patch_core.dll"

func TestPatchCoreEmbedUsesStableResource(t *testing.T) {
	source, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatalf("read app.go: %v", err)
	}

	directive := regexp.MustCompile(`(?m)^//go:embed\s+(\S+)\s*$`).FindSubmatch(source)
	if len(directive) != 2 {
		t.Fatal("app.go must contain one patch_core go:embed directive")
	}
	if got := filepath.ToSlash(string(directive[1])); got != patchCoreResourcePath {
		t.Fatalf("patch_core embed source = %q, want stable resource %q outside Wails build output", got, patchCoreResourcePath)
	}
}

func TestPatchCoreResourceMatchesEmbeddedAMD64DLL(t *testing.T) {
	resource, err := os.ReadFile(filepath.FromSlash(patchCoreResourcePath))
	if err != nil {
		t.Fatalf("read stable patch_core resource: %v", err)
	}
	if !bytes.Equal(resource, patchCoreDLL) {
		t.Fatal("stable patch_core resource differs from bytes compiled into the Go application")
	}

	dll, err := pe.Open(filepath.FromSlash(patchCoreResourcePath))
	if err != nil {
		t.Fatalf("open stable patch_core resource as PE: %v", err)
	}
	defer dll.Close()
	if dll.FileHeader.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		t.Fatalf("patch_core PE machine = %#x, want AMD64 %#x", dll.FileHeader.Machine, pe.IMAGE_FILE_MACHINE_AMD64)
	}
	const imageFileDLL = 0x2000
	if dll.FileHeader.Characteristics&imageFileDLL == 0 {
		t.Fatalf("patch_core PE characteristics %#x do not mark a DLL", dll.FileHeader.Characteristics)
	}
}

func TestReleaseBatchUsesCleanWindowsAMD64Build(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "build-windows.bat"))
	if err != nil {
		t.Fatalf("read build-windows.bat: %v", err)
	}

	line := regexp.MustCompile(`(?mi)^\s*wails\s+build\b.*$`).Find(script)
	if line == nil {
		t.Fatal("build-windows.bat has no wails build command")
	}
	fields := strings.Fields(strings.ToLower(string(line)))
	if !containsField(fields, "-clean") {
		t.Fatalf("release build command %q must clean stale build output", strings.TrimSpace(string(line)))
	}
	if !containsField(fields, "-s") {
		t.Fatalf("release build command %q must reuse the explicitly built frontend", strings.TrimSpace(string(line)))
	}
	if !containsAdjacentFields(fields, "-platform", "windows/amd64") {
		t.Fatalf("release build command %q must pin platform windows/amd64", strings.TrimSpace(string(line)))
	}
}

func TestPatchCoreProjectPublishesStableResource(t *testing.T) {
	project, err := os.ReadFile(filepath.Join("..", "..", "src_dll", "patch_core", "patch_core.vcxproj"))
	if err != nil {
		t.Fatalf("read patch_core project: %v", err)
	}
	normalized := strings.ToLower(filepath.ToSlash(string(project)))
	if !strings.Contains(normalized, `$(projectdir)../../internal/backend/resources/`) {
		t.Fatal("patch_core Release x64 post-build output must publish to the stable resources directory")
	}
	if strings.Contains(normalized, `$(solutiondir)`) || strings.Contains(normalized, `$(projectdir)../../build/bin/`) {
		t.Fatal("patch_core project must not publish an embed input into Wails' disposable build/bin directory")
	}
	if !strings.Contains(normalized, `../thirdparty/libmem/lib/debug`) {
		t.Fatal("patch_core Debug x64 must link the bundled debug libmem library")
	}
}

func TestPatchCoreSourceClosesVerifiedMonsterSafetyIssues(t *testing.T) {
	sourceBytes, err := os.ReadFile(filepath.Join("..", "..", "src_dll", "patch_core", "dllmain.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, required := range []string{
		`if (!g_damageMeter)`,
		`strcmp(patchId, "all") == 0`,
		`batch patch id is unsupported`,
		`InstallPlayerPointerHook`,
		`StampMonsterCave(cave, 96`,
		`StampMonsterCave(cave, 192`,
		`0x1FBDEB4`,
		`0xB29128`,
		`0x22CB316`,
	} {
		if !strings.Contains(source, required) {
			t.Errorf("patch_core source missing monster safety guard %q", required)
		}
	}
	for _, removed := range []string{`PatchCrocodileDamageHook`, `0x23FD449`, `0xAA1539`, `0xA09ADF`, `0x1F7123F`} {
		if strings.Contains(source, removed) {
			t.Errorf("patch_core source still carries the retired monster implementation %q", removed)
		}
	}
}

func containsField(fields []string, want string) bool {
	for _, field := range fields {
		if field == want {
			return true
		}
	}
	return false
}

func containsAdjacentFields(fields []string, first, second string) bool {
	for index := 0; index+1 < len(fields); index++ {
		if fields[index] == first && fields[index+1] == second {
			return true
		}
	}
	return false
}
