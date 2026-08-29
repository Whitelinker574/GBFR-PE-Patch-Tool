package backend

import (
	"bytes"
	"encoding/binary"
	"os"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWeaponMemoryScanVerifiesExecutableBeforeChoosingVersionLayout(t *testing.T) {
	source, err := os.ReadFile("weapon_memory.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	start := strings.Index(body, "func (a *App) scanWeaponMemoryLocked()")
	end := strings.Index(body[start:], "\nfunc (a *App) WeaponMemoryGetStatus")
	if start < 0 || end < 0 {
		t.Fatal("scanWeaponMemoryLocked body not found")
	}
	body = body[start : start+end]
	verify := strings.Index(body, "verifyRuntimePatchExecutableLocked")
	resolve := strings.Index(body, "resolveWeaponMemoryHookLocked")
	if verify < 0 || resolve < 0 || verify > resolve {
		t.Fatalf("weapon scan must verify the executable before choosing a versioned or AOB entry")
	}
}

func TestWeaponMemoryLocatorFallsBackToUniqueFullGuard(t *testing.T) {
	image := make([]byte, 4096)
	image[0], image[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(image[0x3C:0x40], 0x80)
	copy(image[0x80:0x84], []byte{'P', 'E', 0, 0})
	binary.LittleEndian.PutUint32(image[0xD0:0xD4], uint32(len(image)))
	const relocatedOffset = 0x280
	copy(image[relocatedOffset:], weaponMemoryGuardBytes)
	base := uintptr(unsafe.Pointer(&image[0]))
	app := &App{hProcess: windows.CurrentProcess(), moduleBase: base, runtimePatchVerifiedDigest: game205ExecutableSHA256}

	got, err := app.resolveWeaponMemoryHookLocked()
	runtime.KeepAlive(image)
	if err != nil {
		t.Fatalf("relocated unique weapon guard was rejected: %v", err)
	}
	if want := base + relocatedOffset; got != want {
		t.Fatalf("weapon hook=0x%X, want relocated guard 0x%X", got, want)
	}
}

func TestWeaponMemory203SignatureAndPhysicalOffsetsAreExact(t *testing.T) {
	if weaponMemoryHookRVA != 0x415118C {
		t.Fatalf("weapon current-view RVA = 0x%X, want 0x415118C", weaponMemoryHookRVA)
	}
	wantGuard := []byte{
		0x48, 0x89, 0xD7, 0x48, 0x89, 0xCE, 0x83, 0x7A, 0x40, 0x00, 0x7E, 0x67,
		0x48, 0x8B, 0x4E, 0x50, 0x48, 0x85, 0xC9, 0x74, 0x07, 0xB2, 0x01,
	}
	if !bytes.Equal(weaponMemoryGuardBytes, wantGuard) || !isWeaponMemoryGuard(wantGuard, false) {
		t.Fatalf("weapon 2.0.3 guard = % X, want % X", weaponMemoryGuardBytes, wantGuard)
	}
	for i := range wantGuard {
		mutated := append([]byte(nil), wantGuard...)
		mutated[i] ^= 1
		if isWeaponMemoryGuard(mutated, false) {
			t.Fatalf("strict 2.0.3 guard accepted mismatch at byte %d", i)
		}
	}
	if weaponMemorySkillWindowOffset != 0xA4 || weaponMemorySkillWindowSize != 0x28 {
		t.Fatalf("weapon skill window = +0x%X/%d, want +0xA4/0x28", weaponMemorySkillWindowOffset, weaponMemorySkillWindowSize)
	}
}

func TestBuildWeaponMemoryCavePreservesR10AndCapturesRDX(t *testing.T) {
	cave := uintptr(0x140520000)
	returnAddr := uintptr(0x141151192)
	code, err := buildWeaponMemoryCave(cave, returnAddr, weaponMemoryOriginalBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(code[:4], []byte{0x41, 0x52, 0x49, 0xBA}) {
		t.Fatalf("code cave does not preserve r10: % X", code[:4])
	}
	if got := uintptr(binary.LittleEndian.Uint64(code[4:12])); got != cave+weaponMemoryCaveDataOffset {
		t.Fatalf("capture address = 0x%X, want 0x%X", got, cave+weaponMemoryCaveDataOffset)
	}
	if !bytes.Equal(code[12:17], []byte{0x49, 0x89, 0x12, 0x41, 0x5A}) {
		t.Fatalf("code cave must capture rdx then restore r10: % X", code[12:17])
	}
	original, err := decodeWeaponMemoryCave(cave, code)
	if err != nil || !bytes.Equal(original, weaponMemoryOriginalBytes) {
		t.Fatalf("owned weapon cave should decode: original=% X err=%v", original, err)
	}
	jumpOffset := weaponMemoryOriginalOffset + weaponMemoryHookSize
	if got := relJumpTarget(cave+jumpOffset, code[jumpOffset:jumpOffset+5]); got != returnAddr {
		t.Fatalf("return jump target = 0x%X, want 0x%X", got, returnAddr)
	}
}

func TestReadWeaponMemoryStatusUsesVerifiedRecordOffsets(t *testing.T) {
	hProcess, page := allocateSigilMemoryTestPage(t)
	hook := page
	cave := page + 0x100
	recordAddr := page + 0x500
	code, err := buildWeaponMemoryCave(cave, hook+weaponMemoryHookSize, weaponMemoryOriginalBytes)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := makeRelJump(hook, cave, int(weaponMemoryHookSize))
	if err != nil {
		t.Fatal(err)
	}
	record := make([]byte, weaponMemoryRecordSize)
	binary.LittleEndian.PutUint32(record[0x04:0x08], 0xAABBCCDD)
	binary.LittleEndian.PutUint32(record[0x58:0x5C], 150)
	for index := 0; index < weaponMemoryPhysicalSlotCount; index++ {
		offset := int(weaponMemorySkillWindowOffset) + index*8
		binary.LittleEndian.PutUint32(record[offset:offset+4], uint32(index+1))
		binary.LittleEndian.PutUint32(record[offset+4:offset+8], uint32(10+index))
	}
	binary.LittleEndian.PutUint64(code[weaponMemoryCaveDataOffset:weaponMemoryCaveDataOffset+8], uint64(recordAddr))
	writeSigilMemoryTestBytes(t, hProcess, cave, code)
	writeSigilMemoryTestBytes(t, hProcess, hook, patch)
	writeSigilMemoryTestBytes(t, hProcess, recordAddr, record)

	app := &App{
		hProcess: hProcess, moduleBase: hook - weaponMemoryHookRVA,
		weaponMemoryHookAddr: hook, weaponMemoryCaveAddr: cave,
		weaponMemoryOriginal: append([]byte(nil), weaponMemoryOriginalBytes...),
	}
	status, err := app.readWeaponMemoryStatusLocked()
	if err != nil {
		t.Fatal(err)
	}
	if status.SelectedAddr != uint64(recordAddr) || status.WeaponID != 0xAABBCCDD || status.WeaponLevel != 150 {
		t.Fatalf("weapon identity fields decoded from wrong offsets: %+v", status)
	}
	if len(status.Skills) != weaponMemoryPhysicalSlotCount {
		t.Fatalf("skills = %d, want %d physical slots", len(status.Skills), weaponMemoryPhysicalSlotCount)
	}
	for index, skill := range status.Skills {
		if skill.Index != index || skill.Hash != uint32(index+1) || skill.Level != uint32(10+index) {
			t.Fatalf("skill %d decoded from wrong offsets: %+v", index+1, skill)
		}
	}
}

func TestReleaseWeaponMemoryHookRetainsUnprovenRecoveryLease(t *testing.T) {
	app := &App{
		weaponMemoryCaveAddr: 0x12340000,
		weaponMemoryOriginal: append([]byte(nil), weaponMemoryOriginalBytes...),
	}
	if err := app.releaseWeaponMemoryHookLocked(); err == nil {
		t.Fatal("release silently accepted a weapon cave lease without a live entry/handle")
	}
	if app.weaponMemoryCaveAddr != 0x12340000 || !bytes.Equal(app.weaponMemoryOriginal, weaponMemoryOriginalBytes) {
		t.Fatalf("failed release discarded recovery lease: cave=0x%X original=% X", app.weaponMemoryCaveAddr, app.weaponMemoryOriginal)
	}
}

func TestWeaponMemoryRecoveryStateParticipatesInSharedRuntimeLease(t *testing.T) {
	app := &App{weaponMemoryCaveAddr: 0x140520000, weaponMemoryOwnerToken: "weapon-current"}
	if !app.hasActiveRuntimeHookLeaseLocked() {
		t.Fatal("shared detach would ignore weapon hook recovery state")
	}
	if got := app.runtimeOwnerTokenLocked(runtimeOwnerWeapon); got != "weapon-current" {
		t.Fatalf("weapon owner token = %q, want weapon-current", got)
	}
}

func TestWeaponMemoryOptionsCoverVerifiedWeaponTableTraits(t *testing.T) {
	previous := getCurrentLanguage()
	defer setCurrentLanguage(previous)
	setCurrentLanguage("zh")

	options, err := (&App{}).WeaponMemoryGetOptions()
	if err != nil {
		t.Fatal(err)
	}
	byHash := make(map[uint32]WeaponMemoryOption, len(options.Traits))
	for _, option := range options.Traits {
		byHash[option.Hash] = option
	}
	data, err := loadLoadoutWeaponStats()
	if err != nil {
		t.Fatal(err)
	}
	for text, traitID := range data.TraitIDs {
		hash, err := ParseHashHex(text)
		if err != nil || isEmptyWeaponMemorySkill(hash) {
			continue
		}
		if _, unused := weaponMemoryUnusedTraitHashes[hash]; unused {
			continue
		}
		option, ok := byHash[hash]
		if !ok {
			t.Errorf("verified weapon trait %s (%s) is missing from live editor candidates", text, traitID)
			continue
		}
		if option.DisplayName == "" || option.DisplayName == "未收录词条" {
			t.Errorf("verified weapon trait %s (%s) has no resolved Chinese name", text, traitID)
		}
	}
}

func TestWeaponMemoryObservedUnboundTechniqueHashIsNamedInBothLanguages(t *testing.T) {
	previous := getCurrentLanguage()
	defer setCurrentLanguage(previous)

	const observedHash = uint32(0x02D0B733)
	window := make([]byte, weaponMemorySkillWindowSize)
	binary.LittleEndian.PutUint32(window[0:4], observedHash)
	binary.LittleEndian.PutUint32(window[4:8], 15)

	for _, test := range []struct {
		language string
		wantName string
	}{
		{language: "zh", wantName: "超凡技艺"},
		{language: "en", wantName: "Unbound Technique"},
	} {
		setCurrentLanguage(test.language)
		options, err := (&App{}).WeaponMemoryGetOptions()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, option := range options.Traits {
			if option.Hash == observedHash {
				found = true
				if option.DisplayName != test.wantName {
					t.Errorf("%s candidate name = %q, want %q", test.language, option.DisplayName, test.wantName)
				}
				joined := strings.Join(option.SearchTerms, "|")
				for _, want := range []string{"020DB733", "0x020DB733", "02D0B733", "0x02D0B733", test.wantName} {
					if !strings.Contains(joined, want) {
						t.Errorf("%s candidate search terms %q do not contain %q", test.language, joined, want)
					}
				}
			}
		}
		if !found {
			t.Errorf("%s candidates do not include observed runtime hash %08X", test.language, observedHash)
		}
		decoded := decodeWeaponMemorySkills(window)
		if len(decoded) == 0 || decoded[0].Name != test.wantName {
			t.Errorf("%s decoded observed skill = %+v, want name %q", test.language, decoded, test.wantName)
		}
	}
}
