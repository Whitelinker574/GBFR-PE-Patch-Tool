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
const patchCoreReleaseSizeLimit = 10 * 1024 * 1024

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
	if len(resource) > patchCoreReleaseSizeLimit {
		t.Fatalf("patch_core resource size = %d bytes, limit = %d", len(resource), patchCoreReleaseSizeLimit)
	}
	for _, leakedBuildPath := range [][]byte{
		[]byte(`GBFR-Codex-Field-Lab`),
		[]byte(`source-git\src_dll\patch_core`),
	} {
		if bytes.Contains(resource, leakedBuildPath) {
			t.Fatalf("patch_core resource leaks local build path segment %q", leakedBuildPath)
		}
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

func TestReleaseBatchUsesGuardedCleanWindowsAMD64Build(t *testing.T) {
	batch, err := os.ReadFile(filepath.Join("..", "..", "build-windows.bat"))
	if err != nil {
		t.Fatalf("read build-windows.bat: %v", err)
	}
	batchSource := strings.ToLower(string(batch))
	if !strings.Contains(batchSource, `tools\build_windows.ps1`) ||
		!strings.Contains(batchSource, `exit /b %build_exit%`) {
		t.Fatal("build-windows.bat must delegate the complete guarded build and preserve its exit code")
	}
	if strings.Contains(batchSource, "cd /d") || strings.Contains(batchSource, "copy /b") {
		t.Fatal("batch wrapper must not bypass PowerShell rollback with its own directory or copy control flow")
	}

	script, err := os.ReadFile(filepath.Join("..", "..", "tools", "build_windows.ps1"))
	if err != nil {
		t.Fatalf("read guarded Windows build script: %v", err)
	}
	source := string(script)
	for _, required := range []string{
		"-Arguments @('build', '-clean', '-platform', 'windows/amd64', '-s')",
		"Enter-GBFRExclusiveLease",
		"Remove-GBFRStaleNamedPaths",
		"Invoke-GBFRPendingPatchCoreRecovery",
		"Start-GBFRPatchCoreRecoveryTransaction",
		"Complete-GBFRPatchCoreRecoveryTransaction",
		"Rollback-GBFRPatchCoreRecoveryTransaction",
		"Push-Location -LiteralPath",
		"atomically restored and hash-verified",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("guarded Windows build is missing %q", required)
		}
	}
}

func TestReleasePackagerIncludesThirdPartyNoticesAndNativeLicenses(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "tools", "package_windows_release.ps1"))
	if err != nil {
		t.Fatalf("read release packager: %v", err)
	}
	source := string(script)
	for _, required := range []string{
		"THIRD_PARTY_NOTICES.md",
		`src_dll\thirdparty\libmem\licenses\*.txt`,
		"RELEASE_NOTES_$Version.md",
		"BUILD_PROVENANCE.json",
		"status --porcelain=v1 --untracked-files=all",
		`tools\build_windows.ps1`,
		"rev-parse HEAD",
		"Compress-Archive",
		"Get-FileHash -Algorithm SHA256",
		"Release output must be outside the Git repository",
		".partial-",
		"Enter-GBFRExclusiveLease",
		"Start-GBFRPatchCoreRecoveryTransaction",
		"Complete-GBFRPatchCoreRecoveryTransaction",
		"Rollback-GBFRPatchCoreRecoveryTransaction",
		"InheritedBuildLease",
		"Remove-GBFRStaleNamedPaths",
		"$partialNamePattern",
		"$finalHead",
		"Release inputs or executable copies changed after provenance was recorded",
		"$preserveTemporaryOutput",
		"[System.IO.Directory]::Move($temporaryOutput, $outputPath)",
		"Remove-Item -LiteralPath $temporaryOutput -Recurse -Force",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("release packager does not include %q", required)
		}
	}
	licenses, err := filepath.Glob(filepath.Join("..", "..", "src_dll", "thirdparty", "libmem", "licenses", "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(licenses) != 7 {
		t.Fatalf("native license count = %d, want 7", len(licenses))
	}
}

