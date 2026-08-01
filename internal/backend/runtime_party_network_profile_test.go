package backend

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func runtimePartyNetworkProfileFixture(t *testing.T, initial bool) []byte {
	t.Helper()
	size := runtimePartyNetworkPeriodicProfileSize
	group, messageType := uint32(2), uint32(63)
	if initial {
		size = runtimePartyNetworkInitialProfileSize
		group, messageType = 3, 14
	}
	payload := make([]byte, size)
	binary.LittleEndian.PutUint32(payload[0:4], group)
	binary.LittleEndian.PutUint32(payload[4:8], messageType)
	binary.LittleEndian.PutUint32(payload[8:12], uint32(size))
	binary.LittleEndian.PutUint32(payload[12:16], runtimePartyNetworkProfileVersion)
	binary.LittleEndian.PutUint32(payload[runtimePartyNetworkPartyIndexOffset:], 1)
	binary.LittleEndian.PutUint32(payload[runtimePartyNetworkCharacterHashOffset:], runtimeOwnerCharacterHash["PL0400"])
	binary.LittleEndian.PutUint32(payload[runtimePartyNetworkWeaponHashOffset:], 0x1779CD60)
	for index := 0; index < runtimePartyNetworkSigilCount; index++ {
		binary.LittleEndian.PutUint32(payload[runtimePartyNetworkSigilHashOffset+index*4:], uint32(0x1000+index))
		binary.LittleEndian.PutUint32(payload[runtimePartyNetworkSecondaryHashOffset+index*4:], uint32(0x2000+index))
		payload[runtimePartyNetworkSigilLevelOffset+index] = byte(index + 1)
	}
	return payload
}

func TestParseRuntimePartyNetworkProfileAcceptsInitialAndPeriodicFrames(t *testing.T) {
	for _, initial := range []bool{true, false} {
		profile, err := parseRuntimePartyNetworkProfile(runtimePartyNetworkProfileFixture(t, initial))
		if err != nil {
			t.Fatal(err)
		}
		wantKind := runtimePartyNetworkProfilePeriodic
		if initial {
			wantKind = runtimePartyNetworkProfileInitial
		}
		if profile.Kind != wantKind || profile.PartyIndex != 1 || profile.CharacterCode != "PL0400" ||
			profile.CharacterHash != 0x4D0A60C3 || profile.WeaponHash != 0x1779CD60 {
			t.Fatalf("profile header mismatch: %+v", profile)
		}
		for index, sigil := range profile.Sigils {
			if sigil.Index != index || sigil.Hash != uint32(0x1000+index) ||
				sigil.SecondaryTraitHash != uint32(0x2000+index) || sigil.Level != uint32(index+1) {
				t.Fatalf("sigil %d mismatch: %+v", index, sigil)
			}
		}
	}
}

func TestRuntimePartyNetworkProfileFingerprintIgnoresVolatileFrameBytes(t *testing.T) {
	firstPayload := runtimePartyNetworkProfileFixture(t, false)
	secondPayload := append([]byte(nil), firstPayload...)
	for _, offset := range []int{0x1A6, 0x1A7, 0x1F1, 0x1F2, 0x1F3, 0x269, 0x26A, 0x26B, 0x30A, 0x30B} {
		secondPayload[offset] ^= 0x5A
	}
	first, err := parseRuntimePartyNetworkProfile(firstPayload)
	if err != nil {
		t.Fatal(err)
	}
	second, err := parseRuntimePartyNetworkProfile(secondPayload)
	if err != nil {
		t.Fatal(err)
	}
	if runtimePartyNetworkProfileFingerprint(first) != runtimePartyNetworkProfileFingerprint(second) {
		t.Fatal("volatile counters changed the equipment fingerprint")
	}
}

