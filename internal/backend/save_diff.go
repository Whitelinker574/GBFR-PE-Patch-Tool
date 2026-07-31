package backend

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const saveDiffMaximumFileBytes int64 = 64 * 1024 * 1024

func saveDiffText(zh, en string) string {
	if useChinese() {
		return zh
	}
	return en
}

func saveDiffError(zh, en string) error {
	return fmt.Errorf("%s", saveDiffText(zh, en))
}

func saveDiffWrap(zh, en string, err error) error {
	return fmt.Errorf("%s: %w", saveDiffText(zh, en), err)
}

type SaveDiffSummary struct {
	LeftName     string `json:"leftName"`
	RightName    string `json:"rightName"`
	LeftVersion  uint32 `json:"leftVersion"`
	RightVersion uint32 `json:"rightVersion"`
	LeftRecords  int    `json:"leftRecords"`
	RightRecords int    `json:"rightRecords"`
	Added        int    `json:"added"`
	Removed      int    `json:"removed"`
	Changed      int    `json:"changed"`
	Unchanged    int    `json:"unchanged"`
	Different    int    `json:"different"`
}

type SaveDiffEntry struct {
	Key                string         `json:"key"`
	Section            string         `json:"section"`
	ValueType          string         `json:"valueType"`
	IDType             uint32         `json:"idType"`
	UnitID             uint32         `json:"unitId"`
	Occurrence         int            `json:"occurrence"`
	LeftOccurrence     int            `json:"leftOccurrence"`
	RightOccurrence    int            `json:"rightOccurrence"`
	SemanticName       string         `json:"semanticName,omitempty"`
	Category           string         `json:"category"`
	CategoryNameZh     string         `json:"categoryNameZh"`
	CategoryNameEn     string         `json:"categoryNameEn"`
	SemanticNameZh     string         `json:"semanticNameZh"`
	SemanticNameEn     string         `json:"semanticNameEn"`
	SemanticPurposeZh  string         `json:"semanticPurposeZh"`
	SemanticPurposeEn  string         `json:"semanticPurposeEn"`
	SemanticConfidence string         `json:"semanticConfidence"`
	Status             string         `json:"status"`
	LeftIndex          int            `json:"leftIndex"`
	RightIndex         int            `json:"rightIndex"`
	LeftCount          int            `json:"leftCount"`
	RightCount         int            `json:"rightCount"`
	LeftHash           string         `json:"leftHash,omitempty"`
	RightHash          string         `json:"rightHash,omitempty"`
	LeftPreview        string         `json:"leftPreview,omitempty"`
	RightPreview       string         `json:"rightPreview,omitempty"`
	LeftEntity         SaveDiffEntity `json:"leftEntity"`
	RightEntity        SaveDiffEntity `json:"rightEntity"`
	LeftDisplayZh      string         `json:"leftDisplayZh,omitempty"`
	LeftDisplayEn      string         `json:"leftDisplayEn,omitempty"`
	RightDisplayZh     string         `json:"rightDisplayZh,omitempty"`
	RightDisplayEn     string         `json:"rightDisplayEn,omitempty"`
	RiskLevel          string         `json:"riskLevel"`
	RiskReasonZh       string         `json:"riskReasonZh,omitempty"`
	RiskReasonEn       string         `json:"riskReasonEn,omitempty"`
	CopySupported      bool           `json:"copySupported"`
	CopyBlockReason    string         `json:"copyBlockReason,omitempty"`
	CopyBlockReasonZh  string         `json:"copyBlockReasonZh,omitempty"`
	CopyBlockReasonEn  string         `json:"copyBlockReasonEn,omitempty"`
	leftValue          saveDiffValueSummary
	rightValue         saveDiffValueSummary
}

type SaveDiffPageResult struct {
	Items         []SaveDiffEntry `json:"items"`
	NextCursor    int             `json:"nextCursor"`
	HasMore       bool            `json:"hasMore"`
	TotalFiltered int             `json:"totalFiltered"`
}

