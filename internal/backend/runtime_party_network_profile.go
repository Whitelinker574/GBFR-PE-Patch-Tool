package backend

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	runtimePartyNetworkInitialProfileSize  = 784
	runtimePartyNetworkPeriodicProfileSize = 780
	runtimePartyNetworkProfileVersion      = 1
	runtimePartyNetworkWeaponHashOffset    = 0x1BC
	runtimePartyNetworkSigilHashOffset     = 0x1F4
	runtimePartyNetworkSecondaryHashOffset = 0x224
	runtimePartyNetworkSigilLevelOffset    = 0x25C
	runtimePartyNetworkPartyIndexOffset    = 0x2B4
	runtimePartyNetworkCharacterHashOffset = 0x2B8
	runtimePartyNetworkSigilCount          = 12
)

type runtimePartyNetworkProfileKind string

const (
	runtimePartyNetworkProfileInitial  runtimePartyNetworkProfileKind = "initial"
	runtimePartyNetworkProfilePeriodic runtimePartyNetworkProfileKind = "periodic"
)

type runtimePartyNetworkSigil struct {
	Index              int
	Hash               uint32
	SecondaryTraitHash uint32
	Level              uint32
}

type runtimePartyNetworkProfile struct {
	Kind          runtimePartyNetworkProfileKind
	PartyIndex    uint32
	CharacterCode string
	CharacterHash uint32
	WeaponHash    uint32
	Sigils        [runtimePartyNetworkSigilCount]runtimePartyNetworkSigil
}

type runtimePartyNetworkProfileDirection uint8

const (
	runtimePartyNetworkProfileLocal runtimePartyNetworkProfileDirection = iota + 1
	runtimePartyNetworkProfileRemote
)

const runtimePartyNetworkProfileStableReads = 3

type runtimePartyNetworkProfileSlot struct {
	direction   runtimePartyNetworkProfileDirection
	profile     runtimePartyNetworkProfile
	fingerprint [32]byte
	reads       int
	stable      bool
}

// runtimePartyNetworkProfileTracker keeps only decoded equipment fields. The
// sender direction is important: a player's online party index is assigned by
// the session and is not guaranteed to match a fixed local/party1/party2/party3
// memory-array position.
type runtimePartyNetworkProfileTracker struct {
	slots          [4]runtimePartyNetworkProfileSlot
	localPartySlot int
}

func newRuntimePartyNetworkProfileTracker() *runtimePartyNetworkProfileTracker {
	return &runtimePartyNetworkProfileTracker{localPartySlot: -1}
}

func (tracker *runtimePartyNetworkProfileTracker) Reset() {
	if tracker == nil {
		return
	}
	*tracker = runtimePartyNetworkProfileTracker{localPartySlot: -1}
}

func (tracker *runtimePartyNetworkProfileTracker) Observe(direction runtimePartyNetworkProfileDirection, payload []byte) (runtimePartyNetworkProfile, bool, error) {
	if tracker == nil {
		return runtimePartyNetworkProfile{}, false, fmt.Errorf("party profile tracker is nil")
	}
	if direction != runtimePartyNetworkProfileLocal && direction != runtimePartyNetworkProfileRemote {
		return runtimePartyNetworkProfile{}, false, fmt.Errorf("party profile direction %d is invalid", direction)
	}
	profile, err := parseRuntimePartyNetworkProfile(payload)
	if err != nil {
		return runtimePartyNetworkProfile{}, false, err
	}
	slotIndex := int(profile.PartyIndex)
	if direction == runtimePartyNetworkProfileLocal {
		if tracker.localPartySlot >= 0 && tracker.localPartySlot != slotIndex {
			return runtimePartyNetworkProfile{}, false, fmt.Errorf("local party slot changed from %d to %d before the Party session was reset", tracker.localPartySlot, slotIndex)
		}
		tracker.localPartySlot = slotIndex
	} else if tracker.localPartySlot == slotIndex {
		return runtimePartyNetworkProfile{}, false, fmt.Errorf("remote profile conflicts with local party slot %d", slotIndex)
	}

	fingerprint := runtimePartyNetworkProfileFingerprint(profile)
	state := &tracker.slots[slotIndex]
	if state.reads > 0 && state.direction != direction {
		return runtimePartyNetworkProfile{}, false, fmt.Errorf("party slot %d changed direction before the Party session was reset", slotIndex)
	}
	if state.reads == 0 || state.fingerprint != fingerprint {
		*state = runtimePartyNetworkProfileSlot{
			direction: direction, profile: profile, fingerprint: fingerprint, reads: 1,
		}
		return profile, false, nil
	}
	state.profile = profile
	if state.reads < runtimePartyNetworkProfileStableReads {
		state.reads++
	}
	state.stable = state.reads >= runtimePartyNetworkProfileStableReads
	return profile, state.stable, nil
}

