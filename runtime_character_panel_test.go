package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

type fakeRuntimePanelMemory struct {
	bytes map[uintptr]byte
}

type loadoutRuntimePanelStatsAPI interface {
	LoadoutRuntimePanelStats(charaHex string) (*RuntimeCharacterPanelStats, error)
}

var _ loadoutRuntimePanelStatsAPI = (*App)(nil)

func newFakeRuntimePanelMemory() *fakeRuntimePanelMemory {
	return &fakeRuntimePanelMemory{bytes: make(map[uintptr]byte)}
}

func (m *fakeRuntimePanelMemory) ReadAt(address uintptr, destination []byte) error {
	for index := range destination {
		value, ok := m.bytes[address+uintptr(index)]
		if !ok {
			return fmt.Errorf("unmapped read at 0x%X", address+uintptr(index))
		}
		destination[index] = value
	}
	return nil
}

func (m *fakeRuntimePanelMemory) put(address uintptr, value []byte) {
	for index, current := range value {
		m.bytes[address+uintptr(index)] = current
	}
}

func (m *fakeRuntimePanelMemory) putU16(address uintptr, value uint16) {
	encoded := make([]byte, 2)
	binary.LittleEndian.PutUint16(encoded, value)
	m.put(address, encoded)
}

func (m *fakeRuntimePanelMemory) putU32(address uintptr, value uint32) {
	encoded := make([]byte, 4)
	binary.LittleEndian.PutUint32(encoded, value)
	m.put(address, encoded)
}

func (m *fakeRuntimePanelMemory) putI32(address uintptr, value int32) {
	m.putU32(address, uint32(value))
}

func (m *fakeRuntimePanelMemory) putF32(address uintptr, value float32) {
	m.putU32(address, math.Float32bits(value))
}

func (m *fakeRuntimePanelMemory) putPtr(address, value uintptr) {
	encoded := make([]byte, 8)
	binary.LittleEndian.PutUint64(encoded, uint64(value))
	m.put(address, encoded)
}

type runtimePanelFixture struct {
	memory     *fakeRuntimePanelMemory
	moduleBase uintptr
	manager    uintptr
	vector     uintptr
	table      uintptr
	sentinel   uintptr
}

func newRuntimePanelFixture() runtimePanelFixture {
	fixture := runtimePanelFixture{
		memory:     newFakeRuntimePanelMemory(),
		moduleBase: 0x140000000,
		manager:    0x200000000,
		vector:     0x210000000,
		table:      0x220000000,
		sentinel:   0x230000000,
	}
	for _, guard := range runtimeCharacterPanelVersionGuards {
		fixture.memory.put(fixture.moduleBase+guard.RVA, guard.Bytes)
	}
	fixture.memory.putPtr(fixture.moduleBase+runtimeCharacterPanelManagerRVA, fixture.manager)
	fixture.memory.putPtr(fixture.manager+runtimeCharacterPanelVectorBeginOffset, fixture.vector)
	fixture.memory.putPtr(fixture.manager+runtimeCharacterPanelVectorEndOffset, fixture.vector)
	fixture.memory.putPtr(fixture.manager+runtimeCharacterPanelSentinelOffset, fixture.sentinel)
	fixture.memory.putPtr(fixture.manager+runtimeCharacterPanelBucketTableOffset, fixture.table)
	fixture.memory.putU32(fixture.manager+runtimeCharacterPanelBucketMaskOffset, 3)
	for bucket := uintptr(0); bucket < 4; bucket++ {
		fixture.memory.putPtr(fixture.table+bucket*runtimeCharacterPanelBucketStride+runtimeCharacterPanelBucketLastOffset, fixture.sentinel)
		fixture.memory.putPtr(fixture.table+bucket*runtimeCharacterPanelBucketStride+runtimeCharacterPanelBucketHeadOffset, fixture.sentinel)
	}
	return fixture
}

func (fixture runtimePanelFixture) setIDs(ids ...uint32) {
	for index, id := range ids {
		fixture.memory.putU32(fixture.vector+uintptr(index)*4, id)
	}
	fixture.memory.putPtr(fixture.manager+runtimeCharacterPanelVectorEndOffset, fixture.vector+uintptr(len(ids))*4)
}

