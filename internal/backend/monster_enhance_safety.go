package backend

import (
	"encoding/binary"
	"fmt"
	"sort"
	"unsafe"
)

var monsterEnhanceCaveMarker = []byte{'G', 'B', 'F', 'R', 'M', 'H', '0', '3'}

const monsterEnhanceCrocodileAuxRVA = uintptr(0x23FD463)

var (
	monsterEnhanceCrocodileAuxOriginal = []byte{0x83, 0xF8, 0x02, 0xBA, 0x01, 0x00, 0x00, 0x00, 0x0F, 0x4D, 0xD0}
	monsterEnhanceCrocodileAuxPatched  = []byte{0x31, 0xD2, 0x85, 0xC0, 0x0F, 0x4F, 0xD0, 0x90, 0x90, 0x90, 0x90}
)

type monsterEnhanceOwnedPatch struct {
	OwnerToken  string
	Target      uintptr
	Original    []byte
	Patched     []byte
	Cave        uintptr
	CaveSize    uintptr
	AuxTarget   uintptr
	AuxOriginal []byte
	AuxPatched  []byte
}

func monsterEnhanceCaveSize(id string) uintptr {
	switch id {
	case "crocodile_damage", "overdrive_state":
		return 256
	case "inventory_set_45":
		return 32
	case "monster_hp", "monster_damage", "monster_stun":
		return 128
	default:
		return 0
	}
}

func monsterEnhanceCaveMarkerAddress(cave, caveSize uintptr) uintptr {
	if cave == 0 || caveSize < uintptr(len(monsterEnhanceCaveMarker)) {
		return 0
	}
	return cave + caveSize - uintptr(len(monsterEnhanceCaveMarker))
}

func monsterEnhanceRelJumpTarget(source uintptr, entry []byte) (uintptr, bool) {
	if len(entry) < 5 || entry[0] != 0xE9 {
		return 0, false
	}
	delta := int64(int32(binary.LittleEndian.Uint32(entry[1:5])))
	target := int64(source) + 5 + delta
	if target <= 0 {
		return 0, false
	}
	return uintptr(target), true
}

func (a *App) verifyMonsterEnhanceCaveMarker(record monsterEnhanceOwnedPatch) error {
	if record.Cave == 0 || record.CaveSize == 0 {
		return nil
	}
	markerAddr := monsterEnhanceCaveMarkerAddress(record.Cave, record.CaveSize)
	if markerAddr == 0 {
		return fmt.Errorf("monster cave marker address is invalid")
	}
	marker := make([]byte, len(monsterEnhanceCaveMarker))
	if err := readProcessMemory(a.hProcess, markerAddr, unsafe.Pointer(&marker[0]), uintptr(len(marker))); err != nil {
		return fmt.Errorf("read monster cave marker: %w", err)
	}
	if !bytesEqual(marker, monsterEnhanceCaveMarker) {
		return fmt.Errorf("monster cave marker mismatch: %s", bytesToHex(marker))
	}
	return nil
}

func (a *App) readMonsterEnhanceEntry(target uintptr, size int) ([]byte, error) {
	if target == 0 || size <= 0 {
		return nil, fmt.Errorf("monster patch entry is invalid")
	}
	current := make([]byte, size)
	if err := readProcessMemory(a.hProcess, target, unsafe.Pointer(&current[0]), uintptr(len(current))); err != nil {
		return nil, err
	}
	return current, nil
}

func (a *App) writeAndVerifyMonsterEnhanceEntry(target uintptr, desired []byte, label string) error {
	if err := writeCodeMemory(a.hProcess, target, desired); err != nil {
		return fmt.Errorf("write %s: %w", label, err)
	}
	actual, err := a.readMonsterEnhanceEntry(target, len(desired))
	if err != nil {
		return fmt.Errorf("read back %s: %w", label, err)
	}
	if !bytesEqual(actual, desired) {
		return fmt.Errorf("%s readback mismatch: got %s", label, bytesToHex(actual))
	}
	return nil
}