func (tracker *runtimePartyNetworkProfileTracker) StableRemoteProfiles() []runtimePartyNetworkProfile {
	if tracker == nil {
		return nil
	}
	result := make([]runtimePartyNetworkProfile, 0, 3)
	for _, state := range tracker.slots {
		if state.stable && state.direction == runtimePartyNetworkProfileRemote {
			result = append(result, state.profile)
		}
	}
	return result
}

// parseRuntimePartyNetworkProfile decodes the two full-profile messages
// observed at the 2.0.3 PlayFab Party boundary. It deliberately ignores the
// volatile counters in the same frame; callers use the returned equipment
// fields as a stable snapshot rather than persisting the raw network payload.
func parseRuntimePartyNetworkProfile(payload []byte) (runtimePartyNetworkProfile, error) {
	var result runtimePartyNetworkProfile
	if len(payload) < 20 {
		return result, fmt.Errorf("party profile message is too short: %d", len(payload))
	}
	messageGroup := binary.LittleEndian.Uint32(payload[0:4])
	messageType := binary.LittleEndian.Uint32(payload[4:8])
	declaredSize := binary.LittleEndian.Uint32(payload[8:12])
	version := binary.LittleEndian.Uint32(payload[12:16])
	if int(declaredSize) != len(payload) {
		return result, fmt.Errorf("party profile declared size %d does not match payload %d", declaredSize, len(payload))
	}
	if version != runtimePartyNetworkProfileVersion {
		return result, fmt.Errorf("party profile version %d is unsupported", version)
	}
	switch {
	case messageGroup == 3 && messageType == 14 && len(payload) == runtimePartyNetworkInitialProfileSize:
		result.Kind = runtimePartyNetworkProfileInitial
	case messageGroup == 2 && messageType == 63 && len(payload) == runtimePartyNetworkPeriodicProfileSize:
		result.Kind = runtimePartyNetworkProfilePeriodic
	default:
		return result, fmt.Errorf("message %d:%d with %d bytes is not a verified party profile", messageGroup, messageType, len(payload))
	}

	result.PartyIndex = binary.LittleEndian.Uint32(payload[runtimePartyNetworkPartyIndexOffset:])
	if result.PartyIndex >= 4 {
		return runtimePartyNetworkProfile{}, fmt.Errorf("party profile slot %d is outside 0..3", result.PartyIndex)
	}
	result.CharacterHash = binary.LittleEndian.Uint32(payload[runtimePartyNetworkCharacterHashOffset:])
	for ownerCode, hash := range runtimeOwnerCharacterHash {
		if hash == result.CharacterHash {
			result.CharacterCode = ownerCode
			break
		}
	}
	if result.CharacterCode == "" {
		return runtimePartyNetworkProfile{}, fmt.Errorf("party profile character hash %08X is not in the 2.0.3 roster", result.CharacterHash)
	}
	result.WeaponHash = binary.LittleEndian.Uint32(payload[runtimePartyNetworkWeaponHashOffset:])
	for index := 0; index < runtimePartyNetworkSigilCount; index++ {
		result.Sigils[index] = runtimePartyNetworkSigil{
			Index:              index,
			Hash:               binary.LittleEndian.Uint32(payload[runtimePartyNetworkSigilHashOffset+index*4:]),
			SecondaryTraitHash: binary.LittleEndian.Uint32(payload[runtimePartyNetworkSecondaryHashOffset+index*4:]),
			Level:              uint32(payload[runtimePartyNetworkSigilLevelOffset+index]),
		}
	}
	return result, nil
}

