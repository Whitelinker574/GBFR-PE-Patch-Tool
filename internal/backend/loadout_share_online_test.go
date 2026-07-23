package backend

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func encodedShareFrame(t *testing.T, share *LoadoutShare) []byte {
	t.Helper()
	encoded, err := encodeLoadoutShareCode(share)
	if err != nil {
		t.Fatalf("encode share: %v", err)
	}
	frame, err := loadoutShareFrameFromCompatibilityCode(encoded.CompatibilityCode)
	if err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	return frame
}

func TestNormalizeLoadoutShareShortCodeAcceptsCodesAndKnownLinkShapes(t *testing.T) {
	const normalized = "0123456789ABCDEF"
	inputs := []string{
		"0123-4567-89AB-CDEF",
		" 0123456789abcdef ",
		"https://share.example/s/0123456789ABCDEF",
		"https://share.example/api/v1/loadouts/0123-4567-89AB-CDEF",
		"https://share.example/download/0123456789ABCDEF.gbfr-loadout",
	}
	for _, input := range inputs {
		got, err := normalizeLoadoutShareShortCode(input)
		if err != nil {
			t.Fatalf("normalize %q: %v", input, err)
		}
		if got != normalized {
			t.Fatalf("normalize %q = %q", input, got)
		}
	}
	for _, input := range []string{"too-short", "0123456789ABCDEFGHJKMNPQRST", "https://share.example/other/0123456789ABCDEF", "0123456789ABCDEI"} {
		if _, err := normalizeLoadoutShareShortCode(input); err == nil {
			t.Fatalf("accepted invalid input %q", input)
		}
	}
}

func TestLoadoutShareOnlinePublishAndFetchRoundTrip(t *testing.T) {
	frame := encodedShareFrame(t, loadoutShareCodeFixture())
	var stored []byte
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/loadouts":
			if request.Header.Get("Content-Type") != "application/octet-stream" {
				t.Errorf("content type = %q", request.Header.Get("Content-Type"))
			}
			var err error
			stored, err = io.ReadAll(request.Body)
			if err != nil {
				t.Errorf("read request: %v", err)
			}
			response.Header().Set("Content-Type", "application/json")
			response.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(response).Encode(LoadoutPublishedShare{
				Code: "0123-4567-89AB-CDEF", CompactCode: "0123456789ABCDEF", Reused: false,
			})
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/loadouts/0123456789ABCDEF":
			response.Header().Set("Content-Type", "application/vnd.gbfr.loadout")
			_, _ = response.Write(stored)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	published, err := publishLoadoutShareFrame(nil, server.Client(), server.URL, frame)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Code != "0123-4567-89AB-CDEF" ||
		published.URL != server.URL+"/s/0123456789ABCDEF" ||
		published.DownloadURL != server.URL+"/download/0123456789ABCDEF.gbfr-loadout" {
		t.Fatalf("unexpected publish result: %+v", published)
	}
	received, err := fetchLoadoutShareFrame(nil, server.Client(), server.URL, published.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !bytes.Equal(received, frame) {
		t.Fatal("online service changed the frame")
	}
	if _, err := decodeLoadoutShareFrame(received); err != nil {
		t.Fatalf("downloaded frame failed local verification: %v", err)
	}
}

func TestLoadoutShareOnlineRejectsOversizedAndServiceErrors(t *testing.T) {
	if _, err := publishLoadoutShareFrame(nil, http.DefaultClient, "https://invalid.example", make([]byte, loadoutShareOnlineMaxFrameSize+1)); err == nil {
		t.Fatal("oversized publish was accepted")
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusNotFound)
		_, _ = response.Write([]byte(`{"error":"没有找到这套配装"}`))
	}))
	defer server.Close()
	_, err := fetchLoadoutShareFrame(nil, server.Client(), server.URL, "0123-4567-89AB-CDEF")
	if err == nil || !strings.Contains(err.Error(), "没有找到") {
		t.Fatalf("unexpected service error: %v", err)
	}
}

