package backend

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/vmihailenco/msgpack/v5"
)

func sameShareCodeWrightstone(got, want *LoadoutWeaponWrightstone) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	if got.Hash != want.Hash || len(got.Traits) != len(want.Traits) {
		return false
	}
	for index := range got.Traits {
		if got.Traits[index].Index != want.Traits[index].Index ||
			got.Traits[index].Hash != want.Traits[index].Hash ||
			got.Traits[index].Level != want.Traits[index].Level {
			return false
		}
	}
	return true
}

func assertShareCodeProgression(t *testing.T, got, want *LoadoutShare) {
	t.Helper()
	if got.Character == nil || want.Character == nil ||
		got.Character.CharacterLevel != want.Character.CharacterLevel ||
		got.Character.BaseHP != want.Character.BaseHP ||
		got.Character.BaseATK != want.Character.BaseATK ||
		got.Character.BaseStunBits != want.Character.BaseStunBits ||
		got.Character.BaseCritRate != want.Character.BaseCritRate ||
		got.Character.CharacterBaseCaptured != want.Character.CharacterBaseCaptured ||
		got.Character.MasterTotalMSP != want.Character.MasterTotalMSP ||
		got.Character.LegacyProgress != want.Character.LegacyProgress ||
		got.Character.WeaponWrightstonesCaptured != want.Character.WeaponWrightstonesCaptured ||
		!reflect.DeepEqual(got.Character.EnhancementPanel, want.Character.EnhancementPanel) ||
		!reflect.DeepEqual(got.Character.EnhancementNodes, want.Character.EnhancementNodes) ||
		len(got.Character.Weapons) != len(want.Character.Weapons) {
		t.Fatal("share code changed character progression")
	}
	for index := range got.Character.Weapons {
		gotWeapon, wantWeapon := got.Character.Weapons[index], want.Character.Weapons[index]
		gotWeapon.Wrightstone, wantWeapon.Wrightstone = nil, nil
		if !reflect.DeepEqual(gotWeapon, wantWeapon) ||
			!sameShareCodeWrightstone(got.Character.Weapons[index].Wrightstone, want.Character.Weapons[index].Wrightstone) {
			t.Fatalf("share code changed progression weapon %d", index)
		}
	}
	if got.Weapon == nil || want.Weapon == nil {
		if got.Weapon != nil || want.Weapon != nil {
			t.Fatal("share code changed equipped weapon presence")
		}
		return
	}
	gotWeapon, wantWeapon := *got.Weapon, *want.Weapon
	gotWeapon.Wrightstone, wantWeapon.Wrightstone = nil, nil
	if !reflect.DeepEqual(gotWeapon, wantWeapon) ||
		!sameShareCodeWrightstone(got.Weapon.Wrightstone, want.Weapon.Wrightstone) {
		t.Fatal("share code changed equipped weapon state")
	}
}