type saveDiffRecord struct {
	key          string
	groupKey     string
	section      string
	valueType    string
	idType       uint32
	unitID       uint32
	occurrence   int
	index        int
	count        int
	hash         string
	preview      string
	semanticName string
	value        saveDiffValueSummary
}

type saveDiffSession struct {
	summary   SaveDiffSummary
	entries   []SaveDiffEntry
	leftPath  string
	rightPath string
	leftSHA   [sha256.Size]byte
	rightSHA  [sha256.Size]byte
}

type saveDiffExport struct {
	SchemaVersion int                 `json:"schemaVersion"`
	GeneratedAt   string              `json:"generatedAt"`
	Summary       saveDiffExportStats `json:"summary"`
	Changes       []saveDiffExportRow `json:"changes"`
}

type saveDiffExportStats struct {
	LeftVersion  uint32 `json:"leftVersion"`
	RightVersion uint32 `json:"rightVersion"`
	LeftRecords  int    `json:"leftRecords"`
	RightRecords int    `json:"rightRecords"`
	Added        int    `json:"added"`
	Removed      int    `json:"removed"`
	Changed      int    `json:"changed"`
	Different    int    `json:"different"`
}

type saveDiffExportRow struct {
	Section         string `json:"section"`
	ValueType       string `json:"valueType"`
	SemanticName    string `json:"semanticName,omitempty"`
	IDType          uint32 `json:"idType"`
	UnitID          uint32 `json:"unitId"`
	Occurrence      int    `json:"occurrence"`
	LeftOccurrence  int    `json:"leftOccurrence"`
	RightOccurrence int    `json:"rightOccurrence"`
	Status          string `json:"status"`
	LeftIndex       int    `json:"leftIndex"`
	RightIndex      int    `json:"rightIndex"`
	LeftCount       int    `json:"leftCount"`
	RightCount      int    `json:"rightCount"`
	LeftHash        string `json:"leftHash,omitempty"`
	RightHash       string `json:"rightHash,omitempty"`
}

func (a *App) SelectSaveDiffFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: saveDiffText("选择 GBFR 存档", "Select a GBFR save"),
		Filters: []runtime.FileFilter{
			{DisplayName: "GBFR SaveData (*.dat)", Pattern: "*.dat"},
			{DisplayName: saveDiffText("所有文件", "All files"), Pattern: "*.*"},
		},
	})
}

func (a *App) OpenSaveDiff(leftPath, rightPath string) (*SaveDiffSummary, error) {
	leftPath = strings.TrimSpace(leftPath)
	rightPath = strings.TrimSpace(rightPath)
	if leftPath == "" || rightPath == "" {
		return nil, saveDiffError("请选择两份存档", "Select two save files")
	}
	leftAbsolute, err := filepath.Abs(leftPath)
	if err != nil {
		return nil, saveDiffWrap("解析左侧存档路径失败", "Failed to resolve the baseline save path", err)
	}
	rightAbsolute, err := filepath.Abs(rightPath)
	if err != nil {
		return nil, saveDiffWrap("解析右侧存档路径失败", "Failed to resolve the comparison save path", err)
	}
	if strings.EqualFold(filepath.Clean(leftAbsolute), filepath.Clean(rightAbsolute)) {
		return nil, saveDiffError("左右两侧不能选择同一份存档", "Choose two different save files")
	}
	left, leftSHA, err := loadSaveDiffSnapshot(leftAbsolute)
	if err != nil {
		return nil, saveDiffWrap("解析左侧存档失败", "Failed to parse the baseline save", err)
	}
	right, rightSHA, err := loadSaveDiffSnapshot(rightAbsolute)
	if err != nil {
		return nil, saveDiffWrap("解析右侧存档失败", "Failed to parse the comparison save", err)
	}
	session := buildSaveDiffSession(filepath.Base(leftAbsolute), filepath.Base(rightAbsolute), left, right)
	session.leftPath, session.rightPath = leftAbsolute, rightAbsolute
	session.leftSHA, session.rightSHA = leftSHA, rightSHA
	a.saveDiffMu.Lock()
	a.saveDiffSession = session
	a.saveDiffMu.Unlock()
	copy := session.summary
	return &copy, nil
}