func runtimePartyNetworkProfileFingerprint(profile runtimePartyNetworkProfile) [32]byte {
	payload := make([]byte, 0, 4+4+4+runtimePartyNetworkSigilCount*12)
	payload = binary.LittleEndian.AppendUint32(payload, profile.PartyIndex)
	payload = binary.LittleEndian.AppendUint32(payload, profile.CharacterHash)
	payload = binary.LittleEndian.AppendUint32(payload, profile.WeaponHash)
	for _, sigil := range profile.Sigils {
		payload = binary.LittleEndian.AppendUint32(payload, sigil.Hash)
		payload = binary.LittleEndian.AppendUint32(payload, sigil.SecondaryTraitHash)
		payload = binary.LittleEndian.AppendUint32(payload, sigil.Level)
	}
	return sha256.Sum256(payload)
}

// runtimePartyNetworkProfileLoadout converts only fields proven in the Party
// frame. Fields absent from that frame deliberately remain zero/nil so the
// existing detector can later replace the candidate with a strict superset
// from bounded memory or Logs data.
func runtimePartyNetworkProfileLoadout(profile runtimePartyNetworkProfile) (RuntimePatchPartyLoadout, error) {
	if profile.PartyIndex >= 4 || profile.CharacterCode == "" || profile.CharacterHash == 0 || profile.WeaponHash == 0 {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("party profile identity is incomplete")
	}
	if expected := runtimeOwnerCharacterHash[profile.CharacterCode]; expected != profile.CharacterHash {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("party profile character %s/%08X is inconsistent", profile.CharacterCode, profile.CharacterHash)
	}
	if _, err := loadProgressionCatalog(); err != nil {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("party profile weapon catalog: %w", err)
	}
	weaponDefinition, ok := progressionWeaponDefForHash(profile.WeaponHash)
	if !ok {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("party profile weapon %08X is not in the embedded catalog", profile.WeaponHash)
	}
	if !strings.EqualFold(weaponDefinition.OwnerCode, profile.CharacterCode) {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("party profile weapon %08X belongs to %s, not %s", profile.WeaponHash, weaponDefinition.OwnerCode, profile.CharacterCode)
	}
	catalog, err := LoadCatalog()
	if err != nil {
		return RuntimePatchPartyLoadout{}, fmt.Errorf("party profile factor catalog: %w", err)
	}
	names := runtimePatchPartyCharacterNames[profile.CharacterCode]
	characterName := names[0]
	if !useChinese() {
		characterName = names[1]
	}
	if characterName == "" {
		characterName = profile.CharacterCode
	}
	loadout := RuntimePatchPartyLoadout{
		Available: true, Stable: true, SnapshotCount: runtimePartyNetworkProfileStableReads,
		Verification: "network_profile_core",
		Evidence: runtimePatchMonitorText(
			"2.0.3 Party 资料帧三次一致：角色、武器与 12 个因子；其他范围未记录",
			"Three matching 2.0.3 Party profile frames: character, weapon, and 12 sigils; other scopes were not recorded",
		),
		Layout:        "Party profile v1 (3:14/784 or 2:63/780)",
		CharacterCode: profile.CharacterCode,
		CharacterHash: fmt.Sprintf("%08X", profile.CharacterHash),
		CharacterName: characterName,
		Online:        true,
		PartyIndex:    profile.PartyIndex,
		Weapon: RuntimePatchPartyWeapon{
			Hash: profile.WeaponHash, HashHex: fmt.Sprintf("%08X", profile.WeaponHash), Name: progressionWeaponName(weaponDefinition),
		},
		MasteryUnavailableReason: runtimePatchMonitorText("Party 资料帧未确认专精字段", "Mastery fields are not verified in the Party profile"),
		Sigils:                   make([]RuntimePatchPartySigil, 0, runtimePartyNetworkSigilCount),
	}
	for _, observed := range profile.Sigils {
		if runtimePatchPartyEmptyHash(observed.Hash) {
			continue
		}
		definition := catalog.LookupSigilByHash(observed.Hash)
		if definition == nil {
			return RuntimePatchPartyLoadout{}, fmt.Errorf("party profile factor slot %d item %08X is not in the embedded catalog", observed.Index+1, observed.Hash)
		}
		primary, err := catalog.RequireTrait(definition.PrimaryTraitID)
		if err != nil {
			return RuntimePatchPartyLoadout{}, fmt.Errorf("party profile factor slot %d primary trait: %w", observed.Index+1, err)
		}
		primaryHash, err := ParseHashHex(primary.Hash)
		if err != nil {
			return RuntimePatchPartyLoadout{}, fmt.Errorf("party profile factor slot %d primary hash: %w", observed.Index+1, err)
		}
		sigil := RuntimePatchPartySigil{
			Index: observed.Index, Hash: observed.Hash, HashHex: fmt.Sprintf("%08X", observed.Hash),
			Name: displaySigilName(definition), Level: observed.Level,
			PrimaryTraitHash: primaryHash, PrimaryTraitHashHex: fmt.Sprintf("%08X", primaryHash),
			PrimaryTraitName: loadoutTraitDisplayName(catalog, primaryHash), PrimaryTraitLevel: observed.Level,
		}
		if !runtimePatchPartyEmptyHash(observed.SecondaryTraitHash) {
			secondaryName, known := runtimePatchPartyTraitName(catalog, observed.SecondaryTraitHash)
			if !known {
				return RuntimePatchPartyLoadout{}, fmt.Errorf("party profile factor slot %d secondary trait %08X is unknown", observed.Index+1, observed.SecondaryTraitHash)
			}
			sigil.SecondaryTraitHash = observed.SecondaryTraitHash
			sigil.SecondaryTraitHashHex = fmt.Sprintf("%08X", observed.SecondaryTraitHash)
			sigil.SecondaryTraitName = secondaryName
			sigil.SecondaryTraitLevel = observed.Level
		}
		loadout.Sigils = append(loadout.Sigils, sigil)
	}
	loadout.CombinedSkills = runtimePatchPartyCombinedSkills(loadout)
	return loadout, nil
}

