package backend

import (
	"encoding/binary"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strings"
	"testing"
)

func (m *fakeRuntimePanelMemory) putU64(address uintptr, value uint64) {
	encoded := make([]byte, 8)
	binary.LittleEndian.PutUint64(encoded, value)
	m.put(address, encoded)
}

func putRuntimePatchPartySignature(t *testing.T, memory *fakeRuntimePanelMemory, moduleBase uintptr) {
	t.Helper()
	pattern, err := parseRuntimePatchPattern(runtimePatchPartyPointerAOB)
	if err != nil {
		t.Fatal(err)
	}
	bytes := append([]byte(nil), pattern.Values...)
	site := moduleBase + runtimeGameLayouts[0].PartyPointerRVA
	root := moduleBase + runtimeGameLayouts[0].PartySlotTableRVA
	displacement := int64(root) - int64(site+7)
	if displacement < math.MinInt32 || displacement > math.MaxInt32 {
		t.Fatalf("party RIP displacement out of range: %d", displacement)
	}
	binary.LittleEndian.PutUint32(bytes[3:7], uint32(int32(displacement)))
	memory.put(site, bytes)
}

func putRuntimePatchPartyLoadout(memory *fakeRuntimePanelMemory, base, sigilPointer uintptr, partyIndex uint32) {
	stats := make([]byte, runtimePatchPartyStatsSize)
	binary.LittleEndian.PutUint32(stats[0x00:0x04], 100)
	binary.LittleEndian.PutUint32(stats[0x04:0x08], 2000)
	binary.LittleEndian.PutUint32(stats[0x08:0x0C], 17083)
	binary.LittleEndian.PutUint32(stats[0x10:0x14], math.Float32bits(306))
	binary.LittleEndian.PutUint32(stats[0x14:0x18], math.Float32bits(124))
	binary.LittleEndian.PutUint32(stats[0x18:0x1C], 25000)
	memory.put(base+runtimePatchPartyStatsOffset, stats)

	weapon := make([]byte, runtimePatchPartyWeaponSize)
	binary.LittleEndian.PutUint32(weapon[0x04:0x08], 0x02352554)
	binary.LittleEndian.PutUint32(weapon[0x14:0x18], 5)
	binary.LittleEndian.PutUint32(weapon[0x18:0x1C], 99)
	binary.LittleEndian.PutUint32(weapon[0x1C:0x20], 10)
	binary.LittleEndian.PutUint32(weapon[0x20:0x24], 0x7EDD69D0)
	binary.LittleEndian.PutUint32(weapon[0x24:0x28], 15)
	binary.LittleEndian.PutUint32(weapon[0x28:0x2C], 0xDC584F60)
	binary.LittleEndian.PutUint32(weapon[0x2C:0x30], 15)
	binary.LittleEndian.PutUint32(weapon[0x38:0x3C], 1234)
	binary.LittleEndian.PutUint32(weapon[0x44:0x48], 0x7EDD69D0)
	binary.LittleEndian.PutUint32(weapon[0x48:0x4C], 0xDC584F60)
	binary.LittleEndian.PutUint32(weapon[0x58:0x5C], 150)
	binary.LittleEndian.PutUint32(weapon[0x5C:0x60], 500)
	binary.LittleEndian.PutUint32(weapon[0x60:0x64], 3000)
	binary.LittleEndian.PutUint32(weapon[0xA4:0xA8], 0xDC584F60)
	binary.LittleEndian.PutUint32(weapon[0xA8:0xAC], 14)
	binary.LittleEndian.PutUint32(weapon[0xAC:0xB0], 0x7EDD69D0)
	binary.LittleEndian.PutUint32(weapon[0xB0:0xB4], 15)
	memory.put(base+runtimePatchPartyWeaponOffset, weapon)

	overmastery := make([]byte, runtimePatchPartyOvermasterySize)
	binary.LittleEndian.PutUint32(overmastery[0x00:0x04], 0x52A207B5)
	binary.LittleEndian.PutUint32(overmastery[0x04:0x08], 0x00000200)
	binary.LittleEndian.PutUint32(overmastery[0x0C:0x10], math.Float32bits(1000))
	binary.LittleEndian.PutUint32(overmastery[0x10:0x14], 0x54929589)
	binary.LittleEndian.PutUint32(overmastery[0x14:0x18], 0x00000010)
	binary.LittleEndian.PutUint32(overmastery[0x1C:0x20], math.Float32bits(500))
	memory.put(base+runtimePatchPartyOvermasteryOffset, overmastery)

	sigils := make([]byte, runtimePatchPartySigilListSize)
	binary.LittleEndian.PutUint32(sigils[0x00:0x04], 0x50079A1C)
	binary.LittleEndian.PutUint32(sigils[0x04:0x08], 15)
	binary.LittleEndian.PutUint32(sigils[0x08:0x0C], 0xDC584F60)
	binary.LittleEndian.PutUint32(sigils[0x0C:0x10], 15)
	binary.LittleEndian.PutUint32(sigils[0x10:0x14], 0x2D7F2E70)
	binary.LittleEndian.PutUint32(sigils[0x18:0x1C], 15)
	copy(sigils[0x1E8:0x1F8], []byte("PL1600"))
	binary.LittleEndian.PutUint32(sigils[0x22C:0x230], partyIndex)
	memory.putPtr(base+runtimePatchPartySigilPointerOffset, sigilPointer)
	memory.put(sigilPointer, sigils)
}