func loadoutShareCodeFixture() *LoadoutShare {
	share := &LoadoutShare{
		Format: loadoutShareFormat, Version: loadoutShareVersion,
		CharaHash: "4D0A60C3", CharaName: "伊欧", OwnerCode: "PL0400", Name: "装备方案06",
		WeaponHash: "26E7CCB1", WeaponName: "测试武器",
		Character: &LoadoutShareCharacterProgression{
			CharacterLevel: 100, BaseHP: 3156, BaseATK: 666, BaseStunBits: math.Float32bits(8),
			BaseCritRate: 5, CharacterBaseCaptured: true, MasterTotalMSP: 562575, LegacyProgress: 100,
			EnhancementPanel: []int{100, 100}, WeaponWrightstonesCaptured: true,
		},
		Weapon: &LoadoutShareWeaponState{
			StoredHash: "26E7CCB1", XP: 912345, Uncap: 6, Mirage: 99, Awakening: 10,
			Transcendence: 7, ExactState: true, Flags: 3, State: 1,
			SkillHashes: []string{"A7726190", "664E8E32", "12345678", "23456789", "3456789A"},
			Wrightstone: &LoadoutWeaponWrightstone{
				Hash: "09E6F629",
				Traits: []LoadoutWeaponWrightstoneTrait{
					{Index: 0, Hash: "A7726190", Level: 9},
					{Index: 1, Hash: "664E8E32", Level: 5},
					{Index: 2, Hash: "12345678", Level: 3},
				},
			},
		},
	}
	for index := 0; index < 12; index++ {
		slot := index
		share.Sigils = append(share.Sigils, LoadoutShareSigil{
			Index: &slot, Hash: fmt.Sprintf("%08X", 0x10000000+index), Name: fmt.Sprintf("测试因子%02d", index+1),
			Level: 15, PrimaryTraitHash: fmt.Sprintf("%08X", 0x20000000+index), PrimaryTraitLevel: 15,
			SecondaryTraitHash: fmt.Sprintf("%08X", 0x30000000+index), SecondaryTraitLevel: 15,
		})
	}
	for index := 0; index < 4; index++ {
		share.Summons = append(share.Summons, LoadoutShareSummon{
			TypeHash: fmt.Sprintf("%08X", 0x40000000+index), Name: fmt.Sprintf("测试召唤石%d", index+1),
			MainTraitHash: fmt.Sprintf("%08X", 0x41000000+index), MainTraitLevel: 10,
			SubParamHash: fmt.Sprintf("%08X", 0x42000000+index), SubParamLevel: 5, Rank: 2,
		})
		share.Skills = append(share.Skills, LoadoutSkill{
			Hash: fmt.Sprintf("%08X", 0x50000000+index), Name: fmt.Sprintf("测试技能%d", index+1),
		})
		share.OverLimit = append(share.OverLimit, LoadoutShareOverLimit{
			Index: index, AttributeHash: fmt.Sprintf("%08X", 0x60000000+index), Level: 20,
		})
	}
	for index := 0; index < 5; index++ {
		share.WeaponSkillHashes = append(share.WeaponSkillHashes, fmt.Sprintf("%08X", 0x70000000+index))
	}
	for index := 0; index < 50; index++ {
		share.MasteryHashes = append(share.MasteryHashes, fmt.Sprintf("%08X", 0x71000000+index))
	}
	for index := 0; index < 80; index++ {
		share.Character.EnhancementNodes = append(share.Character.EnhancementNodes, LoadoutShareEnhancementNode{
			Index: index, Value: index % 4,
		})
	}
	for index := 0; index < 5; index++ {
		share.Character.Weapons = append(share.Character.Weapons, LoadoutShareProgressionWeapon{
			Hash: fmt.Sprintf("%08X", 0x72000000+index), BaseHash: fmt.Sprintf("%08X", 0x73000000+index),
			InternalID: fmt.Sprintf("WEAPON_%03d", index), Level: 150, Uncap: 6, Mirage: 99,
			Awakening: 10, Transcendence: 7, TranscendenceSkill: "SKILL_TEST",
			Wrightstone: share.Weapon.Wrightstone,
		})
	}
	return share
}

func legacyLoadoutShareCodeFixture(t *testing.T, source *LoadoutShare) string {
	t.Helper()
	compact, err := compactLoadoutShare(source)
	if err != nil {
		t.Fatal(err)
	}
	legacy := &loadoutShareCodePayloadV1{
		ShareVersion: 8, CharaHash: compact.CharaHash, CharaName: compact.CharaName,
		OwnerCode: compact.OwnerCode, Name: compact.Name, WeaponHash: compact.WeaponHash, WeaponName: compact.WeaponName,
		Sigils: compact.Sigils, Summons: compact.Summons, Skills: compact.Skills,
		WeaponSkillHashes: compact.WeaponSkillHashes, MasteryHashes: compact.MasteryHashes,
		Weapon: compact.Weapon, OverLimit: compact.OverLimit,
	}
	if compact.Character != nil {
		legacy.Character = &loadoutShareCodeCharacterV1{
			CharacterLevel: compact.Character.CharacterLevel, MasterTotalMSP: compact.Character.MasterTotalMSP,
			LegacyProgress: compact.Character.LegacyProgress, EnhancementPanel: compact.Character.EnhancementPanel,
			EnhancementNodes: compact.Character.EnhancementNodes, Weapons: compact.Character.Weapons,
			WeaponWrightstonesCaptured: compact.Character.WeaponWrightstonesCaptured,
		}
	}
	packed, err := msgpack.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	writer := brotli.NewWriterLevel(&compressed, brotli.BestCompression)
	if _, err := writer.Write(packed); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	frame := make([]byte, loadoutShareCodeHeaderSize, loadoutShareCodeHeaderSize+compressed.Len())
	copy(frame[:4], loadoutShareCodeFrameMagic)
	frame[4] = loadoutShareCodeLegacyFrameVersion
	frame[5] = loadoutShareCodeCodec
	binary.LittleEndian.PutUint32(frame[6:10], uint32(len(packed)))
	binary.LittleEndian.PutUint32(frame[10:14], crc32.ChecksumIEEE(packed))
	binary.LittleEndian.PutUint32(frame[14:18], uint32(compressed.Len()))
	frame = append(frame, compressed.Bytes()...)
	return loadoutShareCodePrefix + base64.RawURLEncoding.EncodeToString(frame)
}

