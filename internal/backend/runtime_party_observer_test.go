package backend

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runtimePartyObserverMappingFixture(t *testing.T, sequence uint64, direction runtimePartyNetworkProfileDirection, payload []byte) []byte {
	t.Helper()
	data := make([]byte, runtimePartyObserverHeaderSize+int(runtimePartyObserverCapacity)*runtimePartyObserverEventSize)
	binary.LittleEndian.PutUint64(data[0:8], runtimePartyObserverMagic)
	binary.LittleEndian.PutUint32(data[8:12], runtimePartyObserverVersion)
	binary.LittleEndian.PutUint32(data[12:16], runtimePartyObserverCapacity)
	binary.LittleEndian.PutUint64(data[16:24], sequence)
	offset := runtimePartyObserverHeaderSize + int((sequence-1)%uint64(runtimePartyObserverCapacity))*runtimePartyObserverEventSize
	row := data[offset : offset+runtimePartyObserverEventSize]
	binary.LittleEndian.PutUint64(row[0:8], sequence)
	binary.LittleEndian.PutUint64(row[8:16], 12345)
	binary.LittleEndian.PutUint32(row[16:20], runtimePartyObserverProfileKind)
	binary.LittleEndian.PutUint32(row[20:24], uint32(direction))
	binary.LittleEndian.PutUint32(row[24:28], binary.LittleEndian.Uint32(payload[runtimePartyNetworkPartyIndexOffset:]))
	binary.LittleEndian.PutUint32(row[28:32], binary.LittleEndian.Uint32(payload[runtimePartyNetworkCharacterHashOffset:]))
	binary.LittleEndian.PutUint32(row[32:36], binary.LittleEndian.Uint32(payload[runtimePartyNetworkWeaponHashOffset:]))
	binary.LittleEndian.PutUint32(row[36:40], uint32(len(payload)))
	for index := 0; index < runtimePartyNetworkSigilCount; index++ {
		entry := row[40+index*12 : 40+(index+1)*12]
		binary.LittleEndian.PutUint32(entry[0:4], binary.LittleEndian.Uint32(payload[runtimePartyNetworkSigilHashOffset+index*4:]))
		binary.LittleEndian.PutUint32(entry[4:8], binary.LittleEndian.Uint32(payload[runtimePartyNetworkSecondaryHashOffset+index*4:]))
		binary.LittleEndian.PutUint32(entry[8:12], uint32(payload[runtimePartyNetworkSigilLevelOffset+index]))
	}
	return data
}

func TestRuntimePartyObserverSanitizedRingReconstructsVerifiedProfile(t *testing.T) {
	payload := runtimeLoadoutDetectorNetworkProfileFixture(t, 2)
	data := runtimePartyObserverMappingFixture(t, 7, runtimePartyNetworkProfileRemote, payload)
	events, next, err := decodeRuntimePartyObserverMapping(data, 6)
	if err != nil {
		t.Fatal(err)
	}
	if next != 7 || len(events) != 1 || events[0].Sequence != 7 || events[0].Direction != runtimePartyNetworkProfileRemote {
		t.Fatalf("observer ring event mismatch: next=%d events=%+v", next, events)
	}
	profile, err := parseRuntimePartyNetworkProfile(events[0].Payload)
	if err != nil {
		t.Fatal(err)
	}
	if profile.PartyIndex != 2 || profile.CharacterCode != "PL0400" || profile.WeaponHash != 0x1779CD60 || profile.Sigils[0].Hash != 0x5BF84FD1 {
		t.Fatalf("sanitized observer profile mismatch: %+v", profile)
	}
	events, next, err = decodeRuntimePartyObserverMapping(data, next)
	if err != nil || len(events) != 0 || next != 7 {
		t.Fatalf("observer replay duplicated a consumed event: next=%d events=%+v err=%v", next, events, err)
	}
}

func TestRuntimePartyObserverDoesNotAdvancePastUncommittedRow(t *testing.T) {
	payload := runtimeLoadoutDetectorNetworkProfileFixture(t, 2)
	data := runtimePartyObserverMappingFixture(t, 8, runtimePartyNetworkProfileRemote, payload)
	// The header announces sequence 8, but sequence 7 is still unpublished.
	// A later poll must be allowed to consume both instead of losing sequence 7.
	offset := runtimePartyObserverHeaderSize + int((7-1)%uint64(runtimePartyObserverCapacity))*runtimePartyObserverEventSize
	binary.LittleEndian.PutUint64(data[offset:offset+8], 0)
	events, next, err := decodeRuntimePartyObserverMapping(data, 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 || next != 6 {
		t.Fatalf("uncommitted row advanced observer cursor: next=%d events=%+v", next, events)
	}
}

func TestPatchCorePartyObserverHooksOnlyExistingPartyLifecycleCalls(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "src_dll", "patch_core", "dllmain.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		`GetProcAddress(party, "PartyEndpointSendMessage")`,
		`GetProcAddress(party, "PartyStartProcessingStateChanges")`,
		"PartyObserverStartChangesDetour", "CapturePartyObserverStateChanges", "kRuntimePartyObserverMappingName",
		"RestoreLibmemHookAfterDrain(g_partyStartTarget", "RestoreLibmemHookAfterDrain(g_partySendTarget",
		`PatchIdEquals(command, "runtime_party_observer")`,
		"g_partyObserverPublishLock", "InterlockedExchange64(&header->writeSequence, sequence)",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("native Party observer is missing %q", required)
		}
	}
	if strings.Contains(text, "PartyFinishProcessingStateChanges(") {
		t.Fatal("observer must not create a second Party lifecycle processing loop")
	}
}
