package backend

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
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

func TestPreviewUsesLocalizedLoadoutEvidenceAndMergedSkillLedger(t *testing.T) {
	first, second := 1, 7
	share := &LoadoutShare{
		Name: "测试配装", CharaHash: "4D0A60C3", CharaName: "伊欧", OwnerCode: "PL0400",
		WeaponHash: "CDB13688", WeaponName: "Stygian Ornament",
		Sigils: []LoadoutShareSigil{
			{Index: &second, Name: "Quick Cooldown", Level: 15, PrimaryTraitHash: "318D12E9", PrimaryTraitLevel: 14, SecondaryTraitLevel: 7},
			{Index: &first, Name: "Berserker Echo", Level: 15, PrimaryTraitHash: "EE85CD1F", PrimaryTraitLevel: 15, SecondaryTraitLevel: 3},
		},
		Summons:       []LoadoutShareSummon{{Name: "召唤石", Rank: 5, MainTraitLevel: 10, SubParamLevel: 3}},
		MasteryHashes: []string{"11111111", "22222222"},
	}
	entry := &LoadoutEntry{
		Weapon: &LoadoutWeaponContext{
			Skills:      []LoadoutWeaponSkill{{Name: "攻击力", Level: 25, Effect: "攻击力+50%"}},
			Wrightstone: &LoadoutWeaponWrightstone{Name: "Sequestration Wrightstone", Traits: []LoadoutWeaponWrightstoneTrait{{Name: "暴击率", Level: 10}}},
		},
		Sigils: []LoadoutSigil{
			{Index: first, Name: "狂战士回响", PrimaryTraitName: "狂战士回响", SecondaryTraitName: "攻击力"},
			{Index: second, Name: "快速冷却", PrimaryTraitName: "快速冷却", SecondaryTraitName: "伤害上限"},
		},
		Mastery: []LoadoutMasteryNode{
			{Rank: "R1", RankLabel: "1阶专精技能", Name: "魔法连锁", Desc: "攻击力+10%"},
			{Rank: "R1", RankLabel: "1阶专精技能", Name: "魔法连锁", Desc: "攻击力+10%"},
		},
	}
	context := &LoadoutStatContext{
		EquippedSummons: []LoadoutSummon{{Name: "巴哈姆特", Rank: 5, MainTraitName: "攻击力", MainTraitLevel: 10, SubParamName: "暴击率", SubParamLevel: 3, SubParamValue: 12, SubParamUnit: "pct"}},
		OverLimit: []LoadoutOverLimitBonus{
			{Index: 0, AttributeHash: "6CB38EF3", Name: "昏厥值", Level: 10, Value: 20, Unit: "flat"},
			{Index: 1, AttributeHash: "6CB38EF3", Name: "昏厥值", Level: 10, Value: 20, Unit: "flat"},
			{Index: 2, AttributeHash: "43B7581D", Name: "普通攻击伤害上限", Level: 10, Value: 20, Unit: "pct"},
			{Index: 3, AttributeHash: "9C555433", Name: "能力伤害上限", Level: 10, Value: 20, Unit: "pct"},
		},
	}
	simulation := &LoadoutSimulation{Bonuses: []TraitBonus{{Name: "DMG Cap", Level: 45, RawLevel: 55, MaxLevel: 45, Effect: "攻击力+100%", Sources: []string{
		"Sigil 01 · DMG Cap",
		"Weapon · Stygian Ornament · DMG Cap Lv5",
		"Wrightstone · Sequestration Wrightstone · DMG Cap Lv10",
	}}}}

	preview := previewForLoadout(share, entry, context, simulation)
	if preview.Sigils[0].Name != "迅捷能力 V+" || preview.Sigils[1].Name != "狂战士回响 V+" {
		t.Fatalf("preview did not resolve localized sigils by their real slot indexes: %+v", preview.Sigils)
	}
	if preview.Sigils[0].PrimaryLevel != 14 || preview.Sigils[0].SecondaryLevel != 7 || preview.Sigils[1].SecondaryLevel != 3 {
		t.Fatalf("preview merged or lost per-trait levels: %+v", preview.Sigils)
	}
	if preview.Summons[0].MainTrait != "攻击力" || preview.Summons[0].SubParam != "暴击率" || preview.Summons[0].SubParamValue != 12 {
		t.Fatalf("summon effects are incomplete: %+v", preview.Summons[0])
	}
	if len(preview.MasterySkills) != 1 || preview.MasterySkills[0].Count != 2 {
		t.Fatalf("mastery nodes were not grouped for display: %+v", preview.MasterySkills)
	}
	if len(preview.OverLimit) != 4 || preview.OverLimit[0].Name != "昏厥值" || preview.OverLimit[0].Value != 20 || preview.OverLimit[2].Value != 20 {
		t.Fatalf("over-limit preview is incomplete: %+v", preview.OverLimit)
	}
	if len(preview.CombinedSkills) != 1 || preview.CombinedSkills[0].Level != 45 || preview.CombinedSkills[0].RawLevel != 55 {
		t.Fatalf("combined skill ledger is incomplete: %+v", preview.CombinedSkills)
	}
	wantSources := []string{
		"因子01 · 伤害上限",
		"武器 · [黑榫]幽冥华冠 · 伤害上限 Lv5",
		"武器祝福 · 隔绝之祝福 · 伤害上限 Lv10",
	}
	if !reflect.DeepEqual(preview.CombinedSkills[0].Sources, wantSources) {
		t.Fatalf("public preview sources are not fixed Simplified Chinese: got=%v want=%v", preview.CombinedSkills[0].Sources, wantSources)
	}
	if preview.Wrightstone == nil || preview.Wrightstone.Name != "隔绝之祝福" || preview.Wrightstone.Traits[0].Name != "暴击率" {
		t.Fatalf("wrightstone traits are incomplete: %+v", preview.Wrightstone)
	}
}

