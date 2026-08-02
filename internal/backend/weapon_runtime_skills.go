package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"unsafe"
)

const (
	weaponRuntimeSkillsMagic      = "GBFRWK01"
	weaponRuntimeSkillsHeaderSize = 28
	weaponRuntimeSkillsMaxEntries = 2048
)

var weaponRuntimeSkillsMu sync.Mutex

type WeaponRuntimeSkill struct {
	Hash  uint32 `json:"hash"`
	Level uint32 `json:"level"`
}

type WeaponRuntimeSkillsConfig struct {
	SchemaVersion int                  `json:"schemaVersion"`
	WeaponSlot    int32                `json:"weaponSlot"`
	WeaponID      uint32               `json:"weaponId"`
	Skills        []WeaponRuntimeSkill `json:"skills"`
}

type WeaponRuntimeSkillsDeployRequest struct {
	OwnerToken           string               `json:"ownerToken"`
	ExpectedSelectedAddr uint64               `json:"expectedSelectedAddr"`
	WeaponSlot           int32                `json:"weaponSlot"`
	WeaponID             uint32               `json:"weaponId"`
	Skills               []WeaponRuntimeSkill `json:"skills"`
}

type WeaponRuntimeSkillsWorkspace struct {
	Installed        bool                      `json:"installed"`
	Owned            bool                      `json:"owned"`
	RecoveryRequired bool                      `json:"recoveryRequired"`
	State            string                    `json:"state"`
	Detail           string                    `json:"detail"`
	GameRunning      bool                      `json:"gameRunning"`
	Config           WeaponRuntimeSkillsConfig `json:"config"`
}

type WeaponRuntimeSkillsDeployResult struct {
	Active     bool `json:"active"`
	SkillCount int  `json:"skillCount"`
}

func weaponRuntimeSkillsPath() (string, error) {
	return runtimeCompanionPath("weapon-skills.bin")
}

func normalizeWeaponRuntimeSkills(request WeaponRuntimeSkillsDeployRequest) (WeaponRuntimeSkillsConfig, error) {
	if request.WeaponID == 0 || request.WeaponID == EmptyHash {
		return WeaponRuntimeSkillsConfig{}, errors.New("请先读取并选中一把有效武器")
	}
	if request.WeaponSlot < 0 {
		return WeaponRuntimeSkillsConfig{}, errors.New("目标武器槽位无效")
	}
	if len(request.Skills) == 0 {
		return WeaponRuntimeSkillsConfig{}, errors.New("请至少添加一条运行时武器技能")
	}
	if len(request.Skills) > weaponRuntimeSkillsMaxEntries {
		return WeaponRuntimeSkillsConfig{}, fmt.Errorf("运行时武器技能数量超过内部保护上限 %d", weaponRuntimeSkillsMaxEntries)
	}
	config := WeaponRuntimeSkillsConfig{
		SchemaVersion: 1,
		WeaponSlot:    request.WeaponSlot,
		WeaponID:      request.WeaponID,
		Skills:        make([]WeaponRuntimeSkill, len(request.Skills)),
	}
	copy(config.Skills, request.Skills)
	for index, skill := range config.Skills {
		if skill.Hash == 0 || skill.Hash == EmptyHash {
			return WeaponRuntimeSkillsConfig{}, fmt.Errorf("第 %d 条运行时武器技能为空", index+1)
		}
		if skill.Level == 0 || skill.Level > math.MaxInt32 {
			return WeaponRuntimeSkillsConfig{}, fmt.Errorf("第 %d 条运行时武器技能等级必须在 1 到 %d 之间", index+1, math.MaxInt32)
		}
	}
	return config, nil
}

func encodeWeaponRuntimeSkills(config WeaponRuntimeSkillsConfig, enabled bool) ([]byte, error) {
	normalized, err := normalizeWeaponRuntimeSkills(WeaponRuntimeSkillsDeployRequest{
		WeaponSlot: config.WeaponSlot,
		WeaponID:   config.WeaponID,
		Skills:     config.Skills,
	})
	if err != nil {
		return nil, err
	}
	buffer := bytes.NewBuffer(make([]byte, 0, weaponRuntimeSkillsHeaderSize+len(normalized.Skills)*8))
	buffer.WriteString(weaponRuntimeSkillsMagic)
	values := []uint32{1, 0, uint32(normalized.WeaponSlot), normalized.WeaponID, uint32(len(normalized.Skills))}
	if enabled {
		values[1] = 1
	}
	for _, value := range values {
		if err := binary.Write(buffer, binary.LittleEndian, value); err != nil {
			return nil, err
		}
	}
	for _, skill := range normalized.Skills {
		if err := binary.Write(buffer, binary.LittleEndian, skill); err != nil {
			return nil, err
		}
	}
	return buffer.Bytes(), nil
}