func putRuntimePatchPartyExpansionLoadout(memory *fakeRuntimePanelMemory, moduleBase, base uintptr) {
	memory.putU32(base+runtimePatchPartyCharacterHashOffset, 0x0D21B430)
	memory.putU32(base+runtimePatchPartyMasterLevelOffset, 55)
	for index, hash := range []uint32{0x95E40E12, 0x67270513, 0x974342AA, 0x52E3EED1} {
		memory.putU32(base+runtimePatchPartyAbilityOffset+uintptr(index*4), hash)
	}
	summonCatalog, err := loadSummonStatCatalog()
	if err != nil {
		panic(err)
	}
	var summonType, mainTrait, subParam uint32
	for hash := range summonCatalog.types {
		summonType = hash
		break
	}
	for hash := range summonCatalog.main {
		mainTrait = hash
		break
	}
	for hash := range summonCatalog.sub {
		subParam = hash
		break
	}
	for index := 0; index < runtimePatchPartySummonCount; index++ {
		offset := base + runtimePatchPartySummonOffset + uintptr(index*runtimePatchPartySummonStride)
		memory.putU32(offset, summonType)
		memory.putU32(offset+0x04, uint32(index+1))
		memory.putU32(offset+0x08, mainTrait)
		memory.putU32(offset+0x0C, subParam)
		memory.putU32(offset+0x10, 15)
		memory.putU32(offset+0x14, 9)
		memory.putU32(offset+0x18, 0)
	}

	recordNodes := make([]byte, runtimePatchPartyMasteryUnlockSize)
	binary.LittleEndian.PutUint32(recordNodes[0x00:0x04], 0x11110001)
	binary.LittleEndian.PutUint32(recordNodes[0x04:0x08], 1<<3)
	memory.put(base+runtimePatchPartyMasteryUnlockOffset, recordNodes)

	manager := uintptr(0x51000000)
	charBuckets := uintptr(0x52000000)
	charNode := uintptr(0x53000000)
	charKeys := uintptr(0x54000000)
	nodeBuckets := uintptr(0x55000000)
	effectNode := uintptr(0x56000000)
	effectRow := uintptr(0x57000000)
	memory.putPtr(moduleBase+runtimePatchPartyCharaPowerRVA, manager)
	putRuntimePatchPartyMap(memory, manager, runtimePatchPartySkillboardCharMap, charBuckets, charNode, 0x0D21B430)
	memory.putPtr(charNode+0x18, charKeys)
	memory.putPtr(charNode+0x20, charKeys+8)
	memory.putU32(charKeys, 0x01EE7C0A)
	memory.putU32(charKeys+4, 0)
	putRuntimePatchPartyMap(memory, manager, runtimePatchPartySkillboardNodeMap, nodeBuckets, effectNode, 0x01EE7C0A)
	memory.putPtr(effectNode+0x18, effectRow)
	memory.put(effectRow, make([]byte, 0x78))
	memory.putU32(effectRow+0x48, 0x11110001)
	memory.putU32(effectRow+0x5C, 3)
	memory.putU32(effectRow+0x74, 0x100)
}

func putRuntimePatchPartyMap(memory *fakeRuntimePanelMemory, manager uintptr, layout runtimePatchPartyMapLayout, buckets, node uintptr, key uint32) {
	memory.putPtr(manager+layout.endOffset, manager+0x1000+layout.endOffset)
	memory.putPtr(manager+layout.bucketsOffset, buckets)
	memory.putU32(manager+layout.maskOffset, 0)
	memory.putPtr(buckets, node)
	memory.putPtr(buckets+8, node)
	memory.putPtr(node+8, manager+0x1000+layout.endOffset)
	memory.putU32(node+0x10, key)
}

