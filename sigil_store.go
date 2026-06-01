package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cespare/xxhash/v2"
)

const (
	HashSeedIDType     uint32 = 1003
	TraitHashIDType    uint32 = 1701
	TraitLevelIDType   uint32 = 1702
	GemMaxSlotIDType   uint32 = 2701
	GemSlotIDType      uint32 = 2702
	GemIDType          uint32 = 2703
	GemLevelIDType     uint32 = 2704
	GemWornByIDType    uint32 = 2706
	GemFlagsIDType     uint32 = 2707
	EmptyHash          uint32 = 0x887AE0B0
	NormalSigilFlags   uint32 = 2
	SaveHashSeed       uint64 = 0x2F1A43EBCD
	GemSlotBaseID             = 30000
	TraitSlotBase             = 120000000
)

var hashSections = []struct{ start, subSize int }{
	{0x58, 0x80}, {0x30, 0xA0}, {0x28, 0x30}, {0x38, 0xC0}, {0x40, 0xB0},
	{0x68, 0x50}, {0x48, 0x60}, {0x70, 0x90}, {0x50, 0x40}, {0x60, 0x70},
}

// unitEntry holds the position and value info for one FlatBuffer unit entry.
type unitEntry struct {
	IDType    uint32
	UnitID    uint32
	ValueOff  int // absolute offset in data where ValueData[0] lives
	ValueCnt  int // number of elements in ValueData vector
	data      []byte
}

func (e *unitEntry) Uint32() uint32 {
	if e.ValueOff < 0 || e.ValueOff+4 > len(e.data) {
		return 0
	}
	return binary.LittleEndian.Uint32(e.data[e.ValueOff:])
}

func (e *unitEntry) Int32() int32 {
	if e.ValueOff < 0 || e.ValueOff+4 > len(e.data) {
		return 0
	}
	return int32(binary.LittleEndian.Uint32(e.data[e.ValueOff:]))
}

func (e *unitEntry) SetUint32(v uint32) {
	binary.LittleEndian.PutUint32(e.data[e.ValueOff:], v)
}

func (e *unitEntry) SetInt32(v int32) {
	binary.LittleEndian.PutUint32(e.data[e.ValueOff:], uint32(v))
}

type SaveData struct {
	data    []byte
	slotOff int64
	slotLen int64
	path    string
}

func LoadSave(path string) (*SaveData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取存档失败: %w", err)
	}
	if len(data) < 0x34 {
		return nil, fmt.Errorf("存档文件太小")
	}

	slotOff := int64(binary.LittleEndian.Uint64(data[0x1C:0x24]))
	slotLen := int64(binary.LittleEndian.Uint64(data[0x2C:0x34]))
	if slotOff < 0 || slotLen <= 0 || slotOff+slotLen > int64(len(data)) {
		return nil, fmt.Errorf("存档头 slot-data 偏移无效")
	}

	return &SaveData{data: data, slotOff: slotOff, slotLen: slotLen, path: path}, nil
}

func (s *SaveData) slotSpan() []byte {
	return s.data[s.slotOff : s.slotOff+s.slotLen]
}

// findUnit finds a single FlatBuffer unit entry by IDType + UnitID.
func (s *SaveData) findUnit(idType, unitID uint32) (*unitEntry, bool) {
	slot := s.slotSpan()
	slotBase := int(s.slotOff)

	for _, step := range []int{4, 1} {
		for off := 4; off < len(slot)-16; off += step {
			entry, ok := tryReadUnitEntry(slot, off, idType, unitID)
			if !ok {
				continue
			}
			entry.ValueOff += slotBase
			entry.data = s.data
			return entry, true
		}
	}
	return nil, false
}

// findAllUnitsByType finds all FlatBuffer unit entries matching a specific IDType,
// optionally filtered by UnitID >= minUnitID.
func (s *SaveData) findAllUnitsByType(idType uint32) []*unitEntry {
	slot := s.slotSpan()
	slotBase := int(s.slotOff)
	var results []*unitEntry

	for off := 4; off < len(slot)-16; off += 4 {
		entry, ok := tryReadUnitEntry(slot, off, idType, 0)
		if !ok || entry.IDType != idType {
			continue
		}
		entry.ValueOff += slotBase
		entry.data = s.data
		results = append(results, entry)
	}
	return results
}

