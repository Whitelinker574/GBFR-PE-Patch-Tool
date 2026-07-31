package backend

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var audioMixerMu sync.Mutex

var audioMixerCharacters = []AudioMixerCharacter{
	{ID: "NP0000", NameZh: "露莉亚", NameEn: "Lyria"}, {ID: "NP0100", NameZh: "碧", NameEn: "Vyrn"}, {ID: "NP0200", NameZh: "谢洛卡特", NameEn: "Sierokarte"},
	{ID: "PL0000", NameZh: "古兰", NameEn: "Gran"}, {ID: "PL0100", NameZh: "姬塔", NameEn: "Djeeta"}, {ID: "PL0200", NameZh: "卡塔莉娜", NameEn: "Katalina"},
	{ID: "PL0300", NameZh: "拉卡姆", NameEn: "Rackam"}, {ID: "PL0400", NameZh: "伊欧", NameEn: "Io"}, {ID: "PL0500", NameZh: "欧根", NameEn: "Eugen"},
	{ID: "PL0600", NameZh: "萝赛塔", NameEn: "Rosetta"}, {ID: "PL0700", NameZh: "菲莉", NameEn: "Ferry"}, {ID: "PL0800", NameZh: "兰斯洛特", NameEn: "Lancelot"},
	{ID: "PL0900", NameZh: "巴恩", NameEn: "Vane"}, {ID: "PL1000", NameZh: "珀西瓦尔", NameEn: "Percival"}, {ID: "PL1100", NameZh: "齐格飞", NameEn: "Siegfried"},
	{ID: "PL1200", NameZh: "夏洛特", NameEn: "Charlotta"}, {ID: "PL1300", NameZh: "尤达拉哈", NameEn: "Yodarha"}, {ID: "PL1400", NameZh: "娜露梅亚", NameEn: "Narmaya"},
	{ID: "PL1500", NameZh: "冈达葛萨", NameEn: "Ghandagoza"}, {ID: "PL1600", NameZh: "泽塔", NameEn: "Zeta"}, {ID: "PL1700", NameZh: "巴萨拉卡", NameEn: "Vaseraga"},
	{ID: "PL1800", NameZh: "卡莉奥斯特罗", NameEn: "Cagliostro"}, {ID: "PL1900", NameZh: "伊德", NameEn: "Id"}, {ID: "PL2100", NameZh: "圣德芬", NameEn: "Sandalphon"},
	{ID: "PL2200", NameZh: "希耶提", NameEn: "Seofon"}, {ID: "PL2300", NameZh: "索恩", NameEn: "Tweyen"}, {ID: "PL2400", NameZh: "加兰查", NameEn: "Gallanza"},
	{ID: "PL2500", NameZh: "玛琪拉菲菈", NameEn: "Maglielle"}, {ID: "PL2600", NameZh: "贝阿朵丽丝", NameEn: "Beatrix"}, {ID: "PL2700", NameZh: "尤斯提斯", NameEn: "Eustace"},
	{ID: "PL2800", NameZh: "芙劳", NameEn: "Fraux"}, {ID: "PL2900", NameZh: "菲迪埃尔", NameEn: "Fediel"},
}

type AudioMixerCharacter struct {
	ID     string `json:"id"`
	NameZh string `json:"nameZh"`
	NameEn string `json:"nameEn"`
}

type AudioMixerWorkspace struct {
	Installed        bool                  `json:"installed"`
	Owned            bool                  `json:"owned"`
	RecoveryRequired bool                  `json:"recoveryRequired"`
	State            string                `json:"state"`
	GameRunning      bool                  `json:"gameRunning"`
	Diagnostic       bool                  `json:"diagnostic"`
	Characters       []AudioMixerCharacter `json:"characters"`
	Volumes          map[string]int        `json:"volumes"`
	UIVolume         int                   `json:"uiVolume"`
	Detail           string                `json:"detail"`
}

type AudioMixerDeployRequest struct {
	Diagnostic bool           `json:"diagnostic"`
	Volumes    map[string]int `json:"volumes"`
	UIVolume   int            `json:"uiVolume"`
}

type audioMixerConfig struct {
	DiagnosticLog    bool
	CharacterVolumes map[string]int
	UIVolume         int
}

func audioMixerSupportedIDs() map[string]bool {
	result := make(map[string]bool, len(audioMixerCharacters))
	for _, character := range audioMixerCharacters {
		result[character.ID] = true
	}
	return result
}