func loadSaveDiffFile(path string) (*SaveGameFile, error) {
	save, _, err := loadSaveDiffSnapshot(path)
	return save, err
}

func loadSaveDiffSnapshot(path string) (*SaveGameFile, [sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	file, err := os.Open(path)
	if err != nil {
		return nil, digest, saveDiffWrap("读取文件失败", "Failed to read the file", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, digest, saveDiffWrap("读取文件信息失败", "Failed to read file information", err)
	}
	if info.IsDir() {
		return nil, digest, saveDiffError("所选路径不是存档文件", "The selected path is not a save file")
	}
	if info.Size() > saveDiffMaximumFileBytes {
		limit := fmt.Sprintf("%d MiB", saveDiffMaximumFileBytes/(1024*1024))
		return nil, digest, saveDiffError("存档文件超过 "+limit+" 的比较上限", "The save file exceeds the "+limit+" comparison limit")
	}
	data, err := io.ReadAll(io.LimitReader(file, saveDiffMaximumFileBytes+1))
	if err != nil {
		return nil, digest, saveDiffWrap("读取文件失败", "Failed to read the file", err)
	}
	if int64(len(data)) > saveDiffMaximumFileBytes {
		limit := fmt.Sprintf("%d MiB", saveDiffMaximumFileBytes/(1024*1024))
		return nil, digest, saveDiffError("存档文件超过 "+limit+" 的比较上限", "The save file exceeds the "+limit+" comparison limit")
	}
	save, err := ParseSaveData(data)
	if err != nil {
		return nil, digest, err
	}
	return save, sha256.Sum256(data), nil
}

func (a *App) SaveDiffPage(cursor, limit int, search, status, category, copyability, semanticConfidence string) (*SaveDiffPageResult, error) {
	if cursor < 0 {
		cursor = 0
	}
	if limit <= 0 || limit > 200 {
		limit = 80
	}
	search = strings.ToLower(strings.TrimSpace(search))
	status = strings.ToLower(strings.TrimSpace(status))
	category = strings.ToLower(strings.TrimSpace(category))
	copyability = strings.ToLower(strings.TrimSpace(copyability))
	semanticConfidence = strings.ToLower(strings.TrimSpace(semanticConfidence))
	a.saveDiffMu.Lock()
	defer a.saveDiffMu.Unlock()
	if a.saveDiffSession == nil {
		return nil, saveDiffError("尚未打开存档差分", "No save comparison is open")
	}
	filtered := make([]SaveDiffEntry, 0, min(len(a.saveDiffSession.entries), limit*2))
	for _, entry := range a.saveDiffSession.entries {
		if status == "different" && entry.Status == "unchanged" {
			continue
		}
		if status != "" && status != "all" && status != "different" && entry.Status != status {
			continue
		}
		if category != "" && category != "all" && entry.Category != category {
			continue
		}
		switch copyability {
		case "copyable":
			if !entry.CopySupported {
				continue
			}
		case "blocked":
			if entry.CopySupported {
				continue
			}
		}
		if semanticConfidence != "" && semanticConfidence != "all" && entry.SemanticConfidence != semanticConfidence {
			continue
		}
		if search != "" && !saveDiffEntryMatches(entry, search) {
			continue
		}
		filtered = append(filtered, entry)
	}
	if cursor > len(filtered) {
		cursor = len(filtered)
	}
	end := min(cursor+limit, len(filtered))
	items := append([]SaveDiffEntry(nil), filtered[cursor:end]...)
	return &SaveDiffPageResult{Items: items, NextCursor: end, HasMore: end < len(filtered), TotalFiltered: len(filtered)}, nil
}

func (a *App) CloseSaveDiff() {
	a.saveDiffMu.Lock()
	a.saveDiffSession = nil
	a.saveDiffMu.Unlock()
}

func (a *App) ExportSaveDiffJSON() (string, error) {
	a.saveDiffMu.Lock()
	if a.saveDiffSession == nil {
		a.saveDiffMu.Unlock()
		return "", saveDiffError("尚未打开存档差分", "No save comparison is open")
	}
	summary := a.saveDiffSession.summary
	rows := saveDiffExportRows(a.saveDiffSession.entries, summary.Different)
	a.saveDiffMu.Unlock()
	payload := saveDiffExport{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Summary: saveDiffExportStats{
			LeftVersion: summary.LeftVersion, RightVersion: summary.RightVersion,
			LeftRecords: summary.LeftRecords, RightRecords: summary.RightRecords,
			Added: summary.Added, Removed: summary.Removed, Changed: summary.Changed, Different: summary.Different,
		},
		Changes: rows,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", saveDiffWrap("生成差分 JSON 失败", "Failed to generate comparison JSON", err)
	}
	outputPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           saveDiffText("导出脱敏存档差分", "Export sanitized save comparison"),
		DefaultFilename: "gbfr-save-diff.json",
		Filters:         []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || outputPath == "" {
		return "", err
	}
	if err := writeSaveDiffFile(outputPath, data); err != nil {
		return "", err
	}
	return outputPath, nil
}

func saveDiffExportRows(entries []SaveDiffEntry, capacity int) []saveDiffExportRow {
	rows := make([]saveDiffExportRow, 0, capacity)
	for _, entry := range entries {
		if entry.Status == "unchanged" {
			continue
		}
		rows = append(rows, saveDiffExportRow{
			Section: entry.Section, ValueType: entry.ValueType, SemanticName: entry.SemanticName,
			IDType: entry.IDType, UnitID: entry.UnitID,
			Occurrence: entry.Occurrence, LeftOccurrence: entry.LeftOccurrence, RightOccurrence: entry.RightOccurrence,
			Status: entry.Status, LeftIndex: entry.LeftIndex, RightIndex: entry.RightIndex,
			LeftCount: entry.LeftCount, RightCount: entry.RightCount, LeftHash: entry.LeftHash, RightHash: entry.RightHash,
		})
	}
	return rows
}

func encodeSaveDiffCSV(rows []saveDiffExportRow) ([]byte, error) {
	var output bytes.Buffer
	output.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(&output)
	if err := writer.Write([]string{"section", "valueType", "semanticName", "idType", "unitId", "occurrence", "leftOccurrence", "rightOccurrence", "status", "leftIndex", "rightIndex", "leftCount", "rightCount", "leftHash", "rightHash"}); err != nil {
		return nil, err
	}
	for _, row := range rows {
		values := []string{
			row.Section, row.ValueType, row.SemanticName,
			strconv.FormatUint(uint64(row.IDType), 10), strconv.FormatUint(uint64(row.UnitID), 10),
			strconv.Itoa(row.Occurrence), strconv.Itoa(row.LeftOccurrence), strconv.Itoa(row.RightOccurrence), row.Status,
			strconv.Itoa(row.LeftIndex), strconv.Itoa(row.RightIndex), strconv.Itoa(row.LeftCount), strconv.Itoa(row.RightCount),
			row.LeftHash, row.RightHash,
		}
		if err := writer.Write(values); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func (a *App) ExportSaveDiffCSV() (string, error) {
	a.saveDiffMu.Lock()
	if a.saveDiffSession == nil {
		a.saveDiffMu.Unlock()
		return "", saveDiffError("尚未打开存档差分", "No save comparison is open")
	}
	rows := saveDiffExportRows(a.saveDiffSession.entries, a.saveDiffSession.summary.Different)
	a.saveDiffMu.Unlock()
	data, err := encodeSaveDiffCSV(rows)
	if err != nil {
		return "", saveDiffWrap("生成差分 CSV 失败", "Failed to generate comparison CSV", err)
	}
	outputPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           saveDiffText("导出脱敏存档差分 CSV", "Export sanitized save comparison CSV"),
		DefaultFilename: "gbfr-save-diff.csv",
		Filters:         []runtime.FileFilter{{DisplayName: "CSV", Pattern: "*.csv"}},
	})
	if err != nil || outputPath == "" {
		return "", err
	}
	if err := writeSaveDiffFile(outputPath, data); err != nil {
		return "", err
	}
	return outputPath, nil
}

func writeSaveDiffFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".gbfr-save-diff-*.tmp")
	if err != nil {
		return saveDiffWrap("创建差分临时文件失败", "Failed to create the temporary comparison file", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return saveDiffWrap("写入差分失败", "Failed to write the comparison file", err)
	}
	if err := tmp.Sync(); err != nil {
		return saveDiffWrap("同步差分失败", "Failed to flush the comparison file", err)
	}
	if err := tmp.Close(); err != nil {
		return saveDiffWrap("关闭差分临时文件失败", "Failed to close the temporary comparison file", err)
	}
	if err := replaceFileAtomic(tmpPath, path); err != nil {
		return saveDiffWrap("保存差分失败", "Failed to save the comparison file", err)
	}
	committed = true
	return nil
}

func saveDiffEntryMatches(entry SaveDiffEntry, search string) bool {
	values := []string{
		entry.Section, entry.ValueType, entry.Category, entry.CategoryNameZh, entry.CategoryNameEn,
		entry.SemanticName, entry.SemanticNameZh, entry.SemanticNameEn,
		entry.SemanticPurposeZh, entry.SemanticPurposeEn, entry.SemanticConfidence,
		entry.LeftEntity.NameZh, entry.LeftEntity.NameEn, entry.LeftEntity.DetailZh, entry.LeftEntity.DetailEn,
		entry.RightEntity.NameZh, entry.RightEntity.NameEn, entry.RightEntity.DetailZh, entry.RightEntity.DetailEn,
		entry.LeftDisplayZh, entry.LeftDisplayEn, entry.RightDisplayZh, entry.RightDisplayEn,
		entry.RiskLevel, entry.RiskReasonZh, entry.RiskReasonEn,
		entry.LeftHash, entry.RightHash,
		strconv.FormatUint(uint64(entry.IDType), 10), fmt.Sprintf("0x%08X", entry.IDType),
		strconv.FormatUint(uint64(entry.UnitID), 10), fmt.Sprintf("0x%08X", entry.UnitID),
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), search) {
			return true
		}
	}
	return false
}

