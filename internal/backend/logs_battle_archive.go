package backend

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const logsBattleArchivePageMax = 100

type LogsBattleArchiveRequest struct {
	CursorTime int64  `json:"cursorTime"`
	CursorID   int64  `json:"cursorId"`
	Limit      int    `json:"limit"`
	Search     string `json:"search"`
}

type LogsBattleSummary struct {
	ID                   int64    `json:"id"`
	Time                 int64    `json:"time"`
	Duration             int64    `json:"duration"`
	TotalDamage          *int64   `json:"totalDamage,omitempty"`
	DPS                  *float64 `json:"dps,omitempty"`
	QuestID              *uint32  `json:"questId,omitempty"`
	QuestName            string   `json:"questName"`
	PrimaryTarget        *uint32  `json:"primaryTarget,omitempty"`
	Completed            *bool    `json:"completed,omitempty"`
	Protocol             int      `json:"protocol"`
	PlayerNames          []string `json:"playerNames"`
	CharacterTypes       []string `json:"characterTypes"`
	LegalityFindingCount int      `json:"legalityFindingCount,omitempty"`
}

type LogsBattleArchivePage struct {
	Items              []LogsBattleSummary `json:"items"`
	NextCursorTime     int64               `json:"nextCursorTime,omitempty"`
	NextCursorID       int64               `json:"nextCursorId,omitempty"`
	HasMore            bool                `json:"hasMore"`
	SkippedUnsupported int                 `json:"skippedUnsupported"`
	DataSource         string              `json:"dataSource"`
	ReadOnly           bool                `json:"readOnly"`
}

type LogsBattleSkill struct {
	Key            string  `json:"key"`
	Name           string  `json:"name"`
	Hits           int     `json:"hits"`
	Damage         int64   `json:"damage"`
	Percentage     float64 `json:"percentage"`
	MinDamage      int32   `json:"minDamage"`
	MaxDamage      int32   `json:"maxDamage"`
	CappedHits     int     `json:"cappedHits"`
	CappableHits   int     `json:"cappableHits"`
	OvercapPercent float64 `json:"overcapPercent,omitempty"`
}

type LogsBattlePlayer struct {
	Slot             int                        `json:"slot"`
	PlayerName       string                     `json:"playerName"`
	CharacterName    string                     `json:"characterName"`
	CharacterHash    string                     `json:"characterHash,omitempty"`
	CharacterCode    string                     `json:"characterCode,omitempty"`
	Damage           int64                      `json:"damage"`
	DPS              float64                    `json:"dps"`
	Percentage       float64                    `json:"percentage"`
	Skills           []LogsBattleSkill          `json:"skills"`
	Timeline         []int64                    `json:"timeline"`
	Loadout          *RuntimePatchPartyLoadout  `json:"loadout,omitempty"`
	Warnings         []string                   `json:"warnings,omitempty"`
	LegalityFindings []LogsBuildLegalityFinding `json:"legalityFindings,omitempty"`
}

type LogsBattleTarget struct {
	Hash      string  `json:"hash"`
	Damage    int64   `json:"damage"`
	CurrentHP *uint64 `json:"currentHp,omitempty"`
	MaxHP     *uint64 `json:"maxHp,omitempty"`
}

type LogsBattleDetail struct {
	Summary          LogsBattleSummary  `json:"summary"`
	Players          []LogsBattlePlayer `json:"players"`
	Targets          []LogsBattleTarget `json:"targets"`
	Aggregation      string             `json:"aggregation"`
	MissingFields    []string           `json:"missingFields,omitempty"`
	DecodeWarnings   []string           `json:"decodeWarnings,omitempty"`
	EventCount       int                `json:"eventCount"`
	RecognizedEvents int                `json:"recognizedEvents"`
}