func TestLoadoutShareOnlineLive(t *testing.T) {
	endpoint := strings.TrimSpace(os.Getenv("GBFR_TEST_SHARE_ENDPOINT"))
	if endpoint == "" {
		t.Skip("set GBFR_TEST_SHARE_ENDPOINT to run the live R2 round trip")
	}
	_, source := actualLoadoutShareFixture(t)
	frame := encodedShareFrame(t, source)
	published, err := publishLoadoutShareFrame(nil, loadoutShareHTTPClient(), endpoint, frame)
	if err != nil {
		t.Fatalf("publish live share: %v", err)
	}
	received, err := fetchLoadoutShareFrame(nil, loadoutShareHTTPClient(), endpoint, published.Code)
	if err != nil {
		t.Fatalf("fetch live share %s: %v", published.Code, err)
	}
	decoded, err := decodeLoadoutShareFrame(received)
	if err != nil {
		t.Fatalf("decode live share: %v", err)
	}
	if !reflect.DeepEqual(decoded.Sigils, source.Sigils) ||
		!reflect.DeepEqual(decoded.Summons, source.Summons) ||
		!reflect.DeepEqual(decoded.MasteryHashes, source.MasteryHashes) {
		t.Fatal("live service changed loadout data")
	}
	assertShareCodeProgression(t, decoded, source)
	t.Logf("live code=%s url=%s bytes=%d reused=%t", published.Code, published.URL, published.Bytes, published.Reused)
}

func TestLoadoutShareOnlineLiveImportDraft(t *testing.T) {
	code := strings.TrimSpace(os.Getenv("GBFR_TEST_SHARE_CODE"))
	if code == "" || !haveSave(testLoadoutSave) {
		t.Skip("set GBFR_TEST_SHARE_CODE and GBFR_TEST_LOADOUT_SAVE to verify a live import draft")
	}
	frame, err := fetchLoadoutShareFrame(nil, loadoutShareHTTPClient(), loadoutShareServiceURL, code)
	if err != nil {
		t.Fatalf("fetch live share: %v", err)
	}
	share, err := decodeLoadoutShareFrame(frame)
	if err != nil {
		t.Fatalf("decode live share: %v", err)
	}
	draft, err := resolveLoadoutShare(testLoadoutSave, share.CharaHash, share)
	if err != nil {
		t.Fatalf("resolve live share: %v", err)
	}
	for _, sigil := range draft.ConstructedSigils {
		if strings.EqualFold(sigil.ExactSigilHash, "80C94A24") {
			if !strings.EqualFold(sigil.ExactPrimaryTraitHash, "7EDD69D0") {
				t.Fatalf("live combination primary hash changed: %+v", sigil)
			}
			t.Logf("live combination resolved at slot %d: %+v", sigil.Index+1, sigil)
			return
		}
	}
	t.Fatal("live share did not preserve the reported 0x80C94A24 combination")
}

func TestLoadoutShareOnlineLiveCatalogCoverage(t *testing.T) {
	code := strings.TrimSpace(os.Getenv("GBFR_TEST_SHARE_CODE"))
	if code == "" {
		t.Skip("set GBFR_TEST_SHARE_CODE to audit a live share against local catalogs")
	}
	frame, err := fetchLoadoutShareFrame(nil, loadoutShareHTTPClient(), loadoutShareServiceURL, code)
	if err != nil {
		t.Fatalf("fetch live share: %v", err)
	}
	share, err := decodeLoadoutShareFrame(frame)
	if err != nil {
		t.Fatalf("decode live share: %v", err)
	}
	if _, err := loadProgressionCatalog(); err != nil {
		t.Fatal(err)
	}
	if len(share.Skills) != loadoutMaxSkills {
		t.Fatalf("live share active skills=%d, want %d", len(share.Skills), loadoutMaxSkills)
	}
	for index, skill := range share.Skills {
		hash, parseErr := ParseHashHex(skill.Hash)
		if parseErr != nil || !skillBelongsToOwner(hash, share.OwnerCode) {
			t.Fatalf("live share skill %d is not covered by owner catalog: %+v err=%v", index+1, skill, parseErr)
		}
	}
	if len(share.MasteryHashes) != loadoutMaxMastery {
		t.Fatalf("live share mastery slots=%d, want %d", len(share.MasteryHashes), loadoutMaxMastery)
	}
	for index, value := range share.MasteryHashes {
		hash, parseErr := ParseHashHex(value)
		if parseErr != nil {
			t.Fatalf("live share mastery slot %d is invalid: %q", index+1, value)
		}
		if hash == 0 || hash == EmptyHash {
			continue
		}
		node, ok := skillboardNodeForHash(hash)
		if !ok || (share.OwnerCode != "" && node.Char != share.OwnerCode) {
			t.Fatalf("live share mastery slot %d is not covered by owner catalog: %08X", index+1, hash)
		}
	}
	if len(share.OverLimit) != 4 {
		t.Fatalf("live share over-limit slots=%d, want 4", len(share.OverLimit))
	}
	for _, slot := range share.OverLimit {
		if slot.AttributeHash == "" && slot.Level == 0 {
			continue
		}
		hash, parseErr := ParseHashHex(slot.AttributeHash)
		if parseErr != nil {
			t.Fatalf("live share over-limit slot %d is invalid: %v", slot.Index+1, parseErr)
		}
		if _, ok := overLimitCatalog[hash]; !ok {
			t.Fatalf("live share over-limit slot %d is not covered by catalog: %08X", slot.Index+1, hash)
		}
	}
	weaponHash, err := ParseHashHex(share.WeaponHash)
	if err != nil {
		t.Fatalf("live share weapon hash is invalid: %v", err)
	}
	if def, ok := progressionWeaponDefForLoadout(weaponHash); !ok || (share.OwnerCode != "" && def.OwnerCode != "" && def.OwnerCode != share.OwnerCode) {
		t.Fatalf("live share weapon is not covered by owner catalog: %08X", weaponHash)
	}
}

