package backend

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	runtimePartyObserverMappingName = "Local\\GBFRPlayerInfoEditPartyProfilesV1"
	runtimePartyObserverMagic       = uint64(0x31564F5052464247) // GBFRPOV1
	runtimePartyObserverVersion     = uint32(1)
	runtimePartyObserverCapacity    = uint32(128)
	runtimePartyObserverHeaderSize  = 32
	runtimePartyObserverEventSize   = 184

	runtimePartyObserverProfileKind = uint32(1)
	runtimePartyObserverResetKind   = uint32(2)
)

type runtimePartyObserverEvent struct {
	Sequence  uint64
	Kind      uint32
	Direction runtimePartyNetworkProfileDirection
	Payload   []byte
}

func decodeRuntimePartyObserverMapping(data []byte, afterSequence uint64) ([]runtimePartyObserverEvent, uint64, error) {
	if len(data) < runtimePartyObserverHeaderSize {
		return nil, afterSequence, errors.New("Party 配装观察共享内存头不完整")
	}
	magic := binary.LittleEndian.Uint64(data[0:8])
	version := binary.LittleEndian.Uint32(data[8:12])
	capacity := binary.LittleEndian.Uint32(data[12:16])
	writeSequence := binary.LittleEndian.Uint64(data[16:24])
	if magic != runtimePartyObserverMagic || version != runtimePartyObserverVersion {
		return nil, afterSequence, fmt.Errorf("Party 配装观察共享内存版本不兼容: magic=%X version=%d", magic, version)
	}
	if capacity != runtimePartyObserverCapacity {
		return nil, afterSequence, fmt.Errorf("Party 配装观察共享内存容量无效: %d", capacity)
	}
	required := runtimePartyObserverHeaderSize + int(capacity)*runtimePartyObserverEventSize
	if len(data) < required {
		return nil, afterSequence, fmt.Errorf("Party 配装观察共享内存长度=%d，期望至少 %d", len(data), required)
	}
	if writeSequence < afterSequence {
		afterSequence = 0
	}
	start := afterSequence + 1
	if writeSequence > uint64(capacity) && start <= writeSequence-uint64(capacity) {
		start = writeSequence - uint64(capacity) + 1
	}
	consumedSequence := afterSequence
	if start > consumedSequence+1 {
		consumedSequence = start - 1
	}
	events := make([]runtimePartyObserverEvent, 0)
	for sequence := start; sequence <= writeSequence; sequence++ {
		index := uint32((sequence - 1) % uint64(capacity))
		offset := runtimePartyObserverHeaderSize + int(index)*runtimePartyObserverEventSize
		row := data[offset : offset+runtimePartyObserverEventSize]
		if binary.LittleEndian.Uint64(row[0:8]) != sequence {
			break
		}
		consumedSequence = sequence
		kind := binary.LittleEndian.Uint32(row[16:20])
		if kind == runtimePartyObserverResetKind {
			events = append(events, runtimePartyObserverEvent{Sequence: sequence, Kind: kind})
			continue
		}
		if kind != runtimePartyObserverProfileKind {
			continue
		}
		direction := runtimePartyNetworkProfileDirection(binary.LittleEndian.Uint32(row[20:24]))
		if direction != runtimePartyNetworkProfileLocal && direction != runtimePartyNetworkProfileRemote {
			continue
		}
		profileSize := binary.LittleEndian.Uint32(row[36:40])
		if profileSize != runtimePartyNetworkInitialProfileSize && profileSize != runtimePartyNetworkPeriodicProfileSize {
			continue
		}
		payload := make([]byte, profileSize)
		group, messageType := uint32(2), uint32(63)
		if profileSize == runtimePartyNetworkInitialProfileSize {
			group, messageType = 3, 14
		}
		binary.LittleEndian.PutUint32(payload[0:4], group)
		binary.LittleEndian.PutUint32(payload[4:8], messageType)
		binary.LittleEndian.PutUint32(payload[8:12], profileSize)
		binary.LittleEndian.PutUint32(payload[12:16], runtimePartyNetworkProfileVersion)
		binary.LittleEndian.PutUint32(payload[runtimePartyNetworkPartyIndexOffset:], binary.LittleEndian.Uint32(row[24:28]))
		binary.LittleEndian.PutUint32(payload[runtimePartyNetworkCharacterHashOffset:], binary.LittleEndian.Uint32(row[28:32]))
		binary.LittleEndian.PutUint32(payload[runtimePartyNetworkWeaponHashOffset:], binary.LittleEndian.Uint32(row[32:36]))
		for sigilIndex := 0; sigilIndex < runtimePartyNetworkSigilCount; sigilIndex++ {
			entry := row[40+sigilIndex*12 : 40+(sigilIndex+1)*12]
			binary.LittleEndian.PutUint32(payload[runtimePartyNetworkSigilHashOffset+sigilIndex*4:], binary.LittleEndian.Uint32(entry[0:4]))
			binary.LittleEndian.PutUint32(payload[runtimePartyNetworkSecondaryHashOffset+sigilIndex*4:], binary.LittleEndian.Uint32(entry[4:8]))
			level := binary.LittleEndian.Uint32(entry[8:12])
			if level > 255 {
				continue
			}
			payload[runtimePartyNetworkSigilLevelOffset+sigilIndex] = byte(level)
		}
		events = append(events, runtimePartyObserverEvent{Sequence: sequence, Kind: kind, Direction: direction, Payload: payload})
	}
	return events, consumedSequence, nil
}