func newRuntimePatchPartyFixture(t *testing.T) (*fakeRuntimePanelMemory, uintptr) {
	t.Helper()
	memory := newFakeRuntimePanelMemory()
	moduleBase := uintptr(0x10000000)
	putRuntimePatchPartySignature(t, memory, moduleBase)

	root := moduleBase + runtimeGameLayouts[0].PartySlotTableRVA
	entities := [...]uintptr{0x21000000, 0x22000000, 0x23000000, 0x24000000}
	for index, entity := range entities {
		specified := entity + 0x10000
		memory.putPtr(root+uintptr(index)*8, entity)
		memory.putPtr(entity+runtimePatchPartyTransformRootOffset, entity+0x6000)
		memory.putPtr(entity+0x6000+runtimePatchPartyTransformNodeOffset, entity+0x7000)
		memory.putU64(entity+runtimePatchPartyHPOffset, uint64(1000+index))
		memory.putU64(entity+runtimePatchPartyMaxHPOffset, uint64(2000+index))
		memory.putU32(entity+runtimePatchPartyDodgeOffset, uint32(3+index))
		memory.putF32(entity+runtimePatchPartySBAOffset, float32(20+index))
		memory.putF32(entity+runtimePatchPartyMaxSBAOffset, 100)
		memory.putF32(entity+0x7000+runtimePatchPartyPositionXOffset, float32(10+index))
		memory.putF32(entity+0x7000+runtimePatchPartyPositionYOffset, float32(20+index))
		memory.putF32(entity+0x7000+runtimePatchPartyPositionZOffset, float32(30+index))
		putRuntimePatchPartyValidatedHandle(memory, moduleBase, index, uint32(index+1), entity, uint64(0xA000+index), specified)
		putRuntimePatchPartyLoadout(memory, specified, specified+0x30000, uint32(index))
		putRuntimePatchPartyExpansionLoadout(memory, moduleBase, specified)
	}

	companionContainer := uintptr(0x25000000)
	companion := uintptr(0x26000000)
	memory.putPtr(root+runtimePatchPartyCompanionSlotOffset, companionContainer)
	memory.putPtr(companionContainer+runtimePatchPartyCompanionEntityOffset, companion)
	memory.putPtr(companion+runtimePatchPartyTransformRootOffset, companion+0x6000)
	memory.putPtr(companion+0x6000+runtimePatchPartyTransformNodeOffset, companion+0x7000)
	memory.putU64(companion+runtimePatchPartyHPOffset, 500)
	memory.putU64(companion+runtimePatchPartyMaxHPOffset, 900)
	memory.putF32(companion+0x7000+runtimePatchPartyPositionXOffset, 41)
	memory.putF32(companion+0x7000+runtimePatchPartyPositionYOffset, 42)
	memory.putF32(companion+0x7000+runtimePatchPartyPositionZOffset, 43)
	memory.putF32(companion+runtimePatchPartyCompanionDirectXOffset, 51)
	memory.putF32(companion+runtimePatchPartyCompanionDirectYOffset, 52)
	memory.putF32(companion+runtimePatchPartyCompanionDirectZOffset, 53)
	return memory, moduleBase
}

func TestReadRuntimePatchPartySnapshotUsesVerified202LayoutAndOptionalCompanionFields(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Topology.Root != moduleBase+runtimeGameLayouts[0].PartySlotTableRVA {
		t.Fatalf("root=0x%X", snapshot.Topology.Root)
	}
	if got, want := len(snapshot.Result.Entities), 5; got != want {
		t.Fatalf("entity count=%d, want %d", got, want)
	}
	player := snapshot.Result.Entities[0]
	if !player.Present || player.Role != "player" || player.HP != 1000 || player.MaxHP != 2000 || player.DodgeCount == nil || *player.DodgeCount != 3 || player.SBA == nil || *player.SBA != 20 {
		t.Fatalf("player=%+v", player)
	}
	if !player.Capabilities.Loadout || player.Loadout == nil || !player.Loadout.Available || player.Loadout.Weapon.Hash != 0x02352554 || player.Loadout.CharacterCode != "PL1600" || len(player.Loadout.Sigils) != 1 {
		t.Fatalf("player loadout=%+v", player.Loadout)
	}
	if got := player.Loadout.Weapon.Skills; len(got) != 2 || got[0].Hash != 0x7EDD69D0 || got[0].Level != 15 || got[1].Hash != 0xDC584F60 || got[1].Level != 14 {
		t.Fatalf("weapon skills=%+v", got)
	}
	if got := player.Loadout.OverLimit; len(got) != 4 || got[0].AttributeHash != 0x52A207B5 || got[0].Level != 10 || got[1].Level != 5 {
		t.Fatalf("overmastery=%+v", got)
	}
	if player.Loadout.MasterLevel != 55 || len(player.Loadout.Abilities) != 4 || len(player.Loadout.Summons) != 4 {
		t.Fatalf("expansion equipment was not captured: %+v", player.Loadout)
	}
	if !player.Loadout.MasteryAvailable || len(player.Loadout.Mastery) != 1 || player.Loadout.Mastery[0].Hash != "01EE7C0A" {
		t.Fatalf("skillboard capture=%+v reason=%q", player.Loadout.Mastery, player.Loadout.MasteryUnavailableReason)
	}
	companion := snapshot.Result.Entities[4]
	if !companion.Present || companion.Role != "companion" || companion.HP != 500 || companion.MaxHP != 900 {
		t.Fatalf("companion=%+v", companion)
	}
	if companion.DodgeCount != nil || companion.SBA != nil || companion.MaxSBA != nil {
		t.Fatalf("companion must not fabricate dodge/SBA zero values: %+v", companion)
	}
	if companion.Capabilities.Dodge || companion.Capabilities.SBA || !companion.Capabilities.DirectPosition || companion.DirectPosition == nil {
		t.Fatalf("companion capabilities=%+v direct=%+v", companion.Capabilities, companion.DirectPosition)
	}
	if companion.DirectPosition.X != 51 || companion.DirectPosition.Y != 52 || companion.DirectPosition.Z != 53 {
		t.Fatalf("companion direct position=%+v", companion.DirectPosition)
	}
}

func TestRuntimePatchPartyExpansionRejectsWrongCharacterAndAbilityOwner(t *testing.T) {
	for name, mutate := range map[string]func(*fakeRuntimePanelMemory, uintptr){
		"character hash": func(memory *fakeRuntimePanelMemory, base uintptr) {
			memory.putU32(base+runtimePatchPartyCharacterHashOffset, 0x4D0A60C3)
		},
		"ability owner": func(memory *fakeRuntimePanelMemory, base uintptr) {
			memory.putU32(base+runtimePatchPartyAbilityOffset, 0xBAD9BAA3)
		},
	} {
		t.Run(name, func(t *testing.T) {
			memory, moduleBase := newRuntimePatchPartyFixture(t)
			base := uintptr(0x21000000 + 0x10000)
			mutate(memory, base)
			snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
			if err != nil {
				t.Fatal(err)
			}
			loadout := snapshot.Result.Entities[0].Loadout
			if loadout == nil || loadout.Available || loadout.UnavailableReason == "" {
				t.Fatalf("invalid %s was accepted: %+v", name, loadout)
			}
		})
	}
}

