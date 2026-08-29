package backend

import (
	"encoding/binary"
	"os"
	"testing"
)

// This opt-in probe opens the game with query/read rights only. It does not
// install a hook, call a game function, or write any process memory.
func TestGame205LiveReadOnlyLayout(t *testing.T) {
	if os.Getenv("GBFR_GAME_205_LIVE_READONLY") != "1" {
		t.Skip("set GBFR_GAME_205_LIVE_READONLY=1 while GAME 2.0.5 is running")
	}
	process, err := openReadOnlyGameProcessForLayouts(windowsReadOnlyProcessBackend{}, charaProcessName, runtimeCharacterPanelRuntimeLayouts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := process.Close(); err != nil {
			t.Errorf("close read-only process: %v", err)
		}
	})
	if process.version != "2.0.5" {
		t.Fatalf("read-only runtime version=%q, want 2.0.5", process.version)
	}
	layout, err := detectRuntimeGameLayout(process, process.moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	if layout.Version != "2.0.5" {
		t.Fatalf("shared runtime layout=%q, want 2.0.5", layout.Version)
	}
	for _, entry := range []struct {
		name string
		rva  uintptr
	}{{"party root slot", layout.PartySlotTableRVA}, {"entity table slot", layout.PartyEntityTableRVA},
		{"character panel manager", layout.PartyCharaPowerRVA}, {"summon inventory", layout.SummonInventoryPtrRVA},
		{"status manager", 0x7C22E40}, {"virtual sigil system data", 0x7C1EE00}} {
		raw := make([]byte, 8)
		if err := process.ReadAt(process.moduleBase+entry.rva, raw); err != nil {
			t.Fatalf("read %s RVA 0x%X: %v", entry.name, entry.rva, err)
		}
		t.Logf("%s RVA 0x%X value 0x%X", entry.name, entry.rva, binary.LittleEndian.Uint64(raw))
	}
	snapshot, err := readRuntimePatchPartySnapshot(process, process.moduleBase)
	if err != nil {
		t.Fatalf("read 2.0.5 party/loadout snapshot: %v", err)
	}
	if snapshot.Result.GameVersion != "2.0.5" || len(snapshot.Result.Entities) != 5 || !snapshot.Result.Entities[0].Present {
		t.Fatalf("unexpected 2.0.5 party/loadout snapshot: %+v", snapshot.Result)
	}
	if snapshot.Topology.EntityTable == 0 || snapshot.Topology.LoadoutHandleEntities[0] == 0 || snapshot.Topology.LoadoutHandleIDs[0] == 0 {
		t.Fatalf("2.0.5 player loadout handle did not resolve: %+v", snapshot.Topology)
	}
	loadoutAvailable, factors, characterCode := false, 0, ""
	if loadout := snapshot.Result.Entities[0].Loadout; loadout != nil {
		loadoutAvailable, factors, characterCode = loadout.Available, len(loadout.Sigils), loadout.CharacterCode
	}
	t.Logf("player=%s loadoutAvailable=%t factors=%d", characterCode, loadoutAvailable, factors)

	app := NewApp()
	info, err := app.CharaAcquire(1)
	if err != nil {
		t.Fatalf("acquire normal 2.0.5 UI connection: %v", err)
	}
	if info.PID == 0 || info.OwnerToken == "" {
		t.Fatalf("normal 2.0.5 UI connection is incomplete: %+v", info)
	}
	if err := app.CharaRelease(info.OwnerToken); err != nil {
		t.Fatalf("release normal 2.0.5 UI connection: %v", err)
	}
}