// tryReadUnitEntry attempts to read a FlatBuffer UIntSaveDataUnit/IntSaveDataUnit at off.
// If unitID is non-zero, also filters by UnitID. Returns the entry and whether it matched.
func tryReadUnitEntry(slot []byte, off int, idType, unitID uint32) (*unitEntry, bool) {
	vtableDist := int32(binary.LittleEndian.Uint32(slot[off:]))
	if vtableDist == 0 {
		return nil, false
	}

	candidates := []int{off - int(vtableDist), off + int(vtableDist)}
	for _, vtOff := range candidates {
		if vtOff < 0 || vtOff > len(slot)-10 {
			continue
		}
		vtableSize := binary.LittleEndian.Uint16(slot[vtOff:])
		objectSize := binary.LittleEndian.Uint16(slot[vtOff+2:])
		if vtableSize < 10 || objectSize < 4 || int(vtableSize) > 64 || int(objectSize) > len(slot)-off {
			continue
		}

		idField := binary.LittleEndian.Uint16(slot[vtOff+4:])
		dataField := binary.LittleEndian.Uint16(slot[vtOff+8:])
		if idField == 0 || dataField == 0 {
			continue
		}
		if int(idField) > len(slot)-off-4 || int(dataField) > len(slot)-off-4 {
			continue
		}

		foundID := binary.LittleEndian.Uint32(slot[off+int(idField):])
		if foundID != idType {
			continue
		}

		// UnitID field is optional — check if it exists (vtable offset != 0)
		var foundUnitID uint32
		unitField := binary.LittleEndian.Uint16(slot[vtOff+6:])
		if unitField != 0 {
			if int(unitField) > len(slot)-off-4 {
				continue
			}
			foundUnitID = binary.LittleEndian.Uint32(slot[off+int(unitField):])
		}
		// If filtering by a specific unitID, check match
		if unitID != 0 && foundUnitID != unitID {
			continue
		}

		vectorFieldOff := off + int(dataField)
		relVectorOff := binary.LittleEndian.Uint32(slot[vectorFieldOff:])
		vectorOff := vectorFieldOff + int(relVectorOff)
		if vectorOff < 0 || vectorOff > len(slot)-8 {
			continue
		}
		count := int32(binary.LittleEndian.Uint32(slot[vectorOff:]))
		if count <= 0 {
			continue
		}

		return &unitEntry{
			IDType:   foundID,
			UnitID:   foundUnitID,
			ValueOff: vectorOff + 4,
			ValueCnt: int(count),
		}, true
	}
	return nil, false
}

// GetMaxSlotID returns the current max sigil slot ID.
func (s *SaveData) GetMaxSlotID() (int, error) {
	entry, ok := s.findUnit(GemMaxSlotIDType, 0)
	if !ok {
		return 0, fmt.Errorf("找不到 GEMDATA_MAX_SLOT_ID (2701)")
	}
	return int(entry.Uint32()), nil
}

// SetMaxSlotID writes a new max sigil slot ID.
func (s *SaveData) SetMaxSlotID(id int) error {
	entry, ok := s.findUnit(GemMaxSlotIDType, 0)
	if !ok {
		return fmt.Errorf("找不到 GEMDATA_MAX_SLOT_ID (2701)")
	}
	entry.SetUint32(uint32(id))
	return nil
}

// FindEmptyGemSlots returns up to `count` empty sigil slot unit IDs.
// An empty slot has hash == EmptyHash (0x887AE0B0).
func (s *SaveData) FindEmptyGemSlots(count int) ([]int, error) {
	allGemUnits := s.findAllUnitsByType(GemIDType)
	var emptyIDs []int
	for _, u := range allGemUnits {
		if int(u.UnitID) >= GemSlotBaseID && u.Uint32() == EmptyHash {
			emptyIDs = append(emptyIDs, int(u.UnitID))
			if len(emptyIDs) >= count {
				break
			}
		}
	}
	if len(emptyIDs) < count {
		return nil, fmt.Errorf("空因子槽不足 (需要 %d, 找到 %d)", count, len(emptyIDs))
	}
	return emptyIDs, nil
}