func decodeWeaponRuntimeSkills(data []byte) (WeaponRuntimeSkillsConfig, bool, error) {
	if len(data) < weaponRuntimeSkillsHeaderSize || string(data[:8]) != weaponRuntimeSkillsMagic {
		return WeaponRuntimeSkillsConfig{}, false, errors.New("运行时武器技能配置头无效")
	}
	if binary.LittleEndian.Uint32(data[8:12]) != 1 {
		return WeaponRuntimeSkillsConfig{}, false, errors.New("不支持的运行时武器技能配置版本")
	}
	enabled := binary.LittleEndian.Uint32(data[12:16]) == 1
	count := binary.LittleEndian.Uint32(data[24:28])
	if count == 0 || count > weaponRuntimeSkillsMaxEntries || len(data) != weaponRuntimeSkillsHeaderSize+int(count)*8 {
		return WeaponRuntimeSkillsConfig{}, false, errors.New("运行时武器技能配置长度无效")
	}
	config := WeaponRuntimeSkillsConfig{
		SchemaVersion: 1,
		WeaponSlot:    int32(binary.LittleEndian.Uint32(data[16:20])),
		WeaponID:      binary.LittleEndian.Uint32(data[20:24]),
		Skills:        make([]WeaponRuntimeSkill, count),
	}
	for index := range config.Skills {
		offset := weaponRuntimeSkillsHeaderSize + index*8
		config.Skills[index] = WeaponRuntimeSkill{
			Hash:  binary.LittleEndian.Uint32(data[offset : offset+4]),
			Level: binary.LittleEndian.Uint32(data[offset+4 : offset+8]),
		}
	}
	normalized, err := normalizeWeaponRuntimeSkills(WeaponRuntimeSkillsDeployRequest{
		WeaponSlot: config.WeaponSlot,
		WeaponID:   config.WeaponID,
		Skills:     config.Skills,
	})
	return normalized, enabled, err
}

