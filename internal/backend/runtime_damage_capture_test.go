package backend

import (
	"encoding/binary"
	"math"
	"os"
	"testing"
	"time"
)

func writeDamageFixtureEvent(data []byte, capacity uint32, event RuntimeDamageEvent) {
	index := uint32((event.Sequence - 1) % uint64(capacity))
	offset := runtimeDamageHeaderSize + int(index)*runtimeDamageEventSize
	row := data[offset : offset+runtimeDamageEventSize]
	binary.LittleEndian.PutUint64(row[0:8], event.Sequence)
	binary.LittleEndian.PutUint64(row[8:16], event.TickMillis)
	binary.LittleEndian.PutUint64(row[16:24], event.SourceAddress)
	binary.LittleEndian.PutUint64(row[24:32], event.TargetAddress)
	binary.LittleEndian.PutUint32(row[32:36], uint32(event.Damage))
	binary.LittleEndian.PutUint32(row[36:40], uint32(event.DamageCap))
	binary.LittleEndian.PutUint32(row[40:44], math.Float32bits(event.BaseDamage))
	binary.LittleEndian.PutUint32(row[44:48], math.Float32bits(event.AttackRate))
	binary.LittleEndian.PutUint64(row[48:56], event.Flags)
	binary.LittleEndian.PutUint32(row[56:60], event.ActionID)
}

func TestRuntimeDamageCaptureLiveLifecycle(t *testing.T) {
	if os.Getenv("GBFR_RUNTIME_DAMAGE_QA") != "1" {
		t.Skip("set GBFR_RUNTIME_DAMAGE_QA=1 with the 2.0.2 game running")
	}
	app := &App{}
	snapshot, err := app.RuntimeDamageCaptureStart()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if stopErr := app.RuntimeDamageCaptureStop(); stopErr != nil {
			t.Errorf("stop damage capture: %v", stopErr)
		}
	}()
	if !snapshot.Active || snapshot.PID == 0 || snapshot.Version != runtimeDamageVersion || snapshot.Capacity != runtimeDamageCapacity {
		t.Fatalf("live snapshot=%+v", snapshot)
	}
	time.Sleep(150 * time.Millisecond)
	status := readRuntimeCompanionStatus("damage")
	if status.PID != snapshot.PID || status.State != "active" {
		t.Fatalf("runtime status=%+v snapshot=%+v", status, snapshot)
	}
}

func TestDecodeRuntimeDamageSharedMemoryAggregatesAndKeepsLatestWindow(t *testing.T) {
	capacity := uint32(4)
	data := make([]byte, runtimeDamageHeaderSize+int(capacity)*runtimeDamageEventSize)
	binary.LittleEndian.PutUint64(data[0:8], runtimeDamageMagic)
	binary.LittleEndian.PutUint32(data[8:12], runtimeDamageVersion)
	binary.LittleEndian.PutUint32(data[12:16], capacity)
	binary.LittleEndian.PutUint64(data[16:24], 5)
	binary.LittleEndian.PutUint64(data[24:32], 1)
	writeDamageFixtureEvent(data, capacity, RuntimeDamageEvent{Sequence: 2, TickMillis: 1000, Damage: 100, DamageCap: 100, BaseDamage: 110, AttackRate: 1, ActionID: 7})
	writeDamageFixtureEvent(data, capacity, RuntimeDamageEvent{Sequence: 3, TickMillis: 1500, Damage: 40, DamageCap: 100, BaseDamage: 40, AttackRate: 1, ActionID: 7})
	writeDamageFixtureEvent(data, capacity, RuntimeDamageEvent{Sequence: 4, TickMillis: 2000, Damage: 25, DamageCap: -1, BaseDamage: 25, AttackRate: 1, ActionID: 9, Flags: 1 << 15})
	writeDamageFixtureEvent(data, capacity, RuntimeDamageEvent{Sequence: 5, TickMillis: 2500, Damage: 200, DamageCap: 180, BaseDamage: 210, AttackRate: 2, ActionID: 11, Flags: 1 << 13})

	snapshot, err := decodeRuntimeDamageSharedMemory(data, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Events) != 3 || snapshot.Events[0].Sequence != 3 || snapshot.Events[2].Sequence != 5 {
		t.Fatalf("events=%+v", snapshot.Events)
	}
	if snapshot.TotalEvents != 5 || snapshot.DroppedEvents != 1 || snapshot.TotalDamage != 265 || snapshot.DurationMillis != 1000 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	if len(snapshot.Skills) != 3 || snapshot.Events[1].ActionType != "supplementary" || snapshot.Events[1].Cappable {
		t.Fatalf("skills/events=%+v %+v", snapshot.Skills, snapshot.Events)
	}
	if snapshot.Events[2].ActionType != "sba" || !snapshot.Events[2].Capped || snapshot.Events[2].OvercapPct <= 0 {
		t.Fatalf("sba event=%+v", snapshot.Events[2])
	}
}