// GetOccupiedGemCount returns the number of non-empty sigil slots.
func (s *SaveData) GetOccupiedGemCount() int {
	allGemUnits := s.findAllUnitsByType(GemIDType)
	count := 0
	for _, u := range allGemUnits {
		if int(u.UnitID) >= GemSlotBaseID && u.Uint32() != EmptyHash {
			count++
		}
	}
	return count
}

// PatchSigil writes a complete sigil into a slot, replacing whatever was there.
func (s *SaveData) PatchSigil(gemUnitID, newSlotID int, sigilHash uint32, level int,
	primaryTraitHash uint32, primaryLevel int,
	secondaryTraitHash uint32, secondaryLevel int, hasSecondary bool) error {

	gemIndex := gemUnitID - GemSlotBaseID
	primaryTraitUnit := TraitSlotBase + (gemIndex * 100)
	secondaryTraitUnit := primaryTraitUnit + 1

	// --- Gem slot fields ---
	must(s.patchUint(GemSlotIDType, uint32(gemUnitID), uint32(newSlotID)))
	must(s.patchUint(GemIDType, uint32(gemUnitID), sigilHash))
	must(s.patchInt(GemLevelIDType, uint32(gemUnitID), level))
	must(s.patchUint(GemWornByIDType, uint32(gemUnitID), EmptyHash))
	must(s.patchUint(GemFlagsIDType, uint32(gemUnitID), NormalSigilFlags))

	// --- Trait fields ---
	must(s.patchUint(TraitHashIDType, uint32(primaryTraitUnit), primaryTraitHash))
	must(s.patchInt(TraitLevelIDType, uint32(primaryTraitUnit), primaryLevel))

	if hasSecondary {
		must(s.patchUint(TraitHashIDType, uint32(secondaryTraitUnit), secondaryTraitHash))
		must(s.patchInt(TraitLevelIDType, uint32(secondaryTraitUnit), secondaryLevel))
	}
	return nil
}

// ClearSigil zeroes out a sigil slot.
func (s *SaveData) ClearSigil(gemUnitID int) error {
	gemIndex := gemUnitID - GemSlotBaseID
	primaryTraitUnit := TraitSlotBase + (gemIndex * 100)
	secondaryTraitUnit := primaryTraitUnit + 1

	must(s.patchUint(GemIDType, uint32(gemUnitID), EmptyHash))
	must(s.patchInt(GemLevelIDType, uint32(gemUnitID), 0))
	must(s.patchUint(GemWornByIDType, uint32(gemUnitID), EmptyHash))
	must(s.patchUint(GemFlagsIDType, uint32(gemUnitID), 0))
	must(s.patchUint(TraitHashIDType, uint32(primaryTraitUnit), EmptyHash))
	must(s.patchInt(TraitLevelIDType, uint32(primaryTraitUnit), 0))
	must(s.patchUint(TraitHashIDType, uint32(secondaryTraitUnit), EmptyHash))
	must(s.patchInt(TraitLevelIDType, uint32(secondaryTraitUnit), 0))
	return nil
}

func (s *SaveData) patchUint(idType, unitID, value uint32) error {
	entry, ok := s.findUnit(idType, unitID)
	if !ok {
		return fmt.Errorf("找不到 save unit: IDType=%d, UnitID=%d", idType, unitID)
	}
	entry.SetUint32(value)
	return nil
}

func (s *SaveData) patchInt(idType, unitID uint32, value int) error {
	entry, ok := s.findUnit(idType, unitID)
	if !ok {
		return fmt.Errorf("找不到 save unit: IDType=%d, UnitID=%d", idType, unitID)
	}
	entry.SetInt32(int32(value))
	return nil
}