func TestRuntimePatchPartySkillboardFailsClosedOnDamagedMapWithoutLosingEquipment(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	manager, err := readRuntimePatchPointer(memory, moduleBase+runtimePatchPartyCharaPowerRVA)
	if err != nil {
		t.Fatal(err)
	}
	memory.putU32(manager+runtimePatchPartySkillboardNodeMap.maskOffset, ^uint32(0))
	snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	loadout := snapshot.Result.Entities[0].Loadout
	if loadout == nil || !loadout.Available || len(loadout.Abilities) != 4 || len(loadout.Summons) != 4 {
		t.Fatalf("damaged mastery map must not discard verified equipment: %+v", loadout)
	}
	if loadout.MasteryAvailable || loadout.MasteryUnavailableReason == "" || len(loadout.Mastery) != 0 {
		t.Fatalf("damaged mastery map did not fail closed: %+v", loadout)
	}
}

func TestRuntimePatchPartySkillboardRequiresUnlockBitAndMatchingOwner(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	base := uintptr(0x21000000 + 0x10000)
	memory.putU32(base+runtimePatchPartyMasteryUnlockOffset+4, 0)
	snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	loadout := snapshot.Result.Entities[0].Loadout
	if loadout == nil || !loadout.MasteryAvailable || len(loadout.Mastery) != 0 {
		t.Fatalf("locked mastery node was included: %+v", loadout)
	}
}

func TestRuntimePatchPartySkillboardSkipsLiveCompactUIEffectIDs(t *testing.T) {
	for _, effectID := range []uint32{0x0A, 0x0B, 0x100} {
		t.Run(fmt.Sprintf("%08X", effectID), func(t *testing.T) {
			memory, moduleBase := newRuntimePatchPartyFixture(t)
			memory.putU32(0x54000000, 0x22220001)
			memory.putU32(0x56000000+0x10, 0x22220001)
			memory.putU32(0x57000000+0x74, effectID)
			snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
			if err != nil {
				t.Fatal(err)
			}
			loadout := snapshot.Result.Entities[0].Loadout
			if loadout == nil || !loadout.MasteryAvailable || len(loadout.Mastery) != 0 || loadout.MasteryUnavailableReason != "" {
				t.Fatalf("compact UI effect ID was treated as a mastery failure: %+v", loadout)
			}
		})
	}
}

func TestRuntimePatchPartyMinimumMasteryHashMatchesBundled202Catalog(t *testing.T) {
	loadSkillboard()
	minimum := ^uint32(0)
	for _, node := range skillboardAllNodes {
		hash, err := ParseHashHex(node.Hash)
		if err != nil {
			t.Fatal(err)
		}
		if hash < minimum {
			minimum = hash
		}
	}
	if minimum != runtimePatchPartyMinimumMasteryHash {
		t.Fatalf("minimum mastery hash=%08X, want %08X", minimum, runtimePatchPartyMinimumMasteryHash)
	}
}

func TestRuntimePatchPartySkillboardExcludesAutomaticSpecializationRoots(t *testing.T) {
	var root uint32
	for hash := range masterySpecializationRoots {
		if node, ok := skillboardNodeForHash(hash); ok && node.Char == "PL1600" {
			root = hash
			break
		}
	}
	if root == 0 {
		t.Fatal("PL1600 specialization root fixture is missing")
	}
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	memory.putU32(0x54000000, root)
	memory.putU32(0x56000000+0x10, root)
	snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	loadout := snapshot.Result.Entities[0].Loadout
	if loadout == nil || !loadout.MasteryAvailable || len(loadout.Mastery) != 0 {
		t.Fatalf("automatic specialization root leaked into the 3007 configuration: %+v", loadout)
	}
}

func TestRuntimePatchPartySkillboardExcludesNamedRankOneStageEffects(t *testing.T) {
	loadSkillboard()
	var stageEffect uint32
	for _, node := range skillboardAllNodes {
		rank, _, ok := masteryRankOfGrp(node.Grp)
		if node.Char == "PL1600" && ok && rank == "R1" && strings.TrimSpace(node.Name) != "" {
			stageEffect, _ = ParseHashHex(node.Hash)
			break
		}
	}
	if stageEffect == 0 {
		t.Fatal("PL1600 named rank-one stage effect fixture is missing")
	}
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	memory.putU32(0x54000000, stageEffect)
	memory.putU32(0x56000000+0x10, stageEffect)
	snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	loadout := snapshot.Result.Entities[0].Loadout
	if loadout == nil || !loadout.MasteryAvailable || len(loadout.Mastery) != 0 {
		t.Fatalf("named rank-one stage effect leaked into the 3007 configuration: %+v", loadout)
	}
}