func TestRuntimePartyNetworkProfileTrackerUsesDirectionInsteadOfAssumingLocalSlotZero(t *testing.T) {
	tracker := newRuntimePartyNetworkProfileTracker()
	local := runtimePartyNetworkProfileFixture(t, false)
	binary.LittleEndian.PutUint32(local[runtimePartyNetworkPartyIndexOffset:], 1)
	remote := runtimePartyNetworkProfileFixture(t, false)
	binary.LittleEndian.PutUint32(remote[runtimePartyNetworkPartyIndexOffset:], 0)
	binary.LittleEndian.PutUint32(remote[runtimePartyNetworkCharacterHashOffset:], runtimeOwnerCharacterHash["PL1200"])

	for read := 0; read < runtimePartyNetworkProfileStableReads; read++ {
		if _, stable, err := tracker.Observe(runtimePartyNetworkProfileLocal, local); err != nil || (read+1 < runtimePartyNetworkProfileStableReads && stable) {
			t.Fatalf("local read %d: stable=%t err=%v", read+1, stable, err)
		}
		if _, stable, err := tracker.Observe(runtimePartyNetworkProfileRemote, remote); err != nil || stable != (read+1 >= runtimePartyNetworkProfileStableReads) {
			t.Fatalf("remote read %d: stable=%t err=%v", read+1, stable, err)
		}
	}
	profiles := tracker.StableRemoteProfiles()
	if tracker.localPartySlot != 1 || len(profiles) != 1 || profiles[0].PartyIndex != 0 || profiles[0].CharacterCode != "PL1200" {
		t.Fatalf("directional slot mapping mismatch: local=%d remote=%+v", tracker.localPartySlot, profiles)
	}
}

func TestRuntimePartyNetworkProfileTrackerRestabilizesAfterEquipmentChange(t *testing.T) {
	tracker := newRuntimePartyNetworkProfileTracker()
	payload := runtimePartyNetworkProfileFixture(t, false)
	binary.LittleEndian.PutUint32(payload[runtimePartyNetworkPartyIndexOffset:], 2)
	for read := 0; read < runtimePartyNetworkProfileStableReads; read++ {
		if _, _, err := tracker.Observe(runtimePartyNetworkProfileRemote, payload); err != nil {
			t.Fatal(err)
		}
	}
	changed := append([]byte(nil), payload...)
	binary.LittleEndian.PutUint32(changed[runtimePartyNetworkSigilHashOffset:], 0xDEADBEEF)
	if _, stable, err := tracker.Observe(runtimePartyNetworkProfileRemote, changed); err != nil || stable {
		t.Fatalf("first changed frame must restart stabilization: stable=%t err=%v", stable, err)
	}
	if profiles := tracker.StableRemoteProfiles(); len(profiles) != 0 {
		t.Fatalf("stale remote profile remained visible after equipment change: %+v", profiles)
	}
	for read := 1; read < runtimePartyNetworkProfileStableReads; read++ {
		_, stable, err := tracker.Observe(runtimePartyNetworkProfileRemote, changed)
		if err != nil || stable != (read+1 >= runtimePartyNetworkProfileStableReads) {
			t.Fatalf("changed read %d: stable=%t err=%v", read+1, stable, err)
		}
	}
}

func TestRuntimePartyNetworkProfileTrackerRejectsLocalRemoteSlotConflict(t *testing.T) {
	tracker := newRuntimePartyNetworkProfileTracker()
	payload := runtimePartyNetworkProfileFixture(t, false)
	if _, _, err := tracker.Observe(runtimePartyNetworkProfileLocal, payload); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tracker.Observe(runtimePartyNetworkProfileRemote, payload); err == nil {
		t.Fatal("local/remote slot conflict was accepted")
	}
}

