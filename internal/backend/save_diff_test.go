package backend

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildSaveDiffSessionLocatesTypedChanges(t *testing.T) {
	left := &SaveGameFile{SlotData: &SaveDataBinary{VersionMaybe: 7, UIntTable: []UIntSaveDataUnit{
		{IDType: 1801, UnitID: 1, ValueData: []uint32{10, 20}},
		{IDType: 1801, UnitID: 2, ValueData: []uint32{30}},
	}}}
	right := &SaveGameFile{SlotData: &SaveDataBinary{VersionMaybe: 8, UIntTable: []UIntSaveDataUnit{
		{IDType: 1801, UnitID: 1, ValueData: []uint32{10, 21}},
		{IDType: 1801, UnitID: 3, ValueData: []uint32{40}},
	}}}
	session := buildSaveDiffSession("left.dat", "right.dat", left, right)
	if session.summary.Changed != 1 || session.summary.Added != 1 || session.summary.Removed != 1 || session.summary.Different != 3 {
		t.Fatalf("unexpected summary: %+v", session.summary)
	}
	if session.summary.LeftVersion != 7 || session.summary.RightVersion != 8 {
		t.Fatalf("versions lost: %+v", session.summary)
	}
	for _, entry := range session.entries {
		if entry.ValueType != "uint32" || entry.Section != "slotData" || entry.LeftHash == entry.RightHash && entry.Status == "changed" {
			t.Fatalf("typed location/hash missing: %+v", entry)
		}
	}
}

func TestBuildSaveDiffSessionIndexesLargeEntitySetsOnce(t *testing.T) {
	const recordCount = 4096
	leftUnits := make([]UIntSaveDataUnit, recordCount)
	rightUnits := make([]UIntSaveDataUnit, recordCount)
	for index := range leftUnits {
		unit := UIntSaveDataUnit{
			IDType:    WrightstoneItemIDType,
			UnitID:    uint32(index + 1),
			ValueData: []uint32{0x60AC32C8},
		}
		leftUnits[index] = unit
		rightUnits[index] = unit
	}
	left := &SaveGameFile{SlotData: &SaveDataBinary{UIntTable: leftUnits}}
	right := &SaveGameFile{SlotData: &SaveDataBinary{UIntTable: rightUnits}}

	started := time.Now()
	session := buildSaveDiffSession("left.dat", "right.dat", left, right)
	elapsed := time.Since(started)

	if session.summary.Unchanged != recordCount || session.summary.Different != 0 {
		t.Fatalf("large indexed comparison returned the wrong summary: %+v", session.summary)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("large indexed comparison took %s; entity catalogs or record lookups may be running once per row", elapsed)
	}
}

func TestSaveDiffCSVContainsOnlySanitizedEvidence(t *testing.T) {
	rows := []saveDiffExportRow{{Section: "slotData", ValueType: "uint32", SemanticName: "Weapon ID", IDType: 1801, UnitID: 7, Status: "changed", LeftIndex: 1, RightIndex: 1, LeftCount: 2, RightCount: 2, LeftHash: "aaaa", RightHash: "bbbb"}}
	data, err := encodeSaveDiffCSV(rows)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("CSV must include a UTF-8 BOM for spreadsheet compatibility")
	}
	records, err := csv.NewReader(bytes.NewReader(data[3:])).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1][2] != "Weapon ID" || records[1][13] != "aaaa" {
		t.Fatalf("unexpected sanitized CSV: %#v", records)
	}
	for _, forbidden := range []string{"SaveData3.dat", "[123, 456]", "leftPreview", "rightPreview"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("CSV leaked %q", forbidden)
		}
	}
}

func TestSaveDiffIdenticalInputIsEmpty(t *testing.T) {
	save := &SaveGameFile{Binary1: &SaveDataBinary{BoolTable: []BoolSaveDataUnit{{IDType: 7102, UnitID: 4, ValueData: []bool{true, false}}}}}
	session := buildSaveDiffSession("a.dat", "b.dat", save, save)
	if session.summary.Different != 0 || session.summary.Unchanged != 1 {
		t.Fatalf("identical save should have no differences: %+v", session.summary)
	}
}

