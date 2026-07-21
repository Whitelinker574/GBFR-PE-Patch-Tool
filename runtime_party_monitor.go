package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"unsafe"
)

// CT 0.8.4 node 30967 (NBGFR002) resolves the live party slot table through
// this RIP-relative instruction. The full signature and the resolved RVA are
// independently checked against the supplied 2.0.2 executable by the opt-in
// local-EXE truth test.
const (
	ct084PartyPointerAOB      = "488Bxxxxxxxxxx4885xx74xx488BxxFFxxxxxxxxxx4885xx74xx488Bxxxx488Bxxxx488Bxx488DxxxxxxFFxxxxxxxxxxEBxxC5xxxxxxxxxxxxxxC5xxxxxxxxxx488Bxx488D"
	ct084PartyPointerRVA      = uintptr(0x22CECA0)
	ct084PartySlotTableRVA    = uintptr(0x7036860)
	ct084PartySignatureLength = 69

	ct084PartyHPOffset            = uintptr(0x160)
	ct084PartyMaxHPOffset         = uintptr(0x168)
	ct084PartyDodgeOffset         = uintptr(0x5788)
	ct084PartySBAOffset           = uintptr(0x32AC)
	ct084PartyMaxSBAOffset        = uintptr(0x32B0)
	ct084PartyTransformRootOffset = uintptr(0x28)
	ct084PartyTransformNodeOffset = uintptr(0x08)
	ct084PartyPositionXOffset     = uintptr(0xD8)
	ct084PartyPositionYOffset     = uintptr(0xD4)
	ct084PartyPositionZOffset     = uintptr(0xD0)

	ct084PartyCompanionSlotOffset    = uintptr(0x38)
	ct084PartyCompanionEntityOffset  = uintptr(0x70)
	ct084PartyCompanionDirectXOffset = uintptr(0x1588)
	ct084PartyCompanionDirectYOffset = uintptr(0x1584)
	ct084PartyCompanionDirectZOffset = uintptr(0x1580)

	ct084PartyMaximumPlausibleHP         = uint64(1_000_000_000)
	ct084PartyMaximumPlausibleSBA        = float32(1_000_000)
	ct084PartyMaximumCoordinateMagnitude = float32(10_000_000)
	ct084PartySnapshotCount              = 3
)

type CT084Vector3 struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

type CT084PartyCapabilities struct {
	Dodge          bool `json:"dodge"`
	SBA            bool `json:"sba"`
	DirectPosition bool `json:"directPosition"`
}

// CT084PartyEntity mirrors only fields present in the CT 0.8.4 read-only
// pointer panel. Optional pointers intentionally distinguish an unavailable
// capability from a real in-game zero.
type CT084PartyEntity struct {
	Role           string                 `json:"role"`
	Present        bool                   `json:"present"`
	DisplayName    string                 `json:"displayName"`
	Address        uint64                 `json:"address"`
	HP             uint64                 `json:"hp"`
	MaxHP          uint64                 `json:"maxHp"`
	DodgeCount     *uint32                `json:"dodgeCount,omitempty"`
	SBA            *float32               `json:"sba,omitempty"`
	MaxSBA         *float32               `json:"maxSba,omitempty"`
	Position       CT084Vector3           `json:"position"`
	DirectPosition *CT084Vector3          `json:"directPosition,omitempty"`
	Capabilities   CT084PartyCapabilities `json:"capabilities"`
}

type CT084PartyMonitor struct {
	OwnerToken      string             `json:"ownerToken,omitempty"`
	PID             uint32             `json:"pid"`
	ProcessCreated  uint64             `json:"processCreated"`
	RootAddress     uint64             `json:"rootAddress"`
	Entities        []CT084PartyEntity `json:"entities"`
	Source          string             `json:"source"`
	Verification    string             `json:"verification"`
	GameVersion     string             `json:"gameVersion"`
	SnapshotCount   int                `json:"snapshotCount"`
	RuntimeVerified bool               `json:"runtimeVerified"`
}

type ct084PartyTopology struct {
	Root               uintptr
	Entities           [5]uintptr
	TransformNodes     [5][2]uintptr
	CompanionContainer uintptr
}

type ct084PartySnapshot struct {
	Topology ct084PartyTopology
	Result   CT084PartyMonitor
}

