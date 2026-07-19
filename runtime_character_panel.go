package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The character-panel reader is intentionally independent of charaManager.
// That legacy scanner targets the 0x5B70 character-use counter array, while
// this reader follows the 2.0.2 runtime status manager and never writes or
// injects code into the game process.
const (
	runtimeCharacterPanelManagerRVA = uintptr(0x7C24980)

	runtimeCharacterPanelVectorBeginOffset = uintptr(0x08)
	runtimeCharacterPanelVectorEndOffset   = uintptr(0x10)
	runtimeCharacterPanelSentinelOffset    = uintptr(0xA30)
	runtimeCharacterPanelBucketTableOffset = uintptr(0xA40)
	runtimeCharacterPanelBucketMaskOffset  = uintptr(0xA58)

	runtimeCharacterPanelBucketStride     = uintptr(0x10)
	runtimeCharacterPanelBucketLastOffset = uintptr(0x00)
	runtimeCharacterPanelBucketHeadOffset = uintptr(0x08)
	runtimeCharacterPanelNodeNextOffset   = uintptr(0x08)
	runtimeCharacterPanelNodeKeyOffset    = uintptr(0x10)
	runtimeCharacterPanelNodeStatusOffset = uintptr(0x30)

	runtimeCharacterPanelHPOffset            = uintptr(0x04)
	runtimeCharacterPanelAttackOffset        = uintptr(0x08)
	runtimeCharacterPanelStunOffset          = uintptr(0x10)
	runtimeCharacterPanelCritOffset          = uintptr(0x14)
	runtimeCharacterPanelReadyOffset         = uintptr(0x5EBC)
	runtimeCharacterPanelEligibilityOffset   = uintptr(0x5EBE)
	runtimeCharacterPanelCharacterHashOffset = uintptr(0x59F0)

	runtimeCharacterPanelMaxIDs        = 256
	runtimeCharacterPanelMaxChainNodes = 256
	runtimeCharacterPanelMaxHPAttack   = int32(999999)
	runtimeCharacterPanelMaxStun       = float32(999.9000244140625)
	runtimeCharacterPanelMaxCrit       = float32(999)

	runtimeCharacterPanelSource       = "game_runtime_2.0.2"
	runtimeCharacterPanelVerification = "游戏真实回读"

	// The handle has exactly the two rights required by NtQueryInformationProcess
	// and ReadProcessMemory. It deliberately omits PROCESS_VM_WRITE,
	// PROCESS_VM_OPERATION and every injection-capable access right.
	runtimeCharacterPanelProcessAccess = windows.PROCESS_QUERY_INFORMATION | windows.PROCESS_VM_READ
)

type runtimeCharacterPanelVersionGuard struct {
	RVA   uintptr
	Bytes []byte
}

// These anchors were checked byte-for-byte against the shipped 2.0.2 image.
// Guarding the manager lookup, hash-map lookup, ready flag and final-stat
// aggregator prevents an updated executable from being interpreted with stale
// offsets.
var runtimeCharacterPanelVersionGuards = []runtimeCharacterPanelVersionGuard{
	{RVA: 0xD4321, Bytes: []byte{0x48, 0x8B, 0x0D, 0x58, 0x06, 0xB5, 0x07, 0xE8, 0x93, 0x76, 0x20, 0x00}},
	{RVA: 0x2DC081, Bytes: []byte{0x41, 0x8B, 0x55, 0x00, 0x45, 0x8B, 0x84, 0x24, 0x58, 0x0A, 0x00, 0x00, 0x41, 0x21, 0xD0, 0x49, 0x8B, 0x84, 0x24, 0x30, 0x0A, 0x00, 0x00, 0x4D, 0x8B, 0x8C, 0x24, 0x40, 0x0A, 0x00, 0x00, 0x4C, 0x89, 0xC1, 0x48, 0xC1, 0xE1, 0x04, 0x49, 0x8B, 0x4C, 0x09, 0x08}},
	{RVA: 0x2DC11E, Bytes: []byte{0xC6, 0x44, 0x24, 0x38, 0x00, 0x4C, 0x89, 0xE1, 0x4C, 0x89, 0xE2, 0xE8, 0x52, 0x9E, 0x74, 0x00, 0x41, 0xC6, 0x84, 0x24, 0xBC, 0x5E, 0x00, 0x00, 0x01}},
	{RVA: 0xA296F3, Bytes: []byte{0xC5, 0xFA, 0x7E, 0x4B, 0x04, 0xC5, 0xE8, 0x57, 0xD2, 0xC4, 0xE2, 0x71, 0x3D, 0xCA, 0xC4, 0xE2, 0x71, 0x3B, 0x0D, 0xA6, 0xDB, 0xA7, 0x04, 0xC5, 0xF9, 0xD6, 0x4B, 0x04, 0xC5, 0xFB, 0x10, 0x5B, 0x10, 0xC5, 0xE8, 0x5F, 0xD3, 0xC5, 0xFB, 0x12, 0x1D, 0xB0, 0xDB, 0xA7, 0x04, 0xC5, 0xE0, 0x5D, 0xD2, 0xC5, 0xF8, 0x13, 0x53, 0x10}},
}

