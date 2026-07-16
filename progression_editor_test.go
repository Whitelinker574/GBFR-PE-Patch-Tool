package main

import (
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func weaponUnit(unitID, value uint32) *unitEntry {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, value)
	return &unitEntry{IDType: weaponIDType, UnitID: unitID, ValueOff: 0, ValueCnt: 1, data: data}
}

func TestFindWeaponAddSlotRejectsDuplicates(t *testing.T) {
	const target uint32 = 0xAC65075C
	entries := []*unitEntry{
		weaponUnit(weaponSlotBase, EmptyHash),
		weaponUnit(weaponSlotBase+1, target),
		weaponUnit(weaponSlotBase+2, EmptyHash),
	}
	if _, err := findWeaponAddSlot(entries, target); err == nil {
		t.Fatal("expected duplicate weapon to be rejected even when an empty slot appears first")
	}
	if slot, err := findWeaponAddSlot(entries, 0x11E6966A); err != nil || slot != weaponSlotBase {
		t.Fatalf("first empty slot = %d, %v; want %d", slot, err, weaponSlotBase)
	}
}

func TestWeaponExperienceTable(t *testing.T) {
	for level := 1; level <= 150; level++ {
		xp, err := weaponXPForLevel(level)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if got := weaponLevelForXP(xp); got != level {
			t.Fatalf("level %d XP %d round-tripped as %d", level, xp, got)
		}
	}
	if weaponLevelForXP(^uint32(0)) != 150 {
		t.Fatal("XP above the last threshold must clamp to level 150")
	}
}

func TestProgressionCatalog202(t *testing.T) {
	catalog, err := loadProgressionCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Version != "2.0.2" || len(catalog.Items) != 312 || len(catalog.Weapons) != 164 {
		t.Fatalf("unexpected catalog: version=%s items=%d weapons=%d", catalog.Version, len(catalog.Items), len(catalog.Weapons))
	}
	dangerous := 0
	for _, item := range catalog.Items {
		if item.Dangerous {
			dangerous++
		}
	}
	if dangerous != 8 {
		t.Fatalf("expected 8 quarantined item entries, got %d", dangerous)
	}
	localizedWeapons := 0
	for _, weapon := range catalog.Weapons {
		if weapon.NameCN != "" {
			localizedWeapons++
		}
	}
	if localizedWeapons < 156 {
		t.Fatalf("expected current Chinese names for at least 156 weapons, got %d", localizedWeapons)
	}
	if catalog.Weapons[0].NameCN != "启程之剑" {
		t.Fatalf("unexpected first localized weapon name: %q", catalog.Weapons[0].NameCN)
	}
}

func TestAwakeningWeaponAliasesResolveToBaseDefinition(t *testing.T) {
	if _, err := loadProgressionCatalog(); err != nil {
		t.Fatal(err)
	}
	def, ok := progressionWeaponDefForHash(0x56676778) // Io Ascension weapon at awakening 10
	if !ok || def.Hash != "22E79816" || def.WeaponType != "ascension" || !def.CanAwaken {
		t.Fatalf("unexpected Io awakening alias: ok=%v def=%+v", ok, def)
	}
	def, ok = progressionWeaponDefForHash(0x219EE448) // Fediel Ascension weapon at awakening 10
	if !ok || def.Hash != "1EB2B398" || def.WeaponType != "ascension" {
		t.Fatalf("unexpected Fediel awakening alias: ok=%v def=%+v", ok, def)
	}
}

func TestInternalCompatibilityWeaponIsNotShownAsOwnedCopy(t *testing.T) {
	if _, err := loadProgressionCatalog(); err != nil {
		t.Fatal(err)
	}
	if progressionWeaponVisibleInInventory(0xEE1EBC2E) {
		t.Fatal("Io compatibility mirror must not be shown as a separate owned weapon")
	}
	if !progressionWeaponVisibleInInventory(0xC54B8FE6) {
		t.Fatal("Io player-facing weapon must remain visible")
	}
}

func TestWeaponAwakeningHashTransitions(t *testing.T) {
	cases := []struct {
		current uint32
		level   int
		want    uint32
	}{
		{0x22E79816, 0, 0x22E79816},
		{0x22E79816, 2, 0x22E79816},
		{0x22E79816, 3, 0x26AD10BC},
		{0x56676778, 6, 0xFB5818E6},
		{0x56676778, 10, 0x56676778},
		{0xFEBAC81A, 3, 0xBC5A4248},
		{0x1779CD60, 0, 0xFEBAC81A},
		{0xD7CEE3B8, 1, 0x08AC9299},
	}
	for _, tc := range cases {
		if got := weaponHashForAwakening(tc.current, tc.level); got != tc.want {
			t.Fatalf("hashForAwakening(%08X, %d) = %08X; want %08X", tc.current, tc.level, got, tc.want)
		}
	}
}

