package backend

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/klauspost/compress/zstd"
)

func logsArchiveFixtureBlob(t *testing.T) []byte {
	t.Helper()
	player := map[string]any{
		"actorIndex": uint32(0xF0000000), "displayName": "Pilot", "characterName": "Io", "characterType": "PL0400",
		"sigils":  []any{map[string]any{"firstTraitId": uint32(0x06939584), "firstTraitLevel": uint32(15), "secondTraitId": uint32(EmptyHash), "secondTraitLevel": uint32(0), "sigilId": uint32(0x42BB0C1C), "sigilLevel": uint32(15)}},
		"summons": []any{}, "abilities": []any{}, "weaponKey": "", "masterLevel": uint32(0), "skillboard": []any{}, "isOnline": true,
	}
	actor := func(index, actorType uint32) map[string]any {
		return map[string]any{"index": index, "actor_type": actorType, "parent_index": index, "parent_actor_type": actorType}
	}
	event := func(timestamp int64, damage int32, action any, base float32) []any {
		return []any{timestamp, map[string]any{"DamageEvent": map[string]any{
			"source": actor(0xF0000000, 0x4D0A60C3), "target": actor(99, 0xDEADBEEF), "damage": damage, "flags": uint64(0), "action_id": action,
			"damage_cap": int32(1000), "base_damage": base,
			"details":           map[string]any{"uncapped_damage": float32(2000), "damage_cap": int32(1000)},
			"target_current_hp": uint64(9000), "target_max_hp": uint64(10000),
		}}}
	}
	root := map[string]any{
		"playerData": []any{player, nil, nil, nil}, "eventLog": []any{},
		"rawEventLog": []any{event(1000, 900, map[string]any{"Normal": uint32(7)}, 900), event(2000, 1000, "SBA", 1200)},
	}
	raw, err := cbor.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer encoder.Close()
	return encoder.EncodeAll(raw, nil)
}

func createLogsArchiveFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "logs.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE logs (
		id INTEGER PRIMARY KEY, name TEXT NOT NULL, time INTEGER NOT NULL, duration INTEGER NOT NULL, data BLOB NOT NULL, version INTEGER NOT NULL,
		primary_target INTEGER, quest_id INTEGER, quest_completed BOOLEAN, p1_name TEXT, p1_type TEXT, p2_name TEXT, p2_type TEXT,
		p3_name TEXT, p3_type TEXT, p4_name TEXT, p4_type TEXT, total_damage INTEGER
	)`)
	if err != nil {
		t.Fatal(err)
	}
	for id := 1; id <= 3; id++ {
		_, err = db.Exec(`INSERT INTO logs(id,name,time,duration,data,version,primary_target,quest_id,quest_completed,p1_name,p1_type,total_damage) VALUES(?,?,?,?,?,1,?,?,?,?,?,?)`,
			id, "", int64(1000*id), int64(2000), logsArchiveFixtureBlob(t), 0xDEADBEEF, 123456, true, "Pilot", "PL0400", 1900)
		if err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestLogsBattleArchiveUsesReadOnlyKeysetPages(t *testing.T) {
	path := createLogsArchiveFixture(t)
	first, err := readLogsBattleArchivePage(path, LogsBattleArchiveRequest{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.Items[0].ID != 3 || first.NextCursorTime != 2000 || first.NextCursorID != 2 || !first.HasMore || !first.ReadOnly {
		t.Fatalf("unexpected first page: %+v", first)
	}
	second, err := readLogsBattleArchivePage(path, LogsBattleArchiveRequest{CursorTime: first.NextCursorTime, CursorID: first.NextCursorID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].ID != 1 || second.HasMore {
		t.Fatalf("unexpected second page: %+v", second)
	}
}

func TestLogsBattleArchiveCompositeCursorHandlesEqualTimes(t *testing.T) {
	path := createLogsArchiveFixture(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE logs SET time = 5000"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	first, err := readLogsBattleArchivePage(path, LogsBattleArchiveRequest{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.Items[0].ID != 3 || first.Items[1].ID != 2 || first.NextCursorTime != 5000 || first.NextCursorID != 2 {
		t.Fatalf("unexpected equal-time first page: %+v", first)
	}
	second, err := readLogsBattleArchivePage(path, LogsBattleArchiveRequest{CursorTime: first.NextCursorTime, CursorID: first.NextCursorID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].ID != 1 {
		t.Fatalf("equal-time cursor skipped or repeated a row: %+v", second)
	}
}

func TestLogsBattleArchiveSkipsUnsupportedProtocols(t *testing.T) {
	path := createLogsArchiveFixture(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO logs(id,name,time,duration,data,version) VALUES(99,'future',9999,1000,?,2)`, logsArchiveFixtureBlob(t))
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	page, err := readLogsBattleArchivePage(path, LogsBattleArchiveRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.SkippedUnsupported != 1 || len(page.Items) != 3 {
		t.Fatalf("unsupported protocol was exposed or not reported: %+v", page)
	}
	app := &App{logsArchivePath: path}
	t.Cleanup(app.CloseLogsBattleArchive)
	if _, err := app.LogsBattleArchiveDetail(99); err == nil {
		t.Fatal("unsupported protocol detail must fail closed")
	}
}