func TestRuntimePatchPartyLoadoutRejectsUnknownWeaponWithoutBreakingCorePartyValues(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	entity := uintptr(0x22000000)
	specified := entity + 0x10000
	memory.putU32(specified+runtimePatchPartyWeaponOffset+0x04, 0xDEADBEEF)

	snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	member := snapshot.Result.Entities[1]
	if member.HP != 1001 || member.Loadout == nil || member.Loadout.Available || member.Capabilities.Loadout {
		t.Fatalf("invalid loadout must fail closed without hiding core metrics: %+v", member)
	}
	if !strings.Contains(strings.ToLower(member.Loadout.UnavailableReason), "weapon") {
		t.Fatalf("unexpected unavailable reason: %q", member.Loadout.UnavailableReason)
	}
}

func TestRuntimePatchPartyLoadoutRejectsCharacterAndWeaponOwnerMismatch(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	entity := uintptr(0x22000000)
	specified := entity + 0x10000
	memory.put(specified+0x30000+0x1E8, append([]byte("PL0400"), make([]byte, 10)...))

	snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	member := snapshot.Result.Entities[1]
	if member.Loadout == nil || member.Loadout.Available || member.Capabilities.Loadout {
		t.Fatalf("mismatched runtime character must fail closed: %+v", member)
	}
	if !strings.Contains(strings.ToLower(member.Loadout.UnavailableReason), "weapon owner") {
		t.Fatalf("unexpected mismatch reason: %q", member.Loadout.UnavailableReason)
	}
}

func TestRuntimePatchPartyLoadoutRejectsRecordFromAnotherPartySlot(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	// Slot 1 deliberately points at a record tagged as the local player.
	secondEntity := uintptr(0x22000000)
	secondSpecified := secondEntity + 0x10000
	memory.putU32(secondSpecified+0x30000+0x22C, 0)
	snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	member := snapshot.Result.Entities[1]
	if member.Loadout == nil || member.Loadout.Available || member.Capabilities.Loadout {
		t.Fatalf("cross-slot loadout was accepted: %+v", member.Loadout)
	}
	if !strings.Contains(strings.ToLower(member.Loadout.UnavailableReason), "party index") {
		t.Fatalf("unexpected cross-slot rejection: %q", member.Loadout.UnavailableReason)
	}
}

func TestRuntimePatchPartyLoadoutRejectsUnassignedOnlineRecord(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	entity := uintptr(0x22000000)
	specified := entity + 0x10000
	memory.putU32(specified+0x30000+0x1C8, 1)
	memory.putU32(specified+0x30000+0x22C, 0xFF)
	snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	member := snapshot.Result.Entities[1]
	if member.Loadout == nil || member.Loadout.Available || member.Capabilities.Loadout {
		t.Fatalf("unassigned online loadout was accepted: %+v", member.Loadout)
	}
	if !strings.Contains(strings.ToLower(member.Loadout.UnavailableReason), "online") {
		t.Fatalf("unexpected online identity rejection: %q", member.Loadout.UnavailableReason)
	}
}

func TestRuntimePatchPartyLoadoutSupportsVerifiedIndirectSpecifiedInstanceCandidate(t *testing.T) {
	memory := newFakeRuntimePanelMemory()
	entity := uintptr(0x31000000)
	instance := uintptr(0x32000000)
	memory.putPtr(entity+runtimePatchPartySpecifiedInstanceOffset, instance)
	putRuntimePatchPartyLoadout(memory, instance, 0x33000000, 2)

	loadout, base, fingerprint, err := readRuntimePatchPartyLoadoutCandidates(memory, entity)
	if err != nil {
		t.Fatal(err)
	}
	if base != instance || loadout.Layout != "entity+0x70 -> instance+{0x15030,0x15080,0x1AE90}" || !loadout.Available || fingerprint == ([32]byte{}) {
		t.Fatalf("indirect candidate=%+v base=0x%X fingerprint=%X", loadout, base, fingerprint)
	}
}

func TestRuntimePatchPartyWeaponSkillAcceptsExtractedSkillStatusHash(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	name, ok := runtimePatchPartyWeaponSkillName(catalog, 0x1E1CECCE)
	if !ok || name != "浩劫" {
		t.Fatalf("weapon skill 1E1CECCE resolved to name=%q ok=%v", name, ok)
	}
}

func TestRuntimePatchPartyLoadoutAcceptsKnownRuntimeSigilOutsideConstructionCatalog(t *testing.T) {
	previous := getCurrentLanguage()
	setCurrentLanguage("zh")
	t.Cleanup(func() { setCurrentLanguage(previous) })

	for _, test := range []struct {
		hash uint32
		name string
	}{
		{hash: 0x39BEA99B, name: "昏厥V+"},
		{hash: 0x80C94A24, name: "怒发冲冠V+"},
		{hash: 0x6CBA6B0D, name: "怒涛V+"},
	} {
		t.Run(fmt.Sprintf("%08X", test.hash), func(t *testing.T) {
			memory := newFakeRuntimePanelMemory()
			base := uintptr(0x36000000)
			sigilPointer := uintptr(0x37000000)
			putRuntimePatchPartyLoadout(memory, base, sigilPointer, 0)
			memory.putU32(sigilPointer+0x10, test.hash)

			loadout, _, err := readRuntimePatchPartyLoadoutAt(memory, base, "test")
			if err != nil {
				t.Fatal(err)
			}
			if len(loadout.Sigils) != 1 || loadout.Sigils[0].Hash != test.hash || loadout.Sigils[0].Name != test.name {
				t.Fatalf("runtime sigil=%+v", loadout.Sigils)
			}
		})
	}
}