func buildSaveDiffSession(leftName, rightName string, left, right *SaveGameFile) *saveDiffSession {
	leftRecords := flattenSaveRecords(left)
	rightRecords := flattenSaveRecords(right)
	leftGroups := make(map[string][]saveDiffRecord)
	rightGroups := make(map[string][]saveDiffRecord)
	type groupOrderEntry struct {
		key       string
		section   string
		valueType string
		idType    uint32
		unitID    uint32
	}
	groupOrder := make([]groupOrderEntry, 0, len(leftRecords)+len(rightRecords))
	seen := make(map[string]bool, cap(groupOrder))
	registerGroup := func(record saveDiffRecord) {
		if seen[record.groupKey] {
			return
		}
		seen[record.groupKey] = true
		groupOrder = append(groupOrder, groupOrderEntry{
			key: record.groupKey, section: record.section, valueType: record.valueType,
			idType: record.idType, unitID: record.unitID,
		})
	}
	for _, record := range leftRecords {
		leftGroups[record.groupKey] = append(leftGroups[record.groupKey], record)
		registerGroup(record)
	}
	for _, record := range rightRecords {
		rightGroups[record.groupKey] = append(rightGroups[record.groupKey], record)
		registerGroup(record)
	}
	sort.Slice(groupOrder, func(i, j int) bool {
		a, b := groupOrder[i], groupOrder[j]
		if a.section != b.section {
			return a.section < b.section
		}
		if a.valueType != b.valueType {
			return a.valueType < b.valueType
		}
		if a.idType != b.idType {
			return a.idType < b.idType
		}
		if a.unitID != b.unitID {
			return a.unitID < b.unitID
		}
		return a.key < b.key
	})
	entries := make([]SaveDiffEntry, 0, len(leftRecords)+len(rightRecords))
	summary := SaveDiffSummary{
		LeftName: leftName, RightName: rightName,
		LeftVersion: saveSlotVersion(left), RightVersion: saveSlotVersion(right),
		LeftRecords: len(leftRecords), RightRecords: len(rightRecords),
	}
	type recordMatchKey struct {
		count int
		hash  string
	}
	for _, orderedGroup := range groupOrder {
		groupKey := orderedGroup.key
		leftGroup, rightGroup := leftGroups[groupKey], rightGroups[groupKey]
		usedRight := make([]bool, len(rightGroup))
		usedLeft := make([]bool, len(leftGroup))
		rightMatches := make(map[recordMatchKey][]int, len(rightGroup))
		rightMatchCursor := make(map[recordMatchKey]int, len(rightGroup))
		for rightIndex, rightRecord := range rightGroup {
			matchKey := recordMatchKey{count: rightRecord.count, hash: rightRecord.hash}
			rightMatches[matchKey] = append(rightMatches[matchKey], rightIndex)
		}
		for leftIndex, leftRecord := range leftGroup {
			matchKey := recordMatchKey{count: leftRecord.count, hash: leftRecord.hash}
			matchIndexes := rightMatches[matchKey]
			cursor := rightMatchCursor[matchKey]
			if cursor >= len(matchIndexes) {
				continue
			}
			rightIndex := matchIndexes[cursor]
			rightMatchCursor[matchKey] = cursor + 1
			usedLeft[leftIndex], usedRight[rightIndex] = true, true
			rightRecord := rightGroup[rightIndex]
			entries = append(entries, makeSaveDiffEntry(groupKey, &leftRecord, &rightRecord, "unchanged"))
			summary.Unchanged++
		}
		remainingLeft, remainingRight := make([]saveDiffRecord, 0), make([]saveDiffRecord, 0)
		for index, record := range leftGroup {
			if !usedLeft[index] {
				remainingLeft = append(remainingLeft, record)
			}
		}
		for index, record := range rightGroup {
			if !usedRight[index] {
				remainingRight = append(remainingRight, record)
			}
		}
		paired := min(len(remainingLeft), len(remainingRight))
		for index := 0; index < paired; index++ {
			entries = append(entries, makeSaveDiffEntry(groupKey, &remainingLeft[index], &remainingRight[index], "changed"))
			summary.Changed++
		}
		for index := paired; index < len(remainingLeft); index++ {
			entries = append(entries, makeSaveDiffEntry(groupKey, &remainingLeft[index], nil, "removed"))
			summary.Removed++
		}
		for index := paired; index < len(remainingRight); index++ {
			entries = append(entries, makeSaveDiffEntry(groupKey, nil, &remainingRight[index], "added"))
			summary.Added++
		}
	}
	leftEntityIndex := newSaveDiffEntityIndex(left)
	rightEntityIndex := newSaveDiffEntityIndex(right)
	for index := range entries {
		enrichSaveDiffEntry(&entries[index], leftEntityIndex, rightEntityIndex)
	}
	entryOrder := make([]int, len(entries))
	for index := range entryOrder {
		entryOrder[index] = index
	}
	sort.Slice(entryOrder, func(i, j int) bool {
		a, b := entries[entryOrder[i]], entries[entryOrder[j]]
		if a.Section != b.Section {
			return a.Section < b.Section
		}
		if a.ValueType != b.ValueType {
			return a.ValueType < b.ValueType
		}
		if a.IDType != b.IDType {
			return a.IDType < b.IDType
		}
		if a.UnitID != b.UnitID {
			return a.UnitID < b.UnitID
		}
		if a.Occurrence != b.Occurrence {
			return a.Occurrence < b.Occurrence
		}
		return a.Key < b.Key
	})
	sortedEntries := make([]SaveDiffEntry, len(entries))
	for index, sourceIndex := range entryOrder {
		sortedEntries[index] = entries[sourceIndex]
	}
	entries = sortedEntries
	summary.Different = summary.Added + summary.Removed + summary.Changed
	return &saveDiffSession{summary: summary, entries: entries}
}