func (a *App) monsterEnhanceOwnerForCompatibilityCall() string {
	if a.charaOwnerToken != "" {
		return a.charaOwnerToken
	}
	return "compatibility"
}

func (a *App) prepareMonsterEnhanceEnable(ownerToken string, point *monsterPatchPoint) ([]byte, error) {
	if point == nil {
		return nil, fmt.Errorf("monster patch point is nil")
	}
	if existing, ok := a.monsterEnhanceOwned[point.ID]; ok {
		if existing.OwnerToken != ownerToken {
			return nil, fmt.Errorf("%s is owned by another runtime page", point.Name)
		}
		current, err := a.readMonsterEnhanceEntry(existing.Target, len(existing.Patched))
		if err != nil {
			return nil, fmt.Errorf("read owned %s: %w", point.Name, err)
		}
		if !bytesEqual(current, existing.Patched) {
			return nil, fmt.Errorf("owned %s entry changed: %s", point.Name, bytesToHex(current))
		}
		if err := a.verifyMonsterEnhanceCaveMarker(existing); err != nil {
			return nil, fmt.Errorf("owned %s cave validation failed: %w", point.Name, err)
		}
		return nil, fmt.Errorf("%s is already enabled; disable it before changing its value", point.Name)
	}
	target := a.moduleBase + point.RVA
	current, err := a.readMonsterEnhanceEntry(target, len(point.Original))
	if err != nil {
		return nil, fmt.Errorf("read %s before injection: %w", point.Name, err)
	}
	if !bytesEqual(current, point.Original) {
		return nil, fmt.Errorf("%s is not in its original state and is not owned by this tool instance: %s", point.Name, bytesToHex(current))
	}
	return current, nil
}

func (a *App) claimMonsterEnhancePatch(ownerToken string, point *monsterPatchPoint, original []byte) error {
	if point == nil || len(original) == 0 {
		return fmt.Errorf("cannot claim an empty monster patch")
	}
	target := a.moduleBase + point.RVA
	patched, err := a.readMonsterEnhanceEntry(target, len(point.Original))
	if err != nil {
		return fmt.Errorf("read %s after injection: %w", point.Name, err)
	}
	record := monsterEnhanceOwnedPatch{
		OwnerToken: ownerToken,
		Target:     target,
		Original:   append([]byte(nil), original...),
		Patched:    append([]byte(nil), patched...),
	}
	if point.Hook {
		cave, ok := monsterEnhanceRelJumpTarget(target, patched)
		if !ok {
			return fmt.Errorf("%s did not install a complete rel32 jump: %s", point.Name, bytesToHex(patched))
		}
		record.Cave = cave
		record.CaveSize = monsterEnhanceCaveSize(point.ID)
		if record.CaveSize == 0 {
			return fmt.Errorf("%s has no registered cave size", point.Name)
		}
		if err := a.verifyMonsterEnhanceCaveMarker(record); err != nil {
			return fmt.Errorf("%s cave ownership validation failed: %w", point.Name, err)
		}
	} else if point.ID == "sba_chain_timer" {
		if len(patched) < 2 || patched[0] != 0x48 || patched[1] != 0xB8 {
			return fmt.Errorf("%s patch bytes are invalid: %s", point.Name, bytesToHex(patched))
		}
	} else if !bytesEqual(patched, point.Patch) {
		return fmt.Errorf("%s patch bytes are invalid: %s", point.Name, bytesToHex(patched))
	}
	if a.monsterEnhanceOwned == nil {
		a.monsterEnhanceOwned = make(map[string]monsterEnhanceOwnedPatch)
	}
	a.monsterEnhanceOwned[point.ID] = record

	if point.ID == "crocodile_damage" {
		record.AuxTarget = a.moduleBase + monsterEnhanceCrocodileAuxRVA
		record.AuxOriginal = append([]byte(nil), monsterEnhanceCrocodileAuxOriginal...)
		record.AuxPatched = append([]byte(nil), monsterEnhanceCrocodileAuxPatched...)
		a.monsterEnhanceOwned[point.ID] = record
		current, readErr := a.readMonsterEnhanceEntry(record.AuxTarget, len(record.AuxPatched))
		if readErr != nil {
			return fmt.Errorf("read crocodile auxiliary patch: %w", readErr)
		}
		if !bytesEqual(current, record.AuxPatched) {
			return fmt.Errorf("crocodile auxiliary patch is not owned: %s", bytesToHex(current))
		}
	}
	// Store the auxiliary proof only after it has been read and matched. The
	// main record was retained above, so a partial DLL application still has a
	// retryable recovery lease.
	a.monsterEnhanceOwned[point.ID] = record
	return nil
}