func readRuntimePartyObserverMapping() ([]byte, error) {
	name, err := windows.UTF16PtrFromString(runtimePartyObserverMappingName)
	if err != nil {
		return nil, err
	}
	handleRaw, _, callErr := kernel32OpenFileMappingW.Call(uintptr(windows.FILE_MAP_READ), 0, uintptr(unsafe.Pointer(name)))
	if handleRaw == 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return nil, callErr
		}
		return nil, errors.New("Party 配装观察器尚未建立共享内存")
	}
	handle := windows.Handle(handleRaw)
	defer windows.CloseHandle(handle)
	length := uintptr(runtimePartyObserverHeaderSize + int(runtimePartyObserverCapacity)*runtimePartyObserverEventSize)
	address, err := windows.MapViewOfFile(handle, windows.FILE_MAP_READ, 0, 0, length)
	if err != nil {
		return nil, err
	}
	defer windows.UnmapViewOfFile(address)
	view := unsafe.Slice((*byte)(unsafe.Pointer(address)), int(length))
	output := make([]byte, len(view))
	copy(output[:runtimePartyObserverHeaderSize], view[:runtimePartyObserverHeaderSize])
	writeSequence := atomic.LoadUint64((*uint64)(unsafe.Pointer(address + 16)))
	binary.LittleEndian.PutUint64(output[16:24], writeSequence)
	for index := 0; index < int(runtimePartyObserverCapacity); index++ {
		offset := runtimePartyObserverHeaderSize + index*runtimePartyObserverEventSize
		sequenceAddress := address + uintptr(offset)
		sequenceBefore := atomic.LoadUint64((*uint64)(unsafe.Pointer(sequenceAddress)))
		copy(output[offset+8:offset+runtimePartyObserverEventSize], view[offset+8:offset+runtimePartyObserverEventSize])
		sequenceAfter := atomic.LoadUint64((*uint64)(unsafe.Pointer(sequenceAddress)))
		if sequenceBefore == sequenceAfter && sequenceAfter != 0 {
			binary.LittleEndian.PutUint64(output[offset:offset+8], sequenceAfter)
		}
	}
	return output, nil
}

func readRuntimePartyObserverEvents(afterSequence uint64) ([]runtimePartyObserverEvent, uint64, error) {
	data, err := readRuntimePartyObserverMapping()
	if err != nil {
		return nil, afterSequence, err
	}
	return decodeRuntimePartyObserverMapping(data, afterSequence)
}

func writeRuntimePartyObserverConfig(enabled bool) error {
	path, err := runtimeCompanionPath("party-observer.ini")
	if err != nil {
		return err
	}
	flag := 0
	if enabled {
		flag = 1
	}
	return writeRuntimeCompanionFile(path, []byte(fmt.Sprintf("[party-observer]\r\nenabled=%d\r\n", flag)))
}