func readWeaponRuntimeSkillsConfig() (WeaponRuntimeSkillsConfig, bool) {
	path, err := weaponRuntimeSkillsPath()
	if err != nil {
		return WeaponRuntimeSkillsConfig{SchemaVersion: 1, Skills: []WeaponRuntimeSkill{}}, false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > weaponRuntimeSkillsHeaderSize+weaponRuntimeSkillsMaxEntries*8 {
		return WeaponRuntimeSkillsConfig{SchemaVersion: 1, Skills: []WeaponRuntimeSkill{}}, false
	}
	config, enabled, err := decodeWeaponRuntimeSkills(data)
	if err != nil {
		return WeaponRuntimeSkillsConfig{SchemaVersion: 1, Skills: []WeaponRuntimeSkill{}}, false
	}
	return config, enabled
}

func weaponRuntimeSkillsEnabled() bool {
	_, enabled := readWeaponRuntimeSkillsConfig()
	return enabled
}

func writeWeaponRuntimeSkillsConfig(config WeaponRuntimeSkillsConfig, enabled bool) error {
	data, err := encodeWeaponRuntimeSkills(config, enabled)
	if err != nil {
		return err
	}
	path, err := weaponRuntimeSkillsPath()
	if err != nil {
		return err
	}
	return writeRuntimeCompanionFile(path, data)
}

func (a *App) GetWeaponRuntimeSkillsWorkspace() (*WeaponRuntimeSkillsWorkspace, error) {
	weaponRuntimeSkillsMu.Lock()
	defer weaponRuntimeSkillsMu.Unlock()
	config, _ := readWeaponRuntimeSkillsConfig()
	_, processErr := findProcessByName(charaProcessName)
	status := readRuntimeCompanionStatus("weapon-skills")
	active := runtimeCompanionPresent("weapon-skills")
	process, identityErr := findRuntimeProcessInstance()
	owned := identityErr == nil && a.runtimeCompanionOwned("weapon-skills", process)
	recoveryRequired := identityErr == nil && runtimeCompanionRecoveryRequired(status, process)
	detail := status.Detail
	if !active && processErr != nil {
		detail = "请先启动游戏，再开启额外武器技能"
	}
	return &WeaponRuntimeSkillsWorkspace{
		Installed: active, Owned: owned, RecoveryRequired: recoveryRequired,
		State: status.State, Detail: detail, GameRunning: processErr == nil, Config: config,
	}, nil
}

func (a *App) WeaponRuntimeSkillsDeploy(request WeaponRuntimeSkillsDeployRequest) (*WeaponRuntimeSkillsDeployResult, error) {
	weaponRuntimeSkillsMu.Lock()
	defer weaponRuntimeSkillsMu.Unlock()
	config, err := normalizeWeaponRuntimeSkills(request)
	if err != nil {
		return nil, err
	}
	alreadyActive, claimed, err := a.prepareWeaponRuntimeSkillsDeployment(request)
	if err != nil {
		return nil, err
	}
	if err := writeWeaponRuntimeSkillsConfig(config, true); err != nil {
		if claimed {
			a.releaseRuntimeCompanionOwnership("weapon-skills")
		}
		return nil, err
	}
	if alreadyActive {
		return &WeaponRuntimeSkillsDeployResult{Active: true, SkillCount: len(config.Skills)}, nil
	}
	if err := a.startPreparedRuntimeCompanionForDigest("weapon-skills", "runtime_weapon_skills", game203ExecutableSHA256); err != nil {
		_ = writeWeaponRuntimeSkillsConfig(config, false)
		if process, processErr := findRuntimeProcessInstance(); processErr != nil ||
			!runtimeCompanionNeedsStop(readRuntimeCompanionStatus("weapon-skills"), process) {
			a.releaseRuntimeCompanionOwnership("weapon-skills")
		}
		return nil, err
	}
	return &WeaponRuntimeSkillsDeployResult{Active: true, SkillCount: len(config.Skills)}, nil
}

func (a *App) prepareWeaponRuntimeSkillsDeployment(request WeaponRuntimeSkillsDeployRequest) (alreadyActive, claimed bool, err error) {
	if request.OwnerToken == "" || request.ExpectedSelectedAddr == 0 {
		return false, false, errors.New("当前武器读取所有权或选择代次已失效，请重新启用读取并选择武器")
	}
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerWeapon, request.OwnerToken); err != nil {
		return false, false, err
	}
	defer a.procMu.Unlock()
	process := a.currentProcessInstance()
	if err := a.verifyRuntimePatchExecutableLocked(process, "额外武器技能"); err != nil {
		return false, false, err
	}
	if !strings.EqualFold(a.runtimePatchVerifiedDigest, game203ExecutableSHA256) {
		return false, false, errors.New("额外武器技能只支持已核对的 GAME 2.0.3 可执行文件")
	}
	weaponMemoryLifecycleMu.Lock()
	status, statusErr := a.readWeaponMemoryStatusLocked()
	var caveSelected uintptr
	if statusErr == nil && a.weaponMemoryCaveAddr != 0 {
		statusErr = readProcessMemory(a.hProcess, a.weaponMemoryCaveAddr+weaponMemoryCaveDataOffset, unsafe.Pointer(&caveSelected), unsafe.Sizeof(caveSelected))
	}
	weaponMemoryLifecycleMu.Unlock()
	if statusErr != nil {
		return false, false, fmt.Errorf("复核当前武器失败: %w", statusErr)
	}
	if _, err := validateWeaponMemorySelection(uintptr(request.ExpectedSelectedAddr), uintptr(status.SelectedAddr), caveSelected); err != nil {
		return false, false, err
	}
	if status.WeaponSlot != request.WeaponSlot || status.WeaponID != request.WeaponID {
		return false, false, errors.New("当前选中武器已经变化，请重新确认额外技能目标")
	}
	companionStatus := readRuntimeCompanionStatus("weapon-skills")
	ownedGeneration := ""
	if a.ownsRuntimeCompanion("weapon-skills", process) {
		if lease, ok := a.runtimeCompanionLease("weapon-skills", process); ok {
			ownedGeneration = lease.Generation
		}
	}
	alreadyActive, err = runtimeCompanionStartDecision(companionStatus, process, ownedGeneration)
	if err != nil {
		return false, false, err
	}
	if alreadyActive {
		return true, false, nil
	}
	if err := clearStaleInactiveRuntimeCompanionStatus("weapon-skills", companionStatus, process); err != nil {
		return false, false, err
	}
	if err := a.claimRuntimeCompanionOwnership("weapon-skills", process); err != nil {
		return false, false, err
	}
	return false, true, nil
}

func (a *App) WeaponRuntimeSkillsRemove() error {
	weaponRuntimeSkillsMu.Lock()
	defer weaponRuntimeSkillsMu.Unlock()
	config, _ := readWeaponRuntimeSkillsConfig()
	if len(config.Skills) == 0 {
		return a.stopOwnedRuntimeCompanion("weapon-skills", nil)
	}
	return a.stopOwnedRuntimeCompanion("weapon-skills", func() error {
		return writeWeaponRuntimeSkillsConfig(config, false)
	})
}
