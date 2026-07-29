package backend

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	page, err := app.SaveDiffPage(0, 80, "", "all")
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