func TestPatchCoreBuildPublishesStableResourceAtomically(t *testing.T) {
	project, err := os.ReadFile(filepath.Join("..", "..", "src_dll", "patch_core", "patch_core.vcxproj"))
	if err != nil {
		t.Fatalf("read patch_core project: %v", err)
	}
	normalized := strings.ToLower(filepath.ToSlash(string(project)))
	if strings.Contains(normalized, `internal/backend/resources`) || strings.Contains(normalized, `<postbuildevent>`) {
		t.Fatal("MSBuild must not copy directly over the tracked embedded DLL")
	}
	if strings.Count(normalized, `<platformtoolset>v143</platformtoolset>`) != 4 || strings.Contains(normalized, `<platformtoolset>v145</platformtoolset>`) {
		t.Fatal("all patch_core configurations must use the installed Visual Studio 2022 v143 toolset")
	}
	if !strings.Contains(normalized, `../thirdparty/libmem/lib/debug`) {
		t.Fatal("patch_core Debug x64 must link the bundled debug libmem library")
	}
	releaseX64 := regexp.MustCompile(`(?s)<itemdefinitiongroup\s+condition="'\$\(configuration\)\|\$\(platform\)'=='release\|x64'">.*?</itemdefinitiongroup>`).FindString(normalized)
	if releaseX64 == "" || !strings.Contains(releaseX64, `<generatedebuginformation>false</generatedebuginformation>`) {
		t.Fatal("patch_core Release x64 must omit CodeView/PDB paths from the distributed DLL")
	}
	buildScript, err := os.ReadFile(filepath.Join("..", "..", "tools", "build_patch_core.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(buildScript)
	for _, required := range []string{
		"windows_release_common.ps1",
		"Enter-GBFRExclusiveLease",
		"InheritedBuildLease",
		"Assert-GBFRBuildLeaseHeld",
		"Remove-GBFRStaleNamedPaths",
		"Invoke-GBFRPendingPatchCoreRecovery",
		"Start-GBFRPatchCoreRecoveryTransaction",
		"Rollback-GBFRPatchCoreRecoveryTransaction",
		"Copy-GBFRVerifiedFile",
		"[GBFRAtomicFilePublisher]::Replace($embeddedTemporary, $embedded)",
		"Get-GBFRSha256Hex -LiteralPath $embedded",
		"finally",
	} {
		if !strings.Contains(script, required) {
			t.Errorf("native build publisher is missing %q", required)
		}
	}
	if !strings.Contains(releaseX64, `<additionaloptions>/brepro %(additionaloptions)</additionaloptions>`) {
		t.Fatal("patch_core Release x64 must use reproducible linker output")
	}
	common, err := os.ReadFile(filepath.Join("..", "..", "tools", "windows_release_common.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"MoveFileEx",
		"MoveFileWriteThrough",
		"[System.IO.FileShare]::None",
		"[System.IO.FileOptions]::DeleteOnClose",
		"Restore-GBFRFileFromVerifiedBackup",
		"Atomically restored file failed read-back verification",
		"Write-GBFRDurableJsonFileAtomic",
		"Sync-GBFRFile",
		"ownerStartedUtc",
		"Invoke-GBFRPendingPatchCoreRecovery",
	} {
		if !strings.Contains(string(common), required) {
			t.Errorf("Windows build safety helper is missing %q", required)
		}
	}

	safetyTest, err := os.ReadFile(filepath.Join("..", "..", "tools", "test_windows_release_safety.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"A concurrent build lease was not rejected",
		"Formal patch_core.dll was removed by stale cleanup",
		"Formal release directory was removed by stale cleanup",
		"A corrupted rollback backup was accepted",
		"Interrupted-build recovery did not restore the original formal DLL",
		"Completed interrupted-build recovery retained its live journal",
	} {
		if !strings.Contains(string(safetyTest), required) {
			t.Errorf("Windows release safety regression test is missing %q", required)
		}
	}
}

func TestPatchCoreSourceClosesVerifiedMonsterSafetyIssues(t *testing.T) {
	sourceBytes, err := os.ReadFile(filepath.Join("..", "..", "src_dll", "patch_core", "dllmain.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, required := range []string{
		`strcmp(patchId, "all") == 0`,
		`batch patch id is unsupported`,
		`InstallPlayerPointerHook`,
		`StampMonsterCave(cave, 96`,
		`StampMonsterCave(cave, 192`,
		`0x1FBDEB4`,
		`0xB29128`,
		`0x22CB316`,
		`const lm_address_t candidates[] = { 0x356621, 0x34F8F1 }`,
		`41 01 76 04 4C 89 E1 E8 ?? ?? ?? ?? 41 8B 0C 24 31 C0 85 C9 0F 4F C1`,
		`kStableReleaseCandidateMonsterDamageEnabled = true`,
		`kStableReleaseVirtualSigilsEnabled = true`,
	} {
		if !strings.Contains(source, required) {
			t.Errorf("patch_core source missing monster safety guard %q", required)
		}
	}
	for _, removed := range []string{
		`PatchCrocodileDamageHook`, `0x23FD449`, `0xAA1539`, `0xA09ADF`, `0x1F7123F`,
		`DamageMeterState`, `AppendTeamDamageFromRcXEdxR8`, `GBFRPlayerInfoEditDamageMeter`,
	} {
		if strings.Contains(source, removed) {
			t.Errorf("patch_core source still carries the retired monster implementation %q", removed)
		}
	}
}

func TestPatchCoreOverdriveHookPreservesOriginalBranchAndR11(t *testing.T) {
	sourceBytes, err := os.ReadFile(filepath.Join("..", "..", "src_dll", "patch_core", "dllmain.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	start := strings.Index(source, "static bool PatchOverdriveHook")
	if start < 0 {
		t.Fatal("PatchOverdriveHook source block was not found")
	}
	end := strings.Index(source[start:], "static bool PatchInventorySetQuantityHook")
	if end < 0 {
		t.Fatal("PatchOverdriveHook source block was not found")
	}
	body := source[start : start+end]
	for _, required := range []string{
		`code[i++] = 0x41; code[i++] = 0x53; // push r11`,
		`code[i++] = 0x41; code[i++] = 0x5B; // pop r11`,
		`target + 6`,
		`lm_byte_t jmp[6]{ 0xE9 }`,
	} {
		if !strings.Contains(body, required) {
			t.Errorf("overdrive hook is missing %q", required)
		}
	}
	if strings.Contains(body, `lm_byte_t jmp[sizeof(kOverdriveExpected)]`) {
		t.Fatal("overdrive entry still overwrites the original conditional branch")
	}
	if strings.Index(body, "push r11") > strings.Index(body, "pop r11") {
		t.Fatal("overdrive auto path restores r11 before saving it")
	}
}

func TestCodePatchPublishersSuspendAndResumeTargetExecution(t *testing.T) {
	cppBytes, err := os.ReadFile(filepath.Join("..", "..", "src_dll", "patch_core", "dllmain.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	cpp := string(cppBytes)
	for _, required := range []string{
		"ScopedOtherThreadSuspension", "SuspendThread(", "ResumeThread(",
		"suspension.Active() && PatchBytesWhileSuspended", "InstructionPointersOutside", "RetireQOLLevelCaves",
	} {
		if !strings.Contains(cpp, required) {
			t.Errorf("patch_core code publisher is missing %q", required)
		}
	}
	goBytes, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	goSource := string(goBytes)
	for _, required := range []string{"suspendRemoteProcessForCodeWrite(h)", `NewProc("NtSuspendProcess")`, `NewProc("NtResumeProcess")`} {
		if !strings.Contains(goSource, required) {
			t.Errorf("Go code publisher is missing %q", required)
		}
	}
}

func TestRetiredDamageOverlayBackendIsRemoved(t *testing.T) {
	if _, err := os.Stat("damage_overlay_windows.go"); !os.IsNotExist(err) {
		t.Fatalf("retired damage overlay backend still exists: %v", err)
	}
	app, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, retired := range []string{"DamageMeterGetStatus", "DamageMeterReset", "damageOverlay", "damageMeterMapping"} {
		if strings.Contains(string(app), retired) {
			t.Errorf("app.go still contains retired damage runtime symbol %q", retired)
		}
	}
}
