package backend

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	runtimeDamageMappingName = "Local\\GBFRPlayerInfoEditDamageEventsV1"
	runtimeDamageMagic       = uint64(0x31564D4446524247) // GBFRDMV1
	runtimeDamageVersion     = uint32(1)
	runtimeDamageCapacity    = uint32(4096)
	runtimeDamageHeaderSize  = 32
	runtimeDamageEventSize   = 64
)

var runtimeDamageCaptureMu sync.Mutex

type RuntimeDamageEvent struct {
	Sequence      uint64  `json:"sequence"`
	TickMillis    uint64  `json:"tickMillis"`
	SourceAddress uint64  `json:"sourceAddress"`
	TargetAddress uint64  `json:"targetAddress"`
	Damage        int32   `json:"damage"`
	DamageCap     int32   `json:"damageCap"`
	BaseDamage    float32 `json:"baseDamage"`
	AttackRate    float32 `json:"attackRate"`
	Flags         uint64  `json:"flags"`
	ActionID      uint32  `json:"actionId"`
	ActionType    string  `json:"actionType"`
	Cappable      bool    `json:"cappable"`
	Capped        bool    `json:"capped"`
	OvercapPct    float64 `json:"overcapPercent"`
}

type RuntimeDamageSkillSummary struct {
	Key            string  `json:"key"`
	SourceAddress  uint64  `json:"sourceAddress"`
	ActionID       uint32  `json:"actionId"`
	ActionType     string  `json:"actionType"`
	Hits           int     `json:"hits"`
	Damage         int64   `json:"damage"`
	MinDamage      int32   `json:"minDamage"`
	MaxDamage      int32   `json:"maxDamage"`
	CappableHits   int     `json:"cappableHits"`
	CappedHits     int     `json:"cappedHits"`
	OvercapPercent float64 `json:"overcapPercent"`
}

type RuntimeDamageSnapshot struct {
	Active         bool                        `json:"active"`
	PID            uint32                      `json:"pid"`
	Version        uint32                      `json:"version"`
	Capacity       uint32                      `json:"capacity"`
	TotalEvents    uint64                      `json:"totalEvents"`
	DroppedEvents  uint64                      `json:"droppedEvents"`
	FirstTick      uint64                      `json:"firstTick"`
	LastTick       uint64                      `json:"lastTick"`
	DurationMillis uint64                      `json:"durationMillis"`
	TotalDamage    int64                       `json:"totalDamage"`
	DPS            float64                     `json:"dps"`
	Skills         []RuntimeDamageSkillSummary `json:"skills"`
	Events         []RuntimeDamageEvent        `json:"events"`
	Scope          string                      `json:"scope"`
	Detail         string                      `json:"detail"`
}

func runtimeDamageActionType(flags uint64) string {
	switch {
	case flags&(1<<7|1<<50) != 0:
		return "link"
	case flags&(1<<13|1<<14) != 0:
		return "sba"
	case flags&(1<<15) != 0:
		return "supplementary"
	default:
		return "normal"
	}
}