// runtimePartyNetworkProfileMatchesLoadout intentionally does not compare the
// Party slot. This lets a verified equipment fingerprint join the network and
// bounded-memory views without assuming both subsystems expose the same order.
func runtimePartyNetworkProfileMatchesLoadout(profile runtimePartyNetworkProfile, loadout RuntimePatchPartyLoadout) bool {
	if !loadout.Available || !strings.EqualFold(loadout.CharacterHash, fmt.Sprintf("%08X", profile.CharacterHash)) || loadout.Weapon.Hash != profile.WeaponHash {
		return false
	}
	byIndex := make(map[int]RuntimePatchPartySigil, len(loadout.Sigils))
	for _, sigil := range loadout.Sigils {
		byIndex[sigil.Index] = sigil
	}
	for _, observed := range profile.Sigils {
		actual, present := byIndex[observed.Index]
		if runtimePatchPartyEmptyHash(observed.Hash) {
			if present && !runtimePatchPartyEmptyHash(actual.Hash) {
				return false
			}
			continue
		}
		secondaryMatches := actual.SecondaryTraitHash == observed.SecondaryTraitHash ||
			(runtimePatchPartyEmptyHash(actual.SecondaryTraitHash) && runtimePatchPartyEmptyHash(observed.SecondaryTraitHash))
		if !present || actual.Hash != observed.Hash || !secondaryMatches || actual.Level != observed.Level {
			return false
		}
	}
	return true
}