func TestRuntimeDamageAggregationKeepsDifferentSourcesSeparate(t *testing.T) {
	snapshot := aggregateRuntimeDamageSnapshot(RuntimeDamageSnapshot{Events: []RuntimeDamageEvent{
		{Sequence: 1, TickMillis: 1, SourceAddress: 0x1000, Damage: 10, ActionType: "normal", ActionID: 7},
		{Sequence: 2, TickMillis: 2, SourceAddress: 0x2000, Damage: 20, ActionType: "normal", ActionID: 7},
	}})
	if len(snapshot.Skills) != 2 || snapshot.Skills[0].SourceAddress == snapshot.Skills[1].SourceAddress || snapshot.Skills[0].Key == snapshot.Skills[1].Key {
		t.Fatalf("different sources were merged: %+v", snapshot.Skills)
	}
}

func TestDecodeRuntimeDamageSharedMemoryRejectsTornAndInvalidRows(t *testing.T) {
	capacity := uint32(2)
	data := make([]byte, runtimeDamageHeaderSize+int(capacity)*runtimeDamageEventSize)
	binary.LittleEndian.PutUint64(data[0:8], runtimeDamageMagic)
	binary.LittleEndian.PutUint32(data[8:12], runtimeDamageVersion)
	binary.LittleEndian.PutUint32(data[12:16], capacity)
	binary.LittleEndian.PutUint64(data[16:24], 2)
	writeDamageFixtureEvent(data, capacity, RuntimeDamageEvent{Sequence: 1, TickMillis: 1, Damage: -1, BaseDamage: 1, AttackRate: 1})
	writeDamageFixtureEvent(data, capacity, RuntimeDamageEvent{Sequence: 7, TickMillis: 2, Damage: 10, BaseDamage: 10, AttackRate: 1})
	snapshot, err := decodeRuntimeDamageSharedMemory(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Events) != 0 {
		t.Fatalf("invalid/torn rows were accepted: %+v", snapshot.Events)
	}
}

func TestDecodeRuntimeDamageSharedMemoryUsesExactPreCapComparison(t *testing.T) {
	capacity := uint32(3)
	data := make([]byte, runtimeDamageHeaderSize+int(capacity)*runtimeDamageEventSize)
	binary.LittleEndian.PutUint64(data[0:8], runtimeDamageMagic)
	binary.LittleEndian.PutUint32(data[8:12], runtimeDamageVersion)
	binary.LittleEndian.PutUint32(data[12:16], capacity)
	binary.LittleEndian.PutUint64(data[16:24], 3)
	writeDamageFixtureEvent(data, capacity, RuntimeDamageEvent{Sequence: 1, TickMillis: 1, Damage: 95, DamageCap: 100, BaseDamage: 95, AttackRate: 1})
	writeDamageFixtureEvent(data, capacity, RuntimeDamageEvent{Sequence: 2, TickMillis: 2, Damage: 100, DamageCap: 100, BaseDamage: 100, AttackRate: 1})
	writeDamageFixtureEvent(data, capacity, RuntimeDamageEvent{Sequence: 3, TickMillis: 3, Damage: 100, DamageCap: 100, BaseDamage: 100.5, AttackRate: 1})

	snapshot, err := decodeRuntimeDamageSharedMemory(data, 3)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Events[0].Capped || snapshot.Events[1].Capped || !snapshot.Events[2].Capped {
		t.Fatalf("exact cap comparison failed: %+v", snapshot.Events)
	}
}