func TestSaveDiffDuplicateGroupInsertionDoesNotCascade(t *testing.T) {
	unit := func(value uint32) UIntSaveDataUnit {
		return UIntSaveDataUnit{IDType: 1801, UnitID: 7, ValueData: []uint32{value}}
	}
	left := &SaveGameFile{SlotData: &SaveDataBinary{UIntTable: []UIntSaveDataUnit{unit(10), unit(20), unit(30)}}}
	right := &SaveGameFile{SlotData: &SaveDataBinary{UIntTable: []UIntSaveDataUnit{unit(99), unit(10), unit(20), unit(30)}}}
	session := buildSaveDiffSession("left", "right", left, right)
	if session.summary.Added != 1 || session.summary.Changed != 0 || session.summary.Removed != 0 || session.summary.Unchanged != 3 {
		t.Fatalf("duplicate insertion cascaded into false changes: %+v", session.summary)
	}
}

func TestSaveDiffDuplicateGroupReorderMatchesByContent(t *testing.T) {
	unit := func(value uint32) UIntSaveDataUnit {
		return UIntSaveDataUnit{IDType: 1801, UnitID: 7, ValueData: []uint32{value}}
	}
	left := &SaveGameFile{SlotData: &SaveDataBinary{UIntTable: []UIntSaveDataUnit{unit(10), unit(20), unit(30)}}}
	right := &SaveGameFile{SlotData: &SaveDataBinary{UIntTable: []UIntSaveDataUnit{unit(30), unit(10), unit(20)}}}
	session := buildSaveDiffSession("left", "right", left, right)
	if session.summary.Different != 0 || session.summary.Unchanged != 3 {
		t.Fatalf("pure duplicate reorder should preserve logical equality: %+v", session.summary)
	}
	for _, entry := range session.entries {
		if entry.LeftOccurrence < 0 || entry.RightOccurrence < 0 {
			t.Fatalf("physical occurrences were not preserved: %+v", entry)
		}
	}
}

func TestSaveDiffMalformedInputsAreNeverModified(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.dat")
	right := filepath.Join(dir, "right.dat")
	original := []byte("truncated-save")
	if err := os.WriteFile(left, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, original, 0o600); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	if _, err := app.OpenSaveDiff(left, right); err == nil {
		t.Fatal("expected malformed save error")
	}
	for _, path := range []string{left, right} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(original) {
			t.Fatalf("input changed: %s", path)
		}
	}
}

func TestSaveDiffRejectsOversizedInputBeforeReadingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.dat")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(saveDiffMaximumFileBytes + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSaveDiffFile(path); err == nil || !strings.Contains(err.Error(), "64 MiB") {
		t.Fatalf("expected bounded-input error, got %v", err)
	}
}