// RuntimeCharacterPanelStats contains values produced by the game's own 2.0.2
// panel aggregator. Unlike the offline loadout estimate, these fields are not
// recalculated by this application.
type RuntimeCharacterPanelStats struct {
	CharacterHash   string  `json:"characterHash"`
	HP              int32   `json:"hp"`
	Attack          int32   `json:"attack"`
	StunPower       float32 `json:"stunPower"`
	CritRate        float32 `json:"critRate"`
	Source          string  `json:"source"`
	Verification    string  `json:"verification"`
	GameVersion     string  `json:"gameVersion"`
	RuntimeVerified bool    `json:"runtimeVerified"`
}

type runtimeCharacterPanelMemory interface {
	ReadAt(address uintptr, destination []byte) error
}

type remoteRuntimeCharacterPanelMemory struct {
	handle windows.Handle
}

func (memory remoteRuntimeCharacterPanelMemory) ReadAt(address uintptr, destination []byte) error {
	if len(destination) == 0 {
		return nil
	}
	return readProcessMemory(memory.handle, address, unsafe.Pointer(&destination[0]), uintptr(len(destination)))
}

// LoadoutRuntimePanelStats opens a short-lived, read-only handle to the game,
// reads one requested character's computed panel values and closes the handle.
// It does not reuse or mutate App.hProcess/moduleBase and therefore cannot
// disturb the lifecycle of any memory editor page.
func (a *App) LoadoutRuntimePanelStats(charaHex string) (*RuntimeCharacterPanelStats, error) {
	targetHash, err := ParseHashHex(charaHex)
	if err != nil || targetHash == 0 {
		return nil, fmt.Errorf("角色 hash %q 无效", charaHex)
	}
	pid, err := findProcessByName(charaProcessName)
	if err != nil {
		return nil, fmt.Errorf("未找到游戏进程，请先启动游戏")
	}
	handle, err := windows.OpenProcess(runtimeCharacterPanelProcessAccess, false, pid)
	if err != nil {
		return nil, fmt.Errorf("无法以只读方式打开游戏进程: %w", err)
	}
	defer windows.CloseHandle(handle)

	moduleBase, err := getModuleBase(handle)
	if err != nil {
		return nil, fmt.Errorf("无法读取游戏模块基址: %w", err)
	}
	memory := remoteRuntimeCharacterPanelMemory{handle: handle}
	stats, err := readStableRuntimeCharacterPanelSnapshots(func() (RuntimeCharacterPanelStats, error) {
		return readRuntimeCharacterPanel(memory, moduleBase, targetHash)
	})
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

func readStableRuntimeCharacterPanelSnapshots(readSnapshot func() (RuntimeCharacterPanelStats, error)) (RuntimeCharacterPanelStats, error) {
	if readSnapshot == nil {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("游戏面板快照读取器为空")
	}
	var snapshots [3]RuntimeCharacterPanelStats
	for index := range snapshots {
		current, err := readSnapshot()
		if err != nil {
			return RuntimeCharacterPanelStats{}, fmt.Errorf("第 %d 次游戏面板快照读取失败: %w", index+1, err)
		}
		snapshots[index] = current
	}
	if snapshots[0] != snapshots[1] || snapshots[0] != snapshots[2] {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("游戏面板在连续 3 次快照间发生变化，请等待数值稳定后重试")
	}
	snapshots[0].RuntimeVerified = true
	return snapshots[0], nil
}

func readRuntimeCharacterPanel(memory runtimeCharacterPanelMemory, moduleBase uintptr, targetHash uint32) (RuntimeCharacterPanelStats, error) {
	if memory == nil || moduleBase == 0 || targetHash == 0 {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("游戏真实面板读取参数无效")
	}
	if err := verifyRuntimeCharacterPanelVersion(memory, moduleBase); err != nil {
		return RuntimeCharacterPanelStats{}, err
	}

	managerAddress, ok := checkedRuntimePanelAddress(moduleBase, runtimeCharacterPanelManagerRVA)
	if !ok {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("角色状态 manager 地址溢出")
	}
	manager, err := readRuntimePanelPointer(memory, managerAddress)
	if err != nil || manager == 0 {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("读取角色状态 manager 失败: %w", normalizeRuntimePanelReadError(err))
	}

	begin, err := readRuntimePanelPointerOffset(memory, manager, runtimeCharacterPanelVectorBeginOffset)
	if err != nil {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("读取角色 ID 向量起点失败: %w", err)
	}
	end, err := readRuntimePanelPointerOffset(memory, manager, runtimeCharacterPanelVectorEndOffset)
	if err != nil {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("读取角色 ID 向量终点失败: %w", err)
	}
	if begin == 0 || end < begin || (end-begin)%4 != 0 || (end-begin)/4 > runtimeCharacterPanelMaxIDs {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("角色 ID 向量范围异常: begin=0x%X end=0x%X", begin, end)
	}

	sentinel, err := readRuntimePanelPointerOffset(memory, manager, runtimeCharacterPanelSentinelOffset)
	if err != nil || sentinel == 0 {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("读取角色状态 map 哨兵失败: %w", normalizeRuntimePanelReadError(err))
	}
	table, err := readRuntimePanelPointerOffset(memory, manager, runtimeCharacterPanelBucketTableOffset)
	if err != nil || table == 0 {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("读取角色状态 bucket 表失败: %w", normalizeRuntimePanelReadError(err))
	}
	mask, err := readRuntimePanelU32Offset(memory, manager, runtimeCharacterPanelBucketMaskOffset)
	if err != nil {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("读取角色状态 bucket mask 失败: %w", err)
	}
	if mask > 0xFFFF || ((uint64(mask)+1)&uint64(mask)) != 0 {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("角色状态 bucket mask 异常: 0x%X", mask)
	}

	seenIDs := make(map[uint32]struct{}, int((end-begin)/4))
	for cursor := begin; cursor < end; cursor += 4 {
		id, readErr := readRuntimePanelU32(memory, cursor)
		if readErr != nil {
			return RuntimeCharacterPanelStats{}, fmt.Errorf("读取角色 ID 失败: %w", readErr)
		}
		if _, duplicate := seenIDs[id]; duplicate {
			continue
		}
		seenIDs[id] = struct{}{}
		status, found, lookupErr := lookupRuntimeCharacterPanelStatus(memory, table, mask, sentinel, id)
		if lookupErr != nil {
			return RuntimeCharacterPanelStats{}, lookupErr
		}
		if !found || status == 0 {
			continue
		}
		ready, readErr := readRuntimePanelU8Offset(memory, status, runtimeCharacterPanelReadyOffset)
		if readErr != nil {
			return RuntimeCharacterPanelStats{}, fmt.Errorf("读取角色状态 ready 标记失败: %w", readErr)
		}
		eligible, readErr := readRuntimePanelU8Offset(memory, status, runtimeCharacterPanelEligibilityOffset)
		if readErr != nil {
			return RuntimeCharacterPanelStats{}, fmt.Errorf("读取角色状态 eligibility 标记失败: %w", readErr)
		}
		if ready != 1 || eligible == 0 {
			continue
		}
		characterHash, readErr := readRuntimePanelU32Offset(memory, status, runtimeCharacterPanelCharacterHashOffset)
		if readErr != nil {
			return RuntimeCharacterPanelStats{}, fmt.Errorf("读取角色 hash 失败: %w", readErr)
		}
		if characterHash != targetHash {
			continue
		}
		stats, readErr := readRuntimeCharacterPanelValues(memory, status, characterHash)
		if readErr != nil {
			return RuntimeCharacterPanelStats{}, readErr
		}
		return stats, nil
	}
	return RuntimeCharacterPanelStats{}, fmt.Errorf("游戏内尚无角色 %08X 的可用面板结果，请打开角色/装备面板后重试", targetHash)
}

func verifyRuntimeCharacterPanelVersion(memory runtimeCharacterPanelMemory, moduleBase uintptr) error {
	for _, guard := range runtimeCharacterPanelVersionGuards {
		address, ok := checkedRuntimePanelAddress(moduleBase, guard.RVA)
		if !ok {
			return fmt.Errorf("2.0.2 版本守卫地址溢出: RVA 0x%X", guard.RVA)
		}
		actual := make([]byte, len(guard.Bytes))
		if err := memory.ReadAt(address, actual); err != nil {
			return fmt.Errorf("读取 2.0.2 版本守卫 RVA 0x%X 失败: %w", guard.RVA, err)
		}
		if !bytes.Equal(actual, guard.Bytes) {
			return fmt.Errorf("2.0.2 版本守卫不匹配（RVA 0x%X），已拒绝按旧布局读取", guard.RVA)
		}
	}
	return nil
}

func lookupRuntimeCharacterPanelStatus(memory runtimeCharacterPanelMemory, table uintptr, mask uint32, sentinel uintptr, id uint32) (uintptr, bool, error) {
	bucketOffset := uintptr(id&mask) * runtimeCharacterPanelBucketStride
	bucket, ok := checkedRuntimePanelAddress(table, bucketOffset)
	if !ok {
		return 0, false, fmt.Errorf("角色状态 bucket 地址溢出")
	}
	last, err := readRuntimePanelPointerOffset(memory, bucket, runtimeCharacterPanelBucketLastOffset)
	if err != nil {
		return 0, false, fmt.Errorf("读取角色状态 bucket 尾节点失败: %w", err)
	}
	node, err := readRuntimePanelPointerOffset(memory, bucket, runtimeCharacterPanelBucketHeadOffset)
	if err != nil {
		return 0, false, fmt.Errorf("读取角色状态 bucket 头节点失败: %w", err)
	}
	if node == 0 || node == sentinel {
		return 0, false, nil
	}
	visited := make(map[uintptr]struct{})
	for step := 0; step < runtimeCharacterPanelMaxChainNodes; step++ {
		if node == 0 || node == sentinel {
			return 0, false, nil
		}
		if _, duplicate := visited[node]; duplicate {
			return 0, false, fmt.Errorf("角色状态 bucket 链出现循环（id=0x%X）", id)
		}
		visited[node] = struct{}{}
		key, readErr := readRuntimePanelU32Offset(memory, node, runtimeCharacterPanelNodeKeyOffset)
		if readErr != nil {
			return 0, false, fmt.Errorf("读取角色状态节点 key 失败: %w", readErr)
		}
		if key == id {
			status, statusErr := readRuntimePanelPointerOffset(memory, node, runtimeCharacterPanelNodeStatusOffset)
			if statusErr != nil {
				return 0, false, fmt.Errorf("读取角色状态指针失败: %w", statusErr)
			}
			return status, status != 0, nil
		}
		if node == last {
			return 0, false, nil
		}
		next, nextErr := readRuntimePanelPointerOffset(memory, node, runtimeCharacterPanelNodeNextOffset)
		if nextErr != nil {
			return 0, false, fmt.Errorf("读取角色状态 bucket 下一节点失败: %w", nextErr)
		}
		node = next
	}
	return 0, false, fmt.Errorf("角色状态 bucket 链超过安全上限（id=0x%X）", id)
}

func readRuntimeCharacterPanelValues(memory runtimeCharacterPanelMemory, status uintptr, characterHash uint32) (RuntimeCharacterPanelStats, error) {
	hp, err := readRuntimePanelI32Offset(memory, status, runtimeCharacterPanelHPOffset)
	if err != nil {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("读取最终 HP 失败: %w", err)
	}
	attack, err := readRuntimePanelI32Offset(memory, status, runtimeCharacterPanelAttackOffset)
	if err != nil {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("读取最终攻击力失败: %w", err)
	}
	stun, err := readRuntimePanelF32Offset(memory, status, runtimeCharacterPanelStunOffset)
	if err != nil {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("读取最终昏厥值失败: %w", err)
	}
	crit, err := readRuntimePanelF32Offset(memory, status, runtimeCharacterPanelCritOffset)
	if err != nil {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("读取最终暴击率失败: %w", err)
	}
	if hp < 1 || hp > runtimeCharacterPanelMaxHPAttack || attack < 1 || attack > runtimeCharacterPanelMaxHPAttack ||
		math.IsNaN(float64(stun)) || math.IsInf(float64(stun), 0) || stun < 0 || stun > runtimeCharacterPanelMaxStun ||
		math.IsNaN(float64(crit)) || math.IsInf(float64(crit), 0) || crit < 0 || crit > runtimeCharacterPanelMaxCrit {
		return RuntimeCharacterPanelStats{}, fmt.Errorf("游戏面板数值异常：HP=%d 攻击力=%d 昏厥值=%v 暴击率=%v", hp, attack, stun, crit)
	}
	return RuntimeCharacterPanelStats{
		CharacterHash: hashText(characterHash),
		HP:            hp,
		Attack:        attack,
		StunPower:     stun,
		CritRate:      crit,
		Source:        runtimeCharacterPanelSource,
		Verification:  runtimeCharacterPanelVerification,
		GameVersion:   "2.0.2",
	}, nil
}

func checkedRuntimePanelAddress(base, offset uintptr) (uintptr, bool) {
	if ^uintptr(0)-base < offset {
		return 0, false
	}
	return base + offset, true
}

func normalizeRuntimePanelReadError(err error) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("指针为空")
}