func TestRuntimePartyNetworkProfileBuildsPartialLoadoutAndMatchesAcrossSlotOrder(t *testing.T) {
	payload := runtimePartyNetworkProfileFixture(t, false)
	binary.LittleEndian.PutUint32(payload[runtimePartyNetworkWeaponHashOffset:], 0x1779CD60)
	itemHashes := [...]uint32{
		0x5BF84FD1, 0x035A4DDD, 0x791DA8ED, 0x332E9B30,
		0x9300FADB, 0x00612B10, 0x54D8EA04, 0x54D8EA04,
		0xD29CD8E0, 0x54D8EA04, 0xE2B380E5, 0x43F26A91,
	}
	secondaryHashes := [...]uint32{
		0x887AE0B0, 0x73220725, 0x84078CB0, 0x57AB5B10,
		0x24883AF3, 0x3D8153A1, 0x7C2E4D64, 0x7CCFF74F,
		0x95F3FA86, 0xA7726190, 0xDC584F60, 0x11AAE5F5,
	}
	for index := range itemHashes {
		binary.LittleEndian.PutUint32(payload[runtimePartyNetworkSigilHashOffset+index*4:], itemHashes[index])
		binary.LittleEndian.PutUint32(payload[runtimePartyNetworkSecondaryHashOffset+index*4:], secondaryHashes[index])
		payload[runtimePartyNetworkSigilLevelOffset+index] = 15
	}
	profile, err := parseRuntimePartyNetworkProfile(payload)
	if err != nil {
		t.Fatal(err)
	}
	loadout, err := runtimePartyNetworkProfileLoadout(profile)
	if err != nil {
		t.Fatal(err)
	}
	if !loadout.Available || !loadout.Stable || !loadout.Online || loadout.SnapshotCount != 3 || len(loadout.Sigils) != 12 || len(loadout.CombinedSkills) == 0 {
		t.Fatalf("partial network loadout is incomplete: %+v", loadout)
	}
	loadout.PartyIndex = 3 // memory order is not required to equal the network slot
	if !runtimePartyNetworkProfileMatchesLoadout(profile, loadout) {
		t.Fatal("matching equipment was rejected after a slot-order change")
	}
	loadout.Sigils[0].SecondaryTraitHash ^= 1
	if runtimePartyNetworkProfileMatchesLoadout(profile, loadout) {
		t.Fatal("different equipment was accepted")
	}
}

func TestParseRuntimePartyNetworkProfileMatchesSanitized203CaptureCore(t *testing.T) {
	payload := runtimePartyNetworkProfileFixture(t, true)
	sigilHashes := [...]uint32{
		0x5BF84FD1, 0x035A4DDD, 0x791DA8ED, 0x332E9B30,
		0x9300FADB, 0x00612B10, 0x54D8EA04, 0x54D8EA04,
		0xD29CD8E0, 0x54D8EA04, 0xE2B380E5, 0x43F26A91,
	}
	secondaryHashes := [...]uint32{
		0x887AE0B0, 0x73220725, 0x84078CB0, 0x57AB5B10,
		0x24883AF3, 0x3D8153A1, 0x7C2E4D64, 0x7CCFF74F,
		0x95F3FA86, 0xA7726190, 0xDC584F60, 0x11AAE5F5,
	}
	for index := 0; index < runtimePartyNetworkSigilCount; index++ {
		binary.LittleEndian.PutUint32(payload[runtimePartyNetworkSigilHashOffset+index*4:], sigilHashes[index])
		binary.LittleEndian.PutUint32(payload[runtimePartyNetworkSecondaryHashOffset+index*4:], secondaryHashes[index])
		payload[runtimePartyNetworkSigilLevelOffset+index] = 15
	}
	profile, err := parseRuntimePartyNetworkProfile(payload)
	if err != nil {
		t.Fatal(err)
	}
	if profile.PartyIndex != 1 || profile.CharacterCode != "PL0400" || profile.WeaponHash != 0x1779CD60 {
		t.Fatalf("captured profile identity mismatch: %+v", profile)
	}
	for index, sigil := range profile.Sigils {
		if sigil.Hash != sigilHashes[index] || sigil.SecondaryTraitHash != secondaryHashes[index] || sigil.Level != 15 {
			t.Fatalf("captured sigil %d mismatch: %+v", index, sigil)
		}
	}
}

