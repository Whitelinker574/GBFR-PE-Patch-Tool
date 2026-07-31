package backend

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const saveDiffMaximumTransferRecords = 2000

type SaveDiffTransferRequest struct {
	TargetSide string   `json:"targetSide"`
	Keys       []string `json:"keys"`
}

type SaveDiffTransferResult struct {
	Applied      int             `json:"applied"`
	Verified     int             `json:"verified"`
	TargetSide   string          `json:"targetSide"`
	TargetName   string          `json:"targetName"`
	BackupPath   string          `json:"backupPath,omitempty"`
	UpdatedDiffs int             `json:"updatedDiffs"`
	Summary      SaveDiffSummary `json:"summary"`
}

type saveDiffVectorRef struct {
	values []byte
	count  int
	width  int
}

func readSaveDiffRawFile(path string) ([]byte, [sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	file, err := os.Open(path)
	if err != nil {
		return nil, digest, saveDiffWrap("读取存档失败", "Failed to read the save", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, digest, saveDiffWrap("读取存档信息失败", "Failed to read save information", err)
	}
	if info.IsDir() {
		return nil, digest, saveDiffError("所选路径不是存档文件", "The selected path is not a save file")
	}
	if info.Size() > saveDiffMaximumFileBytes {
		return nil, digest, saveDiffError("存档文件超过 64 MiB 的处理上限", "The save exceeds the 64 MiB processing limit")
	}
	data, err := io.ReadAll(io.LimitReader(file, saveDiffMaximumFileBytes+1))
	if err != nil {
		return nil, digest, saveDiffWrap("读取存档失败", "Failed to read the save", err)
	}
	if int64(len(data)) > saveDiffMaximumFileBytes {
		return nil, digest, saveDiffError("存档文件超过 64 MiB 的处理上限", "The save exceeds the 64 MiB processing limit")
	}
	if _, err := ParseSaveData(data); err != nil {
		return nil, digest, err
	}
	return data, sha256.Sum256(data), nil
}

func saveDiffSectionSpan(data []byte, section string) ([]byte, error) {
	if len(data) < 52 {
		return nil, saveDiffError("存档头不完整", "The save header is incomplete")
	}
	var offset, size int64
	switch section {
	case "binary1":
		offset = int64(binary.LittleEndian.Uint64(data[20:28]))
		size = int64(binary.LittleEndian.Uint64(data[36:44]))
	case "slotData":
		offset = int64(binary.LittleEndian.Uint64(data[28:36]))
		size = int64(binary.LittleEndian.Uint64(data[44:52]))
	default:
		return nil, saveDiffError("差异记录属于未知存档区段", "The changed record belongs to an unknown save section")
	}
	if size <= 0 || !validInt64Span(offset, size, int64(len(data))) {
		return nil, saveDiffError("差异记录对应的存档区段无效", "The save section for this changed record is invalid")
	}
	return data[offset : offset+size], nil
}

func saveDiffValueLayout(valueType string) (rootField int, width int, ok bool) {
	switch valueType {
	case "bool":
		return 1, 1, true
	case "byte":
		return 2, 1, true
	case "ubyte":
		return 3, 1, true
	case "int16":
		return 4, 2, true
	case "uint16":
		return 5, 2, true
	case "int32":
		return 6, 4, true
	case "uint32":
		return 7, 4, true
	case "int64":
		return 8, 8, true
	case "uint64":
		return 9, 8, true
	case "float32":
		return 10, 4, true
	default:
		return 0, 0, false
	}
}

func locateSaveDiffVector(data []byte, entry SaveDiffEntry, occurrence int) (saveDiffVectorRef, error) {
	if occurrence < 0 {
		return saveDiffVectorRef{}, saveDiffError("所选方向没有可复制的源记录", "The selected direction has no source record to copy")
	}
	section, err := saveDiffSectionSpan(data, entry.Section)
	if err != nil {
		return saveDiffVectorRef{}, err
	}
	rootField, width, ok := saveDiffValueLayout(entry.ValueType)
	if !ok {
		return saveDiffVectorRef{}, saveDiffError("该记录类型暂不支持原位复制", "This record type cannot be copied in place")
	}
	if len(section) < 4 {
		return saveDiffVectorRef{}, saveDiffError("存档区段过短", "The save section is too short")
	}
	reader := &fbReader{data: section}
	root := int(reader.u32(0))
	table, vtable, vtableSize, _ := reader.readSubTable(root)
	if vtableSize == 0 {
		return saveDiffVectorRef{}, saveDiffError("存档根表无效", "The save root table is invalid")
	}
	field, exists := reader.fieldOff(vtable, vtableSize, rootField)
	if !exists {
		return saveDiffVectorRef{}, saveDiffError("存档中不存在该类型的记录表", "The save does not contain this record table")
	}
	vector := makeTableVec(reader, table, field)
	if vector == nil {
		return saveDiffVectorRef{}, saveDiffError("存档中的记录表为空", "The record table in the save is empty")
	}
	match := 0
	for index := 0; index < vector.count; index++ {
		unitTable, unitVTable, unitVTableSize := vector.read(index)
		if unitVTableSize == 0 {
			continue
		}
		idField, hasID := reader.fieldOff(unitVTable, unitVTableSize, 0)
		valueField, hasValues := reader.fieldOff(unitVTable, unitVTableSize, 2)
		if !hasID || !hasValues || reader.u32(unitTable+int(idField)) != entry.IDType {
			continue
		}
		var unitID uint32
		if unitField, hasUnitID := reader.fieldOff(unitVTable, unitVTableSize, 1); hasUnitID {
			unitID = reader.u32(unitTable + int(unitField))
		}
		if unitID != entry.UnitID {
			continue
		}
		if match != occurrence {
			match++
			continue
		}
		count, start := reader.readVectorAt(unitTable, valueField, width)
		if count <= 0 || !validIntSpan(start, count*width, len(section)) {
			return saveDiffVectorRef{}, saveDiffError("差异记录的值向量无效", "The changed record has an invalid value vector")
		}
		return saveDiffVectorRef{values: section[start : start+count*width], count: count, width: width}, nil
	}
	return saveDiffVectorRef{}, saveDiffError("重新读取时找不到差异记录，请重新比较两份存档", "The changed record was not found during reread; compare the saves again")
}

func saveDiffEntryByKey(entries []SaveDiffEntry, key string) (SaveDiffEntry, bool) {
	for _, entry := range entries {
		if entry.Key == key {
			return entry, true
		}
	}
	return SaveDiffEntry{}, false
}

func rollbackSaveDiffTarget(path string, original []byte) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".save-diff-rollback-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := temp.Write(original); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := replaceFileAtomic(tempPath, path); err != nil {
		return err
	}
	committed = true
	restored, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(restored, original) {
		return saveDiffError("自动恢复后的存档与写入前不一致", "The automatically restored save does not match its pre-write state")
	}
	return nil
}