func decodeRuntimeDamageSharedMemory(data []byte, limit int) (RuntimeDamageSnapshot, error) {
	if len(data) < runtimeDamageHeaderSize {
		return RuntimeDamageSnapshot{}, errors.New("伤害采集共享内存头不完整")
	}
	magic := binary.LittleEndian.Uint64(data[0:8])
	version := binary.LittleEndian.Uint32(data[8:12])
	capacity := binary.LittleEndian.Uint32(data[12:16])
	writeSequence := binary.LittleEndian.Uint64(data[16:24])
	dropped := binary.LittleEndian.Uint64(data[24:32])
	if magic != runtimeDamageMagic || version != runtimeDamageVersion {
		return RuntimeDamageSnapshot{}, fmt.Errorf("伤害采集共享内存版本不兼容: magic=%X version=%d", magic, version)
	}
	if capacity == 0 || capacity > 1<<20 {
		return RuntimeDamageSnapshot{}, fmt.Errorf("伤害采集容量无效: %d", capacity)
	}
	required := runtimeDamageHeaderSize + int(capacity)*runtimeDamageEventSize
	if len(data) < required {
		return RuntimeDamageSnapshot{}, fmt.Errorf("伤害采集共享内存长度=%d，期望至少 %d", len(data), required)
	}
	if limit <= 0 || limit > int(capacity) {
		limit = int(capacity)
	}
	start := uint64(1)
	if writeSequence > uint64(limit) {
		start = writeSequence - uint64(limit) + 1
	}
	events := make([]RuntimeDamageEvent, 0, limit)
	for sequence := start; sequence <= writeSequence; sequence++ {
		index := uint32((sequence - 1) % uint64(capacity))
		offset := runtimeDamageHeaderSize + int(index)*runtimeDamageEventSize
		row := data[offset : offset+runtimeDamageEventSize]
		if binary.LittleEndian.Uint64(row[0:8]) != sequence {
			continue
		}
		event := RuntimeDamageEvent{
			Sequence: sequence, TickMillis: binary.LittleEndian.Uint64(row[8:16]),
			SourceAddress: binary.LittleEndian.Uint64(row[16:24]), TargetAddress: binary.LittleEndian.Uint64(row[24:32]),
			Damage: int32(binary.LittleEndian.Uint32(row[32:36])), DamageCap: int32(binary.LittleEndian.Uint32(row[36:40])),
			BaseDamage: math.Float32frombits(binary.LittleEndian.Uint32(row[40:44])), AttackRate: math.Float32frombits(binary.LittleEndian.Uint32(row[44:48])),
			Flags: binary.LittleEndian.Uint64(row[48:56]), ActionID: binary.LittleEndian.Uint32(row[56:60]),
		}
		if event.Damage < 0 || !isFiniteFloat32(event.BaseDamage) || !isFiniteFloat32(event.AttackRate) {
			continue
		}
		event.ActionType = runtimeDamageActionType(event.Flags)
		event.Cappable = event.DamageCap > 0 && event.DamageCap < 99_999_999 && event.ActionType != "supplementary"
		if event.Cappable {
			event.Capped = event.BaseDamage > float32(event.DamageCap)
			if event.DamageCap > 0 && event.BaseDamage > float32(event.DamageCap) {
				event.OvercapPct = (float64(event.BaseDamage)/float64(event.DamageCap) - 1) * 100
			}
		}
		events = append(events, event)
	}
	return aggregateRuntimeDamageSnapshot(RuntimeDamageSnapshot{
		Active: true, Version: version, Capacity: capacity, TotalEvents: writeSequence, DroppedEvents: dropped, Events: events,
	}), nil
}