func readRuntimePanelPointer(memory runtimeCharacterPanelMemory, address uintptr) (uintptr, error) {
	encoded := make([]byte, 8)
	if err := memory.ReadAt(address, encoded); err != nil {
		return 0, err
	}
	value := binary.LittleEndian.Uint64(encoded)
	if uint64(uintptr(value)) != value {
		return 0, fmt.Errorf("64 位指针超出本机地址范围: 0x%X", value)
	}
	return uintptr(value), nil
}

func readRuntimePanelPointerOffset(memory runtimeCharacterPanelMemory, base, offset uintptr) (uintptr, error) {
	address, ok := checkedRuntimePanelAddress(base, offset)
	if !ok {
		return 0, fmt.Errorf("指针地址溢出")
	}
	return readRuntimePanelPointer(memory, address)
}

func readRuntimePanelU8Offset(memory runtimeCharacterPanelMemory, base, offset uintptr) (byte, error) {
	address, ok := checkedRuntimePanelAddress(base, offset)
	if !ok {
		return 0, fmt.Errorf("byte 地址溢出")
	}
	encoded := make([]byte, 1)
	if err := memory.ReadAt(address, encoded); err != nil {
		return 0, err
	}
	return encoded[0], nil
}

func readRuntimePanelU32(memory runtimeCharacterPanelMemory, address uintptr) (uint32, error) {
	encoded := make([]byte, 4)
	if err := memory.ReadAt(address, encoded); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(encoded), nil
}

func readRuntimePanelU32Offset(memory runtimeCharacterPanelMemory, base, offset uintptr) (uint32, error) {
	address, ok := checkedRuntimePanelAddress(base, offset)
	if !ok {
		return 0, fmt.Errorf("uint32 地址溢出")
	}
	return readRuntimePanelU32(memory, address)
}

func readRuntimePanelI32Offset(memory runtimeCharacterPanelMemory, base, offset uintptr) (int32, error) {
	value, err := readRuntimePanelU32Offset(memory, base, offset)
	return int32(value), err
}

func readRuntimePanelF32Offset(memory runtimeCharacterPanelMemory, base, offset uintptr) (float32, error) {
	value, err := readRuntimePanelU32Offset(memory, base, offset)
	return math.Float32frombits(value), err
}
