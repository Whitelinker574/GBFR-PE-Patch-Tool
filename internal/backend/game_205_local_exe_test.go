package backend

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

const (
	runtimePatchLocalGame205SHA256 = "7189B958FF0FE5238CEA28A2939FFDAD6E3A9ACB14DD274A9FCC8E7E275BD175"
	runtimePatchLocalGame205Size   = int64(123517408)
)

type runtimePatchLocalSectionMemory struct {
	moduleBase uintptr
	sections   []runtimePatchLocalExecutableSection
}

func TestGame205UnpackedTablesMatchEmbeddedDropInputs(t *testing.T) {
	root := os.Getenv("GBFR_GAME_TABLES_205_TEST")
	if root == "" {
		t.Skip("set GBFR_GAME_TABLES_205_TEST to the extracted 2.0.5 system/table directory")
	}
	type requiredTable struct {
		Name   string
		Size   int
		SHA256 string
	}
	required := make([]requiredTable, 0, 11)
	appendRows := func(rows []struct {
		Name   string
		Size   int
		SHA256 string
	}) {
		for _, row := range rows {
			required = append(required, requiredTable(row))
		}
	}
	appendRows(naturalDropRequiredTables)
	appendRows(naturalWrightstoneRequiredTables)
	appendRows(naturalSigilRequiredTables)
	appendRows(naturalDropItemRequiredTables)
	if len(required) != 11 {
		t.Fatalf("embedded natural-drop table coverage=%d, want 11", len(required))
	}
	for _, table := range required {
		path := filepath.Join(root, table.Name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s: %v", table.Name, err)
			continue
		}
		digest, err := naturalDropFileSHA256(path)
		if err != nil {
			t.Errorf("%s hash: %v", table.Name, err)
			continue
		}
		if info.Size() != int64(table.Size) || digest != table.SHA256 {
			t.Errorf("%s size/hash=%d/%s, want %d/%s", table.Name, info.Size(), digest, table.Size, table.SHA256)
		}
	}
	changed := []struct {
		name   string
		size   int64
		sha256 string
	}{{"reward_point.tbl", 273416, "ACA6D0A9EB82CFAC919C712C0B626AD2F31C86494B3F209CF70DB9C0F859ED02"},
		{"skillboard_effect_action_parts.tbl", 396448, "16CCD097882DCC096DB602FFD304BBE1526D5E8647AF6BEA1A3526F236800713"}}
	for _, table := range changed {
		path := filepath.Join(root, table.name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		digest, err := naturalDropFileSHA256(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() != table.size || digest != table.sha256 {
			t.Errorf("2.0.5 changed table %s size/hash=%d/%s", table.name, info.Size(), digest)
		}
	}
}

func TestGame205NativeCompanionSignaturesAreUnique(t *testing.T) {
	_, sections := loadLocalGame205Sections(t)
	source, err := os.ReadFile("../../src_dll/patch_core/dllmain.cpp")
	if err != nil {
		t.Fatal(err)
	}
	literal := regexp.MustCompile(`"((?:[0-9A-F?]{2} ){4,}[0-9A-F?]{2})"`)
	unique := map[string]struct{}{}
	for _, match := range literal.FindAllSubmatch(source, -1) {
		unique[string(match[1])] = struct{}{}
	}
	patterns := make([]string, 0, len(unique))
	for raw := range unique {
		patterns = append(patterns, raw)
	}
	sort.Strings(patterns)
	for _, raw := range patterns {
		pattern, err := parseRuntimePatchPattern(raw)
		if err != nil {
			t.Fatalf("parse native signature %q: %v", raw, err)
		}
		matches := findRuntimePatchLocalPatternMatches(sections, pattern)
		if len(matches) != 1 {
			t.Errorf("native signature %q matches=%s", raw, formatRuntimePatchLocalMatchLocations(matches))
		}
	}
	if len(patterns) < 30 {
		t.Fatalf("native signature coverage=%d, want at least 30", len(patterns))
	}
	t.Logf("verified %d native companion AOB literals", len(patterns))
}

func TestGame205CorePatterns(t *testing.T) {
	_, sections := loadLocalGame205Sections(t)
	layout := runtimeGameLayouts[3]
	checks := []struct {
		name     string
		aob      string
		rva      uintptr
		original []byte
	}{
		{name: "party pointer", aob: runtimePatchPartyPointerAOB, rva: layout.PartyPointerRVA},
		{name: "selected material", aob: runtimePatchSelectedMaterialAOB, rva: layout.SelectedMaterialRVA, original: runtimePatchSelectedMaterialOriginal},
		{name: "selected key item", aob: runtimePatchSelectedKeyItemAOB, rva: layout.SelectedKeyItemRVA, original: runtimePatchSelectedKeyItemOriginal},
		{name: "inventory material", aob: runtimeInventoryMaterialAOB, rva: layout.InventoryMaterialRVA},
		{name: "sigil hook", aob: runtimeSigilHookAOB, rva: layout.SigilHookRVA, original: sigilMemoryOriginalBytes},
		{name: "wrightstone hook", aob: runtimeWrightstoneHookAOB, rva: layout.WrightstoneHookRVA, original: wrightstoneMemoryOriginalBytes},
		{name: "save function", aob: runtimeItemSaveFunctionAOB, rva: layout.SaveFunctionRVA, original: gameSaveFunctionPrologue},
	}
	for _, check := range checks {
		pattern, err := parseRuntimePatchPattern(check.aob)
		if err != nil {
			t.Fatal(err)
		}
		matches := findRuntimePatchLocalPatternMatches(sections, pattern)
		if len(matches) != 1 || uintptr(matches[0].rva) != check.rva {
			t.Errorf("%s matches=%s, want RVA 0x%X", check.name, formatRuntimePatchLocalMatchLocations(matches), check.rva)
			continue
		}
		match := matches[0]
		if len(check.original) != 0 {
			actual := runtimePatchLocalBytesAtRVA(sections, match.rva, len(check.original))
			if !bytes.Equal(actual, check.original) {
				t.Errorf("%s original=% X, prior=% X", check.name, actual, check.original)
			}
		}
		if check.name == "party pointer" {
			entry := runtimePatchLocalBytesAtRVA(sections, match.rva, 7)
			resolved := int64(match.rva) + 7 + int64(int32(binary.LittleEndian.Uint32(entry[3:7])))
			if resolved != int64(layout.PartySlotTableRVA) {
				t.Errorf("party slot table RVA=0x%X, want 0x%X", resolved, layout.PartySlotTableRVA)
			}
		}
	}
}

func TestGame205RuntimeCatalog(t *testing.T) {
	_, sections := loadLocalGame205Sections(t)
	catalog, err := decodeRuntimePatchCatalog(runtimePatchCatalogJSON)
	if err != nil {
		t.Fatal(err)
	}
	matched := 0
	for _, feature := range catalog.Features {
		for siteIndex, catalogSite := range feature.Sites {
			site, err := runtimePatchSiteForExecutable(catalogSite, runtimePatchLocalGame205SHA256)
			if err != nil {
				t.Fatalf("%s site[%d]: resolve prior definition: %v", feature.ID, siteIndex, err)
			}
			pattern, err := parseRuntimePatchPattern(site.AOB)
			if err != nil {
				t.Fatalf("%s site[%d]: parse AOB: %v", feature.ID, siteIndex, err)
			}
			matches := findRuntimePatchLocalPatternMatches(sections, pattern)
			if len(matches) != 1 {
				t.Errorf("%s/%s site[%d]: matches=%s", feature.ID, site.Symbol, siteIndex, formatRuntimePatchLocalMatchLocations(matches))
				continue
			}
			matched++
			actual := runtimePatchLocalBytesAtRVA(sections, matches[0].rva+uint32(site.Offset), len(site.ExpectedOriginalBytes))
			if !bytes.Equal(actual, site.ExpectedOriginalBytes) {
				t.Errorf("%s/%s site[%d]: RVA 0x%X original=% X, prior=% X", feature.ID, site.Symbol, siteIndex, matches[0].rva+uint32(site.Offset), actual, site.ExpectedOriginalBytes)
			}
		}
	}
	if matched != 83 {
		t.Errorf("2.0.5 catalog matches=%d/83", matched)
	}
}

func exactRuntimePatchPattern(value []byte) runtimePatchPattern {
	mask := make([]byte, len(value))
	for index := range mask {
		mask[index] = 0xFF
	}
	return runtimePatchPattern{Values: append([]byte(nil), value...), Mask: mask}
}

func TestGame205StandalonePatterns(t *testing.T) {
	_, sections := loadLocalGame205Sections(t)
	logAOB := func(name, raw string, offset int, original []byte) uint32 {
		t.Helper()
		pattern, err := parseRuntimePatchPattern(raw)
		if err != nil {
			t.Fatal(err)
		}
		matches := findRuntimePatchLocalPatternMatches(sections, pattern)
		if len(matches) != 1 {
			t.Errorf("%s matches=%s", name, formatRuntimePatchLocalMatchLocations(matches))
			return 0
		}
		rva := uint32(int64(matches[0].rva) + int64(offset))
		t.Logf("%s RVA 0x%X", name, rva)
		if len(original) != 0 {
			actual := runtimePatchLocalBytesAtRVA(sections, rva, len(original))
			if !bytes.Equal(actual, original) {
				t.Errorf("%s original=% X, prior=% X", name, actual, original)
			}
		}
		return rva
	}
	for index, spec := range freeConsumptionSites {
		if got := uintptr(logAOB(fmt.Sprintf("free consumption %c", 'A'+rune(index)), spec.AOB, 0, spec.Original)); got != freeConsumption205RVAs[index] {
			t.Errorf("free consumption %c RVA=0x%X, want 0x%X", 'A'+rune(index), got, freeConsumption205RVAs[index])
		}
	}
	if got := uintptr(logAOB("task score", taskScoreAOB, 0, taskScoreOriginal)); got != 0x1FDA459 {
		t.Errorf("task score RVA=0x%X", got)
	}
	wantSideQuest := []uintptr{0xBAC90C, 0xBAC262}
	for index, spec := range taskSideQuestSpecs {
		if got := uintptr(logAOB(fmt.Sprintf("side quest %c", 'A'+rune(index)), spec.AOB, 0, spec.Original)); got != wantSideQuest[index] {
			t.Errorf("side quest %c RVA=0x%X, want 0x%X", 'A'+rune(index), got, wantSideQuest[index])
		}
	}
	if got := uintptr(logAOB("summon duration", summonDurationAOB, 0, summonDuration205Original)); got != summonDurationRVA {
		t.Errorf("summon duration RVA=0x%X, want 0x%X", got, summonDurationRVA)
	}
	wantCooldown := []uint32{0x21EA74D, 0x27FEF5E, 0x33D8B4A}
	for index, spec := range combatCooldownSpecs {
		pattern := runtimePatchPattern{Values: append([]byte(nil), spec.Pattern...), Mask: make([]byte, len(spec.Mask))}
		for maskIndex, exact := range spec.Mask {
			if exact {
				pattern.Mask[maskIndex] = 0xFF
			}
		}
		matches := findRuntimePatchLocalPatternMatches(sections, pattern)
		if len(matches) != 1 {
			t.Errorf("cooldown %c matches=%s", 'A'+rune(index), formatRuntimePatchLocalMatchLocations(matches))
		} else if matches[0].rva != wantCooldown[index] {
			t.Errorf("cooldown %c RVA=0x%X, want 0x%X", 'A'+rune(index), matches[0].rva, wantCooldown[index])
		}
	}
	for _, entry := range []struct {
		name string
		spec combatTuningSiteSpec
	}{{"charge", combatChargeSpec}, {"action speed", combatActionSpeedSpec}} {
		pattern := runtimePatchPattern{Values: append([]byte(nil), entry.spec.Pattern...), Mask: make([]byte, len(entry.spec.Mask))}
		for maskIndex, exact := range entry.spec.Mask {
			if exact {
				pattern.Mask[maskIndex] = 0xFF
			}
		}
		matches := findRuntimePatchLocalPatternMatches(sections, pattern)
		if len(matches) != 1 {
			t.Errorf("%s matches=%s", entry.name, formatRuntimePatchLocalMatchLocations(matches))
		} else {
			want := map[string]uint32{"charge": 0x279F180, "action speed": 0xBB1248}[entry.name]
			if matches[0].rva != want {
				t.Errorf("%s RVA=0x%X, want 0x%X", entry.name, matches[0].rva, want)
			}
		}
	}
	const confluxAOB = "41 B8 D0 07 00 00 31 D2 E8 ?? ?? ?? ?? 48 89 35 ?? ?? ?? ?? 41 B8 D0 2C 00 00 48 89 F1 31 D2 E8 ?? ?? ?? ??"
	confluxRVA := logAOB("Conflux manager initializer", confluxAOB, 0, nil)
	if confluxRVA != 0 {
		store := runtimePatchLocalBytesAtRVA(sections, confluxRVA+13, 7)
		target := int64(confluxRVA+20) + int64(int32(binary.LittleEndian.Uint32(store[3:])))
		if target != 0x7C222F8 {
			t.Errorf("Conflux manager global RVA=0x%X, want 0x7C222F8", target)
		}
	}
	taskRewardPattern := runtimePatchPattern{Values: append([]byte(nil), taskRewardMultiplierPattern...), Mask: make([]byte, len(taskRewardMultiplierMask))}
	for index, exact := range taskRewardMultiplierMask {
		if exact {
			taskRewardPattern.Mask[index] = 0xFF
		}
	}
	taskRewardMatches := findRuntimePatchLocalPatternMatches(sections, taskRewardPattern)
	if len(taskRewardMatches) != 1 {
		t.Errorf("task reward matches=%s", formatRuntimePatchLocalMatchLocations(taskRewardMatches))
	} else {
		entryRVA := uintptr(taskRewardMatches[0].rva)
		original := runtimePatchLocalBytesAtRVA(sections, taskRewardMatches[0].rva, taskRewardMultiplierHookSize)
		managerSlot, err := taskRewardMultiplierManagerSlot(entryRVA, original)
		if err != nil {
			t.Error(err)
		} else if entryRVA != 0x1FDBAE0 || managerSlot != 0x7034F00 {
			t.Errorf("task reward RVA/manager=0x%X/0x%X, want 0x1FDBAE0/0x7034F00", entryRVA, managerSlot)
		}
	}
	for _, entry := range []struct {
		name string
		data []byte
		rva  uint32
	}{{"weapon focus guard", weaponMemoryGuardBytes, 0x41528FC},
		{"spatial gravity context", runtimeSpatialGravityContext, 0x39DA360},
		{"spatial flight", runtimeSpatialFlightHookOriginal, 0xA63BE0},
		{"jump gate context", runtimeSpatialJumpGateContext, 0x1FA11C3}} {
		matches := findRuntimePatchLocalPatternMatches(sections, exactRuntimePatchPattern(entry.data))
		if len(matches) != 1 || matches[0].rva != entry.rva {
			t.Errorf("%s matches=%s, want RVA 0x%X", entry.name, formatRuntimePatchLocalMatchLocations(matches), entry.rva)
		}
	}
	for index, guard := range runtimeCharacterPanel205VersionGuards {
		got := runtimePatchLocalBytesAtRVA(sections, uint32(guard.RVA), len(guard.Bytes))
		if !bytes.Equal(got, guard.Bytes) {
			t.Errorf("character panel 2.0.5 guard %d RVA 0x%X=% X, want % X", index, guard.RVA, got, guard.Bytes)
		}
	}
	for _, entry := range []struct {
		name string
		aob  string
	}{{"jump check masked", "48 81 C1 70 01 00 00 31 FF 31 D2 41 B8 03 00 00 00 E8 ?? ?? ?? ?? 85 C0 74 96 48 8B 46 10"},
		{"character panel ready masked", "C6 44 24 38 00 4C 89 E1 4C 89 E2 E8 ?? ?? ?? ?? 41 C6 84 24 BC 5E 00 00 01"},
		{"character panel aggregate masked", "C5 FA 7E 4B 04 C5 E8 57 D2 C4 E2 71 3D CA C4 E2 71 3B 0D ?? ?? ?? ?? C5 F9 D6 4B 04 C5 FB 10 5B 10 C5 E8 5F D3 C5 FB 12 1D ?? ?? ?? ?? C5 E0 5D D2 C5 F8 13 53 10"}} {
		pattern, err := parseRuntimePatchPattern(entry.aob)
		if err != nil {
			t.Fatal(err)
		}
		matches := findRuntimePatchLocalPatternMatches(sections, pattern)
		want := map[string]uint32{"jump check masked": 0x1FA11E6, "character panel ready masked": 0x2D53BE, "character panel aggregate masked": 0xA23803}[entry.name]
		if len(matches) != 1 || matches[0].rva != want {
			t.Errorf("%s matches=%s, want RVA 0x%X", entry.name, formatRuntimePatchLocalMatchLocations(matches), want)
		}
	}
	for _, entry := range []struct {
		name string
		aob  string
	}{{"virtual apply", "FF C7 83 FF 0D 0F 84 ?? ?? ?? ?? C5 F8 11 75 F0"},
		{"virtual category", "49 FF C5 49 83 FD 0D 0F 84 ?? ?? ?? ??"},
		{"virtual fetch", "84 DB 74 3E 49 8B 87 80 5E 00 00"}} {
		pattern, err := parseRuntimePatchPattern(entry.aob)
		if err != nil {
			t.Fatal(err)
		}
		matches := findRuntimePatchLocalPatternMatches(sections, pattern)
		want := map[string]uint32{"virtual apply": 0xA1F590, "virtual category": 0xA201A0, "virtual fetch": 0xA201BE}[entry.name]
		if len(matches) != 1 || matches[0].rva != want {
			t.Errorf("%s matches=%s, want RVA 0x%X", entry.name, formatRuntimePatchLocalMatchLocations(matches), want)
		}
	}
	virtualGetter := []byte{0x55, 0x41, 0x57, 0x41, 0x56, 0x56, 0x57, 0x53, 0x48, 0x83, 0xEC, 0x28}
	if got := runtimePatchLocalBytesAtRVA(sections, 0xA26720, len(virtualGetter)); !bytes.Equal(got, virtualGetter) {
		t.Errorf("virtual getter RVA 0xA26720=% X, want % X", got, virtualGetter)
	}
}

func TestGame205MonsterPatterns(t *testing.T) {
	_, sections := loadLocalGame205Sections(t)
	checks := []struct {
		name   string
		aob    string
		offset int
		want   uint32
	}{
		{"monster hp", "48 8B 41 10 45 31 C9 48 29 D0 4C 0F 43 C8 B8 01 00 00 00 49 0F 47 C1 45 85 C0 49 0F 44 C1 48 89 41 10 C3", 0, 0x1F758A0},
		{"party monster damage", "48 89 51 18 48 89 51 10 C3 CC CC CC CC CC CC CC 48 89 51 18 C3 CC CC CC CC CC CC CC CC CC CC CC 48 89 51 10 C3", 0x20, 0x1F75890},
		{"monster stun", "C5 FA 58 86 60 ?? ?? ?? C5 FA 5D 86 64 ?? ?? ?? C5 FA 11 86 60 ?? ?? ??", 0, 0xB231D8},
		{"overdrive state", "8B 46 10 83 F8 03 0F 84 ?? ?? ?? ?? 83 F8 01 0F 84 ?? ?? ?? ??", 0, 0x22C6B06},
		{"OD gauge rate", "80 79 50 00 74 13 48 03 51 18 48 C7 C0 FF FF FF FF 48 0F 43 C2 48 89 41 18 C3", 0, 0x22C6FD0},
		{"player damage final", "8B 86 D4 00 00 00 3D 00 E1 F5 05", 6, 0x1FB890E},
	}
	for _, check := range checks {
		pattern, err := parseRuntimePatchPattern(check.aob)
		if err != nil {
			t.Fatal(err)
		}
		matches := findRuntimePatchLocalPatternMatches(sections, pattern)
		if len(matches) != 1 {
			t.Errorf("%s matches=%s", check.name, formatRuntimePatchLocalMatchLocations(matches))
			continue
		}
		got := uint32(int64(matches[0].rva) + int64(check.offset))
		if got != check.want {
			t.Errorf("%s RVA=0x%X, want 0x%X", check.name, got, check.want)
		}
	}
	inlinePattern, err := parseRuntimePatchPattern("48 03 7E 18 48 C7 C0 FF FF FF FF 48 0F 43 C7 48 89 46 18")
	if err != nil {
		t.Fatal(err)
	}
	inlineMatches := findRuntimePatchLocalPatternMatches(sections, inlinePattern)
	found := false
	for _, match := range inlineMatches {
		found = found || match.rva == 0x2B3F92E
	}
	if !found || !bytes.Equal(runtimePatchLocalBytesAtRVA(sections, 0x2B3F92E, len(odRateInlineOriginal)), odRateInlineOriginal) {
		t.Errorf("verified inline OD path missing at RVA 0x2B3F92E; matches=%s", formatRuntimePatchLocalMatchLocations(inlineMatches))
	}
}

func TestGame205QOLFreeCaptainSites(t *testing.T) {
	_, sections := loadLocalGame205Sections(t)
	patterns := []string{
		"56 57 48 83 EC 28 48 89 CE 48 83 79 48 10 72 06 48 8B 76 30 EB 04 48 83 C6 30 48 89 F1 E8 ?? ?? ?? ?? 48 89 F1 48 89 C2 E8 ?? ?? ?? ?? 48 8B 0D ?? ?? ?? ?? 89 01 3D 0B A5 A6 4F",
		"41 56 56 57 55 53 48 83 EC 30 48 8B 39 48 8B 0D ?? ?? ?? ?? E8 ?? ?? ?? ?? 85 C0 7E ?? 48 83 C4 30 5B 5D 5F 5E 41 5E C3 C7 87 68 02 00 00 B0 E0 7A 88",
		"41 57 41 56 56 57 53 48 83 EC 70 48 8B 19 48 8B 0D ?? ?? ?? ?? E8 ?? ?? ?? ?? 85 C0 0F 8F ?? ?? ?? ?? 48 8B 35 ?? ?? ?? ?? 48 8B 05 ?? ?? ?? ?? 8B 88 58 0D 00 00",
		"56 57 53 48 83 EC 70 48 8B 39 48 83 BF B0 01 00 00 00 74 4B 8B 02 48 63 C8 48 8B 15 ?? ?? ?? ?? 48 C1 E1 04 80 7C 0A 04 00",
		"41 57 41 56 56 57 53 48 83 EC 50 4C 89 C7 85 D2 78 ?? 48 89 CE 41 89 D7 48 8B 81 48 03 00 00 48 8B 89 50 03 00 00 48 29 C1 48 C1 F9 02",
	}
	want := []uint32{0x1CA1F80, 0x3F0DC70, 0x3F0DAB0, 0x3F0D8E0, 0x41E1610}
	for index, raw := range patterns {
		pattern, err := parseRuntimePatchPattern(raw)
		if err != nil {
			t.Fatal(err)
		}
		matches := findRuntimePatchLocalPatternMatches(sections, pattern)
		if len(matches) != 1 {
			t.Fatalf("QOL free-captain site %d matches=%s", index, formatRuntimePatchLocalMatchLocations(matches))
		}
		if matches[0].rva != want[index] {
			t.Errorf("QOL free-captain site %d RVA=0x%X, want 0x%X", index, matches[0].rva, want[index])
		}
	}
}

func (memory runtimePatchLocalSectionMemory) ReadAt(address uintptr, destination []byte) error {
	if address < memory.moduleBase {
		return fmt.Errorf("address 0x%X precedes module base", address)
	}
	rva := address - memory.moduleBase
	data := runtimePatchLocalBytesAtRVA(memory.sections, uint32(rva), len(destination))
	if len(data) != len(destination) {
		return fmt.Errorf("RVA 0x%X is unavailable", rva)
	}
	copy(destination, data)
	return nil
}

func loadLocalGame205Sections(t *testing.T) (string, []runtimePatchLocalExecutableSection) {
	t.Helper()
	path := os.Getenv("GBFR_GAME_EXE_205_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_205_TEST to verify the locally supplied game 2.0.5 executable")
	}
	if err := verifyRuntimePatchLocalGameIdentityExact(path, runtimePatchLocalGame205Size, runtimePatchLocalGame205SHA256); err != nil {
		t.Fatalf("verify local game 2.0.5 identity: %v", err)
	}
	sections, err := readRuntimePatchLocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, sections
}

func TestGame205RuntimeLayoutIsDetectedFromExactExecutable(t *testing.T) {
	path, sections := loadLocalGame205Sections(t)
	version, digest, err := naturalDropGameIdentity(path)
	if err != nil || version != "2.0.5" || digest != runtimePatchLocalGame205SHA256 {
		t.Fatalf("natural-drop 2.0.5 identity=%q/%q err=%v", version, digest, err)
	}
	if validated, err := validateNaturalDropGameExecutable(path); err != nil || filepath.Clean(validated) != filepath.Clean(path) {
		t.Fatalf("validate 2.0.5 executable and data.i index=%q err=%v", validated, err)
	}
	const moduleBase = uintptr(0x140000000)
	layout, err := detectRuntimeGameLayout(runtimePatchLocalSectionMemory{moduleBase: moduleBase, sections: sections}, moduleBase)
	if err != nil {
		t.Fatalf("detect game 2.0.5 runtime layout: %v", err)
	}
	if layout.Version != "2.0.5" {
		t.Fatalf("detected version=%q, want 2.0.5", layout.Version)
	}
}