func TestOnlinePreviewUsesChineseCatalogIndependentlyOfDesktopLanguage(t *testing.T) {
	previous := getCurrentLanguage()
	setCurrentLanguage("en")
	t.Cleanup(func() { setCurrentLanguage(previous) })

	if got := previewChineseName("318D12E9", "Quick Cooldown"); got != "迅捷能力" {
		t.Fatalf("public preview trait = %q, want fixed Simplified Chinese", got)
	}
	if got := previewChineseName("", "Berserker Echo"); got != "狂战士" {
		t.Fatalf("public preview fallback trait = %q, want fixed Simplified Chinese", got)
	}
	if got := previewChineseWrightstoneName("Dread Wrightstone"); got != "畏惧之祝福" {
		t.Fatalf("public preview wrightstone = %q, want fixed Simplified Chinese", got)
	}
	for hash, want := range map[string]string{
		"46ABA3C0": "怒发冲冠 V",
		"E92EE838": "体力 V+",
		"9300FADB": "天星之止息 V+",
	} {
		if got := previewChineseSigilName(hash, ""); got != want {
			t.Errorf("public preview sigil %s = %q, want legal item name %q", hash, got, want)
		}
	}
	if got := previewChineseSigilNameForTraits("46ABA3C0", "怒发冲冠 V+", "怒发冲冠", "伤害上限"); got != "怒发冲冠 V+" {
		t.Fatalf("combined public preview sigil = %q, want V+ family title", got)
	}
	for _, test := range []struct {
		hash      string
		fallback  string
		primary   string
		secondary string
		want      string
	}{
		{hash: "B5B23F02", fallback: "HP V+", primary: "体力", secondary: "金刚", want: "体力 V+"},
		{hash: "80C94A24", fallback: "Precise Wrath V+", primary: "怒发冲冠", secondary: "伤害上限", want: "怒发冲冠 V+"},
		{hash: "F1D8F754", fallback: "Divergence V+", primary: "分歧", secondary: "天星之炼", want: "分歧 V+"},
		{hash: "673C5D8F", fallback: "Hero's Awakening+", primary: "勇士的信念", secondary: "勇士的毅力", want: "勇士之觉醒+"},
		{hash: "95CC3CB8", fallback: "Ultramarine's Awakening+", primary: "群青的剑光", secondary: "群青的逆境", want: "群青之觉醒+"},
		{hash: "D8A464F1", fallback: "Bladequeen's Awakening+", primary: "刃姬的小夜曲", secondary: "刃姬的轮舞曲", want: "刃姬之觉醒+"},
		{hash: "23953FD4", fallback: "Thunderwolf's Awakening+", primary: "雷狼的弹匣", secondary: "雷狼的慧眼", want: "雷狼之觉醒+"},
	} {
		if got := previewChineseSigilNameForTraits(test.hash, test.fallback, test.primary, test.secondary); got != test.want {
			t.Errorf("public preview sigil %s = %q, want %q", test.hash, got, test.want)
		}
	}
}