func TestRuntimePatchPartyLoadoutRejectsUnverifiedRuntimeSigilHash(t *testing.T) {
	memory := newFakeRuntimePanelMemory()
	base := uintptr(0x36000000)
	sigilPointer := uintptr(0x37000000)
	putRuntimePatchPartyLoadout(memory, base, sigilPointer, 0)
	memory.putU32(sigilPointer+0x10, 0xDEADBEEF)

	if _, _, err := readRuntimePatchPartyLoadoutAt(memory, base, "test"); err == nil || !strings.Contains(err.Error(), "DEADBEEF") {
		t.Fatalf("unverified runtime sigil error=%v", err)
	}
}

func putRuntimePatchPartyValidatedHandle(memory *fakeRuntimePanelMemory, moduleBase uintptr, slot int, indexPlusOne uint32, entity uintptr, id uint64, specified uintptr) {
	entityTable := uintptr(0x41000000)
	entityArray := uintptr(0x42000000)
	idArray := uintptr(0x43000000)
	memory.putPtr(moduleBase+runtimeGameLayouts[0].PartyEntityTableRVA, entityTable)
	memory.putPtr(entityTable+runtimePatchPartyEntityArrayOffset, entityArray)
	memory.putPtr(entityTable+runtimePatchPartyIDArrayOffset, idArray)
	handle := moduleBase + runtimeGameLayouts[0].PartyHandleTableRVA + uintptr(slot)*runtimePatchPartyHandleStride
	memory.putU32(handle, indexPlusOne)
	memory.putPtr(handle+runtimePatchPartyHandleEntityOffset, entity)
	memory.putU64(handle+runtimePatchPartyHandleIDOffset, id)
	index := uintptr(indexPlusOne - 1)
	memory.putPtr(entityArray+index*8, entity)
	memory.putU64(idArray+index*8, id)
	memory.putPtr(entity+runtimePatchPartySpecifiedInstanceOffset, specified)
}

func TestResolveRuntimePatchPartyLoadoutHandleUsesValidatedSpecifiedInstance(t *testing.T) {
	memory := newFakeRuntimePanelMemory()
	moduleBase := uintptr(0x10000000)
	putRuntimePatchPartyValidatedHandle(memory, moduleBase, 2, 7, 0x44000000, 0x1122334455667788, 0x45000000)

	resolved, err := resolveRuntimePatchPartyLoadoutHandleWithLayout(memory, moduleBase, runtimeGameLayouts[0], 2)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.EntityTable != 0x41000000 || resolved.Entity != 0x44000000 || resolved.ID != 0x1122334455667788 || resolved.Specified != 0x45000000 {
		t.Fatalf("resolved handle=%+v", resolved)
	}
}

func TestResolveRuntimePatchPartyLoadoutHandleRejectsStaleID(t *testing.T) {
	memory := newFakeRuntimePanelMemory()
	moduleBase := uintptr(0x10000000)
	putRuntimePatchPartyValidatedHandle(memory, moduleBase, 0, 3, 0x44000000, 0x1111, 0x45000000)
	memory.putU64(0x43000000+2*8, 0x2222)
	if _, err := resolveRuntimePatchPartyLoadoutHandleWithLayout(memory, moduleBase, runtimeGameLayouts[0], 0); err == nil || !strings.Contains(strings.ToLower(err.Error()), "id") {
		t.Fatalf("stale handle ID error=%v", err)
	}
}

func TestResolveRuntimePatchPartyLoadoutHandleRejectsEntityTableMismatch(t *testing.T) {
	memory := newFakeRuntimePanelMemory()
	moduleBase := uintptr(0x10000000)
	putRuntimePatchPartyValidatedHandle(memory, moduleBase, 1, 4, 0x44000000, 0x1111, 0x45000000)
	memory.putPtr(0x42000000+3*8, 0x46000000)
	if _, err := resolveRuntimePatchPartyLoadoutHandleWithLayout(memory, moduleBase, runtimeGameLayouts[0], 1); err == nil || !strings.Contains(strings.ToLower(err.Error()), "entity") {
		t.Fatalf("entity table mismatch error=%v", err)
	}
}

func TestResolveRuntimePatchPartyLoadoutHandleRejectsNullAndOverflowPointers(t *testing.T) {
	memory := newFakeRuntimePanelMemory()
	moduleBase := uintptr(0x10000000)
	memory.putPtr(moduleBase+runtimeGameLayouts[0].PartyEntityTableRVA, 0)
	if _, err := resolveRuntimePatchPartyLoadoutHandleWithLayout(memory, moduleBase, runtimeGameLayouts[0], 0); err == nil {
		t.Fatal("null entity table was accepted")
	}
	if _, err := resolveRuntimePatchPartyLoadoutHandleWithLayout(memory, ^uintptr(0)-runtimeGameLayouts[0].PartyEntityTableRVA+1, runtimeGameLayouts[0], 0); err == nil || !strings.Contains(strings.ToLower(err.Error()), "overflow") {
		t.Fatalf("overflowing module base error=%v", err)
	}
}