type ct084PartyMemory interface {
	ReadAt(address uintptr, destination []byte) error
}

type remoteCT084PartyMemory struct {
	app *App
}

func (memory remoteCT084PartyMemory) ReadAt(address uintptr, destination []byte) error {
	if memory.app == nil || memory.app.hProcess == 0 {
		return fmt.Errorf("game process handle is empty")
	}
	if len(destination) == 0 {
		return nil
	}
	return readProcessMemory(memory.app.hProcess, address, unsafe.Pointer(&destination[0]), uintptr(len(destination)))
}

// CT084PartyMonitorOwned performs no writes and installs no hook. The Chara
// owner lease pins hProcess/moduleBase/{PID, Created} for all three snapshots,
// so a reused PID or concurrent page cleanup cannot splice two processes into
// one result.
func (a *App) CT084PartyMonitorOwned(token string) (CT084PartyMonitor, error) {
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return CT084PartyMonitor{}, err
	}
	defer a.procMu.Unlock()

	memory := remoteCT084PartyMemory{app: a}
	result, err := readStableCT084PartySnapshots(func() (ct084PartySnapshot, error) {
		return readCT084PartySnapshot(memory, a.moduleBase)
	})
	if err != nil {
		return CT084PartyMonitor{}, err
	}
	result.OwnerToken = token
	result.PID = a.charaPID
	result.ProcessCreated = a.charaCreated
	return result, nil
}

func readStableCT084PartySnapshots(readSnapshot func() (ct084PartySnapshot, error)) (CT084PartyMonitor, error) {
	if readSnapshot == nil {
		return CT084PartyMonitor{}, fmt.Errorf("%s", ct084MonitorText("队伍快照读取器为空", "Party snapshot reader is nil"))
	}
	var frames [ct084PartySnapshotCount]ct084PartySnapshot
	for index := range frames {
		frame, err := readSnapshot()
		if err != nil {
			return CT084PartyMonitor{}, fmt.Errorf("%s: %w", ct084MonitorText(
				fmt.Sprintf("第 %d 次队伍快照读取失败", index+1),
				fmt.Sprintf("party snapshot %d failed", index+1),
			), err)
		}
		frames[index] = frame
	}
	if frames[0].Topology != frames[1].Topology || frames[0].Topology != frames[2].Topology {
		return CT084PartyMonitor{}, fmt.Errorf("%s", ct084MonitorText(
			"队伍根指针、实体或坐标链拓扑在连续三次快照间发生变化，请等待场景稳定后重试",
			"Party root, entity, or coordinate-chain topology changed across three snapshots; wait for a stable scene and retry",
		))
	}
	result := frames[len(frames)-1].Result
	result.RootAddress = uint64(frames[len(frames)-1].Topology.Root)
	result.Source = "game_runtime_ct084_2.0.2"
	result.Verification = ct084MonitorText("连续三快照拓扑验证", "three-snapshot topology verification")
	result.GameVersion = "2.0.2"
	result.SnapshotCount = ct084PartySnapshotCount
	result.RuntimeVerified = true
	return result, nil
}