func TestChineseSigilItemRankSpacingDoesNotInventOrRemovePlus(t *testing.T) {
	for input, want := range map[string]string{
		"怒发冲冠V":   "怒发冲冠 V",
		"怒发冲冠IV":  "怒发冲冠 IV",
		"体力V+":    "体力 V+",
		"天星之止息V+": "天星之止息 V+",
		"躲避性能+":   "躲避性能+",
		"黑龙的战气+":  "黑龙的战气+",
	} {
		if got := normalizeChineseSigilItemName(input); got != want {
			t.Errorf("normalizeChineseSigilItemName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPreviewEncodingKeepsSkillNamesAndLevelsWhenSourcesAreTooLarge(t *testing.T) {
	preview := &loadoutSharePreview{CombinedSkills: make([]loadoutSharePreviewTrait, 50)}
	for index := range preview.CombinedSkills {
		preview.CombinedSkills[index] = loadoutSharePreviewTrait{
			Name: "攻击力", Level: 45, RawLevel: 60, MaxLevel: 45, Effect: strings.Repeat("攻击力+10%", 20),
			Sources: []string{strings.Repeat("因子来源", 100), strings.Repeat("武器来源", 100)},
		}
	}
	encoded := encodeLoadoutSharePreview(preview)
	if encoded == "" {
		t.Fatal("preview was dropped instead of compacted")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(payload) > loadoutSharePreviewMaxBytes {
		t.Fatalf("preview encoding is not bounded: len=%d err=%v", len(payload), err)
	}
	var decoded loadoutSharePreview
	if err := json.Unmarshal(payload, &decoded); err != nil || len(decoded.CombinedSkills) != 50 || decoded.CombinedSkills[0].Name != "攻击力" || decoded.CombinedSkills[0].Level != 45 {
		t.Fatalf("preview lost its skill ledger: %+v err=%v", decoded.CombinedSkills, err)
	}
}

func TestPreviewEncodingAlwaysKeepsCoreAndOverLimitSlots(t *testing.T) {
	preview := &loadoutSharePreview{
		Title: "伊欧测试", CharacterHash: "4D0A60C3", CharacterName: "伊欧",
		WeaponHash: "CDB13688", WeaponName: "[黑榫]幽冥华冠",
		MasterySkills:  make([]loadoutSharePreviewMastery, 50),
		CombinedSkills: make([]loadoutSharePreviewTrait, 80),
		OverLimit: []loadoutSharePreviewOverLimit{
			{Index: 0, AttributeHash: "6CB38EF3", Name: "昏厥值", Level: 10, Value: 20, Unit: "flat"},
			{Index: 1, AttributeHash: "6CB38EF3", Name: "昏厥值", Level: 10, Value: 20, Unit: "flat"},
			{Index: 2, AttributeHash: "43B7581D", Name: "普通攻击伤害上限", Level: 10, Value: 20, Unit: "pct"},
			{Index: 3, AttributeHash: "9C555433", Name: "能力伤害上限", Level: 10, Value: 20, Unit: "pct"},
		},
	}
	for index := range preview.MasterySkills {
		preview.MasterySkills[index] = loadoutSharePreviewMastery{Name: strings.Repeat("专精技能", 20), Effect: strings.Repeat("效果说明", 100), Count: 1}
	}
	for index := range preview.CombinedSkills {
		preview.CombinedSkills[index] = loadoutSharePreviewTrait{Name: strings.Repeat("合并技能", 20), Effect: strings.Repeat("效果说明", 100), Level: 45}
	}
	encoded := encodeLoadoutSharePreview(preview)
	if encoded == "" {
		t.Fatal("oversized preview lost its core summary")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(payload) > loadoutSharePreviewMaxBytes {
		t.Fatalf("core preview encoding is not bounded: len=%d err=%v", len(payload), err)
	}
	var decoded loadoutSharePreview
	if err := json.Unmarshal(payload, &decoded); err != nil || decoded.CharacterName != "伊欧" || len(decoded.OverLimit) != 4 || decoded.OverLimit[0].Value != 20 {
		t.Fatalf("core preview lost identity or over-limit slots: %+v err=%v", decoded, err)
	}
}

func TestLoadoutShareTitleUsesUnicodeCharactersAndWorkerLimit(t *testing.T) {
	input := strings.Repeat("伊", loadoutShareTitleMaxRunes+5)
	got := trimLoadoutShareTitle("  " + input + "  ")
	if len([]rune(got)) != loadoutShareTitleMaxRunes || !strings.HasPrefix(input, got) {
		t.Fatalf("title rune limit=%d, want %d", len([]rune(got)), loadoutShareTitleMaxRunes)
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
	source := loadoutShareCodeFixture()
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