func logsBattleColumns(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`PRAGMA table_info(logs)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns[strings.ToLower(name)] = true
	}
	return columns, rows.Err()
}

func logsColumn(columns map[string]bool, name, fallback string) string {
	if columns[strings.ToLower(name)] {
		return name
	}
	return fallback + " AS " + name
}

func openLogsBattleArchive(path string) (*sql.DB, map[string]bool, error) {
	dsn, err := logsReadOnlyDatabaseDSN(path)
	if err != nil {
		return nil, nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, nil, err
	}
	if err := validateLogsDatabaseSchema(db); err != nil {
		db.Close()
		return nil, nil, err
	}
	columns, err := logsBattleColumns(db)
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return db, columns, nil
}

func localizedLogsQuestName(id *uint32) string {
	if id == nil {
		return runtimePatchMonitorText("未记录任务", "Quest Not Captured")
	}
	if useChinese() {
		return questIDToNameCN(*id)
	}
	return questIDToName(*id)
}

func scanLogsBattleSummary(rows *sql.Rows) (LogsBattleSummary, error) {
	var result LogsBattleSummary
	var totalDamage, questID, primaryTarget, completed sql.NullInt64
	var p1Name, p1Type, p2Name, p2Type, p3Name, p3Type, p4Name, p4Type sql.NullString
	if err := rows.Scan(&result.ID, &result.Time, &result.Duration, &result.Protocol, &totalDamage, &questID, &primaryTarget, &completed,
		&p1Name, &p1Type, &p2Name, &p2Type, &p3Name, &p3Type, &p4Name, &p4Type); err != nil {
		return result, err
	}
	if totalDamage.Valid {
		value := totalDamage.Int64
		result.TotalDamage = &value
		if result.Duration > 0 {
			dps := float64(value) / (float64(result.Duration) / 1000)
			result.DPS = &dps
		}
	}
	if questID.Valid && questID.Int64 >= 0 && questID.Int64 <= int64(^uint32(0)) {
		value := uint32(questID.Int64)
		result.QuestID = &value
	}
	if primaryTarget.Valid && primaryTarget.Int64 >= 0 && primaryTarget.Int64 <= int64(^uint32(0)) {
		value := uint32(primaryTarget.Int64)
		result.PrimaryTarget = &value
	}
	if completed.Valid {
		value := completed.Int64 != 0
		result.Completed = &value
	}
	for _, value := range []sql.NullString{p1Name, p2Name, p3Name, p4Name} {
		if value.Valid && strings.TrimSpace(value.String) != "" {
			result.PlayerNames = append(result.PlayerNames, strings.TrimSpace(value.String))
		}
	}
	for _, value := range []sql.NullString{p1Type, p2Type, p3Type, p4Type} {
		if value.Valid && strings.TrimSpace(value.String) != "" {
			result.CharacterTypes = append(result.CharacterTypes, strings.TrimSpace(value.String))
		}
	}
	result.QuestName = localizedLogsQuestName(result.QuestID)
	return result, nil
}

func logsBattleSummarySelect(columns map[string]bool) string {
	fields := []string{"id", "time", "duration", "version",
		logsColumn(columns, "total_damage", "NULL"), logsColumn(columns, "quest_id", "NULL"),
		logsColumn(columns, "primary_target", "NULL"), logsColumn(columns, "quest_completed", "NULL")}
	for index := 1; index <= 4; index++ {
		fields = append(fields, logsColumn(columns, fmt.Sprintf("p%d_name", index), "NULL"), logsColumn(columns, fmt.Sprintf("p%d_type", index), "NULL"))
	}
	return strings.Join(fields, ", ")
}

func escapeLogsLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	return strings.ReplaceAll(value, "_", "\\_")
}

func readLogsBattleArchivePage(path string, request LogsBattleArchiveRequest) (*LogsBattleArchivePage, error) {
	db, columns, err := openLogsBattleArchive(path)
	if err != nil {
		return nil, fmt.Errorf("打开 Logs 战斗档案失败: %w", err)
	}
	defer db.Close()
	return readLogsBattleArchivePageDB(db, columns, request)
}

func readLogsBattleArchivePageDB(db *sql.DB, columns map[string]bool, request LogsBattleArchiveRequest) (*LogsBattleArchivePage, error) {
	limit := request.Limit
	if limit < 1 || limit > logsBattleArchivePageMax {
		limit = 40
	}
	where, args := []string{"version = 1"}, []any{}
	if request.CursorTime > 0 && request.CursorID > 0 {
		where = append(where, "(time < ? OR (time = ? AND id < ?))")
		args = append(args, request.CursorTime, request.CursorTime, request.CursorID)
	}
	search := strings.TrimSpace(request.Search)
	if search != "" {
		like := "%" + escapeLogsLike(search) + "%"
		searchParts := []string{}
		for index := 1; index <= 4; index++ {
			for _, suffix := range []string{"name", "type"} {
				column := fmt.Sprintf("p%d_%s", index, suffix)
				if columns[column] {
					searchParts = append(searchParts, column+" LIKE ? ESCAPE '\\'")
					args = append(args, like)
				}
			}
		}
		if numeric, parseErr := strconv.ParseUint(search, 0, 32); parseErr == nil && columns["quest_id"] {
			searchParts = append(searchParts, "quest_id = ?")
			args = append(args, numeric)
		}
		if len(searchParts) > 0 {
			where = append(where, "("+strings.Join(searchParts, " OR ")+")")
		} else {
			where = append(where, "0")
		}
	}
	args = append(args, limit+1)
	query := fmt.Sprintf("SELECT %s FROM logs WHERE %s ORDER BY time DESC, id DESC LIMIT ?", logsBattleSummarySelect(columns), strings.Join(where, " AND "))
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]LogsBattleSummary, 0, limit+1)
	for rows.Next() {
		item, err := scanLogsBattleSummary(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	nextTime, nextID := int64(0), int64(0)
	if hasMore && len(items) > 0 {
		nextTime = items[len(items)-1].Time
		nextID = items[len(items)-1].ID
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	if counts, countErr := readLogsLegalityCounts(db, ids); countErr == nil {
		for index := range items {
			items[index].LegalityFindingCount = counts[items[index].ID]
		}
	}
	unsupported := 0
	if err := db.QueryRow("SELECT COUNT(*) FROM logs WHERE version <> 1").Scan(&unsupported); err != nil {
		return nil, err
	}
	return &LogsBattleArchivePage{Items: items, NextCursorTime: nextTime, NextCursorID: nextID, HasMore: hasMore, SkippedUnsupported: unsupported, DataSource: "Relink Logs SQLite v1", ReadOnly: true}, nil
}

func (a *App) replaceLogsArchiveSession(path string) error {
	db, columns, err := openLogsBattleArchive(path)
	if err != nil {
		return err
	}
	a.logsArchiveMu.Lock()
	oldDB := a.logsArchiveDB
	a.logsArchivePath = path
	a.logsArchiveDB = db
	a.logsArchiveColumns = columns
	a.logsArchiveMu.Unlock()
	if oldDB != nil {
		_ = oldDB.Close()
	}
	return nil
}

func (a *App) ensureLogsArchiveSessionLocked() error {
	if a.logsArchiveDB != nil {
		return nil
	}
	if strings.TrimSpace(a.logsArchivePath) == "" {
		return fmt.Errorf("尚未选择 Logs 战斗数据库")
	}
	db, columns, err := openLogsBattleArchive(a.logsArchivePath)
	if err != nil {
		return err
	}
	a.logsArchiveDB = db
	a.logsArchiveColumns = columns
	return nil
}

func (a *App) SelectLogsBattleArchive(request LogsBattleArchiveRequest) (*LogsBattleArchivePage, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择 Logs 战斗数据库", Filters: []runtime.FileFilter{{DisplayName: "Relink Logs (logs.db;*.db;*.sqlite)", Pattern: "logs.db;*.db;*.sqlite;*.sqlite3"}, {DisplayName: "所有文件", Pattern: "*.*"}}})
	if err != nil || path == "" {
		return nil, err
	}
	if err := a.replaceLogsArchiveSession(path); err != nil {
		return nil, err
	}
	return a.LogsBattleArchivePage(request)
}

// LogsBattleArchiveFromCurrent reuses the shared read-only Logs session. It
// is intentionally separate from SelectLogsBattleArchive so a user can still
// choose a different database explicitly.
func (a *App) LogsBattleArchiveFromCurrent(request LogsBattleArchiveRequest) (*LogsBattleArchivePage, error) {
	return a.LogsBattleArchivePage(request)
}

func (a *App) LogsBattleArchivePage(request LogsBattleArchiveRequest) (*LogsBattleArchivePage, error) {
	a.logsArchiveMu.Lock()
	defer a.logsArchiveMu.Unlock()
	if err := a.ensureLogsArchiveSessionLocked(); err != nil {
		return nil, err
	}
	return readLogsBattleArchivePageDB(a.logsArchiveDB, a.logsArchiveColumns, request)
}

func logsActionLabel(action logsActionType) string {
	switch action.Kind {
	case "LinkAttack":
		return runtimePatchMonitorText("连携攻击", "Link Attack")
	case "SBA":
		return runtimePatchMonitorText("奥义", "Skybound Art")
	case "SupplementaryDamage":
		return runtimePatchMonitorText("追击伤害", "Supplementary Damage")
	case "DamageOverTime":
		return runtimePatchMonitorText("持续伤害", "Damage Over Time")
	case "Normal":
		return fmt.Sprintf(runtimePatchMonitorText("技能 #%d", "Skill #%d"), action.ID)
	case "PerfectGuard":
		return runtimePatchMonitorText("精准格挡", "Perfect Guard")
	case "PerfectGuardQuickening":
		return runtimePatchMonitorText("精准格挡·急速", "Perfect Guard: Quickening")
	case "StunEffect":
		return runtimePatchMonitorText("昏厥效果", "Stun Effect")
	default:
		if action.Kind == "" {
			return runtimePatchMonitorText("未记录技能", "Skill Not Captured")
		}
		return action.Kind
	}
}

func logsDamageRows(encounter logsLoadoutEncounter) []logsTimedDamage {
	if len(encounter.RawEventLog) > 0 {
		result := make([]logsTimedDamage, 0, len(encounter.RawEventLog))
		for _, row := range encounter.RawEventLog {
			if row.Damage != nil {
				result = append(result, logsTimedDamage{Time: row.Time, Event: *row.Damage})
			}
		}
		return result
	}
	return encounter.EventLog
}

type logsMutableSkill struct {
	LogsBattleSkill
	overcapBase float64
	overcapCap  float64
}

func logsDamageCapEvidence(event logsDamageEvent) (float64, float64, bool) {
	if event.DamageCap != nil && event.BaseDamage != nil && *event.DamageCap > 0 {
		base, cap := float64(*event.BaseDamage), float64(*event.DamageCap)
		if base >= 0 && !math.IsNaN(base) && !math.IsInf(base, 0) {
			return base, cap, true
		}
	}
	if event.Details != nil && event.Details.DamageCap > 0 {
		base, cap := float64(event.Details.UncappedDamage), float64(event.Details.DamageCap)
		if base >= 0 && !math.IsNaN(base) && !math.IsInf(base, 0) {
			return base, cap, true
		}
	}
	return 0, 0, false
}

func buildLogsBattleDetail(summary LogsBattleSummary, encounter logsLoadoutEncounter) LogsBattleDetail {
	detail := LogsBattleDetail{Summary: summary, Aggregation: runtimePatchMonitorText("Relink Logs 已保存伤害事件的只读重算", "Read-only aggregation of stored Relink Logs damage events")}
	actorToSlot := map[uint32]int{}
	players := make([]LogsBattlePlayer, 0, 4)
	for slot, player := range encounter.PlayerData {
		if player == nil {
			continue
		}
		characterType := player.CharacterType.String()
		owner := normalizeLogsOwnerCode(characterType)
		characterName := strings.TrimSpace(player.CharacterName)
		if names, ok := runtimePatchPartyCharacterNames[owner]; ok {
			characterName = names[0]
			if !useChinese() {
				characterName = names[1]
			}
		}
		entry := LogsBattlePlayer{Slot: slot + 1, PlayerName: strings.TrimSpace(player.DisplayName), CharacterName: characterName, CharacterCode: owner}
		if hash, ok := runtimeOwnerCharacterHash[owner]; ok {
			entry.CharacterHash = hashText(hash)
		}
		if candidate, err := logsPlayerLoadoutShare(summary.Time, player); err == nil {
			entry.Loadout = candidate.Preview
			entry.Warnings = append(entry.Warnings, candidate.Warnings...)
		} else {
			entry.Warnings = append(entry.Warnings, err.Error())
		}
		actorToSlot[player.ActorIndex] = len(players)
		actorToSlot[0xF0000000|uint32(slot)] = len(players)
		players = append(players, entry)
	}
	skills := make([]map[string]*logsMutableSkill, len(players))
	for index := range skills {
		skills[index] = map[string]*logsMutableSkill{}
	}
	damageRows := logsDamageRows(encounter)
	detail.EventCount = len(damageRows)
	targets := map[uint32]*LogsBattleTarget{}
	firstTime, lastTime := int64(0), int64(0)
	for _, row := range damageRows {
		if row.Event.Damage <= 0 {
			continue
		}
		slot, ok := actorToSlot[row.Event.Source.ParentIndex]
		if !ok {
			slot, ok = actorToSlot[row.Event.Source.Index]
		}
		if !ok {
			continue
		}
		detail.RecognizedEvents++
		if firstTime == 0 || row.Time < firstTime {
			firstTime = row.Time
		}
		if row.Time > lastTime {
			lastTime = row.Time
		}
		damage := int64(row.Event.Damage)
		players[slot].Damage += damage
		key := fmt.Sprintf("%s:%d", row.Event.ActionID.Kind, row.Event.ActionID.ID)
		skill := skills[slot][key]
		if skill == nil {
			skill = &logsMutableSkill{LogsBattleSkill: LogsBattleSkill{Key: key, Name: logsActionLabel(row.Event.ActionID), MinDamage: row.Event.Damage, MaxDamage: row.Event.Damage}}
			skills[slot][key] = skill
		}
		skill.Hits++
		skill.Damage += damage
		if row.Event.Damage < skill.MinDamage {
			skill.MinDamage = row.Event.Damage
		}
		if row.Event.Damage > skill.MaxDamage {
			skill.MaxDamage = row.Event.Damage
		}
		if base, cap, ok := logsDamageCapEvidence(row.Event); ok {
			skill.CappableHits++
			skill.overcapBase += base
			skill.overcapCap += cap
			if base > cap {
				skill.CappedHits++
			}
		}
		targetHash := row.Event.Target.ParentActorType
		target := targets[targetHash]
		if target == nil {
			target = &LogsBattleTarget{Hash: hashText(targetHash)}
			targets[targetHash] = target
		}
		target.Damage += damage
		if row.Event.TargetCurrentHP != nil && row.Event.TargetMaxHP != nil && (target.MaxHP == nil || *row.Event.TargetMaxHP >= *target.MaxHP || (target.CurrentHP != nil && *target.CurrentHP == 0)) {
			current, maximum := *row.Event.TargetCurrentHP, *row.Event.TargetMaxHP
			target.CurrentHP, target.MaxHP = &current, &maximum
		}
	}
	durationSeconds := float64(summary.Duration) / 1000
	if durationSeconds <= 0 && lastTime > firstTime {
		durationSeconds = float64(lastTime-firstTime) / 1000
	}
	teamDamage := int64(0)
	for _, player := range players {
		teamDamage += player.Damage
	}
	if firstTime > 0 && lastTime >= firstTime && len(players) > 0 {
		bucketCount := int((lastTime-firstTime)/1000) + 1
		if bucketCount > 120 {
			bucketCount = 120
		}
		for index := range players {
			players[index].Timeline = make([]int64, bucketCount)
		}
		window := max(int64(1), lastTime-firstTime+1)
		for _, row := range damageRows {
			if row.Event.Damage <= 0 {
				continue
			}
			slot, ok := actorToSlot[row.Event.Source.ParentIndex]
			if !ok {
				slot, ok = actorToSlot[row.Event.Source.Index]
			}
			if !ok {
				continue
			}
			bucket := int((row.Time - firstTime) * int64(bucketCount) / window)
			if bucket < 0 {
				bucket = 0
			}
			if bucket >= bucketCount {
				bucket = bucketCount - 1
			}
			players[slot].Timeline[bucket] += int64(row.Event.Damage)
		}
	}
	for index := range players {
		if durationSeconds > 0 {
			players[index].DPS = float64(players[index].Damage) / durationSeconds
		}
		if teamDamage > 0 {
			players[index].Percentage = float64(players[index].Damage) / float64(teamDamage) * 100
		}
		for _, skill := range skills[index] {
			if players[index].Damage > 0 {
				skill.Percentage = float64(skill.Damage) / float64(players[index].Damage) * 100
			}
			if skill.overcapCap > 0 {
				skill.OvercapPercent = skill.overcapBase / skill.overcapCap * 100
			}
			players[index].Skills = append(players[index].Skills, skill.LogsBattleSkill)
		}
		sort.Slice(players[index].Skills, func(i, j int) bool {
			if players[index].Skills[i].Damage != players[index].Skills[j].Damage {
				return players[index].Skills[i].Damage > players[index].Skills[j].Damage
			}
			return players[index].Skills[i].Key < players[index].Skills[j].Key
		})
	}
	detail.Players = players
	for _, target := range targets {
		detail.Targets = append(detail.Targets, *target)
	}
	sort.Slice(detail.Targets, func(i, j int) bool { return detail.Targets[i].Damage > detail.Targets[j].Damage })
	if len(damageRows) == 0 {
		detail.MissingFields = append(detail.MissingFields, runtimePatchMonitorText("伤害事件", "Damage Events"))
	}
	if summary.TotalDamage == nil {
		detail.MissingFields = append(detail.MissingFields, runtimePatchMonitorText("数据库总伤害列", "Database Total Damage Column"))
	} else if teamDamage > 0 && *summary.TotalDamage != teamDamage {
		detail.DecodeWarnings = append(detail.DecodeWarnings, fmt.Sprintf(runtimePatchMonitorText("事件重算总伤害 %d 与数据库摘要 %d 不同；详情保留事件口径", "Event total %d differs from database summary %d; detail keeps the event basis"), teamDamage, *summary.TotalDamage))
	}
	return detail
}

func (a *App) LogsBattleArchiveDetail(id int64) (*LogsBattleDetail, error) {
	a.logsArchiveMu.Lock()
	defer a.logsArchiveMu.Unlock()
	if id <= 0 {
		return nil, fmt.Errorf("战斗档案会话或记录无效")
	}
	if err := a.ensureLogsArchiveSessionLocked(); err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT %s, length(data), substr(data, 1, ?) FROM logs WHERE id = ? AND version = 1", logsBattleSummarySelect(a.logsArchiveColumns))
	rows, err := a.logsArchiveDB.Query(query, logsLoadoutMaximumBlob, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("战斗记录不存在")
	}
	var summary LogsBattleSummary
	var totalDamage, questID, primaryTarget, completed sql.NullInt64
	var p1Name, p1Type, p2Name, p2Type, p3Name, p3Type, p4Name, p4Type sql.NullString
	var blobLength int64
	var blob []byte
	if err := rows.Scan(&summary.ID, &summary.Time, &summary.Duration, &summary.Protocol, &totalDamage, &questID, &primaryTarget, &completed,
		&p1Name, &p1Type, &p2Name, &p2Type, &p3Name, &p3Type, &p4Name, &p4Type, &blobLength, &blob); err != nil {
		return nil, err
	}
	if blobLength <= 0 || blobLength > logsLoadoutMaximumBlob {
		return nil, fmt.Errorf("战斗记录压缩数据大小 %d 字节，允许范围为 1 到 %d 字节", blobLength, logsLoadoutMaximumBlob)
	}
	// Reuse the summary scanner's normalization through a compact in-memory row
	// would obscure errors; normalize the nullable metadata directly here.
	if totalDamage.Valid {
		value := totalDamage.Int64
		summary.TotalDamage = &value
		if summary.Duration > 0 {
			dps := float64(value) / (float64(summary.Duration) / 1000)
			summary.DPS = &dps
		}
	}
	if questID.Valid && questID.Int64 >= 0 && questID.Int64 <= int64(^uint32(0)) {
		value := uint32(questID.Int64)
		summary.QuestID = &value
	}
	if primaryTarget.Valid && primaryTarget.Int64 >= 0 && primaryTarget.Int64 <= int64(^uint32(0)) {
		value := uint32(primaryTarget.Int64)
		summary.PrimaryTarget = &value
	}
	if completed.Valid {
		value := completed.Int64 != 0
		summary.Completed = &value
	}
	for _, value := range []sql.NullString{p1Name, p2Name, p3Name, p4Name} {
		if value.Valid && strings.TrimSpace(value.String) != "" {
			summary.PlayerNames = append(summary.PlayerNames, strings.TrimSpace(value.String))
		}
	}
	for _, value := range []sql.NullString{p1Type, p2Type, p3Type, p4Type} {
		if value.Valid && strings.TrimSpace(value.String) != "" {
			summary.CharacterTypes = append(summary.CharacterTypes, strings.TrimSpace(value.String))
		}
	}
	summary.QuestName = localizedLogsQuestName(summary.QuestID)
	encounter, err := decodeLogsLoadoutEncounter(blob)
	if err != nil {
		return nil, fmt.Errorf("解码战斗记录失败: %w", err)
	}
	detail := buildLogsBattleDetail(summary, encounter)
	if findings, findingErr := readLogsLegalityFindings(a.logsArchiveDB, id); findingErr == nil {
		for index := range detail.Players {
			slot := detail.Players[index].Slot - 1
			detail.Players[index].LegalityFindings = append(detail.Players[index].LegalityFindings, findings[slot]...)
		}
	} else {
		detail.DecodeWarnings = append(detail.DecodeWarnings, fmt.Sprintf(runtimePatchMonitorText("Logs 合法性结果无法读取：%v", "Could not read stored Logs legality findings: %v"), findingErr))
	}
	return &detail, nil
}

func (a *App) CloseLogsBattleArchive() {
	a.logsArchiveMu.Lock()
	db := a.logsArchiveDB
	a.logsArchivePath = ""
	a.logsArchiveDB = nil
	a.logsArchiveColumns = nil
	a.logsArchiveMu.Unlock()
	if db != nil {
		_ = db.Close()
	}
}