func readCT084PartySnapshot(memory ct084PartyMemory, moduleBase uintptr) (ct084PartySnapshot, error) {
	if memory == nil || moduleBase == 0 {
		return ct084PartySnapshot{}, fmt.Errorf("%s", ct084MonitorText("队伍监测读取参数无效", "Invalid party monitor read parameters"))
	}
	root, err := verifyCT084PartyPointerSignature(memory, moduleBase)
	if err != nil {
		return ct084PartySnapshot{}, err
	}

	var snapshot ct084PartySnapshot
	snapshot.Topology.Root = root
	snapshot.Result.Entities = make([]CT084PartyEntity, 0, 5)
	roles := [...]string{"player", "party1", "party2", "party3"}
	for index, role := range roles {
		entity, readErr := readCT084Pointer(memory, root+uintptr(index)*8)
		if readErr != nil {
			return ct084PartySnapshot{}, fmt.Errorf("%s: %w", ct084PartyRoleName(role), normalizeCT084PartyReadError(readErr))
		}
		if entity == 0 {
			if index == 0 {
				return ct084PartySnapshot{}, fmt.Errorf("%s: %w", ct084PartyRoleName(role), normalizeCT084PartyReadError(nil))
			}
			snapshot.Result.Entities = append(snapshot.Result.Entities, emptyCT084PartyEntity(role))
			continue
		}
		result, nodes, readErr := readCT084PartyEntity(memory, entity, role, true, false)
		if readErr != nil {
			return ct084PartySnapshot{}, readErr
		}
		snapshot.Topology.Entities[index] = entity
		snapshot.Topology.TransformNodes[index] = nodes
		snapshot.Result.Entities = append(snapshot.Result.Entities, result)
	}

	container, err := readCT084Pointer(memory, root+ct084PartyCompanionSlotOffset)
	if err != nil {
		return ct084PartySnapshot{}, fmt.Errorf("%s: %w", ct084PartyRoleName("companion"), normalizeCT084PartyReadError(err))
	}
	if container == 0 {
		snapshot.Result.Entities = append(snapshot.Result.Entities, emptyCT084PartyEntity("companion"))
		return snapshot, nil
	}
	snapshot.Topology.CompanionContainer = container
	companion, err := readCT084Pointer(memory, container+ct084PartyCompanionEntityOffset)
	if err != nil {
		return ct084PartySnapshot{}, fmt.Errorf("%s: %w", ct084PartyRoleName("companion"), normalizeCT084PartyReadError(err))
	}
	if companion == 0 {
		snapshot.Result.Entities = append(snapshot.Result.Entities, emptyCT084PartyEntity("companion"))
		return snapshot, nil
	}
	companionResult, companionNodes, err := readCT084PartyEntity(memory, companion, "companion", false, true)
	if err != nil {
		return ct084PartySnapshot{}, err
	}
	snapshot.Topology.Entities[4] = companion
	snapshot.Topology.TransformNodes[4] = companionNodes
	snapshot.Result.Entities = append(snapshot.Result.Entities, companionResult)
	return snapshot, nil
}

func emptyCT084PartyEntity(role string) CT084PartyEntity {
	return CT084PartyEntity{
		Role:         role,
		DisplayName:  ct084PartyRoleName(role),
		Position:     CT084Vector3{},
		Capabilities: CT084PartyCapabilities{},
	}
}

func verifyCT084PartyPointerSignature(memory ct084PartyMemory, moduleBase uintptr) (uintptr, error) {
	pattern, err := parseCT084Pattern(ct084PartyPointerAOB)
	if err != nil {
		return 0, err
	}
	if len(pattern.Values) != ct084PartySignatureLength {
		return 0, fmt.Errorf("party pointer signature length=%d, want %d", len(pattern.Values), ct084PartySignatureLength)
	}
	site, ok := checkedCT084MonitorAddress(moduleBase, ct084PartyPointerRVA)
	if !ok {
		return 0, fmt.Errorf("party pointer signature address overflow")
	}
	actual := make([]byte, len(pattern.Values))
	if err := memory.ReadAt(site, actual); err != nil {
		return 0, fmt.Errorf("%s: %w", ct084MonitorText("读取队伍指针签名失败", "Read party pointer signature"), err)
	}
	if !matchCT084Pattern(actual, pattern) {
		return 0, fmt.Errorf("%s: RVA 0x%X", ct084MonitorText("队伍指针签名与游戏 2.0.2 不匹配", "Party pointer signature does not match game 2.0.2"), ct084PartyPointerRVA)
	}
	displacement := int64(int32(binary.LittleEndian.Uint32(actual[3:7])))
	resolvedSigned := int64(site) + 7 + displacement
	if resolvedSigned <= 0 || uint64(resolvedSigned) > uint64(^uintptr(0)) {
		return 0, fmt.Errorf("party pointer RIP target overflow")
	}
	resolved := uintptr(resolvedSigned)
	expected, ok := checkedCT084MonitorAddress(moduleBase, ct084PartySlotTableRVA)
	if !ok || resolved != expected {
		return 0, fmt.Errorf("%s: got RVA 0x%X, want 0x%X", ct084MonitorText("队伍根槽解析结果不匹配", "Party root slot resolution mismatch"), resolved-moduleBase, ct084PartySlotTableRVA)
	}
	return resolved, nil
}