func (fixture runtimePanelFixture) addStatus(id uint32, node, status uintptr, characterHash uint32, hp, attack int32, stun, crit float32) {
	bucket := uintptr(id & 3)
	fixture.memory.putPtr(fixture.table+bucket*runtimeCharacterPanelBucketStride+runtimeCharacterPanelBucketLastOffset, node)
	fixture.memory.putPtr(fixture.table+bucket*runtimeCharacterPanelBucketStride+runtimeCharacterPanelBucketHeadOffset, node)
	fixture.memory.putPtr(node+runtimeCharacterPanelNodeNextOffset, fixture.sentinel)
	fixture.memory.putU32(node+runtimeCharacterPanelNodeKeyOffset, id)
	fixture.memory.putPtr(node+runtimeCharacterPanelNodeStatusOffset, status)
	fixture.memory.putI32(status+runtimeCharacterPanelHPOffset, hp)
	fixture.memory.putI32(status+runtimeCharacterPanelAttackOffset, attack)
	fixture.memory.putF32(status+runtimeCharacterPanelStunOffset, stun)
	fixture.memory.putF32(status+runtimeCharacterPanelCritOffset, crit)
	fixture.memory.putU32(status+runtimeCharacterPanelCharacterHashOffset, characterHash)
	fixture.memory.putU16(status+runtimeCharacterPanelReadyOffset, 1)
	fixture.memory.putU16(status+runtimeCharacterPanelEligibilityOffset, 1)
}

func TestReadRuntimeCharacterPanelReturnsExactGameValuesForRequestedCharacter(t *testing.T) {
	fixture := newRuntimePanelFixture()
	fixture.setIDs(0x11, 0x22)
	fixture.addStatus(0x11, 0x240000000, 0x250000000, 0xAABBCCDD, 188975, 101641, 29.325, 83.5)
	fixture.addStatus(0x22, 0x240001000, 0x250010000, 0x10203040, 43210, 9876, 77, 25)

	got, err := readRuntimeCharacterPanel(fixture.memory, fixture.moduleBase, 0xAABBCCDD)
	if err != nil {
		t.Fatal(err)
	}
	if got.CharacterHash != "AABBCCDD" || got.RuntimeID != "00000011" || got.HP != 188975 || got.Attack != 101641 || got.StunPower != 293.25 || got.RawStunPower != 29.325 || got.CritRate != 83.5 {
		t.Fatalf("runtime panel values or identity were not read with the verified display scale: %+v", got)
	}
	if got.StunField.RawType != "f32" || got.StunField.RelativeOffset != 0x10 || got.StunField.DisplayScale != 10 || got.StunField.StableReads != 1 {
		t.Fatalf("stun field evidence is incomplete: %+v", got.StunField)
	}
	if got.RuntimeVerified || got.Source != "game_runtime_2.0.2" || got.Verification != runtimeCharacterPanelVerification || got.GameVersion != "2.0.2" {
		t.Fatalf("one snapshot must identify its source but remain unverified until the stability gate: %+v", got)
	}
}