func isFiniteFloat32(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

func aggregateRuntimeDamageSnapshot(snapshot RuntimeDamageSnapshot) RuntimeDamageSnapshot {
	type accumulator struct {
		RuntimeDamageSkillSummary
		overcapTotal float64
	}
	byKey := make(map[string]*accumulator)
	order := make([]string, 0)
	for _, event := range snapshot.Events {
		if snapshot.FirstTick == 0 || event.TickMillis < snapshot.FirstTick {
			snapshot.FirstTick = event.TickMillis
		}
		if event.TickMillis > snapshot.LastTick {
			snapshot.LastTick = event.TickMillis
		}
		snapshot.TotalDamage += int64(event.Damage)
		key := fmt.Sprintf("%016X:%s:%08X", event.SourceAddress, event.ActionType, event.ActionID)
		item := byKey[key]
		if item == nil {
			item = &accumulator{RuntimeDamageSkillSummary: RuntimeDamageSkillSummary{
				Key: key, SourceAddress: event.SourceAddress, ActionID: event.ActionID, ActionType: event.ActionType, MinDamage: event.Damage, MaxDamage: event.Damage,
			}}
			byKey[key] = item
			order = append(order, key)
		}
		item.Hits++
		item.Damage += int64(event.Damage)
		if event.Damage < item.MinDamage {
			item.MinDamage = event.Damage
		}
		if event.Damage > item.MaxDamage {
			item.MaxDamage = event.Damage
		}
		if event.Cappable {
			item.CappableHits++
			if event.Capped {
				item.CappedHits++
			}
			item.overcapTotal += event.OvercapPct
		}
	}
	if snapshot.LastTick >= snapshot.FirstTick {
		snapshot.DurationMillis = snapshot.LastTick - snapshot.FirstTick
	}
	if snapshot.DurationMillis > 0 {
		snapshot.DPS = float64(snapshot.TotalDamage) / (float64(snapshot.DurationMillis) / 1000)
	}
	snapshot.Skills = make([]RuntimeDamageSkillSummary, 0, len(order))
	for _, key := range order {
		item := byKey[key]
		if item.CappableHits > 0 {
			item.OvercapPercent = item.overcapTotal / float64(item.CappableHits)
		}
		snapshot.Skills = append(snapshot.Skills, item.RuntimeDamageSkillSummary)
	}
	return snapshot
}

var (
	kernel32OpenFileMappingW = windows.NewLazySystemDLL("kernel32.dll").NewProc("OpenFileMappingW")
)

func readRuntimeDamageMapping() ([]byte, error) {
	name, err := windows.UTF16PtrFromString(runtimeDamageMappingName)
	if err != nil {
		return nil, err
	}
	handleRaw, _, callErr := kernel32OpenFileMappingW.Call(uintptr(windows.FILE_MAP_READ), 0, uintptr(unsafe.Pointer(name)))
	if handleRaw == 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return nil, callErr
		}
		return nil, errors.New("当前会话伤害采集尚未建立共享内存")
	}
	handle := windows.Handle(handleRaw)
	defer windows.CloseHandle(handle)
	length := uintptr(runtimeDamageHeaderSize + int(runtimeDamageCapacity)*runtimeDamageEventSize)
	address, err := windows.MapViewOfFile(handle, windows.FILE_MAP_READ, 0, 0, length)
	if err != nil {
		return nil, err
	}
	defer windows.UnmapViewOfFile(address)
	view := unsafe.Slice((*byte)(unsafe.Pointer(address)), int(length))
	output := make([]byte, len(view))
	copy(output[:16], view[:16])
	writeSequence := atomic.LoadUint64((*uint64)(unsafe.Pointer(address + 16)))
	droppedEvents := atomic.LoadUint64((*uint64)(unsafe.Pointer(address + 24)))
	binary.LittleEndian.PutUint64(output[16:24], writeSequence)
	binary.LittleEndian.PutUint64(output[24:32], droppedEvents)
	for index := 0; index < int(runtimeDamageCapacity); index++ {
		offset := runtimeDamageHeaderSize + index*runtimeDamageEventSize
		sequenceAddress := address + uintptr(offset)
		sequenceBefore := atomic.LoadUint64((*uint64)(unsafe.Pointer(sequenceAddress)))
		if sequenceBefore == 0 {
			continue
		}
		copy(output[offset+8:offset+runtimeDamageEventSize], view[offset+8:offset+runtimeDamageEventSize])
		sequenceAfter := atomic.LoadUint64((*uint64)(unsafe.Pointer(sequenceAddress)))
		if sequenceBefore == sequenceAfter && sequenceAfter != 0 {
			binary.LittleEndian.PutUint64(output[offset:offset+8], sequenceAfter)
		}
	}
	return output, nil
}

func runtimeDamageConfigPath() (string, error) { return runtimeCompanionPath("damage.ini") }

func writeRuntimeDamageConfig(enabled bool) error {
	path, err := runtimeDamageConfigPath()
	if err != nil {
		return err
	}
	flag := 0
	if enabled {
		flag = 1
	}
	return writeRuntimeCompanionFile(path, []byte(fmt.Sprintf("[damage]\r\nenabled=%d\r\n", flag)))
}

func (a *App) RuntimeDamageCaptureStart() (*RuntimeDamageSnapshot, error) {
	runtimeDamageCaptureMu.Lock()
	defer runtimeDamageCaptureMu.Unlock()
	if err := writeRuntimeDamageConfig(true); err != nil {
		return nil, err
	}
	if err := a.startRuntimeCompanion("damage", "runtime_damage"); err != nil {
		_ = writeRuntimeDamageConfig(false)
		return nil, err
	}
	return a.RuntimeDamageCaptureSnapshot(512)
}

func (a *App) RuntimeDamageCaptureSnapshot(limit int) (*RuntimeDamageSnapshot, error) {
	status := readRuntimeCompanionStatus("damage")
	active := a.runtimeCompanionActive("damage")
	if !active {
		return &RuntimeDamageSnapshot{Active: false, PID: status.PID, Detail: status.Detail}, nil
	}
	data, err := readRuntimeDamageMapping()
	if err != nil {
		return nil, err
	}
	snapshot, err := decodeRuntimeDamageSharedMemory(data, limit)
	if err != nil {
		return nil, err
	}
	snapshot.PID = status.PID
	snapshot.Scope = "global-unattributed"
	snapshot.Detail = status.Detail
	return &snapshot, nil
}

func (a *App) RuntimeDamageCaptureStop() error {
	runtimeDamageCaptureMu.Lock()
	defer runtimeDamageCaptureMu.Unlock()
	return a.stopOwnedRuntimeCompanion("damage", func() error { return writeRuntimeDamageConfig(false) })
}
