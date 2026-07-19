package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func installRunningGeneratorProcessFinder(t *testing.T) *int {
	t.Helper()
	calls := 0
	original := generatorFindProcessByName
	generatorFindProcessByName = func(name string) (uint32, error) {
		calls++
		if name != charaProcessName {
			t.Fatalf("生成器检查了错误的进程名 %q", name)
		}
		return 4242, nil
	}
	t.Cleanup(func() { generatorFindProcessByName = original })
	return &calls
}

func TestSigilApplyQueueRejectsDefaultSaveWhileGameIsRunning(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	calls := installRunningGeneratorProcessFinder(t)
	defaultPath := filepath.Join(defaultSaveGamesDir(), "SaveData2.dat")

	_, err := NewSigilGen().ApplyQueue(defaultPath)
	if err == nil || !strings.Contains(err.Error(), "退出游戏") {
		t.Fatalf("默认存档写入应被游戏进程闸门拒绝，实际错误: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("默认存档应检查一次游戏进程，实际 %d 次", *calls)
	}
}

func TestSigilApplyQueueAllowsCustomOutputWhileGameIsRunning(t *testing.T) {
	t.Setenv("LOCALAPPDATA", filepath.Join(t.TempDir(), "default-save-root"))
	calls := installRunningGeneratorProcessFinder(t)
	customPath := filepath.Join(t.TempDir(), "exports", "SaveData2.dat")

	_, err := NewSigilGen().ApplyQueue(customPath)
	if err == nil {
		t.Fatal("空队列仍应由正常参数校验拒绝")
	}
	if strings.Contains(err.Error(), "退出游戏") {
		t.Fatalf("自定义另存路径不应被游戏进程闸门拒绝: %v", err)
	}
	if *calls != 0 {
		t.Fatalf("自定义另存路径不应查询游戏进程，实际 %d 次", *calls)
	}
}

func TestWrightstoneApplyQueueRejectsDefaultSaveWhileGameIsRunning(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	calls := installRunningGeneratorProcessFinder(t)
	defaultPath := filepath.Join(defaultSaveGamesDir(), "SaveData3.dat")

	_, err := NewWrightstoneGen().ApplyQueue(defaultPath)
	if err == nil || !strings.Contains(err.Error(), "退出游戏") {
		t.Fatalf("默认存档写入应被游戏进程闸门拒绝，实际错误: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("默认存档应检查一次游戏进程，实际 %d 次", *calls)
	}
}

func TestWrightstoneApplyQueueAllowsCustomOutputWhileGameIsRunning(t *testing.T) {
	t.Setenv("LOCALAPPDATA", filepath.Join(t.TempDir(), "default-save-root"))
	calls := installRunningGeneratorProcessFinder(t)
	customPath := filepath.Join(t.TempDir(), "exports", "SaveData3.dat")

	_, err := NewWrightstoneGen().ApplyQueue(customPath)
	if err == nil {
		t.Fatal("空队列仍应由正常参数校验拒绝")
	}
	if strings.Contains(err.Error(), "退出游戏") {
		t.Fatalf("自定义另存路径不应被游戏进程闸门拒绝: %v", err)
	}
	if *calls != 0 {
		t.Fatalf("自定义另存路径不应查询游戏进程，实际 %d 次", *calls)
	}
}

func TestGeneratorWriteGateRejectsPaddedDefaultSavePath(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	calls := installRunningGeneratorProcessFinder(t)
	defaultPath := "  " + filepath.Join(defaultSaveGamesDir(), "SaveData2.dat") + "  "

	if err := ensureGeneratorWriteAllowed(defaultPath); err == nil || !strings.Contains(err.Error(), "退出游戏") {
		t.Fatalf("带首尾空白的默认存档路径也应被拒绝，实际错误: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("规范化后的默认存档应检查一次游戏进程，实际 %d 次", *calls)
	}
}

func TestSigilDestructiveWritesRejectDefaultSaveWhileGameIsRunning(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	defaultPath := filepath.Join(defaultSaveGamesDir(), "SaveData1.dat")

	t.Run("remove all", func(t *testing.T) {
		calls := installRunningGeneratorProcessFinder(t)
		_, err := NewSigilGen().RemoveAllSigils(filepath.Join(t.TempDir(), "missing.dat"), defaultPath)
		if err == nil || !strings.Contains(err.Error(), "退出游戏") {
			t.Fatalf("清空默认存档应被游戏进程闸门拒绝，实际错误: %v", err)
		}
		if *calls != 1 {
			t.Fatalf("清空默认存档应检查一次游戏进程，实际 %d 次", *calls)
		}
	})

	t.Run("delete selected", func(t *testing.T) {
		calls := installRunningGeneratorProcessFinder(t)
		_, err := NewSigilGen().DeleteSelectedSigils([]int{GemSlotBaseID}, defaultPath)
		if err == nil || !strings.Contains(err.Error(), "退出游戏") {
			t.Fatalf("删除默认存档因子应被游戏进程闸门拒绝，实际错误: %v", err)
		}
		if *calls != 1 {
			t.Fatalf("删除默认存档因子应检查一次游戏进程，实际 %d 次", *calls)
		}
	})
}

func TestVerifyWrittenWrightstonesReturnsReloadFailure(t *testing.T) {
	sentinel := errors.New("forced reload failure")
	expected := []wrightstoneWriteExpectation{{ItemUnitID: WrightstoneSlotBaseID}}
	gen := NewWrightstoneGen()
	gen.loadSaveForVerification = func(string) (*SaveData, error) { return nil, sentinel }

	_, err := gen.verifyWrittenWrightstones("output.dat", 1, expected,
		func(*SaveData, wrightstoneWriteExpectation) error { return nil },
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("重载失败必须返回给调用方，实际: %v", err)
	}
}

func TestVerifyWrittenWrightstonesReturnsRecordMismatch(t *testing.T) {
	sentinel := errors.New("forced record mismatch")
	expected := []wrightstoneWriteExpectation{{ItemUnitID: WrightstoneSlotBaseID}}
	gen := NewWrightstoneGen()
	gen.loadSaveForVerification = func(string) (*SaveData, error) { return &SaveData{}, nil }

	_, err := gen.verifyWrittenWrightstones("output.dat", 1, expected,
		func(*SaveData, wrightstoneWriteExpectation) error { return sentinel },
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("任一记录回读不符必须返回给调用方，实际: %v", err)
	}
	if !strings.Contains(err.Error(), "第 1 个祝福") {
		t.Fatalf("错误应标明失败记录，实际: %v", err)
	}
}

func TestVerifyWrittenWrightstonesRejectsVerifiedCreatedMismatch(t *testing.T) {
	expected := []wrightstoneWriteExpectation{{ItemUnitID: WrightstoneSlotBaseID}}
	gen := NewWrightstoneGen()
	gen.loadSaveForVerification = func(string) (*SaveData, error) { return &SaveData{}, nil }

	_, err := gen.verifyWrittenWrightstones("output.dat", 2, expected,
		func(*SaveData, wrightstoneWriteExpectation) error { return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "已创建 2") || !strings.Contains(err.Error(), "已验证 1") {
		t.Fatalf("验证数量不符必须报错并带数量，实际: %v", err)
	}
}

func queuedWrightstoneGeneratorOnSaveCopy(t *testing.T) (*WrightstoneGen, string) {
	t.Helper()
	if !haveSave(testLoadoutSave) {
		t.Skipf("test save does not exist: %s", testLoadoutSave)
	}
	work := filepath.Join(t.TempDir(), "wrightstone-transaction.dat")
	raw, err := os.ReadFile(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(work, raw, 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewWrightstoneGen()
	if _, err := gen.LoadSaveFile(work); err != nil {
		t.Fatal(err)
	}
	wrightstones, err := gen.GetWrightstoneList()
	if err != nil || len(wrightstones) == 0 {
		t.Fatalf("load wrightstone catalog: %v (%d records)", err, len(wrightstones))
	}
	traits, err := gen.GetTraitList()
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]WrightstoneTraitInfo, len(traits))
	for _, trait := range traits {
		byID[trait.InternalID] = trait
	}
	pickLevel := func(levels []int, naturalMax int) int {
		for _, level := range levels {
			if level >= 1 && level <= naturalMax {
				return level
			}
		}
		return 0
	}

	var item WrightstoneQueueItem
	for _, wrightstone := range wrightstones {
		first, ok := byID[wrightstone.DefaultTraitID]
		if !ok {
			continue
		}
		firstLevel := pickLevel(first.AllowedLevels, 20)
		if firstLevel == 0 {
			continue
		}
		var second, third WrightstoneTraitInfo
		var secondLevel, thirdLevel int
		for _, trait := range traits {
			if trait.InternalID == first.InternalID {
				continue
			}
			if second.InternalID == "" {
				if level := pickLevel(trait.AllowedLevels, 15); level != 0 {
					second, secondLevel = trait, level
				}
				continue
			}
			if trait.InternalID != second.InternalID {
				if level := pickLevel(trait.AllowedLevels, 10); level != 0 {
					third, thirdLevel = trait, level
					break
				}
			}
		}
		if second.InternalID == "" || third.InternalID == "" {
			continue
		}
		item = WrightstoneQueueItem{
			WrightstoneID: wrightstone.InternalID,
			FirstTraitID:  first.InternalID,
			FirstLevel:    firstLevel,
			SecondTraitID: second.InternalID,
			SecondLevel:   secondLevel,
			ThirdTraitID:  third.InternalID,
			ThirdLevel:    thirdLevel,
			Quantity:      1,
		}
		break
	}
	if item.WrightstoneID == "" {
		t.Fatal("catalog has no writable wrightstone test item")
	}
	if err := gen.AddToQueue(item); err != nil {
		t.Fatal(err)
	}
	return gen, work
}

func assertSigilGeneratorState(t *testing.T, gen *SigilGen, wantData []byte, wantQueue []QueueItem) {
	t.Helper()
	if !bytes.Equal(gen.save.data, wantData) {
		t.Fatal("sigil generator mutated its in-memory save after a failed transaction")
	}
	if !reflect.DeepEqual(gen.queue, wantQueue) {
		t.Fatalf("sigil generator queue changed after a failed transaction: got %+v want %+v", gen.queue, wantQueue)
	}
}

func assertWrightstoneGeneratorState(t *testing.T, gen *WrightstoneGen, wantData []byte, wantQueue []WrightstoneQueueItem) {
	t.Helper()
	if !bytes.Equal(gen.save.data, wantData) {
		t.Fatal("wrightstone generator mutated its in-memory save after a failed transaction")
	}
	if !reflect.DeepEqual(gen.queue, wantQueue) {
		t.Fatalf("wrightstone generator queue changed after a failed transaction: got %+v want %+v", gen.queue, wantQueue)
	}
}

func TestSigilApplyQueueRestoresStateAfterWriteFailureAndRetryDoesNotDuplicate(t *testing.T) {
	gen, _, _ := queuedSigilGeneratorOnSaveCopy(t)
	wantData := append([]byte(nil), gen.save.data...)
	wantQueue := append([]QueueItem(nil), gen.queue...)
	beforeCount := gen.save.GetOccupiedGemCount()
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := gen.ApplyQueue(filepath.Join(blocker, "output.dat")); err == nil {
		t.Fatal("ApplyQueue unexpectedly succeeded through a non-directory output path")
	}
	assertSigilGeneratorState(t, gen, wantData, wantQueue)

	retryPath := filepath.Join(t.TempDir(), "retry.dat")
	result, err := gen.ApplyQueue(retryPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.CreatedCount != 1 || len(gen.GetQueue()) != 0 {
		t.Fatalf("retry result=%+v queue=%+v", result, gen.GetQueue())
	}
	written, err := LoadSave(retryPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := written.GetOccupiedGemCount(); got != beforeCount+1 {
		t.Fatalf("retry appended duplicate sigils: count=%d want=%d", got, beforeCount+1)
	}
}

func TestSigilApplyQueueRestoresStateAfterReadbackFailureAndRetryDoesNotDuplicate(t *testing.T) {
	gen, work, _ := queuedSigilGeneratorOnSaveCopy(t)
	wantData := append([]byte(nil), gen.save.data...)
	wantQueue := append([]QueueItem(nil), gen.queue...)
	beforeCount := gen.save.GetOccupiedGemCount()
	sentinel := errors.New("forced sigil readback failure")
	gen.loadSaveForVerification = func(string) (*SaveData, error) { return nil, sentinel }

	if _, err := gen.ApplyQueue(work); !errors.Is(err, sentinel) {
		t.Fatalf("ApplyQueue error=%v, want %v", err, sentinel)
	}
	assertSigilGeneratorState(t, gen, wantData, wantQueue)

	gen.loadSaveForVerification = LoadSave
	result, err := gen.ApplyQueue(work)
	if err != nil {
		t.Fatal(err)
	}
	if result.CreatedCount != 1 || len(gen.GetQueue()) != 0 {
		t.Fatalf("retry result=%+v queue=%+v", result, gen.GetQueue())
	}
	written, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	if got := written.GetOccupiedGemCount(); got != beforeCount+1 {
		t.Fatalf("same-path retry appended duplicate sigils: count=%d want=%d", got, beforeCount+1)
	}
}

func TestWrightstoneApplyQueueRestoresStateAfterWriteFailureAndRetryDoesNotDuplicate(t *testing.T) {
	gen, _ := queuedWrightstoneGeneratorOnSaveCopy(t)
	wantData := append([]byte(nil), gen.save.data...)
	wantQueue := append([]WrightstoneQueueItem(nil), gen.queue...)
	beforeCount := gen.save.GetOccupiedWrightstoneCount()
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := gen.ApplyQueue(filepath.Join(blocker, "output.dat")); err == nil {
		t.Fatal("ApplyQueue unexpectedly succeeded through a non-directory output path")
	}
	assertWrightstoneGeneratorState(t, gen, wantData, wantQueue)

	retryPath := filepath.Join(t.TempDir(), "retry.dat")
	result, err := gen.ApplyQueue(retryPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.CreatedCount != 1 || len(gen.GetQueue()) != 0 {
		t.Fatalf("retry result=%+v queue=%+v", result, gen.GetQueue())
	}
	written, err := LoadSave(retryPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := written.GetOccupiedWrightstoneCount(); got != beforeCount+1 {
		t.Fatalf("retry appended duplicate wrightstones: count=%d want=%d", got, beforeCount+1)
	}
}

func TestWrightstoneApplyQueueRestoresStateAfterReadbackFailureAndRetryDoesNotDuplicate(t *testing.T) {
	gen, work := queuedWrightstoneGeneratorOnSaveCopy(t)
	wantData := append([]byte(nil), gen.save.data...)
	wantQueue := append([]WrightstoneQueueItem(nil), gen.queue...)
	beforeCount := gen.save.GetOccupiedWrightstoneCount()
	sentinel := errors.New("forced wrightstone readback failure")
	gen.loadSaveForVerification = func(string) (*SaveData, error) { return nil, sentinel }

	if _, err := gen.ApplyQueue(work); !errors.Is(err, sentinel) {
		t.Fatalf("ApplyQueue error=%v, want %v", err, sentinel)
	}
	assertWrightstoneGeneratorState(t, gen, wantData, wantQueue)

	gen.loadSaveForVerification = LoadSave
	result, err := gen.ApplyQueue(work)
	if err != nil {
		t.Fatal(err)
	}
	if result.CreatedCount != 1 || len(gen.GetQueue()) != 0 {
		t.Fatalf("retry result=%+v queue=%+v", result, gen.GetQueue())
	}
	written, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	if got := written.GetOccupiedWrightstoneCount(); got != beforeCount+1 {
		t.Fatalf("same-path retry appended duplicate wrightstones: count=%d want=%d", got, beforeCount+1)
	}
}

func TestSigilGeneratorSerializesQueueMutationWithApply(t *testing.T) {
	gen, _, _ := queuedSigilGeneratorOnSaveCopy(t)
	enteredVerification := make(chan struct{})
	allowVerification := make(chan struct{})
	gen.loadSaveForVerification = func(path string) (*SaveData, error) {
		close(enteredVerification)
		<-allowVerification
		return LoadSave(path)
	}
	applyDone := make(chan error, 1)
	go func() {
		_, err := gen.ApplyQueue(filepath.Join(t.TempDir(), "serialized-sigil.dat"))
		applyDone <- err
	}()
	<-enteredVerification

	clearDone := make(chan struct{})
	go func() {
		gen.ClearQueue()
		close(clearDone)
	}()
	select {
	case <-clearDone:
		t.Fatal("ClearQueue ran concurrently with ApplyQueue")
	case <-time.After(75 * time.Millisecond):
	}
	close(allowVerification)
	if err := <-applyDone; err != nil {
		t.Fatal(err)
	}
	select {
	case <-clearDone:
	case <-time.After(time.Second):
		t.Fatal("ClearQueue remained blocked after ApplyQueue completed")
	}
}

func TestWrightstoneGeneratorSerializesQueueMutationWithApply(t *testing.T) {
	gen, _ := queuedWrightstoneGeneratorOnSaveCopy(t)
	enteredVerification := make(chan struct{})
	allowVerification := make(chan struct{})
	gen.loadSaveForVerification = func(path string) (*SaveData, error) {
		close(enteredVerification)
		<-allowVerification
		return LoadSave(path)
	}
	applyDone := make(chan error, 1)
	go func() {
		_, err := gen.ApplyQueue(filepath.Join(t.TempDir(), "serialized-wrightstone.dat"))
		applyDone <- err
	}()
	<-enteredVerification

	clearDone := make(chan struct{})
	go func() {
		gen.ClearQueue()
		close(clearDone)
	}()
	select {
	case <-clearDone:
		t.Fatal("ClearQueue ran concurrently with ApplyQueue")
	case <-time.After(75 * time.Millisecond):
	}
	close(allowVerification)
	if err := <-applyDone; err != nil {
		t.Fatal(err)
	}
	select {
	case <-clearDone:
	case <-time.After(time.Second):
		t.Fatal("ClearQueue remained blocked after ApplyQueue completed")
	}
}

func TestGeneratorQueueSnapshotsDoNotExposeMutableBackingStorage(t *testing.T) {
	sigil, _, _ := queuedSigilGeneratorOnSaveCopy(t)
	sigilQueue := sigil.GetQueue()
	sigilQueue[0].Quantity = 999
	if sigil.queue[0].Quantity == 999 {
		t.Fatal("SigilGen.GetQueue exposed its mutable backing slice")
	}

	wrightstone, _ := queuedWrightstoneGeneratorOnSaveCopy(t)
	wrightstoneQueue := wrightstone.GetQueue()
	wrightstoneQueue[0].Quantity = 999
	if wrightstone.queue[0].Quantity == 999 {
		t.Fatal("WrightstoneGen.GetQueue exposed its mutable backing slice")
	}
}

func TestGeneratorsRejectQuantityAboveUIAndWriteLimit(t *testing.T) {
	t.Run("sigil", func(t *testing.T) {
		gen, _, _ := queuedSigilGeneratorOnSaveCopy(t)
		item := gen.GetQueue()[0]
		gen.ClearQueue()
		item.Quantity = 1000
		if err := gen.AddToQueue(item); err == nil || !strings.Contains(err.Error(), "999") {
			t.Fatalf("quantity 1000 should be rejected with the shared limit, got %v", err)
		}
		if queue := gen.GetQueue(); len(queue) != 0 {
			t.Fatalf("rejected quantity must not enter or expand the queue: %+v", queue)
		}
	})

	t.Run("wrightstone", func(t *testing.T) {
		gen, _ := queuedWrightstoneGeneratorOnSaveCopy(t)
		item := gen.GetQueue()[0]
		gen.ClearQueue()
		item.Quantity = 1000
		if err := gen.AddToQueue(item); err == nil || !strings.Contains(err.Error(), "999") {
			t.Fatalf("quantity 1000 should be rejected with the shared limit, got %v", err)
		}
		if queue := gen.GetQueue(); len(queue) != 0 {
			t.Fatalf("rejected quantity must not enter or expand the queue: %+v", queue)
		}
	})
}