func shareCodeFrameFixture(t *testing.T, compact *loadoutShareCodePayload, frameVersion byte) string {
	t.Helper()
	packed, err := msgpack.Marshal(compact)
	if err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	writer := brotli.NewWriterLevel(&compressed, brotli.BestCompression)
	if _, err := writer.Write(packed); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	frame := make([]byte, loadoutShareCodeHeaderSize, loadoutShareCodeHeaderSize+compressed.Len())
	copy(frame[:4], loadoutShareCodeFrameMagic)
	frame[4] = frameVersion
	frame[5] = loadoutShareCodeCodec
	binary.LittleEndian.PutUint32(frame[6:10], uint32(len(packed)))
	binary.LittleEndian.PutUint32(frame[10:14], crc32.ChecksumIEEE(packed))
	binary.LittleEndian.PutUint32(frame[14:18], uint32(compressed.Len()))
	frame = append(frame, compressed.Bytes()...)
	return loadoutShareCodePrefix + base64.RawURLEncoding.EncodeToString(frame)
}

func TestLoadoutShareCodeStillDecodesV10Frame(t *testing.T) {
	source := loadoutShareCodeFixture()
	compact, err := compactLoadoutShare(source)
	if err != nil {
		t.Fatal(err)
	}
	compact.ShareVersion = 10
	decoded, err := decodeLoadoutShareCode(shareCodeFrameFixture(t, compact, loadoutShareCodeFrameVersion))
	if err != nil {
		t.Fatalf("decode v10 frame: %v", err)
	}
	if decoded.Version != 10 || decoded.Character == nil || !reflect.DeepEqual(decoded.Character.EnhancementNodes, source.Character.EnhancementNodes) {
		t.Fatalf("v10 frame changed during decode: %+v", decoded)
	}
}

func TestLoadoutShareCodeStillDecodesLegacyV8Frame(t *testing.T) {
	source := loadoutShareCodeFixture()
	decoded, err := decodeLoadoutShareCode(legacyLoadoutShareCodeFixture(t, source))
	if err != nil {
		t.Fatalf("decode legacy v8 code: %v", err)
	}
	if decoded.Version != 8 || decoded.Character == nil {
		t.Fatalf("legacy share identity changed: %+v", decoded)
	}
	if decoded.Character.CharacterBaseCaptured || decoded.Character.BaseHP != 0 ||
		decoded.Character.BaseATK != 0 || decoded.Character.BaseStunBits != 0 ||
		decoded.Character.BaseCritRate != 0 {
		t.Fatalf("legacy v8 code fabricated a character base snapshot: %+v", decoded.Character)
	}
	if decoded.Character.CharacterLevel != source.Character.CharacterLevel ||
		decoded.Character.MasterTotalMSP != source.Character.MasterTotalMSP ||
		!reflect.DeepEqual(decoded.Character.EnhancementPanel, source.Character.EnhancementPanel) ||
		!reflect.DeepEqual(decoded.Character.EnhancementNodes, source.Character.EnhancementNodes) {
		t.Fatal("legacy v8 character progression changed")
	}
}

