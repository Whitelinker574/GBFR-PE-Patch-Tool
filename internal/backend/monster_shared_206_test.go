package backend

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func sharedMonsterNativeBytes(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "src_dll", "patch_core", "monster_shared_206_templates.h"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?s)static const lm_byte_t ` + regexp.QuoteMeta(name) + `\[\] = \{(.*?)\};`).FindSubmatch(raw)
	if len(match) != 2 {
		t.Fatalf("missing native template %s", name)
	}
	body := regexp.MustCompile(`//[^\r\n]*`).ReplaceAllString(string(match[1]), "")
	var code []byte
	for _, part := range strings.Split(body, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		value, err := strconv.ParseUint(part, 0, 8)
		if err != nil {
			t.Fatal(err)
		}
		code = append(code, byte(value))
	}
	return code
}

func TestGame206SharedMonsterContextsAndNativeReplayAgree(t *testing.T) {
	_, sections := loadLocalGame206Sections(t)
	if !bytes.Equal(sharedMonsterNativeBytes(t, "kMonsterHpShared206Tail"), monsterHPShared206Tail) {
		t.Fatal("Go/native HP tail differs")
	}
	if !bytes.Equal(runtimePatchLocalBytesAtRVA(sections, uint32(monsterODShared206RVA-6), 6), monsterODShared206Prefix) {
		t.Fatal("OD enable branch changed")
	}
	for _, row := range []struct {
		name           string
		rva            uintptr
		original, tail []byte
	}{
		{"kMonsterHpShared206Original", monsterHPShared206RVA, monsterHPShared206Original, monsterHPShared206Tail},
		{"kMonsterOdShared206Original", monsterODShared206RVA, monsterODShared206Original, odInline206Sites[0].Tail},
	} {
		if !bytes.Equal(sharedMonsterNativeBytes(t, row.name), row.original) {
			t.Fatal("Go/native replay differs", row.name)
		}
		got := runtimePatchLocalBytesAtRVA(sections, uint32(row.rva), len(row.original)+len(row.tail))
		want := append(append([]byte(nil), row.original...), row.tail...)
		if !bytes.Equal(got, want) {
			t.Fatalf("shared context changed at %X", row.rva)
		}
		matches := findRuntimePatchLocalPatternMatches(sections, exactRuntimePatchPattern(want))
		if len(matches) != 1 || uintptr(matches[0].rva) != row.rva {
			t.Fatal("shared context is not unique")
		}
	}
}

func TestSharedMonster206LiveLifecycle(t *testing.T) {
	if os.Getenv("GBFR_LIVE_SHARED_MONSTER_206") != "1" {
		t.Skip("set GBFR_LIVE_SHARED_MONSTER_206=1 for live owned HP/OD install and restore")
	}
	app := NewApp()
	info, err := app.CharaAcquire(2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.CharaRelease(info.OwnerToken) })
	for _, id := range []string{"monster_hp", "od_rate"} {
		t.Run(id, func(t *testing.T) {
			if _, err := app.MonsterEnhanceSetPatchValueEnabledOwned(info.OwnerToken, id, true, 2); err != nil {
				t.Fatal(err)
			}
			record, ok := app.monsterEnhanceOwned[id]
			if !ok || record.AuxTarget == 0 {
				t.Fatal("shared lifecycle missing auxiliary ownership")
			}
			if id == "od_rate" && len(record.ExtraAux) != 2 {
				t.Fatalf("OD requires three auxiliary entries: %+v", record)
			}
			if _, err := app.MonsterEnhanceSetPatchValueEnabledOwned(info.OwnerToken, id, false, 2); err != nil {
				t.Fatal(err)
			}
			if len(app.monsterEnhanceOwned) != 0 {
				t.Fatal("restoration left owned entries")
			}
			current, err := app.readMonsterEnhanceEntry(record.Target, len(record.Original))
			if err != nil || !bytes.Equal(current, record.Original) {
				t.Fatal("main original not restored", err)
			}
			current, err = app.readMonsterEnhanceEntry(record.AuxTarget, len(record.AuxOriginal))
			if err != nil || !bytes.Equal(current, record.AuxOriginal) {
				t.Fatal("auxiliary original not restored", err)
			}
			for _, extra := range record.ExtraAux {
				current, err = app.readMonsterEnhanceEntry(extra.Target, len(extra.Original))
				if err != nil || !bytes.Equal(current, extra.Original) {
					t.Fatal("extra original not restored", err)
				}
			}
		})
	}
}

