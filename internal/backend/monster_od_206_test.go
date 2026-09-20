package backend

import (
	"bytes"
	"fmt"
	"golang.org/x/sys/windows"
	"testing"
	"unsafe"
)

func TestMonsterOdMultipleAuxiliaryRestoreAndPartialRollback(t *testing.T) {
	for _, mask := range []int{7, 3, 6} {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			h := windows.CurrentProcess()
			page, err := virtualAllocRemote(h, 0x4000, windows.PAGE_EXECUTE_READWRITE)
			if err != nil {
				t.Fatal(err)
			}
			defer virtualFreeRemote(h, page)
			point := findMonsterPatchPoint("od_rate")
			app := &App{hProcess: h, moduleBase: page - point.RVA}
			original := [][]byte{point.Original, odRateInlineOriginal, odRateInlineOriginal}
			targets := []uintptr{page, page + 0x100, page + 0x200}
			caves := []uintptr{page + 0x800, page + 0x1000, page + 0x1800}
			sizes := []uintptr{96, 128, 128}
			patches := make([][]byte, 3)
			for i := range targets {
				patches[i], err = makeRelJump(targets[i], caves[i], len(original[i]))
				if err != nil {
					t.Fatal(err)
				}
				value := original[i]
				if mask&(1<<i) != 0 {
					value = patches[i]
				}
				if err := writeCodeMemory(h, targets[i], value); err != nil {
					t.Fatal(err)
				}
				address := monsterEnhanceCaveMarkerAddress(caves[i], sizes[i])
				if err := writeProcessMemory(h, address, unsafe.Pointer(&monsterEnhanceCaveMarker[0]), uintptr(len(monsterEnhanceCaveMarker))); err != nil {
					t.Fatal(err)
				}
			}
			aux := &monsterEnhanceAuxPreflight{Target: targets[1], Original: original[1], CaveSize: 128, Additional: []monsterEnhanceAuxPreflight{{Target: targets[2], Original: original[2], CaveSize: 128}}}
			if mask == 7 {
				if err := app.claimMonsterEnhancePatchWithAux("test", point, original[0], aux); err != nil {
					t.Fatal(err)
				}
				// A changed secondary entry must retain the recovery record until retry.
				damaged := append([]byte(nil), patches[2]...)
				damaged[1] ^= 1
				if err := writeCodeMemory(h, targets[2], damaged); err != nil {
					t.Fatal(err)
				}
				if err := app.restoreMonsterEnhanceOwned("test", point.ID, false); err == nil {
					t.Fatal("foreign auxiliary change was overwritten")
				}
				if len(app.monsterEnhanceOwned) != 1 {
					t.Fatal("failed recovery state was lost")
				}
				if err := writeCodeMemory(h, targets[2], patches[2]); err != nil {
					t.Fatal(err)
				}
				if err := app.restoreMonsterEnhanceOwned("test", point.ID, false); err != nil {
					t.Fatal(err)
				}
			} else if err := app.rollbackMonsterEnhanceFailedEnableWithAux("test", point, original[0], aux); err != nil {
				t.Fatal(err)
			}
			for i, target := range targets {
				got, err := app.readMonsterEnhanceEntry(target, len(original[i]))
				if err != nil || !bytes.Equal(got, original[i]) {
					t.Fatalf("entry %d not restored: %X %v", i, got, err)
				}
			}
			if len(app.monsterEnhanceOwned) != 0 {
				t.Fatal("restoration left owned records")
			}
		})
	}
}

func TestGame206BothInlineOdTails(t *testing.T) {
	_, sections := loadLocalGame206Sections(t)
	for _, site := range odInline206Sites {
		expected := append(append([]byte(nil), odRateInlineOriginal...), site.Tail...)
		pattern := exactRuntimePatchPattern(expected)
		matches := findRuntimePatchLocalPatternMatches(sections, pattern)
		if len(matches) != 1 || uintptr(matches[0].rva) != site.RVA {
			t.Fatalf("OD full epilogue not unique at %X: %s", site.RVA, formatRuntimePatchLocalMatchLocations(matches))
		}
	}
}