func TestRuntimePatchPartyLoadoutRejectsInvalidSigilPointerAndAddressOverflow(t *testing.T) {
	memory := newFakeRuntimePanelMemory()
	base := uintptr(0x34000000)
	putRuntimePatchPartyLoadout(memory, base, 0x35000000, 0)
	memory.putPtr(base+runtimePatchPartySigilPointerOffset, 1)
	if _, _, _, err := readRuntimePatchPartyLoadoutCandidates(memory, base); err == nil || !strings.Contains(strings.ToLower(err.Error()), "sigil") {
		t.Fatalf("invalid sigil pointer error=%v", err)
	}
	if _, _, _, err := readRuntimePatchPartyLoadoutCandidates(memory, ^uintptr(0)-0x20); err == nil {
		t.Fatal("overflowing entity base was accepted")
	}
}

func TestReadStableRuntimePatchPartySnapshotsRejectsChangingLoadoutFingerprint(t *testing.T) {
	base := runtimePatchPartyTopology{Root: 0x100, Entities: [5]uintptr{1, 2, 3, 4, 5}}
	changed := base
	base.LoadoutFingerprints[1][0] = 1
	changed.LoadoutFingerprints[1][0] = 2
	frames := []runtimePatchPartySnapshot{{Topology: base}, {Topology: changed}, {Topology: changed}}
	index := 0
	_, err := readStableRuntimePatchPartySnapshots(func() (runtimePatchPartySnapshot, error) {
		frame := frames[index]
		index++
		return frame, nil
	})
	if err == nil || (!strings.Contains(err.Error(), "拓扑") && !strings.Contains(strings.ToLower(err.Error()), "topology")) {
		t.Fatalf("changing loadout fingerprint error=%v", err)
	}
}

func TestReadRuntimePatchPartySnapshotAcceptsEmptyTrainingPartySlots(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	root := moduleBase + runtimeGameLayouts[0].PartySlotTableRVA
	for index := 1; index < 4; index++ {
		memory.putPtr(root+uintptr(index)*8, 0)
	}

	snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(snapshot.Result.Entities), 5; got != want {
		t.Fatalf("entity count=%d, want %d", got, want)
	}
	if !snapshot.Result.Entities[0].Present {
		t.Fatal("player must remain present")
	}
	for index := 1; index < 4; index++ {
		entity := snapshot.Result.Entities[index]
		if entity.Present || entity.Address != 0 || entity.HP != 0 || entity.MaxHP != 0 {
			t.Fatalf("empty slot %d fabricated runtime data: %+v", index, entity)
		}
		if snapshot.Topology.Entities[index] != 0 || snapshot.Topology.TransformNodes[index] != [2]uintptr{} {
			t.Fatalf("empty slot %d retained topology: %+v", index, snapshot.Topology)
		}
	}
}

func TestReadRuntimePatchPartySnapshotRejectsLoadoutHandleFromAnotherRootSlot(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	const otherEntity = uintptr(0x22000000)
	const otherSpecified = otherEntity + 0x10000
	putRuntimePatchPartyValidatedHandle(memory, moduleBase, 0, 2, otherEntity, 0xA001, otherSpecified)

	snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	player := snapshot.Result.Entities[0]
	if player.Loadout == nil || player.Loadout.Available || player.Capabilities.Loadout {
		t.Fatalf("cross-slot loadout handle was accepted: %+v", player)
	}
	if !strings.Contains(strings.ToLower(player.Loadout.UnavailableReason), "slot") && !strings.Contains(player.Loadout.UnavailableReason, "槽") {
		t.Fatalf("cross-slot loadout error is unclear: %q", player.Loadout.UnavailableReason)
	}
}

func TestReadRuntimePatchPartySnapshotAcceptsMissingCompanion(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	root := moduleBase + runtimeGameLayouts[0].PartySlotTableRVA
	memory.putPtr(root+runtimePatchPartyCompanionSlotOffset, 0)

	snapshot, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	companion := snapshot.Result.Entities[4]
	if companion.Present || companion.Role != "companion" || companion.Address != 0 {
		t.Fatalf("missing companion=%+v", companion)
	}
}

func TestReadRuntimePatchPartySnapshotStillRejectsMissingPlayer(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	memory.putPtr(moduleBase+runtimeGameLayouts[0].PartySlotTableRVA, 0)
	_, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err == nil || (!strings.Contains(err.Error(), "玩家") && !strings.Contains(strings.ToLower(err.Error()), "player")) {
		t.Fatalf("missing player error=%v", err)
	}
}