func TestReadRuntimeWeaponWrightstoneUsesEffectiveRuntimeLevels(t *testing.T) {
	memory := newFakeRuntimePanelMemory()
	status := uintptr(0x250000000)
	memory.put(status, make([]byte, runtimeCharacterEffectiveWeaponSkillOffset+runtimeCharacterEffectiveWeaponSkillCount*8))
	memory.putU32(status+runtimeCharacterWeaponSlotOffset, 52)
	memory.putU32(status+runtimeCharacterWeaponHashOffset, 0x1779CD60)
	memory.putU32(status+runtimeCharacterWrightstoneTraitOffset, 0xCEB700EE)
	memory.putU32(status+runtimeCharacterWrightstoneTraitOffset+4, 20)
	memory.putU32(status+runtimeCharacterWrightstoneTraitOffset+8, 0x57AB5B10)
	memory.putU32(status+runtimeCharacterWrightstoneTraitOffset+12, 15)
	memory.putU32(status+runtimeCharacterWrightstoneTraitOffset+16, 0x8D78A19B)
	memory.putU32(status+runtimeCharacterWrightstoneTraitOffset+20, 10)
	memory.putU32(status+runtimeCharacterWrightstoneHashOffset, 0x09E6F629)
	memory.putU32(status+runtimeCharacterEffectiveWeaponSkillOffset, 0x1E1CECCE)
	memory.putU32(status+runtimeCharacterEffectiveWeaponSkillOffset+4, 32)
	memory.putU32(status+runtimeCharacterEffectiveWeaponSkillOffset+8, 0x7CCFF74F)
	memory.putU32(status+runtimeCharacterEffectiveWeaponSkillOffset+12, 22)

	snapshot, err := readRuntimeWeaponWrightstoneSnapshot(memory, status)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.WeaponSlotID != 52 || snapshot.WeaponHash != 0x1779CD60 || snapshot.WrightstoneHash != 0x09E6F629 {
		t.Fatalf("runtime weapon identity = %+v", snapshot)
	}
	want := []runtimeWeaponTrait{{Hash: 0xCEB700EE, Level: 20}, {Hash: 0x57AB5B10, Level: 15}, {Hash: 0x8D78A19B, Level: 10}}
	if len(snapshot.Traits) != len(want) {
		t.Fatalf("runtime wrightstone traits = %+v", snapshot.Traits)
	}
	for index := range want {
		if snapshot.Traits[index] != want[index] {
			t.Fatalf("runtime wrightstone trait %d = %+v, want %+v", index, snapshot.Traits[index], want[index])
		}
	}
	wantSkills := []runtimeWeaponTrait{{Hash: 0x1E1CECCE, Level: 32}, {Hash: 0x7CCFF74F, Level: 22}}
	if len(snapshot.Skills) != len(wantSkills) {
		t.Fatalf("runtime weapon skills = %+v", snapshot.Skills)
	}
	for index := range wantSkills {
		if snapshot.Skills[index] != wantSkills[index] {
			t.Fatalf("runtime weapon skill %d = %+v, want %+v", index, snapshot.Skills[index], wantSkills[index])
		}
	}
}

func TestRuntimeWeaponSnapshotOffsetsMatchObserved202RuntimeLayout(t *testing.T) {
	if runtimeCharacterWeaponSlotOffset != 0x50 || runtimeCharacterWeaponHashOffset != 0x54 ||
		runtimeCharacterWrightstoneTraitOffset != 0x70 || runtimeCharacterWrightstoneHashOffset != 0x88 ||
		runtimeCharacterEffectiveWeaponSkillOffset != 0xF4 {
		t.Fatalf("unexpected runtime weapon layout: slot=%#x hash=%#x traits=%#x stone=%#x skills=%#x",
			runtimeCharacterWeaponSlotOffset, runtimeCharacterWeaponHashOffset,
			runtimeCharacterWrightstoneTraitOffset, runtimeCharacterWrightstoneHashOffset,
			runtimeCharacterEffectiveWeaponSkillOffset)
	}
}

func TestReadRuntimeCharacterGrowthSnapshotUsesCharacterSpecificRuntimeValues(t *testing.T) {
	memory := newFakeRuntimePanelMemory()
	status := uintptr(0x250000000)
	memory.putI32(status+runtimeCharacterBaseLevelOffset, 100)
	memory.putI32(status+runtimeCharacterBaseHPOffset, 3430)
	memory.putI32(status+runtimeCharacterBaseAttackOffset, 677)
	memory.putF32(status+runtimeCharacterBaseStunOffset, 8)
	memory.putI32(status+runtimeCharacterBaseCritOffset, 5)
	memory.putI32(status+runtimeCharacterMasterHPOffset, 2400)
	memory.putI32(status+runtimeCharacterMasterAttackOffset, 1200)
	memory.putI32(status+runtimeCharacterFateHPOffset, 640)
	memory.putI32(status+runtimeCharacterFateAttackOffset, 165)
	memory.putF32(status+runtimeCharacterPermanentAttackOffset, 5271)
	memory.putF32(status+runtimeCharacterPermanentHPOffset, 32550)
	memory.putF32(status+runtimeCharacterPermanentCritOffset, 45)
	memory.putF32(status+runtimeCharacterPermanentStunOffset, 6.1)

	got, err := readRuntimeCharacterGrowthSnapshot(memory, status)
	if err != nil {
		t.Fatal(err)
	}
	if got.Level != 100 || got.BaseHP != 3430 || got.BaseATK != 677 || got.BaseStun != 8 || got.BaseCritRate != 5 ||
		got.MasterHP != 2400 || got.MasterATK != 1200 || got.FateHP != 640 || got.FateATK != 165 {
		t.Fatalf("runtime character growth fields = %+v", got)
	}
	if got.Permanent.Attack != 5271 || got.Permanent.HP != 32550 || got.Permanent.CritRate != 45 ||
		math.Abs(got.Permanent.StunPanel-61) > 0.001 {
		t.Fatalf("runtime permanent aggregate = %+v", got.Permanent)
	}
}