func makeSaveDiffEntry(groupKey string, left, right *saveDiffRecord, status string) SaveDiffEntry {
	base := right
	if left != nil {
		base = left
	}
	semantic := saveDiffSemanticFor(base.idType)
	entry := SaveDiffEntry{
		Key:     fmt.Sprintf("%s/l%d-r%d/%s", groupKey, saveDiffOccurrence(left), saveDiffOccurrence(right), status),
		Section: base.section, ValueType: base.valueType, IDType: base.idType, UnitID: base.unitID,
		Occurrence:     max(saveDiffOccurrence(left), saveDiffOccurrence(right)),
		LeftOccurrence: saveDiffOccurrence(left), RightOccurrence: saveDiffOccurrence(right),
		SemanticName: semantic.NameEn,
		Category:     semantic.Category, CategoryNameZh: semantic.CategoryZh, CategoryNameEn: semantic.CategoryEn,
		SemanticNameZh: semantic.NameZh, SemanticNameEn: semantic.NameEn,
		SemanticPurposeZh: semantic.PurposeZh, SemanticPurposeEn: semantic.PurposeEn,
		SemanticConfidence: semantic.Confidence,
		Status:             status, LeftIndex: -1, RightIndex: -1,
	}
	if left != nil {
		entry.LeftIndex, entry.LeftCount, entry.LeftHash, entry.LeftPreview = left.index, left.count, left.hash, left.preview
		entry.leftValue = left.value
	}
	if right != nil {
		entry.RightIndex, entry.RightCount, entry.RightHash, entry.RightPreview = right.index, right.count, right.hash, right.preview
		entry.rightValue = right.value
	}
	switch {
	case left == nil || right == nil:
		entry.CopyBlockReasonZh = "该记录只存在于一侧，不能在不重建存档结构的情况下安全复制"
		entry.CopyBlockReasonEn = "This record exists on only one side and cannot be copied without rebuilding the save structure"
	case left.count != right.count:
		entry.CopyBlockReasonZh = "左右记录长度不同，不能原位替换"
		entry.CopyBlockReasonEn = "The two records have different lengths and cannot be replaced in place"
	case left.idType == SaveID_HashSeed:
		entry.CopyBlockReasonZh = "校验种子由存档自身维护，不能从另一份存档复制"
		entry.CopyBlockReasonEn = "The save owns its checksum seed; it cannot be copied from another save"
	}
	entry.CopyBlockReason = saveDiffText(entry.CopyBlockReasonZh, entry.CopyBlockReasonEn)
	return entry
}