// Execute the exact templates compiled into patch_core, in private scratch
// memory in this test process. No game process, save, or network is involved.
func executeSharedMonsterTemplate(t *testing.T, od bool, current uint64, delta int64, scale float64, enabled bool, minimumOne bool) uint64 {
	t.Helper()
	h := windows.CurrentProcess()
	page, err := virtualAllocRemote(h, 0x4000, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		t.Fatal(err)
	}
	defer virtualFreeRemote(h, page)
	entry, cave, data := page, page+0x800, page+0x2000
	const sentinel = uint64(0x1122334455667788)
	// Save all nonvolatile registers used by the harness, including XMM6.
	wrapper := []byte{0x53, 0x56, 0x57, 0x48, 0x83, 0xEC, 0x10, 0x0F, 0x11, 0x34, 0x24,
		0x48, 0x89, 0xC8, 0x89, 0xD3, 0x48, 0x89, 0xD6, 0x44, 0x89, 0xC7, 0x49, 0xBA}
	var immediate [8]byte
	binary.LittleEndian.PutUint64(immediate[:], sentinel)
	wrapper = append(wrapper, immediate[:]...)
	wrapper = append(wrapper, 0x66, 0x49, 0x0F, 0x6E, 0xC2, 0x66, 0x49, 0x0F, 0x6E, 0xF2)
	if od {
		wrapper = append(wrapper, 0x80, 0x78, 0x50, 0, 0x74, 0x05)
	}
	jump, err := makeRelJump(entry+uintptr(len(wrapper)), cave, 5)
	if err != nil {
		t.Fatal(err)
	}
	wrapper = append(wrapper, jump...)
	continuation := entry + uintptr(len(wrapper))
	if !od {
		wrapper = append(wrapper, monsterHPShared206Tail...)
	}
	wrapper = append(wrapper, 0x0F, 0x11, 0x40, 0x20, 0x0F, 0x11, 0x70, 0x30, 0x48, 0x89, 0x58, 0x40)
	if od {
		wrapper = append(wrapper, 0x48, 0x8B, 0x40, 0x18)
	} else {
		wrapper = append(wrapper, 0x48, 0x8B, 0x40, 0x10)
	}
	wrapper = append(wrapper, 0x0F, 0x10, 0x34, 0x24, 0x48, 0x83, 0xC4, 0x10, 0x5F, 0x5E, 0x5B, 0xC3)
	name := "kMonsterHpShared206Code"
	if od {
		name = "kMonsterOdShared206Code"
	}
	code := sharedMonsterNativeBytes(t, name)
	if len(code) < 8 || code[len(code)-5] != 0xE9 {
		t.Fatal("template return jump missing")
	}
	scaleAt := bytes.Index(code, []byte{0xF2, 0x0F, 0x59, 0x05}) + 4
	if scaleAt < 4 {
		t.Fatal("template scale displacement missing")
	}
	binary.LittleEndian.PutUint32(code[scaleAt:scaleAt+4], uint32(len(code)-(scaleAt+4)))
	back := int64(continuation) - int64(cave+uintptr(len(code)))
	if back < math.MinInt32 || back > math.MaxInt32 {
		t.Fatal("test return out of range")
	}
	binary.LittleEndian.PutUint32(code[len(code)-4:], uint32(int32(back)))
	binary.LittleEndian.PutUint64(immediate[:], math.Float64bits(scale))
	code = append(code, immediate[:]...)
	if err := writeCodeMemory(h, entry, wrapper); err != nil {
		t.Fatal(err)
	}
	if err := writeCodeMemory(h, cave, code); err != nil {
		t.Fatal(err)
	}
	state := make([]byte, 0x60)
	offset := 16
	if od {
		offset = 24
	}
	binary.LittleEndian.PutUint64(state[offset:], current)
	if enabled {
		state[0x50] = 1
	}
	if err := writeProcessMemory(h, data, unsafe.Pointer(&state[0]), uintptr(len(state))); err != nil {
		t.Fatal(err)
	}
	flag := uintptr(0)
	if minimumOne {
		flag = 1
	}
	result, _, _ := syscall.SyscallN(entry, data, uintptr(delta), flag)
	if err := readProcessMemory(h, data, unsafe.Pointer(&state[0]), uintptr(len(state))); err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint64(state[0x20:]) != sentinel || binary.LittleEndian.Uint64(state[0x30:]) != sentinel {
		t.Fatal("XMM0/XMM6 were clobbered")
	}
	if binary.LittleEndian.Uint64(state[0x40:]) != uint64(uint32(delta)) {
		t.Fatal("RBX damage input was clobbered")
	}
	if uint64(result) != binary.LittleEndian.Uint64(state[offset:]) {
		t.Fatal("return and stored result disagree")
	}
	return uint64(result)
}

func TestSharedMonsterHpNativeTemplatePreservesUnsignedDamageAndRegisters(t *testing.T) {
	for _, tc := range []struct {
		current uint64
		damage  uint32
		scale   float64
		minimum bool
	}{
		{100, 20, .5, false}, {100, 0, 2, false}, {100, 1, .01, false}, {100, 200, 1, false}, {100, 200, 1, true}, {100, 100, 1, false}, {100, 100, 1, true},
		{5000000000, 0xF0000000, 1, false}, {5000000000, 0xFFFFFFFF, .5, false}, {math.MaxUint64, 30, 2, false},
		{100000000000, 0xF0000000, 8, false},
	} {
		scaled := uint64(float64(tc.damage) * tc.scale)
		if tc.damage > 0 && scaled == 0 {
			scaled = 1
		}
		want := uint64(0)
		if tc.current >= scaled {
			want = tc.current - scaled
		}
		if tc.minimum && want < 1 {
			want = 1
		}
		if got := executeSharedMonsterTemplate(t, false, tc.current, int64(tc.damage), tc.scale, true, tc.minimum); got != want {
			t.Fatalf("HP %v => %d want %d", tc, got, want)
		}
	}
}

func TestSharedMonsterOdNativeTemplateRetainsEnableAndUnsignedCarrySemantics(t *testing.T) {
	for _, tc := range []struct {
		current uint64
		delta   int64
		scale   float64
		enabled bool
	}{
		{10, 5, 2, true}, {100, 5, .5, true}, {10, -3, 1, true}, {0, -5, .5, true}, {math.MaxUint64 - 1, 10, 2, true}, {10, 5, 2, false}, {10, 0, 2, true}, {10, 1, .0001, true},
	} {
		want := tc.current
		if tc.enabled {
			sum, carry := bits.Add64(tc.current, uint64(int64(float64(tc.delta)*tc.scale)), 0)
			want = sum
			if carry != 0 {
				want = math.MaxUint64
			}
		}
		if got := executeSharedMonsterTemplate(t, true, tc.current, tc.delta, tc.scale, tc.enabled, false); got != want {
			t.Fatalf("OD %v => %d want %d", tc, got, want)
		}
	}
}