func TestLoadoutShareCodeRoundTrip(t *testing.T) {
	source := loadoutShareCodeFixture()
	encoded, err := encodeLoadoutShareCode(source)
	if err != nil {
		t.Fatalf("encode share code: %v", err)
	}
	if !strings.HasPrefix(encoded.CompatibilityCode, loadoutShareCodePrefix) {
		t.Fatalf("share code prefix = %q", encoded.CompatibilityCode[:min(16, len(encoded.CompatibilityCode))])
	}
	if encoded.FrameBytes >= encoded.JSONBytes {
		t.Fatalf("share code did not shrink JSON: frame=%d json=%d", encoded.FrameBytes, encoded.JSONBytes)
	}
	if encoded.FrameBytes > 2048 {
		t.Fatalf("real complete loadout frame unexpectedly large: %d bytes", encoded.FrameBytes)
	}

	decoded, err := decodeLoadoutShareCode(encoded.CompatibilityCode)
	if err != nil {
		t.Fatalf("decode share code: %v", err)
	}
	if decoded.Format != source.Format || decoded.Version != source.Version ||
		decoded.CharaHash != source.CharaHash || decoded.OwnerCode != source.OwnerCode ||
		decoded.Name != source.Name || decoded.WeaponHash != source.WeaponHash {
		t.Fatalf("share identity changed: got=%+v want=%+v", decoded, source)
	}
	if !reflect.DeepEqual(decoded.Sigils, source.Sigils) ||
		!reflect.DeepEqual(decoded.Summons, source.Summons) ||
		!reflect.DeepEqual(decoded.Skills, source.Skills) ||
		!reflect.DeepEqual(decoded.WeaponSkillHashes, source.WeaponSkillHashes) ||
		!reflect.DeepEqual(decoded.MasteryHashes, source.MasteryHashes) ||
		!reflect.DeepEqual(decoded.OverLimit, source.OverLimit) {
		t.Fatal("share code changed an equipped loadout field")
	}
	assertShareCodeProgression(t, decoded, source)

	t.Logf("json=%d packed=%d compressed=%d frame=%d compatibility=%d chars",
		encoded.JSONBytes, encoded.PackedBytes, encoded.CompressedBytes, encoded.FrameBytes, len(encoded.CompatibilityCode))
}

func TestLoadoutShareCodeRejectsCorruptionAndAcceptsWrappedText(t *testing.T) {
	source := loadoutShareCodeFixture()
	encoded, err := encodeLoadoutShareCode(source)
	if err != nil {
		t.Fatalf("encode share code: %v", err)
	}
	body := strings.TrimPrefix(encoded.CompatibilityCode, loadoutShareCodePrefix)
	wrapped := loadoutShareCodePrefix + body[:len(body)/2] + "\r\n " + body[len(body)/2:]
	if _, err := decodeLoadoutShareCode(wrapped); err != nil {
		t.Fatalf("wrapped code should decode: %v", err)
	}

	offset := len(loadoutShareCodePrefix) + len(body)/2
	last := encoded.CompatibilityCode[offset]
	replacement := byte('A')
	if last == replacement {
		replacement = 'B'
	}
	corrupt := encoded.CompatibilityCode[:offset] + string(replacement) + encoded.CompatibilityCode[offset+1:]
	if _, err := decodeLoadoutShareCode(corrupt); err == nil {
		t.Fatal("corrupted share code was accepted")
	}
	if _, err := decodeLoadoutShareCode(encoded.CompatibilityCode + "A"); err == nil {
		t.Fatal("share code with appended data was accepted")
	}
	if _, err := decodeLoadoutShareCode("not-a-share-code"); err == nil {
		t.Fatal("invalid prefix was accepted")
	}
}

func TestLoadoutShareCodeRealFixtureResolvesIdentically(t *testing.T) {
	_, source := actualLoadoutShareFixture(t)
	encoded, err := encodeLoadoutShareCode(source)
	if err != nil {
		t.Fatalf("encode real share: %v", err)
	}
	decoded, err := decodeLoadoutShareCode(encoded.CompatibilityCode)
	if err != nil {
		t.Fatalf("decode real share: %v", err)
	}
	if !reflect.DeepEqual(decoded.Sigils, source.Sigils) ||
		!reflect.DeepEqual(decoded.Summons, source.Summons) ||
		!reflect.DeepEqual(decoded.Skills, source.Skills) ||
		!reflect.DeepEqual(decoded.WeaponSkillHashes, source.WeaponSkillHashes) ||
		!reflect.DeepEqual(decoded.MasteryHashes, source.MasteryHashes) ||
		!reflect.DeepEqual(decoded.OverLimit, source.OverLimit) {
		t.Fatal("decoded real share changed an equipped loadout field")
	}
	assertShareCodeProgression(t, decoded, source)
	t.Logf("real json=%d packed=%d compressed=%d frame=%d compatibility=%d chars",
		encoded.JSONBytes, encoded.PackedBytes, encoded.CompressedBytes, encoded.FrameBytes, len(encoded.CompatibilityCode))
}
