package main

import (
	"encoding/binary"
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
	fixture.addStatus(0x11, 0x240000000, 0x250000000, 0xAABBCCDD, 188975, 101641, 293.25, 83.5)
	fixture.addStatus(0x22, 0x240001000, 0x250010000, 0x10203040, 43210, 9876, 77, 25)

	got, err := readRuntimeCharacterPanel(fixture.memory, fixture.moduleBase, 0xAABBCCDD)
	if err != nil {
		t.Fatal(err)
	}
	if got.CharacterHash != "AABBCCDD" || got.HP != 188975 || got.Attack != 101641 || got.StunPower != 293.25 || got.CritRate != 83.5 {
		t.Fatalf("runtime panel values were transformed instead of read exactly: %+v", got)
	}
	if got.RuntimeVerified || got.Source != "game_runtime_2.0.2" || got.Verification != runtimeCharacterPanelVerification || got.GameVersion != "2.0.2" {
		t.Fatalf("one snapshot must identify its source but remain unverified until the stability gate: %+v", got)
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