func TestRealSaveShareCanMaterializeMissingEquippedWeapon(t *testing.T) {
	if !haveSave(testLoadoutSave) {
		t.Skip("set GBFR_TEST_LOADOUT_SAVE to verify missing equipped weapon materialization")
	}
	groups, err := (&App{}).LoadoutList(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	var source *LoadoutEntry
	var share *LoadoutShare
	for groupIndex := range groups {
		for loadoutIndex := range groups[groupIndex].Loadouts {
			candidate := &groups[groupIndex].Loadouts[loadoutIndex]
			if candidate.IsParty || candidate.WeaponSlotID == 0 || candidate.WeaponHash == "" || len(candidate.Mastery) == 0 {
				continue
			}
			candidateShare, buildErr := buildLoadoutShare(testLoadoutSave, candidate.UnitID)
			if buildErr != nil || candidateShare.OwnerCode == "" {
				continue
			}
			source = candidate
			share = candidateShare
			break
		}
		if source != nil {
			break
		}
	}
	if source == nil {
		t.Fatal("target fixture has no saved loadout with an equipped weapon")
	}
	input, err := os.ReadFile(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	sourceDigest := sha256.Sum256(input)
	work := filepath.Join(t.TempDir(), "SaveData2.dat")
	if err := os.WriteFile(work, input, 0o600); err != nil {
		t.Fatal(err)
	}
	save, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	wantHash, err := ParseHashHex(share.WeaponHash)
	if err != nil {
		t.Fatal(err)
	}
	var removedUnitID uint32
	for _, entry := range save.findAllUnitsByType(weaponIDType) {
		if entry.ValueCnt == 1 && entry.Uint32() == wantHash {
			removedUnitID = entry.UnitID
			break
		}
	}
	if removedUnitID == 0 {
		t.Fatalf("target fixture does not own live share weapon %08X", wantHash)
	}
	if err := save.patchUint(weaponIDType, removedUnitID, EmptyHash); err != nil {
		t.Fatal(err)
	}
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := save.Write(work); err != nil {
		t.Fatal(err)
	}
	charaHash, err := ParseHashHex(share.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	targetCharacterUnitID, err := loadoutCharacterUnitForHash(save, charaHash)
	if err != nil {
		t.Fatal(err)
	}
	targetLevel, levelOK := save.findUnitExact(1308, targetCharacterUnitID)
	targetFate, fateOK := save.findUnitExact(1318, targetCharacterUnitID)
	if !levelOK || !fateOK || targetLevel.ValueCnt != 1 || targetFate.ValueCnt != 1 {
		t.Fatal("target fixture lacks character level or Fate episode fields")
	}
	wantTargetLevel, wantTargetFate := targetLevel.Uint32(), targetFate.Uint32()
	if constructed, constructErr := loadoutShareEquippedWeaponConstruction(share); constructErr != nil || constructed == nil {
		t.Fatalf("live share cannot describe its missing equipped weapon: constructed=%+v err=%v", constructed, constructErr)
	}
	draft, err := resolveLoadoutShare(work, share.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if missing := draft.MissingByScope["weapon"]; len(missing) != 0 {
		t.Fatalf("share has an exact equipped weapon snapshot but import still requires a target instance: %v", missing)
	}
	targetStats, err := (&App{}).LoadoutStatContext(work, share.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Capabilities.TargetCharacterLevel != targetStats.Level ||
		draft.Capabilities.TargetFateDataAvailable != targetStats.PermanentGrowth.FateDataAvailable ||
		draft.Capabilities.TargetFateEpisodeCount != targetStats.PermanentGrowth.FateEpisodeCount {
		t.Fatalf("import compatibility omitted target level/Fate state: caps=%+v growth=%+v", draft.Capabilities, targetStats.PermanentGrowth)
	}
	if draft.ApplyPayload == nil || draft.ApplyPayload.ConstructedWeapon == nil || draft.WeaponSlotID != 0 {
		t.Fatalf("missing weapon was not staged for atomic construction: %+v", draft)
	}
	sigils, skills, mastery := loadoutVectors(*source)
	draft.ApplyPayload.ApplyWeaponWrightstone = share.Weapon != nil && share.Weapon.Wrightstone != nil
	output := filepath.Join(t.TempDir(), "SaveData2.dat")
	result, err := (&App{}).LoadoutApplyWithResources(work, output, LoadoutApplyRequest{
		Changes: []LoadoutWrite{{
			UnitID: source.UnitID, ExpectCharaHash: source.CharaHash, Op: "write", Name: source.Name,
			SigilSlotIDs: sigils, SkillHashes: skills, WeaponSkillHashes: append([]string(nil), share.WeaponSkillHashes...),
			MasteryHashes: mastery,
		}},
		ImportPayload: draft.ApplyPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CreatedWeaponCount != 1 || result.SlotsWritten != 1 {
		t.Fatalf("missing weapon import did not report one atomic construction: %+v", result)
	}
	groups, err = (&App{}).LoadoutList(output)
	if err != nil {
		t.Fatal(err)
	}
	var imported *LoadoutEntry
	for groupIndex := range groups {
		for loadoutIndex := range groups[groupIndex].Loadouts {
			if groups[groupIndex].Loadouts[loadoutIndex].UnitID == source.UnitID {
				imported = &groups[groupIndex].Loadouts[loadoutIndex]
				break
			}
		}
	}
	if imported == nil || imported.WeaponSlotID == 0 || imported.Weapon == nil {
		t.Fatalf("imported preset did not bind the constructed weapon: %+v", imported)
	}
	if !strings.EqualFold(imported.Weapon.StoredHash, share.Weapon.StoredHash) ||
		imported.Weapon.XP != share.Weapon.XP || imported.Weapon.Uncap != share.Weapon.Uncap ||
		imported.Weapon.Mirage != share.Weapon.Mirage || imported.Weapon.Awakening != share.Weapon.Awakening ||
		imported.Weapon.Transcendence != share.Weapon.Transcendence {
		t.Fatalf("constructed weapon readback mismatch: got=%+v want=%+v", imported.Weapon, share.Weapon)
	}
	outputSave, err := LoadSave(output)
	if err != nil {
		t.Fatal(err)
	}
	gotLevel, gotLevelOK := outputSave.findUnitExact(1308, targetCharacterUnitID)
	gotFate, gotFateOK := outputSave.findUnitExact(1318, targetCharacterUnitID)
	if !gotLevelOK || !gotFateOK || gotLevel.Uint32() != wantTargetLevel || gotFate.Uint32() != wantTargetFate {
		t.Fatalf("loadout import changed target character level/Fate: level=%v/%d Fate=%v/%08X",
			gotLevel, wantTargetLevel, gotFate, wantTargetFate)
	}
	weaponUnitID, err := exactWeaponUnitForSlot(outputSave, imported.WeaponSlotID)
	if err != nil {
		t.Fatal(err)
	}
	skillValues := readFixedVec(outputSave, weaponExtraIDType, weaponUnitID, 5)
	skillHashes := make([]string, 0, len(skillValues))
	for _, value := range skillValues {
		skillHashes = append(skillHashes, hashText(value))
	}
	if !reflect.DeepEqual(skillHashes, share.Weapon.SkillHashes) {
		t.Fatalf("constructed weapon five-skill readback mismatch: got=%v want=%v", skillHashes, share.Weapon.SkillHashes)
	}
	if current, readErr := os.ReadFile(testLoadoutSave); readErr != nil || sha256.Sum256(current) != sourceDigest {
		t.Fatalf("real input save changed during isolated write test: %v", readErr)
	}
}
