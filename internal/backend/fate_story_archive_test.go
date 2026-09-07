package backend

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFateArchiveAssociationsMatchExtracted205Table(t *testing.T) {
	p, err := filepath.Abs(filepath.Join("..", "..", "..", "field-extracted-205-audit", "tables-205.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Skip("extracted 2.0.5 table is unavailable")
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(p)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT Key, StoryNoteArchiveId1, StoryNoteArchiveId2 FROM fate_episode")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	fromGame := map[string]bool{}
	for rows.Next() {
		var key, a, b string
		if err := rows.Scan(&key, &a, &b); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{a, b} {
			if id != "" {
				fromGame[key+":"+id] = true
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range fateStoryArchiveCatalog {
		for _, key := range row.finalEpisodeKeys {
			if !fromGame[key+":"+row.archiveID] {
				t.Fatalf("association not in game table: %s %s", key, row.archiveID)
			}
			count++
		}
	}
	if count != len(fromGame) {
		t.Fatalf("missing table associations: %d/%d", count, len(fromGame))
	}
}

func TestGame205FateArchiveNativeStorageAndUnlock(t *testing.T) {
	_, sections := loadLocalGame205Sections(t)
	for _, check := range []struct {
		rva   uint32
		bytes string
	}{
		{0xC12C9, "48b800000000dd1e0000"},                  // registered 7901: archive hash
		{0xC1381, "48b800000000de1e0000"},                  // registered 7902: flags
		{0x413DEC, "8b510489d083c80139c2740b4883c1048901"}, // flags |= 1
		{0x3CEBFD5, "f6400401"},                            // archive list tests bit 0
		{0x3CEC0AE, "8b4a0489c883c802"},                    // viewing sets bit 1 separately
	} {
		want, err := hex.DecodeString(check.bytes)
		if err != nil {
			t.Fatal(err)
		}
		got := runtimePatchLocalBytesAtRVA(sections, check.rva, len(want))
		if !bytes.Equal(got, want) {
			t.Fatalf("native archive contract changed at %X: %X", check.rva, got)
		}
	}
}

func archiveFixture(t *testing.T) (string, *SaveData) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(defaultSaveGamesDir(), "SaveData3.dat"))
	if err != nil {
		t.Skip("local save fixture unavailable")
	}
	path := filepath.Join(t.TempDir(), "ArchiveFixture.dat")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, save
}
func saveArchiveFixture(t *testing.T, path string, save *SaveData) {
	t.Helper()
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, save.data, 0o600); err != nil {
		t.Fatal(err)
	}
}
func archiveRequest(t *testing.T, path string, ids ...string) FateStoryArchiveRepairRequest {
	t.Helper()
	s, err := (&App{}).FateEpisodeEditableInspect(path)
	if err != nil {
		t.Fatal(err)
	}
	return FateStoryArchiveRepairRequest{Path: path, ExpectedRevision: s.Revision, ArchiveIDs: ids}
}

func TestFateStoryArchiveCatalogCoversEveryPlayableCharacter(t *testing.T) {
	if len(fateStoryArchiveCatalog) != 29 {
		t.Fatal("incomplete archive catalog")
	}
	owners := map[string]bool{}
	ids := map[string]bool{}
	for _, row := range fateStoryArchiveCatalog {
		if ids[row.archiveID] {
			t.Fatal("duplicate archive")
		}
		ids[row.archiveID] = true
		for _, owner := range row.characterCodes {
			owners[owner] = true
		}
	}
	for _, owner := range fateCharacterCodes {
		if !owners[owner] {
			t.Fatalf("missing %s", owner)
		}
	}
	if !ids["ARC_OTHER_069"] || gbfrHash32("ARC_OTHER_069") != 0x1e6d7154 {
		t.Fatal("Fediel identity mismatch")
	}
}

func TestFateStoryArchiveNativeUnlockAllCharactersPreservesEveryOtherByte(t *testing.T) {
	path, save := archiveFixture(t)
	records, err := readFateArchiveRecords(save)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := inspectFateEpisodeLayout(save)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	flagsBefore := map[uint32]uint32{}
	for i, row := range fateStoryArchiveCatalog {
		for _, key := range row.finalEpisodeKeys {
			layout.stateByHash[gbfrHash32(key)].SetUint32(30)
		}
		hash := gbfrHash32(row.archiveID)
		record, ok := records.byHash[hash]
		if !ok {
			t.Fatalf("fixture lacks %s", row.archiveID)
		}
		flags := []uint32{0, 2, 0x80}[i%3]
		record.flags.SetUint32(flags)
		flagsBefore[hash] = flags
		ids = append(ids, row.archiveID)
	}
	saveArchiveFixture(t, path, save)
	before := append([]byte(nil), save.data...)
	request := archiveRequest(t, path, ids...)
	result, err := (&App{}).RepairFateStoryArchives(request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed != 29 || result.Verified != 29 || result.BackupPath == "" {
		t.Fatalf("repair incomplete: %+v", result)
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil || !bytes.Equal(backup, before) {
		t.Fatal("backup does not match original")
	}
	after, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	afterRecords, err := readFateArchiveRecords(after)
	if err != nil {
		t.Fatal(err)
	}
	for hash, flags := range flagsBefore {
		record := afterRecords.byHash[hash]
		if record.flags.Uint32() != flags|1 {
			t.Fatalf("wrong unlock flags for %08X", hash)
		}
		record.flags.SetUint32(flags)
	}
	if err := after.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after.data, before) {
		t.Fatal("repair altered unrelated data (including progression, mission states or old 2555 vector)")
	}
	second, err := (&App{}).RepairFateStoryArchives(archiveRequest(t, path, ids...))
	if err != nil || second.Changed != 0 || second.BackupPath != "" {
		t.Fatalf("repeat repair is not a no-op: %+v %v", second, err)
	}
}

func TestFateStoryArchiveCreatesMissingFedielRecordInEmptySlot(t *testing.T) {
	path, save := archiveFixture(t)
	records, err := readFateArchiveRecords(save)
	if err != nil {
		t.Fatal(err)
	}
	record := records.byHash[gbfrHash32("ARC_OTHER_069")]
	record.hash.SetUint32(EmptyHash)
	record.flags.SetUint32(0)
	layout, err := inspectFateEpisodeLayout(save)
	if err != nil {
		t.Fatal(err)
	}
	layout.stateByHash[gbfrHash32("FATE_PL2900_10")].SetUint32(30)
	saveArchiveFixture(t, path, save)
	snapshot, err := (&App{}).FateEpisodeEditableInspect(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range snapshot.StoryArchives {
		if row.ArchiveID == "ARC_OTHER_069" {
			found = row.Missing && !row.Recorded
		}
	}
	if !found {
		t.Fatal("absent archive not offered for repair")
	}
	result, err := (&App{}).RepairFateStoryArchives(archiveRequest(t, path, "ARC_OTHER_069"))
	if err != nil || result.Changed != 1 {
		t.Fatalf("missing record not restored: %+v %v", result, err)
	}
	after, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	ar, err := readFateArchiveRecords(after)
	if err != nil {
		t.Fatal(err)
	}
	restored, ok := ar.byHash[gbfrHash32("ARC_OTHER_069")]
	if !ok || restored.flags.Uint32() != 1 {
		t.Fatal("restored record remains locked")
	}
}

func TestFateStoryArchiveRollbackAndRejection(t *testing.T) {
	path, save := archiveFixture(t)
	records, err := readFateArchiveRecords(save)
	if err != nil {
		t.Fatal(err)
	}
	records.byHash[gbfrHash32("ARC_OTHER_069")].flags.SetUint32(0)
	layout, err := inspectFateEpisodeLayout(save)
	if err != nil {
		t.Fatal(err)
	}
	layout.stateByHash[gbfrHash32("FATE_PL2900_10")].SetUint32(30)
	saveArchiveFixture(t, path, save)
	before := append([]byte(nil), save.data...)
	request := archiveRequest(t, path, "ARC_OTHER_069")
	_, err = repairFateStoryArchivesLocked(request, func(string) (*SaveData, error) { return nil, errors.New("forced failure") })
	if err == nil {
		t.Fatal("readback failure ignored")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("rollback failed")
	}
	stale := request
	stale.ExpectedRevision = fateEpisodeRevision([]byte("stale"))
	if _, err := (&App{}).RepairFateStoryArchives(stale); err == nil {
		t.Fatal("stale request accepted")
	}
	request.ArchiveIDs = []string{"ARC_OTHER_999"}
	if _, err := (&App{}).RepairFateStoryArchives(request); err == nil {
		t.Fatal("unknown archive accepted")
	}
	layout.stateByHash[gbfrHash32("FATE_PL2900_10")].SetUint32(0)
	saveArchiveFixture(t, path, save)
	if _, err := (&App{}).RepairFateStoryArchives(archiveRequest(t, path, "ARC_OTHER_069")); err == nil {
		t.Fatal("unfinished fate accepted")
	}
}