func TestParseRuntimePartyNetworkProfileRejectsUnverifiedFrames(t *testing.T) {
	tests := map[string]func([]byte){
		"declared size": func(payload []byte) { binary.LittleEndian.PutUint32(payload[8:12], uint32(len(payload)-1)) },
		"version":       func(payload []byte) { binary.LittleEndian.PutUint32(payload[12:16], 2) },
		"message":       func(payload []byte) { binary.LittleEndian.PutUint32(payload[4:8], 62) },
		"party slot":    func(payload []byte) { binary.LittleEndian.PutUint32(payload[runtimePartyNetworkPartyIndexOffset:], 4) },
		"character": func(payload []byte) {
			binary.LittleEndian.PutUint32(payload[runtimePartyNetworkCharacterHashOffset:], 0xDEADBEEF)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			payload := runtimePartyNetworkProfileFixture(t, false)
			mutate(payload)
			if _, err := parseRuntimePartyNetworkProfile(payload); err == nil {
				t.Fatal("unverified frame was accepted")
			}
		})
	}
}

func TestRuntimePartyNetworkProfileAgainstExternalCapture(t *testing.T) {
	directory := os.Getenv("GBFR_PARTY_CAPTURE_ACCEPTANCE_DIR")
	if directory == "" {
		t.Skip("set GBFR_PARTY_CAPTURE_ACCEPTANCE_DIR to replay an external read-only Party capture")
	}
	events, err := os.Open(filepath.Join(directory, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer events.Close()
	type event struct {
		Captured int    `json:"captured"`
		Kind     string `json:"kind"`
		Sequence int    `json:"seq"`
		SHA256   string `json:"sha256"`
	}
	initial := make(map[uint32]uint32)
	periodic := make(map[uint32]uint32)
	counts := map[runtimePartyNetworkProfileKind]int{}
	tracker := newRuntimePartyNetworkProfileTracker()
	scanner := bufio.NewScanner(events)
	for scanner.Scan() {
		var observed event
		if err := json.Unmarshal(scanner.Bytes(), &observed); err != nil {
			t.Fatal(err)
		}
		if (observed.Kind != "send" && observed.Kind != "recv") ||
			(observed.Captured != runtimePartyNetworkInitialProfileSize && observed.Captured != runtimePartyNetworkPeriodicProfileSize) {
			continue
		}
		if len(observed.SHA256) < 12 {
			t.Fatalf("event %d has an incomplete digest", observed.Sequence)
		}
		name := fmt.Sprintf("%06d-%s-%s.bin", observed.Sequence, observed.Kind, observed.SHA256[:12])
		payload, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		profile, err := parseRuntimePartyNetworkProfile(payload)
		if err != nil {
			t.Fatalf("event %d: %v", observed.Sequence, err)
		}
		target := periodic
		if profile.Kind == runtimePartyNetworkProfileInitial {
			target = initial
		}
		if previous, ok := target[profile.PartyIndex]; ok && previous != profile.CharacterHash {
			t.Fatalf("slot %d changed character from %08X to %08X", profile.PartyIndex, previous, profile.CharacterHash)
		}
		target[profile.PartyIndex] = profile.CharacterHash
		counts[profile.Kind]++
		direction := runtimePartyNetworkProfileRemote
		if observed.Kind == "send" {
			direction = runtimePartyNetworkProfileLocal
		}
		if _, _, err := tracker.Observe(direction, payload); err != nil {
			t.Fatalf("event %d tracker: %v", observed.Sequence, err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(initial) != 4 || len(periodic) != 4 {
		t.Fatalf("profile streams did not cover four stable slots: initial=%d periodic=%d", len(initial), len(periodic))
	}
	for slot, character := range initial {
		if periodic[slot] != character {
			t.Fatalf("slot %d initial/periodic character mismatch: %08X/%08X", slot, character, periodic[slot])
		}
	}
	if tracker.localPartySlot != 1 {
		t.Fatalf("captured local online slot=%d, want 1", tracker.localPartySlot)
	}
	if profiles := tracker.StableRemoteProfiles(); len(profiles) != 3 {
		t.Fatalf("captured stable remote profiles=%d, want 3", len(profiles))
	}
	t.Logf("validated %d initial and %d periodic full-profile messages across four anonymous slots", counts[runtimePartyNetworkProfileInitial], counts[runtimePartyNetworkProfilePeriodic])
}