func TestReadStableRuntimePatchPartySnapshotsAcceptsDynamicValuesAndReturnsLastFrame(t *testing.T) {
	topology := runtimePatchPartyTopology{Root: 0x100, Entities: [5]uintptr{1, 2, 3, 4, 5}}
	frames := []runtimePatchPartySnapshot{
		{Topology: topology, Result: RuntimePatchPartyMonitor{Entities: []RuntimePatchPartyEntity{{Role: "player", HP: 10, MaxHP: 100}}}},
		{Topology: topology, Result: RuntimePatchPartyMonitor{Entities: []RuntimePatchPartyEntity{{Role: "player", HP: 20, MaxHP: 100}}}},
		{Topology: topology, Result: RuntimePatchPartyMonitor{Entities: []RuntimePatchPartyEntity{{Role: "player", HP: 30, MaxHP: 100}}}},
	}
	index := 0
	result, err := readStableRuntimePatchPartySnapshots(func() (runtimePatchPartySnapshot, error) {
		frame := frames[index]
		index++
		return frame, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Entities[0].HP != 30 || !result.RuntimeVerified || result.SnapshotCount != 3 {
		t.Fatalf("stable result=%+v", result)
	}
}

func TestReadStableRuntimePatchPartySnapshotsRejectsTopologyChanges(t *testing.T) {
	base := runtimePatchPartyTopology{Root: 0x100, Entities: [5]uintptr{1, 2, 3, 4, 5}}
	changed := base
	changed.TransformNodes[2][1] = 0xDEAD
	frames := []runtimePatchPartySnapshot{{Topology: base}, {Topology: changed}, {Topology: changed}}
	index := 0
	_, err := readStableRuntimePatchPartySnapshots(func() (runtimePatchPartySnapshot, error) {
		frame := frames[index]
		index++
		return frame, nil
	})
	if err == nil || (!strings.Contains(err.Error(), "拓扑") && !strings.Contains(strings.ToLower(err.Error()), "topology")) {
		t.Fatalf("topology change error=%v", err)
	}
}

func TestReadStableRuntimePatchPartySnapshotsRejectsOnlineIdentityChanges(t *testing.T) {
	topology := runtimePatchPartyTopology{Root: 0x100, Entities: [5]uintptr{1, 2, 3, 4, 5}}
	makeFrame := func(hash uint32) runtimePatchPartySnapshot {
		return runtimePatchPartySnapshot{Topology: topology, Result: RuntimePatchPartyMonitor{Entities: []RuntimePatchPartyEntity{{Role: "party1", Loadout: &RuntimePatchPartyLoadout{Available: true, Online: true, PartyIndex: 1, CharacterCode: "PL1600", Weapon: RuntimePatchPartyWeapon{Hash: hash}, Sigils: []RuntimePatchPartySigil{{Index: 0, Hash: 0x1000 + hash, Level: 15}}}}}}}
	}
	frames := []runtimePatchPartySnapshot{makeFrame(1), makeFrame(2), makeFrame(2)}
	index := 0
	_, err := readStableRuntimePatchPartySnapshots(func() (runtimePatchPartySnapshot, error) {
		frame := frames[index]
		index++
		return frame, nil
	})
	if err == nil || (!strings.Contains(err.Error(), "身份") && !strings.Contains(strings.ToLower(err.Error()), "identity")) {
		t.Fatalf("identity change error=%v", err)
	}
}

func TestValidateRuntimePatchPartyEntityRejectsImpossibleValues(t *testing.T) {
	dodge := uint32(3)
	sba := float32(50)
	maxSBA := float32(100)
	valid := RuntimePatchPartyEntity{
		Role: "player", Present: true, HP: 100, MaxHP: 200,
		DodgeCount: &dodge, SBA: &sba, MaxSBA: &maxSBA,
		Position:     RuntimePatchVector3{X: 1, Y: 2, Z: 3},
		Capabilities: RuntimePatchPartyCapabilities{Dodge: true, SBA: true},
	}
	if err := validateRuntimePatchPartyEntity(valid); err != nil {
		t.Fatalf("valid entity rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*RuntimePatchPartyEntity)
	}{
		{name: "hp above max", mutate: func(v *RuntimePatchPartyEntity) { v.HP = v.MaxHP + 1 }},
		{name: "zero max hp", mutate: func(v *RuntimePatchPartyEntity) { v.MaxHP = 0 }},
		{name: "impossible max hp", mutate: func(v *RuntimePatchPartyEntity) { v.MaxHP = runtimePatchPartyMaximumPlausibleHP + 1 }},
		{name: "nan sba", mutate: func(v *RuntimePatchPartyEntity) { value := float32(math.NaN()); v.SBA = &value }},
		{name: "infinite max sba", mutate: func(v *RuntimePatchPartyEntity) { value := float32(math.Inf(1)); v.MaxSBA = &value }},
		{name: "sba above max", mutate: func(v *RuntimePatchPartyEntity) { value := float32(101); v.SBA = &value }},
		{name: "invalid position", mutate: func(v *RuntimePatchPartyEntity) { v.Position.X = float32(math.Inf(-1)) }},
		{name: "position out of world bounds", mutate: func(v *RuntimePatchPartyEntity) { v.Position.Z = runtimePatchPartyMaximumCoordinateMagnitude + 1 }},
		{name: "missing dodge capability value", mutate: func(v *RuntimePatchPartyEntity) { v.DodgeCount = nil }},
		{name: "unexpected companion dodge", mutate: func(v *RuntimePatchPartyEntity) { v.Capabilities.Dodge = false }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if err := validateRuntimePatchPartyEntity(candidate); err == nil {
				t.Fatalf("invalid entity accepted: %+v", candidate)
			}
		})
	}
}

func TestRuntimePatchPartyMonitorOwnedUsesCharaOwnerProcessLease(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "runtime_party_monitor.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body *ast.BlockStmt
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "RuntimePatchPartyMonitorOwned" {
			body = fn.Body
			break
		}
	}
	if body == nil || !blockCallsSelector(body, "a", "acquireOwnedRuntimeWriteLease") {
		t.Fatal("RuntimePatchPartyMonitorOwned must validate the Chara owner while pinning PID/Created and hProcess")
	}
}