func TestFindWeaponAddSlotRejectsAwakeningVariantDuplicate(t *testing.T) {
	entries := []*unitEntry{
		weaponUnit(weaponSlotBase, EmptyHash),
		weaponUnit(weaponSlotBase+1, 0x56676778),
	}
	if _, err := findWeaponAddSlot(entries, 0x22E79816); err == nil {
		t.Fatal("expected an awakened variant to block adding the same base weapon")
	}
}

func TestProgressionNamesUseOnlySelectedLanguage(t *testing.T) {
	previous := getCurrentLanguage()
	t.Cleanup(func() { setCurrentLanguage(previous) })

	item := ProgressionItemDef{Hash: "DB1D4F35", NameEN: "Cobblestone", NameCN: "圆石"}
	weapon := ProgressionWeaponDef{Hash: "AC65075C", Name: "Traveller's Sword (Gran)", NameCN: "启程之剑"}

	setCurrentLanguage("zh")
	if got := progressionItemName(item); got != "圆石" {
		t.Fatalf("Chinese item name = %q", got)
	}
	if got := progressionWeaponName(weapon); got != "启程之剑" {
		t.Fatalf("Chinese weapon name = %q", got)
	}
	if got := cnName("Alpha+"); got != "阿尔法+" {
		t.Fatalf("Chinese sigil name = %q", got)
	}

	setCurrentLanguage("en")
	if got := progressionItemName(item); got != "Cobblestone" {
		t.Fatalf("English item name = %q", got)
	}
	if got := progressionWeaponName(weapon); got != "Traveller's Sword (Gran)" {
		t.Fatalf("English weapon name = %q", got)
	}
	if got := cnName("Alpha+"); got != "Alpha+" {
		t.Fatalf("English sigil name = %q", got)
	}
}