func TestLogsBattleArchiveDetailAggregatesStoredDamageAndLoadout(t *testing.T) {
	app := &App{logsArchivePath: createLogsArchiveFixture(t)}
	t.Cleanup(app.CloseLogsBattleArchive)
	detail, err := app.LogsBattleArchiveDetail(3)
	if err != nil {
		t.Fatal(err)
	}
	if detail.EventCount != 2 || detail.RecognizedEvents != 2 || len(detail.Players) != 1 {
		t.Fatalf("incomplete detail: %+v", detail)
	}
	player := detail.Players[0]
	if player.Damage != 1900 || len(player.Skills) != 2 || len(player.Timeline) != 2 {
		t.Fatalf("wrong player aggregation: %+v", player)
	}
	if player.Skills[0].Key != "SBA:0" || player.Skills[0].CappedHits != 1 || player.Skills[0].OvercapPercent != 120 {
		t.Fatalf("wrong cap aggregation: %+v", player.Skills)
	}
	if len(detail.Targets) != 1 || detail.Targets[0].Damage != 1900 || detail.Targets[0].MaxHP == nil || *detail.Targets[0].MaxHP != 10000 {
		t.Fatalf("wrong target aggregation: %+v", detail.Targets)
	}
}

func TestLogsBattleArchiveDetailRejectsOversizedBlobBeforeDecode(t *testing.T) {
	path := createLogsArchiveFixture(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE logs SET data = zeroblob(?) WHERE id = 3", logsLoadoutMaximumBlob+1); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	app := &App{logsArchivePath: path}
	t.Cleanup(app.CloseLogsBattleArchive)
	_, err = app.LogsBattleArchiveDetail(3)
	if err == nil || !strings.Contains(err.Error(), "压缩数据大小") {
		t.Fatalf("oversized battle blob was not rejected at the SQL boundary: %v", err)
	}
}

func TestLogsBattleArchiveCapDenominatorRequiresBaseDamage(t *testing.T) {
	encounter, err := decodeLogsLoadoutEncounter(logsArchiveFixtureBlob(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(encounter.RawEventLog) < 2 || encounter.RawEventLog[1].Damage == nil {
		t.Fatal("fixture damage event missing")
	}
	encounter.RawEventLog[1].Damage.BaseDamage = nil
	encounter.RawEventLog[1].Damage.Details = nil
	detail := buildLogsBattleDetail(LogsBattleSummary{Duration: 2000}, encounter)
	if len(detail.Players) != 1 {
		t.Fatalf("player missing: %+v", detail)
	}
	for _, skill := range detail.Players[0].Skills {
		if skill.Key == "SBA:0" && (skill.CappableHits != 0 || skill.CappedHits != 0 || skill.OvercapPercent != 0) {
			t.Fatalf("missing base damage entered cap denominator: %+v", skill)
		}
	}
}

func TestLogsBattleArchiveUsesDjeetamodDetailsAsCapFallback(t *testing.T) {
	encounter, err := decodeLogsLoadoutEncounter(logsArchiveFixtureBlob(t))
	if err != nil {
		t.Fatal(err)
	}
	event := encounter.RawEventLog[1].Damage
	if event == nil || event.Details == nil {
		t.Fatal("djeetamod damage details were not decoded")
	}
	event.BaseDamage = nil
	event.DamageCap = nil
	detail := buildLogsBattleDetail(LogsBattleSummary{Duration: 2000}, encounter)
	for _, skill := range detail.Players[0].Skills {
		if skill.Key == "SBA:0" {
			if skill.CappableHits != 1 || skill.CappedHits != 1 || skill.OvercapPercent != 200 {
				t.Fatalf("djeetamod cap fallback mismatch: %+v", skill)
			}
			return
		}
	}
	t.Fatal("SBA skill missing")
}

func TestLogsBattleArchiveAcceptsLegacyBaseSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE logs (id INTEGER PRIMARY KEY, name TEXT NOT NULL, time INTEGER NOT NULL, duration INTEGER NOT NULL, data BLOB NOT NULL, version INTEGER NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO logs VALUES(1,'',1000,2000,?,1)`, logsArchiveFixtureBlob(t))
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	page, err := readLogsBattleArchivePage(path, LogsBattleArchiveRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].TotalDamage != nil || page.Items[0].QuestName == "" {
		t.Fatalf("legacy metadata was fabricated or omitted: %+v", page.Items)
	}
	filtered, err := readLogsBattleArchivePage(path, LogsBattleArchiveRequest{Search: "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Items) != 0 {
		t.Fatalf("a schema without searchable columns must not ignore the search: %+v", filtered.Items)
	}
}

func TestLogsBattleArchiveSearchTreatsWildcardsAsLiterals(t *testing.T) {
	path := createLogsArchiveFixture(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE logs SET p1_name = CASE id WHEN 1 THEN 'Pilot_One' WHEN 2 THEN 'Pilot%Two' ELSE 'Pilot\\Three' END`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	for search, expectedID := range map[string]int64{"_": 1, "%": 2, `\`: 3} {
		page, err := readLogsBattleArchivePage(path, LogsBattleArchiveRequest{Search: search, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || page.Items[0].ID != expectedID {
			t.Fatalf("search %q was not literal: %+v", search, page.Items)
		}
	}
}

func createLargeLogsArchiveFixture(b *testing.B, count int) string {
	b.Helper()
	path := filepath.Join(b.TempDir(), "logs.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE logs (
		id INTEGER PRIMARY KEY, name TEXT NOT NULL, time INTEGER NOT NULL, duration INTEGER NOT NULL, data BLOB NOT NULL, version INTEGER NOT NULL,
		primary_target INTEGER, quest_id INTEGER, quest_completed BOOLEAN, p1_name TEXT, p1_type TEXT, p2_name TEXT, p2_type TEXT,
		p3_name TEXT, p3_type TEXT, p4_name TEXT, p4_type TEXT, total_damage INTEGER
	)`); err != nil {
		b.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		b.Fatal(err)
	}
	statement, err := tx.Prepare(`INSERT INTO logs(id,name,time,duration,data,version,p1_name,p1_type,total_damage) VALUES(?,?,?,?,?,1,?,?,?)`)
	if err != nil {
		b.Fatal(err)
	}
	for id := 1; id <= count; id++ {
		if _, err = statement.Exec(id, "", int64(id)*1000, 120000, []byte{0}, "Pilot", "PL0400", id*1000); err != nil {
			statement.Close()
			tx.Rollback()
			b.Fatal(err)
		}
	}
	if err = statement.Close(); err != nil {
		b.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		b.Fatal(err)
	}
	return path
}

func BenchmarkLogsBattleArchivePage10000(b *testing.B) {
	path := createLargeLogsArchiveFixture(b, 10000)
	requests := map[string]LogsBattleArchiveRequest{
		"first-page": {Limit: 40},
		"deep-page":  {CursorTime: 1000000, CursorID: 1000, Limit: 40},
	}
	for name, request := range requests {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for index := 0; index < b.N; index++ {
				page, err := readLogsBattleArchivePage(path, request)
				if err != nil {
					b.Fatal(err)
				}
				if len(page.Items) != 40 {
					b.Fatalf("unexpected page length: %d", len(page.Items))
				}
			}
		})
	}
}