func saveDiffOccurrence(record *saveDiffRecord) int {
	if record == nil {
		return -1
	}
	return record.occurrence
}

func saveSlotVersion(save *SaveGameFile) uint32 {
	if save != nil && save.SlotData != nil {
		return save.SlotData.VersionMaybe
	}
	return 0
}

func flattenSaveRecords(save *SaveGameFile) []saveDiffRecord {
	if save == nil {
		return nil
	}
	var records []saveDiffRecord
	appendSection := func(name string, data *SaveDataBinary) {
		if data == nil {
			return
		}
		for i, unit := range data.IntTable {
			records = appendDiffRecord(records, name, "int32", unit.IDType, unit.UnitID, i, unit.ValueData)
		}
		for i, unit := range data.UIntTable {
			records = appendDiffRecord(records, name, "uint32", unit.IDType, unit.UnitID, i, unit.ValueData)
		}
		for i, unit := range data.BoolTable {
			records = appendDiffRecord(records, name, "bool", unit.IDType, unit.UnitID, i, unit.ValueData)
		}
		for i, unit := range data.FloatTable {
			records = appendDiffRecord(records, name, "float32", unit.IDType, unit.UnitID, i, unit.ValueData)
		}
		for i, unit := range data.ByteTable {
			records = appendDiffRecord(records, name, "byte", unit.IDType, unit.UnitID, i, unit.ValueData)
		}
		for i, unit := range data.UByteTable {
			records = appendDiffRecord(records, name, "ubyte", unit.IDType, unit.UnitID, i, unit.ValueData)
		}
		for i, unit := range data.ShortTable {
			records = appendDiffRecord(records, name, "int16", unit.IDType, unit.UnitID, i, unit.ValueData)
		}
		for i, unit := range data.UShortTable {
			records = appendDiffRecord(records, name, "uint16", unit.IDType, unit.UnitID, i, unit.ValueData)
		}
		for i, unit := range data.LongTable {
			records = appendDiffRecord(records, name, "int64", unit.IDType, unit.UnitID, i, unit.ValueData)
		}
		for i, unit := range data.ULongTable {
			records = appendDiffRecord(records, name, "uint64", unit.IDType, unit.UnitID, i, unit.ValueData)
		}
	}
	appendSection("binary1", save.Binary1)
	appendSection("slotData", save.SlotData)
	occurrences := make(map[string]int)
	for index := range records {
		base := fmt.Sprintf("%s/%s/%d/%d", records[index].section, records[index].valueType, records[index].idType, records[index].unitID)
		records[index].groupKey = base
		records[index].occurrence = occurrences[base]
		occurrences[base]++
		records[index].key = fmt.Sprintf("%s/%d", base, records[index].occurrence)
	}
	return records
}