func readCT084PartyEntity(memory ct084PartyMemory, address uintptr, role string, supportsCombat, supportsDirectPosition bool) (CT084PartyEntity, [2]uintptr, error) {
	var nodes [2]uintptr
	hp, err := readCT084U64At(memory, address+ct084PartyHPOffset)
	if err != nil {
		return CT084PartyEntity{}, nodes, fmt.Errorf("%s HP: %w", ct084PartyRoleName(role), err)
	}
	maxHP, err := readCT084U64At(memory, address+ct084PartyMaxHPOffset)
	if err != nil {
		return CT084PartyEntity{}, nodes, fmt.Errorf("%s max HP: %w", ct084PartyRoleName(role), err)
	}
	nodes[0], err = readCT084Pointer(memory, address+ct084PartyTransformRootOffset)
	if err != nil || nodes[0] == 0 {
		return CT084PartyEntity{}, nodes, fmt.Errorf("%s transform root: %w", ct084PartyRoleName(role), normalizeCT084PartyReadError(err))
	}
	nodes[1], err = readCT084Pointer(memory, nodes[0]+ct084PartyTransformNodeOffset)
	if err != nil || nodes[1] == 0 {
		return CT084PartyEntity{}, nodes, fmt.Errorf("%s transform node: %w", ct084PartyRoleName(role), normalizeCT084PartyReadError(err))
	}
	position, err := readCT084Vector3(memory, nodes[1], ct084PartyPositionXOffset, ct084PartyPositionYOffset, ct084PartyPositionZOffset)
	if err != nil {
		return CT084PartyEntity{}, nodes, fmt.Errorf("%s position: %w", ct084PartyRoleName(role), err)
	}
	result := CT084PartyEntity{
		Role:         role,
		Present:      true,
		DisplayName:  ct084PartyRoleName(role),
		Address:      uint64(address),
		HP:           hp,
		MaxHP:        maxHP,
		Position:     position,
		Capabilities: CT084PartyCapabilities{Dodge: supportsCombat, SBA: supportsCombat, DirectPosition: supportsDirectPosition},
	}
	if supportsCombat {
		dodge, readErr := readCT084U32At(memory, address+ct084PartyDodgeOffset)
		if readErr != nil {
			return CT084PartyEntity{}, nodes, fmt.Errorf("%s dodge count: %w", ct084PartyRoleName(role), readErr)
		}
		sba, readErr := readCT084F32At(memory, address+ct084PartySBAOffset)
		if readErr != nil {
			return CT084PartyEntity{}, nodes, fmt.Errorf("%s SBA: %w", ct084PartyRoleName(role), readErr)
		}
		maxSBA, readErr := readCT084F32At(memory, address+ct084PartyMaxSBAOffset)
		if readErr != nil {
			return CT084PartyEntity{}, nodes, fmt.Errorf("%s max SBA: %w", ct084PartyRoleName(role), readErr)
		}
		result.DodgeCount = &dodge
		result.SBA = &sba
		result.MaxSBA = &maxSBA
	}
	if supportsDirectPosition {
		direct, readErr := readCT084Vector3(memory, address, ct084PartyCompanionDirectXOffset, ct084PartyCompanionDirectYOffset, ct084PartyCompanionDirectZOffset)
		if readErr != nil {
			return CT084PartyEntity{}, nodes, fmt.Errorf("%s direct position: %w", ct084PartyRoleName(role), readErr)
		}
		result.DirectPosition = &direct
	}
	if err := validateCT084PartyEntity(result); err != nil {
		return CT084PartyEntity{}, nodes, fmt.Errorf("%s: %w", ct084PartyRoleName(role), err)
	}
	return result, nodes, nil
}