func (a *App) ApplySaveDiffTransfers(request SaveDiffTransferRequest) (*SaveDiffTransferResult, error) {
	targetSide := strings.ToLower(strings.TrimSpace(request.TargetSide))
	if targetSide != "left" && targetSide != "right" {
		return nil, saveDiffError("请选择写入左侧或右侧存档", "Choose whether to write the left or right save")
	}
	if len(request.Keys) == 0 {
		return nil, saveDiffError("尚未选择要复制的差异", "No differences are selected for copying")
	}
	if len(request.Keys) > saveDiffMaximumTransferRecords {
		return nil, saveDiffError("一次最多复制 2000 条差异", "At most 2,000 differences can be copied at once")
	}

	a.saveDiffMu.Lock()
	defer a.saveDiffMu.Unlock()
	session := a.saveDiffSession
	if session == nil {
		return nil, saveDiffError("尚未打开存档差分", "No save comparison is open")
	}

	targetPath, sourcePath := session.leftPath, session.rightPath
	expectedTargetSHA, expectedSourceSHA := session.leftSHA, session.rightSHA
	if targetSide == "right" {
		targetPath, sourcePath = session.rightPath, session.leftPath
		expectedTargetSHA, expectedSourceSHA = session.rightSHA, session.leftSHA
	}
	if err := ensureGeneratorWriteAllowed(targetPath); err != nil {
		return nil, err
	}
	targetOriginal, targetSHA, err := readSaveDiffRawFile(targetPath)
	if err != nil {
		return nil, saveDiffWrap("重新读取写入目标失败", "Failed to reread the write target", err)
	}
	sourceRaw, sourceSHA, err := readSaveDiffRawFile(sourcePath)
	if err != nil {
		return nil, saveDiffWrap("重新读取复制来源失败", "Failed to reread the copy source", err)
	}
	if targetSHA != expectedTargetSHA || sourceSHA != expectedSourceSHA {
		return nil, saveDiffError("比较后存档文件已经变化；为避免覆盖新进度，请重新比较后再操作", "A save changed after comparison. Compare again to avoid overwriting newer progress")
	}
	targetParsed, err := ParseSaveData(targetOriginal)
	if err != nil {
		return nil, saveDiffWrap("重新解析写入目标失败", "Failed to reparse the write target", err)
	}
	sourceParsed, err := ParseSaveData(sourceRaw)
	if err != nil {
		return nil, saveDiffWrap("重新解析复制来源失败", "Failed to reparse the copy source", err)
	}
	currentLeft, currentRight := targetParsed, sourceParsed
	if targetSide == "right" {
		currentLeft, currentRight = sourceParsed, targetParsed
	}

	unique := make(map[string]bool, len(request.Keys))
	keys := make([]string, 0, len(request.Keys))
	for _, rawKey := range request.Keys {
		key := strings.TrimSpace(rawKey)
		if key == "" || unique[key] {
			continue
		}
		unique[key] = true
		keys = append(keys, key)
	}
	sort.Strings(keys)
	working := append([]byte(nil), targetOriginal...)
	type verification struct {
		entry            SaveDiffEntry
		sourceOccurrence int
		targetOccurrence int
		expected         []byte
	}
	checks := make([]verification, 0, len(keys))
	currentLeftIndex := newSaveDiffEntityIndex(currentLeft)
	currentRightIndex := newSaveDiffEntityIndex(currentRight)
	for _, key := range keys {
		entry, ok := saveDiffEntryByKey(session.entries, key)
		if !ok {
			return nil, saveDiffError("变更单里有已经失效的差异，请重新比较", "The change list contains a stale difference; compare again")
		}
		if !entry.CopySupported {
			reason := entry.CopyBlockReason
			if reason == "" {
				reason = saveDiffText("该差异不能安全复制", "This difference cannot be copied safely")
			}
			return nil, fmt.Errorf("%s: %s", entry.SemanticName, reason)
		}
		revalidated := entry
		enrichSaveDiffEntry(&revalidated, currentLeftIndex, currentRightIndex)
		if !revalidated.CopySupported {
			reason := revalidated.CopyBlockReason
			if reason == "" {
				reason = saveDiffText("重新解析后无法确认左右记录属于同一个具体对象", "Reparsing could not confirm that both records belong to the same entity")
			}
			return nil, fmt.Errorf("%s: %s", entry.SemanticName, reason)
		}
		sourceOccurrence, targetOccurrence := entry.RightOccurrence, entry.LeftOccurrence
		if targetSide == "right" {
			sourceOccurrence, targetOccurrence = entry.LeftOccurrence, entry.RightOccurrence
		}
		sourceVector, err := locateSaveDiffVector(sourceRaw, entry, sourceOccurrence)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.SemanticName, err)
		}
		targetVector, err := locateSaveDiffVector(working, entry, targetOccurrence)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.SemanticName, err)
		}
		if sourceVector.count != targetVector.count || sourceVector.width != targetVector.width {
			return nil, saveDiffError("左右记录结构已经不一致，请重新比较", "The two record structures no longer match; compare again")
		}
		expected := append([]byte(nil), sourceVector.values...)
		copy(targetVector.values, expected)
		checks = append(checks, verification{
			entry: entry, sourceOccurrence: sourceOccurrence, targetOccurrence: targetOccurrence, expected: expected,
		})
	}
	if len(checks) == 0 {
		return nil, saveDiffError("变更单中没有可应用的差异", "The change list contains no applicable differences")
	}

	// Check the target once more immediately before committing. The normal
	// managed-save path also requires the game to be fully closed.
	_, latestTargetSHA, err := readSaveDiffRawFile(targetPath)
	if err != nil {
		return nil, err
	}
	if latestTargetSHA != targetSHA {
		return nil, saveDiffError("准备写入时目标存档又发生了变化，已取消操作", "The target save changed while preparing the write; the operation was cancelled")
	}

	target, err := newSaveData(targetPath, working)
	if err != nil {
		return nil, saveDiffWrap("准备目标存档失败", "Failed to prepare the target save", err)
	}
	if err := target.FixChecksums(); err != nil {
		return nil, saveDiffWrap("更新存档校验失败", "Failed to update the save checksum", err)
	}
	if err := target.Write(targetPath); err != nil {
		return nil, saveDiffWrap("写入目标存档失败", "Failed to write the target save", err)
	}

	written, _, verifyErr := readSaveDiffRawFile(targetPath)
	if verifyErr == nil {
		for _, check := range checks {
			actual, locateErr := locateSaveDiffVector(written, check.entry, check.targetOccurrence)
			if locateErr != nil {
				verifyErr = locateErr
				break
			}
			if !bytes.Equal(actual.values, check.expected) {
				verifyErr = saveDiffError("写入后的字段与复制来源不一致", "A written field does not match its copy source")
				break
			}
		}
	}
	if verifyErr != nil {
		if rollbackErr := rollbackSaveDiffTarget(targetPath, targetOriginal); rollbackErr != nil {
			return nil, fmt.Errorf("%s: %v；%s: %w",
				saveDiffText("写入回读失败", "Write readback failed"), verifyErr,
				saveDiffText("自动恢复也失败，请保留备份并停止继续操作", "Automatic restoration also failed; keep the backup and stop"), rollbackErr)
		}
		return nil, fmt.Errorf("%s: %w", saveDiffText("写入回读失败，已自动恢复写入前存档", "Write readback failed; the pre-write save was restored automatically"), verifyErr)
	}

	left, leftSHA, err := loadSaveDiffSnapshot(session.leftPath)
	if err != nil {
		return nil, saveDiffWrap("写入成功，但刷新左侧存档失败", "The write succeeded, but the left save could not be refreshed", err)
	}
	right, rightSHA, err := loadSaveDiffSnapshot(session.rightPath)
	if err != nil {
		return nil, saveDiffWrap("写入成功，但刷新右侧存档失败", "The write succeeded, but the right save could not be refreshed", err)
	}
	refreshed := buildSaveDiffSession(filepath.Base(session.leftPath), filepath.Base(session.rightPath), left, right)
	refreshed.leftPath, refreshed.rightPath = session.leftPath, session.rightPath
	refreshed.leftSHA, refreshed.rightSHA = leftSHA, rightSHA
	a.saveDiffSession = refreshed

	return &SaveDiffTransferResult{
		Applied: len(checks), Verified: len(checks), TargetSide: targetSide,
		TargetName: filepath.Base(targetPath), BackupPath: target.LastBackupPath(),
		UpdatedDiffs: refreshed.summary.Different, Summary: refreshed.summary,
	}, nil
}