func normalizeAudioMixerConfig(request AudioMixerDeployRequest) (audioMixerConfig, error) {
	allowed := audioMixerSupportedIDs()
	volumes := make(map[string]int, len(allowed))
	for id := range allowed {
		volumes[id] = 100
	}
	for rawID, volume := range request.Volumes {
		id := strings.ToUpper(strings.TrimSpace(rawID))
		if !allowed[id] {
			return audioMixerConfig{}, fmt.Errorf("未知角色音轨: %s", rawID)
		}
		if volume < 0 || volume > 100 {
			return audioMixerConfig{}, fmt.Errorf("%s 音量必须在 0 到 100 之间", id)
		}
		volumes[id] = volume
	}
	if request.UIVolume < 0 || request.UIVolume > 100 {
		return audioMixerConfig{}, fmt.Errorf("界面音效音量必须在 0 到 100 之间")
	}
	return audioMixerConfig{DiagnosticLog: request.Diagnostic, CharacterVolumes: volumes, UIVolume: request.UIVolume}, nil
}

func audioMixerRuntimePath() (string, error) { return runtimeCompanionPath("audio.ini") }

func readAudioMixerConfig() audioMixerConfig {
	value := audioMixerConfig{CharacterVolumes: make(map[string]int, len(audioMixerCharacters)), UIVolume: 100}
	for _, character := range audioMixerCharacters {
		value.CharacterVolumes[character.ID] = 100
	}
	path, err := audioMixerRuntimePath()
	if err != nil {
		return value
	}
	ini := readRuntimeINI(path)
	value.DiagnosticLog = ini["audio"]["diagnostic"] == "1"
	if parsed, err := strconv.Atoi(ini["ui"]["volume"]); err == nil && parsed >= 0 && parsed <= 100 {
		value.UIVolume = parsed
	}
	for _, character := range audioMixerCharacters {
		if parsed, err := strconv.Atoi(ini["volumes"][character.ID]); err == nil && parsed >= 0 && parsed <= 100 {
			value.CharacterVolumes[character.ID] = parsed
		}
	}
	return value
}

func writeAudioMixerRuntimeConfig(config audioMixerConfig, enabled bool) error {
	path, err := audioMixerRuntimePath()
	if err != nil {
		return err
	}
	flag, diagnostic := 0, 0
	if enabled {
		flag = 1
	}
	if config.DiagnosticLog {
		diagnostic = 1
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "[audio]\r\nenabled=%d\r\ndiagnostic=%d\r\n[ui]\r\nvolume=%d\r\n[volumes]\r\n", flag, diagnostic, config.UIVolume)
	for _, id := range audioMixerSortedIDs() {
		fmt.Fprintf(&builder, "%s=%d\r\n", id, config.CharacterVolumes[id])
	}
	return writeRuntimeCompanionFile(path, []byte(builder.String()))
}

func (a *App) GetAudioMixerWorkspace(_ string) (*AudioMixerWorkspace, error) {
	audioMixerMu.Lock()
	defer audioMixerMu.Unlock()
	config := readAudioMixerConfig()
	_, processErr := findProcessByName(charaProcessName)
	status := readRuntimeCompanionStatus("audio")
	active := runtimeCompanionPresent("audio")
	process, processIdentityErr := findRuntimeProcessInstance()
	owned := processIdentityErr == nil && a.runtimeCompanionOwned("audio", process)
	recoveryRequired := processIdentityErr == nil && runtimeCompanionRecoveryRequired(status, process)
	detail := status.Detail
	if !active && processErr != nil {
		detail = "请先启动游戏，再从本页开启角色语音运行时"
	}
	characters := append([]AudioMixerCharacter(nil), audioMixerCharacters...)
	return &AudioMixerWorkspace{Installed: active, Owned: owned, RecoveryRequired: recoveryRequired, State: status.State, GameRunning: processErr == nil, Diagnostic: config.DiagnosticLog, Characters: characters, Volumes: config.CharacterVolumes, UIVolume: config.UIVolume, Detail: detail}, nil
}

func (a *App) DeployAudioMixerMod(request AudioMixerDeployRequest) error {
	audioMixerMu.Lock()
	defer audioMixerMu.Unlock()
	config, err := normalizeAudioMixerConfig(request)
	if err != nil {
		return err
	}
	if err := writeAudioMixerRuntimeConfig(config, true); err != nil {
		return err
	}
	if err := a.startRuntimeCompanion("audio", "runtime_audio"); err != nil {
		_ = writeAudioMixerRuntimeConfig(config, false)
		return err
	}
	return nil
}

func (a *App) RemoveAudioMixerMod(_ string) error {
	audioMixerMu.Lock()
	defer audioMixerMu.Unlock()
	config := readAudioMixerConfig()
	return a.stopOwnedRuntimeCompanion("audio", func() error { return writeAudioMixerRuntimeConfig(config, false) })
}

func audioMixerSortedIDs() []string {
	ids := make([]string, 0, len(audioMixerCharacters))
	for _, character := range audioMixerCharacters {
		ids = append(ids, character.ID)
	}
	sort.Strings(ids)
	return ids
}