// This integration test is opt-in because repository CI does not contain a
// copyrighted game save. It always writes to t.TempDir and verifies that the
// original file is byte-for-byte unchanged.
func TestProgressionRealSaveCopy(t *testing.T) {
	source := os.Getenv("GBFR_TEST_SAVE")
	if source == "" {
		t.Skip("GBFR_TEST_SAVE not set")
	}
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	originalHash := sha256.Sum256(original)
	app := NewApp()
	before, err := app.ProgressionLoad(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, weapon := range before.Weapons {
		if weapon.Hash == "56676778" && (weapon.OwnerCode != "PL0400" || weapon.WeaponType != "ascension") {
			t.Fatalf("Io ascension weapon lost catalog metadata: %+v", weapon)
		}
		if weapon.Hash == "1779CD60" && (weapon.OwnerCode != "PL0400" || weapon.WeaponType != "terminus") {
			t.Fatalf("Io terminus weapon lost catalog metadata: %+v", weapon)
		}
	}

	var itemDef ProgressionItemDef
	for _, item := range progressionCatalogCache.Items {
		if !item.Dangerous {
			itemDef = item
			break
		}
	}
	if itemDef.Hash == "" {
		t.Fatal("no safe item in catalog")
	}
	ownedWeaponHashes := make(map[string]bool, len(before.Weapons))
	for _, weapon := range before.Weapons {
		ownedWeaponHashes[weapon.Hash] = true
	}
	var weaponDef ProgressionWeaponDef
	for _, weapon := range progressionCatalogCache.Weapons {
		if !ownedWeaponHashes[weapon.Hash] {
			weaponDef = weapon
			break
		}
	}
	if weaponDef.Hash == "" || before.EmptyWeapons == 0 {
		t.Skip("save has no free weapon slot or already owns the full catalog")
	}

	output := filepath.Join(t.TempDir(), "progression-edited.dat")
	result, err := app.ProgressionApply(source, output,
		[]ProgressionResourceChange{{Kind: "rupees", Value: 1234567}},
		[]ProgressionItemChange{{Hash: itemDef.Hash, Quantity: 7, Mode: "set"}},
		[]ProgressionWeaponChange{{Action: "add", Hash: weaponDef.Hash, Level: 150, Uncap: 6, Mirage: 99, Awakening: 0}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.VerifiedChanges != 3 {
		t.Fatalf("verified %d changes, want 3", result.VerifiedChanges)
	}
	after, err := app.ProgressionLoad(output)
	if err != nil {
		t.Fatal(err)
	}
	if after.Rupees != 1234567 || len(after.Weapons) != len(before.Weapons)+1 {
		t.Fatalf("unexpected result: rupees=%d weapons %d -> %d", after.Rupees, len(before.Weapons), len(after.Weapons))
	}

	// Exercise the two in-place editors on another temporary copy. Their
	// timestamped backups must exist and the new values must survive a reload.
	inPlace := filepath.Join(t.TempDir(), "counts-edited.dat")
	editedBytes, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inPlace, editedBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	characters, err := app.GetCharacterStats(inPlace, false)
	if err != nil || len(characters) == 0 {
		t.Fatalf("characters: %v, count=%d", err, len(characters))
	}
	characterResult, err := app.UpdateCharacterStats(inPlace, []CharacterCountChange{{Slot: characters[0].Slot, Count: characters[0].Count + 1}})
	if err != nil {
		t.Fatal(err)
	}
	if characterResult.Verified != 1 {
		t.Fatalf("verified %d character changes", characterResult.Verified)
	}
	if _, err := os.Stat(characterResult.BackupPath); err != nil {
		t.Fatalf("character backup: %v", err)
	}
	quests, err := app.GetQuests(inPlace)
	if err != nil || len(quests) == 0 {
		t.Fatalf("quests: %v, count=%d", err, len(quests))
	}
	questResult, err := app.UpdateQuestCounts(inPlace, []QuestCountChange{{Index: quests[0].Index, QuestID: quests[0].QuestID, Count: quests[0].Clears + 1}})
	if err != nil {
		t.Fatal(err)
	}
	if questResult.Verified != 1 {
		t.Fatalf("verified %d quest changes", questResult.Verified)
	}
	if _, err := os.Stat(questResult.BackupPath); err != nil {
		t.Fatalf("quest backup: %v", err)
	}
	current, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(current) != originalHash {
		t.Fatal("integration test modified the source save")
	}
}

func TestProgressionWeaponStagesRealSaveCopy(t *testing.T) {
	source := os.Getenv("GBFR_TEST_SAVE")
	if source == "" {
		t.Skip("GBFR_TEST_SAVE not set")
	}
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	originalHash := sha256.Sum256(original)
	app := NewApp()
	before, err := app.ProgressionLoad(source)
	if err != nil {
		t.Fatal(err)
	}
	var target OwnedProgressionWeapon
	for _, weapon := range before.Weapons {
		if weapon.CanAwaken && weapon.Level == 150 && weapon.Uncap == 6 {
			target = weapon
			break
		}
	}
	if target.UnitID == 0 {
		t.Skip("save has no fully uncapped awakening weapon")
	}
	targetAwakening := 3
	if target.Awakening == 3 {
		targetAwakening = 10
	}
	targetTranscendence := 7
	if target.Transcendence == 7 {
		targetTranscendence = 6
	}
	out := filepath.Join(t.TempDir(), "weapon-stages-edited.dat")
	change := ProgressionWeaponChange{
		Action: "update", UnitID: target.UnitID, Hash: target.Hash,
		Level: 150, Uncap: 6, Mirage: target.Mirage,
		Awakening: targetAwakening, Transcendence: targetTranscendence,
		TranscendenceSkill: "BBD77C33",
	}
	if _, err := app.ProgressionApply(source, out, nil, nil, []ProgressionWeaponChange{change}); err != nil {
		t.Fatal(err)
	}
	after, err := app.ProgressionLoad(out)
	if err != nil {
		t.Fatal(err)
	}
	var edited *OwnedProgressionWeapon
	for i := range after.Weapons {
		if after.Weapons[i].UnitID == target.UnitID {
			edited = &after.Weapons[i]
			break
		}
	}
	if edited == nil || edited.Awakening != targetAwakening || edited.Transcendence != targetTranscendence || edited.OwnerCode == "" {
		t.Fatalf("unexpected edited weapon: %+v", edited)
	}
	if targetTranscendence == 7 && edited.TranscendenceSkill != "BBD77C33" {
		t.Fatalf("unexpected transcendence skill: %+v", edited)
	}
	unchanged, err := os.ReadFile(source)
	if err != nil || sha256.Sum256(unchanged) != originalHash {
		t.Fatal("source save was modified")
	}
}

func TestIssue18CharacterLayouts(t *testing.T) {
	dir := os.Getenv("GBFR_ISSUE18_DIR")
	if dir == "" {
		t.Skip("GBFR_ISSUE18_DIR not set")
	}
	app := NewApp()
	for _, test := range []struct {
		file string
		slot uint32
		name string
	}{
		{"SaveData1.dat", 10, "菲莉"},   // DLC-created layout
		{"SaveData2.dat", 10, "兰斯洛特"}, // converted pre-DLC layout
	} {
		stats, err := app.GetCharacterStats(filepath.Join(dir, test.file), false)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, stat := range stats {
			if stat.Slot == test.slot {
				found = true
				if stat.Name != test.name {
					t.Fatalf("%s slot %d named %s, want %s", test.file, test.slot, stat.Name, test.name)
				}
			}
		}
		if !found {
			t.Fatalf("%s slot %d missing", test.file, test.slot)
		}
	}
}
