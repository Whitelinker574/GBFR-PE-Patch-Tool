package main

import (
	"encoding/binary"
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

func putCT084PartySignature(t *testing.T, memory *fakeRuntimePanelMemory, moduleBase uintptr) {
	t.Helper()
	pattern, err := parseCT084Pattern(ct084PartyPointerAOB)
	if err != nil {
		t.Fatal(err)
	}
	bytes := append([]byte(nil), pattern.Values...)
	site := moduleBase + ct084PartyPointerRVA
	root := moduleBase + ct084PartySlotTableRVA
	displacement := int64(root) - int64(site+7)
	if displacement < math.MinInt32 || displacement > math.MaxInt32 {
		t.Fatalf("party RIP displacement out of range: %d", displacement)
	}
	binary.LittleEndian.PutUint32(bytes[3:7], uint32(int32(displacement)))
	memory.put(site, bytes)
}

func newCT084PartyFixture(t *testing.T) (*fakeRuntimePanelMemory, uintptr) {
	t.Helper()
	memory := newFakeRuntimePanelMemory()
	moduleBase := uintptr(0x10000000)
	putCT084PartySignature(t, memory, moduleBase)

	root := moduleBase + ct084PartySlotTableRVA
	entities := [...]uintptr{0x21000000, 0x22000000, 0x23000000, 0x24000000}
	for index, entity := range entities {
		memory.putPtr(root+uintptr(index)*8, entity)
		memory.putPtr(entity+ct084PartyTransformRootOffset, entity+0x6000)
		memory.putPtr(entity+0x6000+ct084PartyTransformNodeOffset, entity+0x7000)
		memory.putU64(entity+ct084PartyHPOffset, uint64(1000+index))
		memory.putU64(entity+ct084PartyMaxHPOffset, uint64(2000+index))
		memory.putU32(entity+ct084PartyDodgeOffset, uint32(3+index))
		memory.putF32(entity+ct084PartySBAOffset, float32(20+index))
		memory.putF32(entity+ct084PartyMaxSBAOffset, 100)
		memory.putF32(entity+0x7000+ct084PartyPositionXOffset, float32(10+index))
		memory.putF32(entity+0x7000+ct084PartyPositionYOffset, float32(20+index))
		memory.putF32(entity+0x7000+ct084PartyPositionZOffset, float32(30+index))
	}

	companionContainer := uintptr(0x25000000)
	companion := uintptr(0x26000000)
	memory.putPtr(root+ct084PartyCompanionSlotOffset, companionContainer)
	memory.putPtr(companionContainer+ct084PartyCompanionEntityOffset, companion)
	memory.putPtr(companion+ct084PartyTransformRootOffset, companion+0x6000)
	memory.putPtr(companion+0x6000+ct084PartyTransformNodeOffset, companion+0x7000)
	memory.putU64(companion+ct084PartyHPOffset, 500)
	memory.putU64(companion+ct084PartyMaxHPOffset, 900)
	memory.putF32(companion+0x7000+ct084PartyPositionXOffset, 41)
	memory.putF32(companion+0x7000+ct084PartyPositionYOffset, 42)
	memory.putF32(companion+0x7000+ct084PartyPositionZOffset, 43)
	memory.putF32(companion+ct084PartyCompanionDirectXOffset, 51)
	memory.putF32(companion+ct084PartyCompanionDirectYOffset, 52)
	memory.putF32(companion+ct084PartyCompanionDirectZOffset, 53)
	return memory, moduleBase
}

func TestReadCT084PartySnapshotUsesVerified202LayoutAndOptionalCompanionFields(t *testing.T) {
	memory, moduleBase := newCT084PartyFixture(t)
	snapshot, err := readCT084PartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Topology.Root != moduleBase+ct084PartySlotTableRVA {
		t.Fatalf("root=0x%X", snapshot.Topology.Root)
	}
	if got, want := len(snapshot.Result.Entities), 5; got != want {
		t.Fatalf("entity count=%d, want %d", got, want)
	}
	player := snapshot.Result.Entities[0]
	if !player.Present || player.Role != "player" || player.HP != 1000 || player.MaxHP != 2000 || player.DodgeCount == nil || *player.DodgeCount != 3 || player.SBA == nil || *player.SBA != 20 {
		t.Fatalf("player=%+v", player)
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

func TestReadCT084PartySnapshotAcceptsEmptyTrainingPartySlots(t *testing.T) {
	memory, moduleBase := newCT084PartyFixture(t)
	root := moduleBase + ct084PartySlotTableRVA
	for index := 1; index < 4; index++ {
		memory.putPtr(root+uintptr(index)*8, 0)
	}

	snapshot, err := readCT084PartySnapshot(memory, moduleBase)
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

func TestReadCT084PartySnapshotAcceptsMissingCompanion(t *testing.T) {
	memory, moduleBase := newCT084PartyFixture(t)
	root := moduleBase + ct084PartySlotTableRVA
	memory.putPtr(root+ct084PartyCompanionSlotOffset, 0)

	snapshot, err := readCT084PartySnapshot(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	companion := snapshot.Result.Entities[4]
	if companion.Present || companion.Role != "companion" || companion.Address != 0 {
		t.Fatalf("missing companion=%+v", companion)
	}
}

func TestReadCT084PartySnapshotStillRejectsMissingPlayer(t *testing.T) {
	memory, moduleBase := newCT084PartyFixture(t)
	memory.putPtr(moduleBase+ct084PartySlotTableRVA, 0)
	_, err := readCT084PartySnapshot(memory, moduleBase)
	if err == nil || (!strings.Contains(err.Error(), "玩家") && !strings.Contains(strings.ToLower(err.Error()), "player")) {
		t.Fatalf("missing player error=%v", err)
	}
}

func TestReadStableCT084PartySnapshotsAcceptsDynamicValuesAndReturnsLastFrame(t *testing.T) {
	topology := ct084PartyTopology{Root: 0x100, Entities: [5]uintptr{1, 2, 3, 4, 5}}
	frames := []ct084PartySnapshot{
		{Topology: topology, Result: CT084PartyMonitor{Entities: []CT084PartyEntity{{Role: "player", HP: 10, MaxHP: 100}}}},
		{Topology: topology, Result: CT084PartyMonitor{Entities: []CT084PartyEntity{{Role: "player", HP: 20, MaxHP: 100}}}},
		{Topology: topology, Result: CT084PartyMonitor{Entities: []CT084PartyEntity{{Role: "player", HP: 30, MaxHP: 100}}}},
	}
	index := 0
	result, err := readStableCT084PartySnapshots(func() (ct084PartySnapshot, error) {
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

func TestReadStableCT084PartySnapshotsRejectsTopologyChanges(t *testing.T) {
	base := ct084PartyTopology{Root: 0x100, Entities: [5]uintptr{1, 2, 3, 4, 5}}
	changed := base
	changed.TransformNodes[2][1] = 0xDEAD
	frames := []ct084PartySnapshot{{Topology: base}, {Topology: changed}, {Topology: changed}}
	index := 0
	_, err := readStableCT084PartySnapshots(func() (ct084PartySnapshot, error) {
		frame := frames[index]
		index++
		return frame, nil
	})
	if err == nil || (!strings.Contains(err.Error(), "拓扑") && !strings.Contains(strings.ToLower(err.Error()), "topology")) {
		t.Fatalf("topology change error=%v", err)
	}
}

func TestValidateCT084PartyEntityRejectsImpossibleValues(t *testing.T) {
	dodge := uint32(3)
	sba := float32(50)
	maxSBA := float32(100)
	valid := CT084PartyEntity{
		Role: "player", Present: true, HP: 100, MaxHP: 200,
		DodgeCount: &dodge, SBA: &sba, MaxSBA: &maxSBA,
		Position:     CT084Vector3{X: 1, Y: 2, Z: 3},
		Capabilities: CT084PartyCapabilities{Dodge: true, SBA: true},
	}
	if err := validateCT084PartyEntity(valid); err != nil {
		t.Fatalf("valid entity rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*CT084PartyEntity)
	}{
		{name: "hp above max", mutate: func(v *CT084PartyEntity) { v.HP = v.MaxHP + 1 }},
		{name: "zero max hp", mutate: func(v *CT084PartyEntity) { v.MaxHP = 0 }},
		{name: "impossible max hp", mutate: func(v *CT084PartyEntity) { v.MaxHP = ct084PartyMaximumPlausibleHP + 1 }},
		{name: "nan sba", mutate: func(v *CT084PartyEntity) { value := float32(math.NaN()); v.SBA = &value }},
		{name: "infinite max sba", mutate: func(v *CT084PartyEntity) { value := float32(math.Inf(1)); v.MaxSBA = &value }},
		{name: "sba above max", mutate: func(v *CT084PartyEntity) { value := float32(101); v.SBA = &value }},
		{name: "invalid position", mutate: func(v *CT084PartyEntity) { v.Position.X = float32(math.Inf(-1)) }},
		{name: "position out of world bounds", mutate: func(v *CT084PartyEntity) { v.Position.Z = ct084PartyMaximumCoordinateMagnitude + 1 }},
		{name: "missing dodge capability value", mutate: func(v *CT084PartyEntity) { v.DodgeCount = nil }},
		{name: "unexpected companion dodge", mutate: func(v *CT084PartyEntity) { v.Capabilities.Dodge = false }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if err := validateCT084PartyEntity(candidate); err == nil {
				t.Fatalf("invalid entity accepted: %+v", candidate)
			}
		})
	}
}

func TestCT084PartyMonitorOwnedUsesCharaOwnerProcessLease(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "runtime_party_monitor.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body *ast.BlockStmt
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "CT084PartyMonitorOwned" {
			body = fn.Body
			break
		}
	}
	if body == nil || !blockCallsSelector(body, "a", "acquireOwnedRuntimeWriteLease") {
		t.Fatal("CT084PartyMonitorOwned must validate the Chara owner while pinning PID/Created and hProcess")
	}
}