func TestRuntimeCharacterGrowthOffsetsMatchObserved202Layout(t *testing.T) {
	if runtimeCharacterBaseLevelOffset != 0x5B44 || runtimeCharacterBaseHPOffset != 0x5B48 ||
		runtimeCharacterBaseAttackOffset != 0x5B4C || runtimeCharacterBaseStunOffset != 0x5B54 ||
		runtimeCharacterBaseCritOffset != 0x5B58 || runtimeCharacterMasterHPOffset != 0x5B64 ||
		runtimeCharacterMasterAttackOffset != 0x5B68 || runtimeCharacterFateHPOffset != 0x5B70 ||
		runtimeCharacterFateAttackOffset != 0x5B74 {
		t.Fatal("runtime character growth offsets no longer match the verified 2.0.2 layout")
	}
}

func TestStableRuntimeCharacterGrowthRejectsChangingFateValues(t *testing.T) {
	calls := 0
	_, err := readStableRuntimeCharacterGrowthSnapshots(func() (runtimeCharacterGrowthSnapshot, error) {
		calls++
		return runtimeCharacterGrowthSnapshot{Level: 100, BaseHP: 3430, BaseATK: 677, BaseStun: 8, BaseCritRate: 5,
			MasterHP: 2400, MasterATK: 1200, FateHP: 640 + calls, FateATK: 165,
			Permanent: LoadoutPermanentPanelStats{Attack: 5271, HP: 32550, CritRate: 45, StunRaw: 6.1, StunPanel: 61}}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "连续三次") {
		t.Fatalf("changing runtime character growth must fail stability gate: %v", err)
	}
}

func TestLocateRuntimeCharacterPanelStatusReturnsTheMatchedStatusObject(t *testing.T) {
	fixture := newRuntimePanelFixture()
	fixture.setIDs(0x11, 0x22)
	wantStatus := uintptr(0x250010000)
	fixture.addStatus(0x11, 0x240000000, 0x250000000, 0x10203040, 1, 2, 3, 4)
	fixture.addStatus(0x22, 0x240001000, wantStatus, 0xAABBCCDD, 5, 6, 7, 8)

	status, err := locateRuntimeCharacterPanelStatus(fixture.memory, fixture.moduleBase, 0xAABBCCDD)
	if err != nil {
		t.Fatal(err)
	}
	if status != wantStatus {
		t.Fatalf("status object = 0x%X, want 0x%X", status, wantStatus)
	}
}

func TestLocateRuntimeCharacterPanelStatusUsesMapKeyWhenObjectHashIsUnavailable(t *testing.T) {
	fixture := newRuntimePanelFixture()
	fixture.setIDs(0xAABBCCDD)
	wantStatus := uintptr(0x250000000)
	// Field evidence from the live 2.0.2 build shows that the map key is the
	// directory hash while the old +0x59F0 candidate is zero for every object.
	fixture.addStatus(0xAABBCCDD, 0x240000000, wantStatus, 0, 188975, 101641, 29.3, 83.5)

	status, err := locateRuntimeCharacterPanelStatus(fixture.memory, fixture.moduleBase, 0xAABBCCDD)
	if err != nil {
		t.Fatal(err)
	}
	if status != wantStatus {
		t.Fatalf("status object = 0x%X, want map-key object 0x%X", status, wantStatus)
	}
}

func TestLocateRuntimeCharacterPanelStatusDoesNotUseReadyEligibilityAsEnumerationFilters(t *testing.T) {
	fixture := newRuntimePanelFixture()
	fixture.setIDs(0xAABBCCDD)
	wantStatus := uintptr(0x250000000)
	fixture.addStatus(0xAABBCCDD, 0x240000000, wantStatus, 0, 188975, 101641, 29.3, 83.5)
	fixture.memory.putU16(wantStatus+runtimeCharacterPanelReadyOffset, 0)
	fixture.memory.putU16(wantStatus+runtimeCharacterPanelEligibilityOffset, 0)

	status, err := locateRuntimeCharacterPanelStatus(fixture.memory, fixture.moduleBase, 0xAABBCCDD)
	if err != nil {
		t.Fatal(err)
	}
	if status != wantStatus {
		t.Fatalf("status object = 0x%X, want usable exact-key object 0x%X", status, wantStatus)
	}
}

func TestLocateRuntimeCharacterPanelStatusChoosesExactMapKeyAmongMultipleObjects(t *testing.T) {
	fixture := newRuntimePanelFixture()
	fixture.setIDs(0x10203040, 0xAABBCCDD)
	fixture.addStatus(0x10203040, 0x240000000, 0x250000000, 0, 43210, 9876, 7.7, 25)
	wantStatus := uintptr(0x250010000)
	fixture.addStatus(0xAABBCCDD, 0x240001000, wantStatus, 0, 188975, 101641, 29.3, 83.5)

	status, err := locateRuntimeCharacterPanelStatus(fixture.memory, fixture.moduleBase, 0xAABBCCDD)
	if err != nil {
		t.Fatal(err)
	}
	if status != wantStatus {
		t.Fatalf("status object = 0x%X, want exact map-key object 0x%X", status, wantStatus)
	}
}

func TestRuntimeCharacterPanelDiagnosticsExposeMappingFieldsWithoutAddresses(t *testing.T) {
	fixture := newRuntimePanelFixture()
	fixture.setIDs(0x4D0A60C3)
	fixture.addStatus(0x4D0A60C3, 0x240000000, 0x250000000, 0, 132977, 111259, 30.599998, 124)

	diagnostic, err := enumerateRuntimeCharacterPanelDiagnostics(fixture.memory, fixture.moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostic.Objects) != 1 {
		t.Fatalf("diagnostic objects = %d, want 1", len(diagnostic.Objects))
	}
	object := diagnostic.Objects[0]
	if object.DirectoryName != "伊欧" || object.DirectoryHash != "4D0A60C3" || object.RuntimeID != "4D0A60C3" || object.MapKey != "4D0A60C3" || object.CandidateObjectHash != "00000000" {
		t.Fatalf("diagnostic identity mapping is incomplete: %+v", object)
	}
	if object.Panel == nil || object.Panel.StunPower != 306 || object.Panel.RawStunPower != 30.599998 || object.Panel.StunField.RelativeOffset != 0x10 || object.Panel.StunField.StableReads != 3 {
		t.Fatalf("diagnostic panel evidence is incomplete: %+v", object.Panel)
	}
	encoded, err := json.Marshal(diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"0x250000000", "moduleBase", "statusAddress", "pid"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("diagnostic JSON leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestRuntimeCharacterPanelLayoutDescriptorIsVersionedAndEvidenceBound(t *testing.T) {
	layout := runtimeCharacterPanelLayoutDescriptor()
	if layout.SchemaVersion != 1 || layout.LayoutID == "" || layout.GameExecutableSHA256 != runtimeCharacterPanelGameEXESHA256 || len(layout.Guards) != len(runtimeCharacterPanelVersionGuards) {
		t.Fatalf("layout identity is incomplete: %+v", layout)
	}
	if len(layout.AccessChain) < 4 || len(layout.Fields) != 17 {
		t.Fatalf("layout access chain/fields are incomplete: %+v", layout)
	}
	if layout.Fields[2].Name != "stunPower" || layout.Fields[2].RawType != "f32" || layout.Fields[2].RelativeOffset != "0x10" || layout.Fields[2].DisplayScale != 10 || layout.Fields[2].SampleRoleCount < 2 {
		t.Fatalf("stun layout evidence is incomplete: %+v", layout.Fields[2])
	}
	if layout.Fields[15].Name != "fateHP" || layout.Fields[15].RelativeOffset != "0x5B70" || layout.Fields[15].SampleRoleCount != 2 {
		t.Fatalf("runtime Fate layout evidence is incomplete: %+v", layout.Fields[15])
	}
}

func TestStableRuntimeCharacterPanelRequiresThreeIdenticalSnapshots(t *testing.T) {
	want := RuntimeCharacterPanelStats{
		CharacterHash:   "AABBCCDD",
		HP:              188975,
		Attack:          101641,
		StunPower:       293.25,
		CritRate:        83.5,
		Source:          runtimeCharacterPanelSource,
		Verification:    runtimeCharacterPanelVerification,
		GameVersion:     "2.0.2",
		RuntimeVerified: true,
	}
	want.HPField.StableReads = 3
	want.AttackField.StableReads = 3
	want.StunField.StableReads = 3
	want.CritField.StableReads = 3
	snapshot := want
	snapshot.RuntimeVerified = false
	calls := 0
	got, err := readStableRuntimeCharacterPanelSnapshots(func() (RuntimeCharacterPanelStats, error) {
		calls++
		return snapshot, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || got != want {
		t.Fatalf("stable runtime read used %d snapshots and returned %+v, want three and %+v", calls, got, want)
	}
}

func TestStableRuntimeCharacterPanelRejectsChangingSnapshots(t *testing.T) {
	snapshots := []RuntimeCharacterPanelStats{
		{CharacterHash: "AABBCCDD", HP: 100, Attack: 200, StunPower: 3, CritRate: 4},
		{CharacterHash: "AABBCCDD", HP: 101, Attack: 200, StunPower: 3, CritRate: 4},
		{CharacterHash: "AABBCCDD", HP: 100, Attack: 200, StunPower: 3, CritRate: 4},
	}
	calls := 0
	_, err := readStableRuntimeCharacterPanelSnapshots(func() (RuntimeCharacterPanelStats, error) {
		current := snapshots[calls]
		calls++
		return current, nil
	})
	if err == nil || !strings.Contains(err.Error(), "3 次") || !strings.Contains(err.Error(), "变化") {
		t.Fatalf("changing snapshots must fail closed, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("stability check took %d snapshots, want exactly 3", calls)
	}
}

func TestReadRuntimeCharacterPanelRejectsWrongGameBuildBeforeFollowingPointers(t *testing.T) {
	fixture := newRuntimePanelFixture()
	fixture.memory.put(fixture.moduleBase+runtimeCharacterPanelVersionGuards[0].RVA, make([]byte, len(runtimeCharacterPanelVersionGuards[0].Bytes)))

	_, err := readRuntimeCharacterPanel(fixture.memory, fixture.moduleBase, 0xAABBCCDD)
	if err == nil || !strings.Contains(err.Error(), "2.0.2") || !strings.Contains(err.Error(), "版本守卫") {
		t.Fatalf("wrong game build must fail closed with a useful error, got %v", err)
	}
}

func TestReadRuntimeCharacterPanelSkipsUnreadyAndIneligibleStatuses(t *testing.T) {
	fixture := newRuntimePanelFixture()
	fixture.setIDs(1, 2, 3)
	fixture.addStatus(1, 0x240000000, 0x250000000, 0xAABBCCDD, 1, 1, 1, 1)
	fixture.memory.putU16(0x250000000+runtimeCharacterPanelReadyOffset, 0)
	fixture.addStatus(2, 0x240001000, 0x250010000, 0xAABBCCDD, 2, 2, 2, 2)
	fixture.memory.putU16(0x250010000+runtimeCharacterPanelEligibilityOffset, 0)
	fixture.addStatus(3, 0x240002000, 0x250020000, 0xAABBCCDD, 333, 444, 55, 66)

	got, err := readRuntimeCharacterPanel(fixture.memory, fixture.moduleBase, 0xAABBCCDD)
	if err != nil {
		t.Fatal(err)
	}
	if got.HP != 333 || got.Attack != 444 {
		t.Fatalf("reader selected an unready/ineligible status: %+v", got)
	}
}

func TestReadRuntimeCharacterPanelTreatsReadyAndEligibilityAsSingleByteFlags(t *testing.T) {
	fixture := newRuntimePanelFixture()
	fixture.setIDs(1, 2)
	fixture.addStatus(1, 0x240000000, 0x250000000, 0xAABBCCDD, 111, 111, 11, 11)
	// eligibility (+5EBE) is zero; the unrelated byte at +5EBF must not make it
	// eligible when the game stores these flags with byte writes.
	fixture.memory.put(0x250000000+runtimeCharacterPanelEligibilityOffset, []byte{0, 1})
	fixture.addStatus(2, 0x240001000, 0x250010000, 0xAABBCCDD, 222, 333, 44, 55)
	// ready (+5EBC) is one; the unrelated byte at +5EBD must not make it look
	// like the uint16 value 0xFF01.
	fixture.memory.put(0x250010000+runtimeCharacterPanelReadyOffset, []byte{1, 0xFF})

	got, err := readRuntimeCharacterPanel(fixture.memory, fixture.moduleBase, 0xAABBCCDD)
	if err != nil {
		t.Fatal(err)
	}
	if got.HP != 222 || got.Attack != 333 {
		t.Fatalf("single-byte flags were polluted by adjacent bytes: %+v", got)
	}
}

func TestReadRuntimeCharacterPanelRejectsCorruptContainers(t *testing.T) {
	t.Run("oversized vector", func(t *testing.T) {
		fixture := newRuntimePanelFixture()
		fixture.memory.putPtr(fixture.manager+runtimeCharacterPanelVectorEndOffset, fixture.vector+uintptr(runtimeCharacterPanelMaxIDs+1)*4)
		_, err := readRuntimeCharacterPanel(fixture.memory, fixture.moduleBase, 0xAABBCCDD)
		if err == nil || !strings.Contains(err.Error(), "角色 ID 向量") {
			t.Fatalf("oversized vector must fail closed, got %v", err)
		}
	})

	t.Run("cyclic bucket chain", func(t *testing.T) {
		fixture := newRuntimePanelFixture()
		fixture.setIDs(1)
		node := uintptr(0x240000000)
		fixture.memory.putPtr(fixture.table+runtimeCharacterPanelBucketStride+runtimeCharacterPanelBucketLastOffset, 0x240001000)
		fixture.memory.putPtr(fixture.table+runtimeCharacterPanelBucketStride+runtimeCharacterPanelBucketHeadOffset, node)
		fixture.memory.putPtr(node+runtimeCharacterPanelNodeNextOffset, node)
		fixture.memory.putU32(node+runtimeCharacterPanelNodeKeyOffset, 9)
		status := uintptr(0x250000000)
		fixture.memory.putPtr(node+runtimeCharacterPanelNodeStatusOffset, status)
		fixture.memory.putU16(status+runtimeCharacterPanelReadyOffset, 1)
		fixture.memory.putU16(status+runtimeCharacterPanelEligibilityOffset, 1)
		fixture.memory.putU32(status+runtimeCharacterPanelCharacterHashOffset, 0)
		_, err := readRuntimeCharacterPanel(fixture.memory, fixture.moduleBase, 0xAABBCCDD)
		if err == nil || !strings.Contains(err.Error(), "循环") {
			t.Fatalf("cyclic chain must fail closed, got %v", err)
		}
	})
}

func TestReadRuntimeCharacterPanelRejectsNaNAndImpossibleValues(t *testing.T) {
	tests := []struct {
		name string
		hp   int32
		atk  int32
		stun float32
		crit float32
	}{
		{name: "negative HP", hp: -1, atk: 10, stun: 1, crit: 1},
		{name: "zero HP", hp: 0, atk: 10, stun: 1, crit: 1},
		{name: "impossible attack", hp: 10, atk: 1000000, stun: 1, crit: 1},
		{name: "impossible stun", hp: 10, atk: 10, stun: 1000, crit: 1},
		{name: "impossible crit", hp: 10, atk: 10, stun: 1, crit: 1000},
		{name: "NaN stun", hp: 10, atk: 10, stun: float32(math.NaN()), crit: 1},
		{name: "infinite crit", hp: 10, atk: 10, stun: 1, crit: float32(math.Inf(1))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRuntimePanelFixture()
			fixture.setIDs(1)
			fixture.addStatus(1, 0x240000000, 0x250000000, 0xAABBCCDD, test.hp, test.atk, test.stun, test.crit)
			_, err := readRuntimeCharacterPanel(fixture.memory, fixture.moduleBase, 0xAABBCCDD)
			if err == nil || !strings.Contains(err.Error(), "异常") {
				t.Fatalf("invalid panel data must fail closed, got %v", err)
			}
		})
	}
}