func appendDiffRecord[T any](records []saveDiffRecord, section, valueType string, idType, unitID uint32, index int, values []T) []saveDiffRecord {
	encoded := fmt.Sprintf("%s:%#v", valueType, values)
	digest := sha256.Sum256([]byte(encoded))
	return append(records, saveDiffRecord{
		section: section, valueType: valueType, idType: idType, unitID: unitID, index: index,
		count: len(values), hash: hex.EncodeToString(digest[:8]), preview: saveValuePreview(values, valueType),
		semanticName: saveRecordSemanticName(idType), value: summarizeSaveDiffValues(values),
	})
}

func saveValuePreview(values any, valueType string) string {
	value := reflect.ValueOf(values)
	if !value.IsValid() || value.Len() == 0 {
		return "[]"
	}
	limit := min(value.Len(), 6)
	parts := make([]string, 0, limit+1)
	for i := 0; i < limit; i++ {
		item := value.Index(i).Interface()
		if valueType == "byte" || valueType == "ubyte" {
			parts = append(parts, fmt.Sprintf("%02X", item))
		} else {
			parts = append(parts, fmt.Sprint(item))
		}
	}
	if value.Len() > limit {
		parts = append(parts, "…")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func saveRecordSemanticName(idType uint32) string {
	semantic := saveDiffSemanticFor(idType)
	if semantic.Confidence == "unknown" {
		return ""
	}
	return semantic.NameEn
}