func validateCT084PartyEntity(entity CT084PartyEntity) error {
	if !entity.Present {
		if entity.Address != 0 || entity.HP != 0 || entity.MaxHP != 0 || entity.DodgeCount != nil || entity.SBA != nil || entity.MaxSBA != nil || entity.DirectPosition != nil || entity.Capabilities != (CT084PartyCapabilities{}) || entity.Position != (CT084Vector3{}) {
			return fmt.Errorf("absent party slot contains runtime entity data")
		}
		return nil
	}
	if entity.MaxHP == 0 || entity.MaxHP > ct084PartyMaximumPlausibleHP || entity.HP > entity.MaxHP {
		return fmt.Errorf("HP is outside [0,max] or max HP is implausible: %d/%d", entity.HP, entity.MaxHP)
	}
	if entity.Capabilities.Dodge != (entity.DodgeCount != nil) {
		return fmt.Errorf("dodge capability and value availability disagree")
	}
	if entity.Capabilities.SBA != (entity.SBA != nil && entity.MaxSBA != nil) || (entity.SBA == nil) != (entity.MaxSBA == nil) {
		return fmt.Errorf("SBA capability and value availability disagree")
	}
	if entity.Capabilities.DirectPosition != (entity.DirectPosition != nil) {
		return fmt.Errorf("direct-position capability and value availability disagree")
	}
	if entity.SBA != nil {
		current, maximum := *entity.SBA, *entity.MaxSBA
		if !finiteCT084Float(current) || !finiteCT084Float(maximum) || maximum <= 0 || maximum > ct084PartyMaximumPlausibleSBA || current < 0 || current > maximum {
			return fmt.Errorf("SBA is invalid: %v/%v", current, maximum)
		}
	}
	if err := validateCT084Vector3(entity.Position); err != nil {
		return err
	}
	if entity.DirectPosition != nil {
		if err := validateCT084Vector3(*entity.DirectPosition); err != nil {
			return fmt.Errorf("direct position: %w", err)
		}
	}
	return nil
}

func validateCT084Vector3(value CT084Vector3) error {
	for _, coordinate := range []float32{value.X, value.Y, value.Z} {
		if !finiteCT084Float(coordinate) || float32(math.Abs(float64(coordinate))) > ct084PartyMaximumCoordinateMagnitude {
			return fmt.Errorf("coordinate is non-finite or outside world bounds: %v", coordinate)
		}
	}
	return nil
}

func finiteCT084Float(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

func readCT084Vector3(memory ct084PartyMemory, base, xOffset, yOffset, zOffset uintptr) (CT084Vector3, error) {
	x, err := readCT084F32At(memory, base+xOffset)
	if err != nil {
		return CT084Vector3{}, err
	}
	y, err := readCT084F32At(memory, base+yOffset)
	if err != nil {
		return CT084Vector3{}, err
	}
	z, err := readCT084F32At(memory, base+zOffset)
	if err != nil {
		return CT084Vector3{}, err
	}
	return CT084Vector3{X: x, Y: y, Z: z}, nil
}

func readCT084Pointer(memory ct084PartyMemory, address uintptr) (uintptr, error) {
	value, err := readCT084U64At(memory, address)
	if err != nil {
		return 0, err
	}
	if uint64(uintptr(value)) != value {
		return 0, fmt.Errorf("pointer 0x%X is outside the local address width", value)
	}
	return uintptr(value), nil
}

func readCT084U64At(memory ct084PartyMemory, address uintptr) (uint64, error) {
	encoded := make([]byte, 8)
	if err := memory.ReadAt(address, encoded); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(encoded), nil
}

func readCT084U32At(memory ct084PartyMemory, address uintptr) (uint32, error) {
	encoded := make([]byte, 4)
	if err := memory.ReadAt(address, encoded); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(encoded), nil
}

func readCT084F32At(memory ct084PartyMemory, address uintptr) (float32, error) {
	value, err := readCT084U32At(memory, address)
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(value), nil
}

func checkedCT084MonitorAddress(base, offset uintptr) (uintptr, bool) {
	if ^uintptr(0)-base < offset {
		return 0, false
	}
	return base + offset, true
}

func normalizeCT084PartyReadError(err error) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("pointer is null")
}

func ct084PartyRoleName(role string) string {
	if useChinese() {
		switch role {
		case "player":
			return "玩家"
		case "party1":
			return "队伍成员 1"
		case "party2":
			return "队伍成员 2"
		case "party3":
			return "队伍成员 3"
		case "companion":
			return "碧的小红龙"
		}
	}
	switch role {
	case "player":
		return "Player"
	case "party1":
		return "Party Member 1"
	case "party2":
		return "Party Member 2"
	case "party3":
		return "Party Member 3"
	case "companion":
		return "Vyrn"
	default:
		return role
	}
}

func ct084MonitorText(chinese, english string) string {
	if useChinese() {
		return chinese
	}
	return english
}