func TestSaveDiffOversizedInputErrorFollowsApplicationLanguage(t *testing.T) {
	previous := getCurrentLanguage()
	setCurrentLanguage("en")
	t.Cleanup(func() { setCurrentLanguage(previous) })

	path := filepath.Join(t.TempDir(), "oversized.dat")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(saveDiffMaximumFileBytes + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = loadSaveDiffFile(path)
	if err == nil {
		t.Fatal("expected bounded-input error")
	}
	if strings.ContainsAny(err.Error(), "存档超过只读比较上限") || !strings.Contains(err.Error(), "64 MiB") {
		t.Fatalf("English error leaked Chinese or lost its limit: %q", err)
	}
}

func TestSaveDiffMalformedInputErrorFollowsApplicationLanguage(t *testing.T) {
	previous := getCurrentLanguage()
	setCurrentLanguage("en")
	t.Cleanup(func() { setCurrentLanguage(previous) })

	path := filepath.Join(t.TempDir(), "malformed.dat")
	if err := os.WriteFile(path, []byte("truncated-save"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadSaveDiffFile(path)
	if err == nil {
		t.Fatal("expected malformed-save error")
	}
	if strings.ContainsAny(err.Error(), "文件太小不是有效的存档") || !strings.Contains(err.Error(), "too small") {
		t.Fatalf("English parse error leaked Chinese or lost its cause: %q", err)
	}
}

func TestSaveDiffExportSchemaContainsNoPathsOrRawPreviews(t *testing.T) {
	summary := SaveDiffSummary{LeftName: "SteamID-left.dat", RightName: "right.dat", LeftVersion: 1, RightVersion: 1, Different: 1}
	entry := SaveDiffEntry{Section: "slotData", ValueType: "uint64", IDType: 9999, UnitID: 1, Status: "changed", LeftHash: "aaaa", RightHash: "bbbb", LeftPreview: "[76561198000000000]"}
	payload := saveDiffExport{SchemaVersion: 1, Summary: saveDiffExportStats{LeftVersion: summary.LeftVersion, RightVersion: summary.RightVersion, Different: 1}, Changes: []saveDiffExportRow{{Section: entry.Section, ValueType: entry.ValueType, IDType: entry.IDType, UnitID: entry.UnitID, Status: entry.Status, LeftHash: entry.LeftHash, RightHash: entry.RightHash}}}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{summary.LeftName, summary.RightName, entry.LeftPreview, `"leftPreview"`, `"rightPreview"`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("export leaked %q: %s", forbidden, text)
		}
	}
}

func TestSaveDiffRealLocalSaveIsReadOnlyAndPageable(t *testing.T) {
	left := filepath.Join(defaultSaveGamesDir(), "SaveData3.dat")
	right := filepath.Join(defaultSaveGamesDir(), "SaveData3_BackUp.dat")
	for _, path := range []string{left, right} {
		if _, err := os.Stat(path); err != nil {
			t.Skip("local real-save pair is unavailable")
		}
	}
	before := make(map[string][32]byte, 2)
	for _, path := range []string{left, right} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = sha256.Sum256(data)
	}
	app := NewApp()
	summary, err := app.OpenSaveDiff(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if summary.LeftRecords == 0 || summary.RightRecords == 0 {
		t.Fatalf("real saves produced empty logical records: %+v", summary)
	}
	page, err := app.SaveDiffPage(0, 80, "", "all", "all", "all", "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) == 0 || len(page.Items) > 80 || page.TotalFiltered < len(page.Items) {
		t.Fatalf("invalid real-save page: %+v", page)
	}
	for _, path := range []string{left, right} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if sha256.Sum256(data) != before[path] {
			t.Fatalf("real source save was modified: %s", path)
		}
	}
}

func saveDiffTransferFixture(t *testing.T) string {
	t.Helper()
	candidates := []string{testLoadoutSave}
	for slot := 1; slot <= 3; slot++ {
		candidates = append(candidates, filepath.Join(defaultSaveGamesDir(), "SaveData"+string(rune('0'+slot))+".dat"))
	}
	for _, candidate := range candidates {
		if candidate != "" {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	t.Skip("no real save is available for a copy-only transfer test")
	return ""
}

func prepareSaveDiffTransferPair(t *testing.T) (leftPath, rightPath string, changed SaveDiffEntry) {
	t.Helper()
	fixture := saveDiffTransferFixture(t)
	original, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	leftPath = filepath.Join(t.TempDir(), "left.dat")
	rightPath = filepath.Join(filepath.Dir(leftPath), "right.dat")
	if err := os.WriteFile(leftPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	right := append([]byte(nil), original...)
	parsed, err := ParseSaveData(right)
	if err != nil {
		t.Fatal(err)
	}
	var record saveDiffRecord
	for _, candidate := range flattenSaveRecords(parsed) {
		semantic := saveDiffSemanticFor(candidate.idType)
		if candidate.section == "slotData" &&
			candidate.idType != SaveID_HashSeed &&
			candidate.count > 0 &&
			semantic.Confidence == "known" &&
			!saveDiffEntityMustMatch(semantic.Category) {
			record = candidate
			break
		}
	}
	if record.count == 0 {
		t.Skip("real save has no safe uint32 record for a transfer test")
	}
	probe := SaveDiffEntry{
		Section: record.section, ValueType: record.valueType, IDType: record.idType,
		UnitID: record.unitID, LeftOccurrence: record.occurrence, RightOccurrence: record.occurrence,
	}
	vector, err := locateSaveDiffVector(right, probe, record.occurrence)
	if err != nil {
		t.Fatal(err)
	}
	vector.values[0] ^= 1
	if err := os.WriteFile(rightPath, right, 0o600); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	if _, err := app.OpenSaveDiff(leftPath, rightPath); err != nil {
		t.Fatal(err)
	}
	page, err := app.SaveDiffPage(0, 200, "", "different", "all", "all", "all")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range page.Items {
		if entry.Section == record.section && entry.ValueType == record.valueType && entry.IDType == record.idType && entry.UnitID == record.unitID {
			if !entry.CopySupported {
				t.Fatalf("same-structure record was not copyable: %+v", entry)
			}
			return leftPath, rightPath, entry
		}
	}
	t.Fatal("the prepared value change was not found in the diff")
	return "", "", SaveDiffEntry{}
}

func TestApplySaveDiffTransfersCopiesBacksUpAndReadsBack(t *testing.T) {
	leftPath, rightPath, entry := prepareSaveDiffTransferPair(t)
	app := NewApp()
	if _, err := app.OpenSaveDiff(leftPath, rightPath); err != nil {
		t.Fatal(err)
	}
	result, err := app.ApplySaveDiffTransfers(SaveDiffTransferRequest{TargetSide: "left", Keys: []string{entry.Key}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 1 || result.Verified != 1 || result.TargetName != filepath.Base(leftPath) {
		t.Fatalf("unexpected transfer result: %+v", result)
	}
	if result.BackupPath == "" {
		t.Fatal("in-place transfer did not report a backup")
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("reported transfer backup is missing: %v", err)
	}
	left, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	right, err := os.ReadFile(rightPath)
	if err != nil {
		t.Fatal(err)
	}
	leftVector, err := locateSaveDiffVector(left, entry, entry.LeftOccurrence)
	if err != nil {
		t.Fatal(err)
	}
	rightVector, err := locateSaveDiffVector(right, entry, entry.RightOccurrence)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(leftVector.values, rightVector.values) {
		t.Fatal("target vector does not match its source after verified transfer")
	}
}

func TestApplySaveDiffTransfersRejectsStaleTargetBeforeBackup(t *testing.T) {
	leftPath, rightPath, entry := prepareSaveDiffTransferPair(t)
	app := NewApp()
	if _, err := app.OpenSaveDiff(leftPath, rightPath); err != nil {
		t.Fatal(err)
	}
	left, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	left[len(left)-1] ^= 1
	if err := os.WriteFile(leftPath, left, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = app.ApplySaveDiffTransfers(SaveDiffTransferRequest{TargetSide: "left", Keys: []string{entry.Key}})
	if err == nil || (!strings.Contains(err.Error(), "重新比较") && !strings.Contains(err.Error(), "Compare again")) {
		t.Fatalf("stale target should fail closed, got %v", err)
	}
	backups, globErr := filepath.Glob(leftPath + ".pre-edit-*.bak")
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(backups) != 0 {
		t.Fatalf("stale-target rejection created backups: %v", backups)
	}
}

func TestOpenSaveDiffRejectsSelectingTheSameSaveTwice(t *testing.T) {
	fixture := saveDiffTransferFixture(t)
	if _, err := NewApp().OpenSaveDiff(fixture, fixture); err == nil {
		t.Fatal("the same save was accepted on both sides")
	}
}

func TestSaveDiffSemanticMetadataKeepsUnknownFieldsExplicit(t *testing.T) {
	left := &SaveGameFile{SlotData: &SaveDataBinary{UIntTable: []UIntSaveDataUnit{
		{IDType: SaveID_Rupees, UnitID: 0, ValueData: []uint32{100}},
		{IDType: 9999, UnitID: 77, ValueData: []uint32{1}},
	}}}
	right := &SaveGameFile{SlotData: &SaveDataBinary{UIntTable: []UIntSaveDataUnit{
		{IDType: SaveID_Rupees, UnitID: 0, ValueData: []uint32{200}},
		{IDType: 9999, UnitID: 77, ValueData: []uint32{2}},
	}}}
	session := buildSaveDiffSession("left", "right", left, right)
	var known, unknown *SaveDiffEntry
	for index := range session.entries {
		switch session.entries[index].IDType {
		case SaveID_Rupees:
			known = &session.entries[index]
		case 9999:
			unknown = &session.entries[index]
		}
	}
	if known == nil || known.Category != "currency" || known.SemanticConfidence != "known" ||
		known.SemanticNameZh != "卢比" || known.SemanticNameEn != "Rupees" ||
		known.SemanticPurposeZh == "" || known.SemanticPurposeEn == "" {
		t.Fatalf("known field lost bilingual semantics: %+v", known)
	}
	if unknown == nil || unknown.Category != "unknown" || unknown.SemanticConfidence != "unknown" ||
		unknown.SemanticNameZh != "未知字段" || unknown.SemanticNameEn != "Unknown Field" ||
		!strings.Contains(unknown.SemanticPurposeZh, "尚无可重复证据") ||
		!strings.Contains(unknown.SemanticPurposeEn, "No repeatable evidence") {
		t.Fatalf("unknown field was hidden or guessed: %+v", unknown)
	}
}

func TestSaveDiffPageFiltersAndSearchesBilingualSemantics(t *testing.T) {
	left := &SaveGameFile{SlotData: &SaveDataBinary{UIntTable: []UIntSaveDataUnit{
		{IDType: SaveID_Rupees, UnitID: 0, ValueData: []uint32{100}},
		{IDType: SaveID_HashSeed, UnitID: 0, ValueData: []uint32{1}},
		{IDType: 9999, UnitID: 77, ValueData: []uint32{1}},
	}}}
	right := &SaveGameFile{SlotData: &SaveDataBinary{UIntTable: []UIntSaveDataUnit{
		{IDType: SaveID_Rupees, UnitID: 0, ValueData: []uint32{200}},
		{IDType: SaveID_HashSeed, UnitID: 0, ValueData: []uint32{2}},
		{IDType: 9999, UnitID: 77, ValueData: []uint32{2}},
	}}}
	app := NewApp()
	app.saveDiffSession = buildSaveDiffSession("left", "right", left, right)

	assertIDs := func(search, category, copyability, confidence string, want ...uint32) {
		t.Helper()
		page, err := app.SaveDiffPage(0, 80, search, "different", category, copyability, confidence)
		if err != nil {
			t.Fatal(err)
		}
		got := make([]uint32, 0, len(page.Items))
		for _, entry := range page.Items {
			got = append(got, entry.IDType)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("filter (%q, %q, %q, %q) returned %v, want %v", search, category, copyability, confidence, got, want)
		}
	}

	assertIDs("卢比", "all", "all", "all", SaveID_Rupees)
	assertIDs("Rupees", "all", "all", "all", SaveID_Rupees)
	assertIDs("0x00000450", "all", "all", "all", SaveID_Rupees)
	assertIDs("", "currency", "copyable", "known", SaveID_Rupees)
	assertIDs("", "system", "blocked", "known", SaveID_HashSeed)
	assertIDs("", "unknown", "all", "unknown", 9999)
}

func TestSaveDiffResolvesCharacterLevelFromSameSideIdentity(t *testing.T) {
	left := &SaveGameFile{SlotData: &SaveDataBinary{
		IntTable:  []IntSaveDataUnit{{IDType: 1308, UnitID: 10004, ValueData: []int32{88}}},
		UIntTable: []UIntSaveDataUnit{{IDType: SaveID_CharacterID, UnitID: 10004, ValueData: []uint32{0x4D0A60C3}}},
	}}
	right := &SaveGameFile{SlotData: &SaveDataBinary{
		IntTable:  []IntSaveDataUnit{{IDType: 1308, UnitID: 10004, ValueData: []int32{100}}},
		UIntTable: []UIntSaveDataUnit{{IDType: SaveID_CharacterID, UnitID: 10004, ValueData: []uint32{0x4D0A60C3}}},
	}}
	session := buildSaveDiffSession("left", "right", left, right)
	var level *SaveDiffEntry
	for index := range session.entries {
		if session.entries[index].IDType == 1308 {
			level = &session.entries[index]
			break
		}
	}
	if level == nil {
		t.Fatal("character-level difference was not found")
	}
	if level.LeftEntity.NameZh != "伊欧" || level.LeftEntity.NameEn != "Io" ||
		level.RightEntity.Key != level.LeftEntity.Key ||
		level.LeftDisplayZh != "Lv88" || level.RightDisplayZh != "Lv100" ||
		!level.CopySupported || level.RiskLevel != "low" {
		t.Fatalf("character-level semantics were not resolved: %+v", level)
	}
}

func TestSaveDiffBlocksSameUnitIDWhenCharacterIdentityChanged(t *testing.T) {
	left := &SaveGameFile{SlotData: &SaveDataBinary{
		IntTable:  []IntSaveDataUnit{{IDType: 1308, UnitID: 10004, ValueData: []int32{88}}},
		UIntTable: []UIntSaveDataUnit{{IDType: SaveID_CharacterID, UnitID: 10004, ValueData: []uint32{0x4D0A60C3}}},
	}}
	right := &SaveGameFile{SlotData: &SaveDataBinary{
		IntTable:  []IntSaveDataUnit{{IDType: 1308, UnitID: 10004, ValueData: []int32{100}}},
		UIntTable: []UIntSaveDataUnit{{IDType: SaveID_CharacterID, UnitID: 10004, ValueData: []uint32{0xBDEF7181}}},
	}}
	session := buildSaveDiffSession("left", "right", left, right)
	for _, entry := range session.entries {
		if entry.IDType != 1308 {
			continue
		}
		if entry.LeftEntity.NameZh != "伊欧" || entry.RightEntity.NameZh != "珀西瓦尔" ||
			entry.CopySupported || entry.RiskLevel != "blocked" ||
			!strings.Contains(entry.CopyBlockReasonZh, "具体对象不同") {
			t.Fatalf("cross-character UnitID collision was not blocked: %+v", entry)
		}
		return
	}
	t.Fatal("character-level difference was not found")
}

func TestSaveDiffResolvesInventoryItemAndQuantityFromCatalog(t *testing.T) {
	const unitID uint32 = 70001
	left := &SaveGameFile{SlotData: &SaveDataBinary{
		IntTable:  []IntSaveDataUnit{{IDType: SaveID_ItemCount, UnitID: unitID, ValueData: []int32{3}}},
		UIntTable: []UIntSaveDataUnit{{IDType: SaveID_ItemID, UnitID: unitID, ValueData: []uint32{0xDB1D4F35}}},
	}}
	right := &SaveGameFile{SlotData: &SaveDataBinary{
		IntTable:  []IntSaveDataUnit{{IDType: SaveID_ItemCount, UnitID: unitID, ValueData: []int32{9}}},
		UIntTable: []UIntSaveDataUnit{{IDType: SaveID_ItemID, UnitID: unitID, ValueData: []uint32{0xDB1D4F35}}},
	}}
	session := buildSaveDiffSession("left", "right", left, right)
	for _, entry := range session.entries {
		if entry.IDType != SaveID_ItemCount {
			continue
		}
		if entry.LeftEntity.NameZh != "圆石" || entry.LeftEntity.NameEn != "Cobblestone" ||
			entry.LeftDisplayZh != "×3" || entry.RightDisplayZh != "×9" || !entry.CopySupported {
			t.Fatalf("inventory item was not resolved from the 2.0.2 catalog: %+v", entry)
		}
		return
	}
	t.Fatal("item-count difference was not found")
}