// FixChecksums recomputes XXHash64 for the hash-protected sections.
func (s *SaveData) FixChecksums() error {
	slot := s.slotSpan()

	// Read hash seed (IDType 1003)
	seedEntry, ok := s.findUnit(HashSeedIDType, 0)
	if !ok {
		return fmt.Errorf("找不到 SAVEDATA_HASHSEED (1003)")
	}
	idx := int(seedEntry.Uint32() % uint32(len(hashSections)))

	// Hash table offset is stored at (slotLen - 0x14)
	if int(s.slotLen) < 0x14 {
		return fmt.Errorf("slot data 太小，无 hash table")
	}
	hashesOff := int(binary.LittleEndian.Uint32(slot[s.slotLen-0x14:]))
	if hashesOff+(len(hashSections)*8) > int(s.slotLen) {
		return fmt.Errorf("hash table 偏移超出 slot data 范围")
	}

	section := hashSections[idx]
	hashStart := section.start
	hashLen := hashesOff - (section.start + section.subSize)
	if hashLen <= 0 || hashStart+hashLen > len(slot) {
		return fmt.Errorf("hash 区间无效")
	}

	d := xxhash.NewWithSeed(SaveHashSeed)
	d.Write(slot[hashStart : hashStart+hashLen])
	hash := d.Sum64()
	binary.LittleEndian.PutUint64(slot[hashesOff+idx*8:], hash)
	return nil
}

// Write saves the modified data to a new file path.
func (s *SaveData) Write(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}
	return os.WriteFile(path, s.data, 0644)
}

// VerifySigil re-reads a sigil slot and checks all fields match expected values.
func (s *SaveData) VerifySigil(gemUnitID int, sigilHash uint32, level int,
	primaryHash uint32, primaryLevel int,
	secondaryHash uint32, secondaryLevel int, hasSecondary bool) error {

	gemIndex := gemUnitID - GemSlotBaseID
	primaryTraitUnit := TraitSlotBase + (gemIndex * 100)
	secondaryTraitUnit := primaryTraitUnit + 1

	check := func(idType, unitID, expected uint32, label string) error {
		entry, ok := s.findUnit(idType, unitID)
		if !ok {
			return fmt.Errorf("验证失败: 找不到 %s", label)
		}
		actual := entry.Uint32()
		if actual != expected {
			return fmt.Errorf("验证失败 %s: 期望 0x%08X, 实际 0x%08X", label, expected, actual)
		}
		return nil
	}
	checkInt := func(idType, unitID uint32, expected int, label string) error {
		entry, ok := s.findUnit(idType, unitID)
		if !ok {
			return fmt.Errorf("验证失败: 找不到 %s", label)
		}
		actual := entry.Int32()
		if int(actual) != expected {
			return fmt.Errorf("验证失败 %s: 期望 %d, 实际 %d", label, expected, actual)
		}
		return nil
	}

	if err := check(GemIDType, uint32(gemUnitID), sigilHash, "因子哈希"); err != nil {
		return err
	}
	if err := checkInt(GemLevelIDType, uint32(gemUnitID), level, "因子等级"); err != nil {
		return err
	}
	if err := check(GemWornByIDType, uint32(gemUnitID), EmptyHash, "装备角色"); err != nil {
		return err
	}
	if err := check(GemFlagsIDType, uint32(gemUnitID), NormalSigilFlags, "因子标记"); err != nil {
		return err
	}
	if err := check(TraitHashIDType, uint32(primaryTraitUnit), primaryHash, "主特性哈希"); err != nil {
		return err
	}
	if err := checkInt(TraitLevelIDType, uint32(primaryTraitUnit), primaryLevel, "主特性等级"); err != nil {
		return err
	}
	if hasSecondary {
		if err := check(TraitHashIDType, uint32(secondaryTraitUnit), secondaryHash, "副特性哈希"); err != nil {
			return err
		}
		if err := checkInt(TraitLevelIDType, uint32(secondaryTraitUnit), secondaryLevel, "副特性等级"); err != nil {
			return err
		}
	}
	return nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// tryPatchUint attempts to write a uint value, returning nil if the entry is missing.
func (s *SaveData) tryPatchUint(idType, unitID, value uint32) error {
	entry, ok := s.findUnit(idType, unitID)
	if !ok {
		return nil // missing entry is non-fatal for bulk operations
	}
	entry.SetUint32(value)
	return nil
}

// tryPatchInt attempts to write an int value, returning nil if the entry is missing.
func (s *SaveData) tryPatchInt(idType, unitID uint32, value int) error {
	entry, ok := s.findUnit(idType, unitID)
	if !ok {
		return nil
	}
	entry.SetInt32(int32(value))
	return nil
}