func (a *App) monsterEnhanceHookMarked(point *monsterPatchPoint, entry []byte) bool {
	if point == nil || !point.Hook {
		return false
	}
	cave, ok := monsterEnhanceRelJumpTarget(a.moduleBase+point.RVA, entry)
	if !ok {
		return false
	}
	record := monsterEnhanceOwnedPatch{Cave: cave, CaveSize: monsterEnhanceCaveSize(point.ID)}
	return record.CaveSize != 0 && a.verifyMonsterEnhanceCaveMarker(record) == nil
}

func (a *App) restoreMonsterEnhanceOwned(ownerToken, id string, forceAllOwners bool) error {
	if len(a.monsterEnhanceOwned) == 0 {
		return nil
	}
	ids := make([]string, 0, len(a.monsterEnhanceOwned))
	for patchID, record := range a.monsterEnhanceOwned {
		if id != "all" && patchID != id {
			continue
		}
		if !forceAllOwners && record.OwnerToken != ownerToken {
			continue
		}
		ids = append(ids, patchID)
	}
	sort.Strings(ids)
	for _, patchID := range ids {
		record := a.monsterEnhanceOwned[patchID]
		current, err := a.readMonsterEnhanceEntry(record.Target, len(record.Patched))
		if err != nil {
			return fmt.Errorf("monster %s entry read failed: %w", patchID, err)
		}
		if !bytesEqual(current, record.Original) {
			if !bytesEqual(current, record.Patched) {
				return fmt.Errorf("monster %s entry is no longer owned: %s", patchID, bytesToHex(current))
			}
			if record.Cave != 0 {
				cave, ok := monsterEnhanceRelJumpTarget(record.Target, current)
				if !ok || cave != record.Cave {
					return fmt.Errorf("monster %s jump target changed", patchID)
				}
				if err := a.verifyMonsterEnhanceCaveMarker(record); err != nil {
					return fmt.Errorf("monster %s cave ownership failed: %w", patchID, err)
				}
			}
			if err := a.writeAndVerifyMonsterEnhanceEntry(record.Target, record.Original, "monster "+patchID+" entry restore"); err != nil {
				return err
			}
		}
		if record.AuxTarget != 0 {
			aux, readErr := a.readMonsterEnhanceEntry(record.AuxTarget, len(record.AuxPatched))
			if readErr != nil {
				return fmt.Errorf("monster %s auxiliary read failed: %w", patchID, readErr)
			}
			if !bytesEqual(aux, record.AuxOriginal) {
				if !bytesEqual(aux, record.AuxPatched) {
					return fmt.Errorf("monster %s auxiliary entry is no longer owned: %s", patchID, bytesToHex(aux))
				}
				if err := a.writeAndVerifyMonsterEnhanceEntry(record.AuxTarget, record.AuxOriginal, "monster "+patchID+" auxiliary restore"); err != nil {
					return err
				}
			}
		}
		delete(a.monsterEnhanceOwned, patchID)
	}
	if len(a.monsterEnhanceOwned) == 0 {
		a.monsterEnhanceOwned = nil
	}
	return nil
}
