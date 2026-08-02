package backend

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	logsLegalityMaximumRows    = 256
	logsLegalityMaximumPayload = 64 * 1024
)

// LogsBuildLegalityFinding is a verdict already produced by GBFR Logs and
// stored beside the encounter.  It is intentionally attributed to that source:
// this application renders the evidence but does not turn a table mismatch or
// an improbable roll into an accusation about a player.
type LogsBuildLegalityFinding struct {
	Rule       string   `json:"rule"`
	Label      string   `json:"label"`
	Detail     string   `json:"detail"`
	Odds       *float64 `json:"odds,omitempty"`
	HardBreach bool     `json:"hardBreach"`
	Source     string   `json:"source"`
}

type logsLegalityValue struct {
	Kind  string          `json:"kind"`
	Value json.RawMessage `json:"value"`
}

type logsLegalityPayload struct {
	Rule     string            `json:"rule"`
	Observed logsLegalityValue `json:"observed"`
	Allowed  logsLegalityValue `json:"allowed"`
	Odds     *float64          `json:"odds"`
}

func logsTableExists(db *sql.DB, name string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count)
	return count > 0, err
}

func logsLegalityRuleLabel(rule string) string {
	labelsCN := map[string]string{
		"wrightstoneTraitLevel": "祝福石槽位等级",
		"sigilTraitLevel":       "因子词条等级",
		"sigilLockedPair":       "角色专属因子固定词条",
		"sigilQuestLockedTrait": "任务限定词条归属",
		"sigilSingleTraitOnly":  "单词条因子出现副词条",
		"overmasteryValue":      "上限突破数值",
		"overmasteryAllMaxed":   "四项上限突破同时满值",
		"summonTrait":           "召唤石主词条来源",
		"summonBonusSource":     "召唤石附加效果来源",
		"summonBonusMagnitude":  "召唤石附加效果数值",
		"summonPerfectCount":    "多颗召唤石同时完美",
		"masterTraitCount":      "专精节点数量",
	}
	labelsEN := map[string]string{
		"wrightstoneTraitLevel": "Wrightstone slot level",
		"sigilTraitLevel":       "Sigil trait level",
		"sigilLockedPair":       "Character-sigil locked pair",
		"sigilQuestLockedTrait": "Quest-locked trait placement",
		"sigilSingleTraitOnly":  "Second trait on a single-trait sigil",
		"overmasteryValue":      "Overmastery value",
		"overmasteryAllMaxed":   "All four overmasteries at maximum",
		"summonTrait":           "Summon main-trait source",
		"summonBonusSource":     "Summon bonus source",
		"summonBonusMagnitude":  "Summon bonus magnitude",
		"summonPerfectCount":    "Multiple perfect summons",
		"masterTraitCount":      "Master-trait count",
	}
	if useChinese() {
		if label := labelsCN[rule]; label != "" {
			return label
		}
		return "未识别的 Logs 检测规则"
	}
	if label := labelsEN[rule]; label != "" {
		return label
	}
	return "Unknown Logs legality rule"
}

func logsLegalityValueText(value logsLegalityValue) string {
	if len(value.Value) == 0 || string(value.Value) == "null" {
		return runtimePatchMonitorText("未提供", "Not provided")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, value.Value); err != nil {
		return runtimePatchMonitorText("无法读取", "Unreadable")
	}
	return compact.String()
}

func parseLogsLegalityFinding(rule string, raw string) (LogsBuildLegalityFinding, error) {
	if len(raw) == 0 || len(raw) > logsLegalityMaximumPayload {
		return LogsBuildLegalityFinding{}, fmt.Errorf("Logs 合法性记录大小超出边界")
	}
	var payload logsLegalityPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return LogsBuildLegalityFinding{}, err
	}
	rule = strings.TrimSpace(rule)
	if payload.Rule != "" && payload.Rule != rule {
		return LogsBuildLegalityFinding{}, fmt.Errorf("Logs 合法性规则与记录内容不一致")
	}
	if rule == "" {
		return LogsBuildLegalityFinding{}, fmt.Errorf("Logs 合法性规则为空")
	}
	observed, allowed := logsLegalityValueText(payload.Observed), logsLegalityValueText(payload.Allowed)
	detail := fmt.Sprintf(runtimePatchMonitorText("记录值 %s；自然表范围 %s。", "Observed %s; table-allowed %s."), observed, allowed)
	if payload.Odds != nil {
		detail += " " + runtimePatchMonitorText("这是低概率提示，不等同于证明修改。", "This is an improbability notice, not proof of modification.")
	} else {
		detail += " " + runtimePatchMonitorText("按 Logs 1.12.6 当前游戏表，此组合没有自然生成路径。", "Under the current GBFR Logs 1.12.6 tables, this combination has no natural generation path.")
	}
	return LogsBuildLegalityFinding{
		Rule: rule, Label: logsLegalityRuleLabel(rule), Detail: detail,
		Odds: payload.Odds, HardBreach: payload.Odds == nil, Source: "GBFR Logs 1.12.6 stored finding",
	}, nil
}

func readLogsLegalityFindings(db *sql.DB, logID int64) (map[int][]LogsBuildLegalityFinding, error) {
	result := map[int][]LogsBuildLegalityFinding{}
	exists, err := logsTableExists(db, "legality_findings")
	if err != nil || !exists {
		return result, err
	}
	rows, err := db.Query(`SELECT player_index, rule, payload FROM legality_findings WHERE log_id = ? ORDER BY rowid LIMIT ?`, logID, logsLegalityMaximumRows+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
		if count > logsLegalityMaximumRows {
			return nil, fmt.Errorf("Logs 合法性记录超过单场 %d 条边界", logsLegalityMaximumRows)
		}
		var playerIndex int
		var rule, payload string
		if err := rows.Scan(&playerIndex, &rule, &payload); err != nil {
			return nil, err
		}
		if playerIndex < 0 || playerIndex > 3 {
			continue
		}
		finding, err := parseLogsLegalityFinding(rule, payload)
		if err != nil {
			finding = LogsBuildLegalityFinding{
				Rule:       strings.TrimSpace(rule),
				Label:      logsLegalityRuleLabel(strings.TrimSpace(rule)),
				Detail:     fmt.Sprintf(runtimePatchMonitorText("这条 Logs 检测记录已损坏或超出读取边界，无法安全展示：%v", "This stored Logs finding is malformed or exceeds the read boundary and cannot be displayed safely: %v"), err),
				HardBreach: false,
				Source:     "GBFR Logs 1.12.6 stored finding",
			}
		}
		result[playerIndex] = append(result[playerIndex], finding)
	}
	return result, rows.Err()
}

func readLogsLegalityCounts(db *sql.DB, logIDs []int64) (map[int64]int, error) {
	result := map[int64]int{}
	if len(logIDs) == 0 {
		return result, nil
	}
	exists, err := logsTableExists(db, "legality_findings")
	if err != nil || !exists {
		return result, err
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(logIDs)), ",")
	args := make([]any, len(logIDs))
	for index, id := range logIDs {
		args[index] = id
	}
	rows, err := db.Query(`SELECT log_id, COUNT(*) FROM legality_findings WHERE log_id IN (`+placeholders+`) GROUP BY log_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		result[id] = count
	}
	return result, rows.Err()
}
