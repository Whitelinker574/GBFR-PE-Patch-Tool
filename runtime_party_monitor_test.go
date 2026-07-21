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

func putRuntimePatchPartySignature(t *testing.T, memory *fakeRuntimePanelMemory, moduleBase uintptr) {
	t.Helper()
	pattern, err := parseRuntimePatchPattern(runtimePatchPartyPointerAOB)
	if err != nil {
		t.Fatal(err)
	}
	bytes := append([]byte(nil), pattern.Values...)
	site := moduleBase + runtimePatchPartyPointerRVA
	root := moduleBase + runtimePatchPartySlotTableRVA
	displacement := int64(root) - int64(site+7)
	if displacement < math.MinInt32 || displacement > math.MaxInt32 {
		t.Fatalf("party RIP displacement out of range: %d", displacement)
	}
	binary.LittleEndian.PutUint32(bytes[3:7], uint32(int32(displacement)))
	memory.put(site, bytes)
}

func newRuntimePatchPartyFixture(t *testing.T) (*fakeRuntimePanelMemory, uintptr) {
	t.Helper()
	memory := newFakeRuntimePanelMemory()
	moduleBase := uintptr(0x10000000)
	putRuntimePatchPartySignature(t, memory, moduleBase)

	root := moduleBase + runtimePatchPartySlotTableRVA
	entities := [...]uintptr{0x21000000, 0x22000000, 0x23000000, 0x24000000}
	for index, entity := range entities {
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
	if snapshot.Topology.Root != moduleBase+runtimePatchPartySlotTableRVA {
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

func TestReadRuntimePatchPartySnapshotAcceptsEmptyTrainingPartySlots(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	root := moduleBase + runtimePatchPartySlotTableRVA
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

func TestReadRuntimePatchPartySnapshotAcceptsMissingCompanion(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	root := moduleBase + runtimePatchPartySlotTableRVA
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
	memory.putPtr(moduleBase+runtimePatchPartySlotTableRVA, 0)
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
