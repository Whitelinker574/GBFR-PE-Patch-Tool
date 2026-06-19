package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	steamAppID  = "881020"
	gameExeName = "granblue_fantasy_relink.exe"
	gameFolder  = "Granblue Fantasy Relink"
	appVersion  = "v1.5.1"
	repoOwner   = "BitterG"
	repoName    = "GBFR-PE-Patch-Tool"
)

// ── 补丁定义 ──

// PatchDef 描述一个补丁点
type PatchDef struct {
	ID         string // 唯一标识
	Name       string // 显示名称
	RVA        uint32 // 补丁目标 RVA
	OrigBytes  []byte // 原始字节（用于校验和恢复）
	PatchSize  int    // 补丁覆盖的字节数
	NeedCave   bool   // 是否需要代码跳板
	CallTarget uint32 // 跳板中 call 的目标 RVA（仅 NeedCave 时使用）
}

var patchDefs = []PatchDef{
	{
		ID:        "mission",
		Name:      "挑战次数",
		RVA:       0x003583FF,
		OrigBytes: []byte{0xB8, 0x3F, 0x42, 0x0F, 0x00, 0x41, 0x0F, 0x42, 0xC0},
		PatchSize: 9,
		NeedCave:  false,
	},
	{
		ID:        "likes",
		Name:      "点赞数值",
		RVA:       0x00A919CF,
		OrigBytes: []byte{0xB8, 0x3F, 0x42, 0x0F, 0x00, 0x0F, 0x42, 0xC6},
		PatchSize: 8,
		NeedCave:  false,
	},
}

// ── 状态结构 ──

type PatchStatus struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	State        string `json:"state"` // "original" | "patched" | "unknown"
	CurrentValue uint32 `json:"currentValue"`
	CurrentBytes string `json:"currentBytes"`
}

type StatusInfo struct {
	ExePath      string        `json:"exePath"`
	FileExists   bool          `json:"fileExists"`
	FileSize     int64         `json:"fileSize"`
	BackupExists bool          `json:"backupExists"`
	BackupSize   int64         `json:"backupSize"`
	Patches      []PatchStatus `json:"patches"`
}

type UpdateAsset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type UpdateInfo struct {
	CurrentVersion string        `json:"currentVersion"`
	LatestVersion  string        `json:"latestVersion"`
	HasUpdate      bool          `json:"hasUpdate"`
	ReleaseURL     string        `json:"releaseUrl"`
	Body           string        `json:"body"`
	Assets         []UpdateAsset `json:"assets"`
}

// ── App ──

type App struct {
	ctx               context.Context
	exePath           string
	hProcess          windows.Handle
	moduleBase        uintptr
	managerPtr        uintptr
	charaPID          uint32
	countdownAddr     uintptr
	faceAccessoryAddr uintptr
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) GetAppVersion() string {
	return appVersion
}

func (a *App) CheckUpdate() (UpdateInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return UpdateInfo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", repoName+"/"+appVersion)

	resp, err := client.Do(req)
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("检查更新失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{}, fmt.Errorf("检查更新失败: GitHub 返回 %s", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return UpdateInfo{}, fmt.Errorf("解析更新信息失败: %w", err)
	}

	info := UpdateInfo{
		CurrentVersion: appVersion,
		LatestVersion:  release.TagName,
		HasUpdate:      compareVersionTags(release.TagName, appVersion) > 0,
		ReleaseURL:     release.HTMLURL,
		Body:           release.Body,
	}
	for _, asset := range release.Assets {
		info.Assets = append(info.Assets, UpdateAsset{Name: asset.Name, URL: asset.URL})
	}
	return info, nil
}

func (a *App) OpenReleasePage(url string) error {
	if strings.TrimSpace(url) == "" {
		url = fmt.Sprintf("https://github.com/%s/%s/releases", repoOwner, repoName)
	}
	runtime.BrowserOpenURL(a.ctx, url)
	return nil
}

func compareVersionTags(a, b string) int {
	ap := parseVersionTag(a)
	bp := parseVersionTag(b)
	for i := 0; i < len(ap); i++ {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	return 0
}

func parseVersionTag(tag string) [3]int {
	var parts [3]int
	cleaned := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	fields := strings.Split(cleaned, ".")
	for i := 0; i < len(parts) && i < len(fields); i++ {
		text := fields[i]
		if idx := strings.IndexAny(text, "-+"); idx >= 0 {
			text = text[:idx]
		}
		if n, err := strconv.Atoi(text); err == nil {
			parts[i] = n
		}
	}
	return parts
}

// AutoDetect 自动扫描 Steam 安装路径
func (a *App) AutoDetect() string {
	for _, dir := range findSteamLibraryFolders() {
		candidate := filepath.Join(dir, "steamapps", "common", gameFolder, gameExeName)
		if _, err := os.Stat(candidate); err == nil {
			a.exePath = candidate
			return candidate
		}
	}
	return ""
}

// SetExePath 手动设置 exe 路径
func (a *App) SetExePath(path string) (StatusInfo, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return StatusInfo{}, fmt.Errorf("文件不存在: %s", path)
	}
	a.exePath = path
	return a.GetStatus(""), nil
}

// GetStatus 获取所有补丁点的状态
func (a *App) GetStatus(exePath string) StatusInfo {
	if exePath != "" {
		a.exePath = exePath
	}
	info := StatusInfo{ExePath: a.exePath}
	if a.exePath == "" {
		return info
	}

	bakPath := a.exePath + ".bak"
	if fi, err := os.Stat(a.exePath); err == nil {
		info.FileExists = true
		info.FileSize = fi.Size()
	}
	if fi, err := os.Stat(bakPath); err == nil {
		info.BackupExists = true
		info.BackupSize = fi.Size()
	}
	if !info.FileExists {
		return info
	}

	data, err := os.ReadFile(a.exePath)
	if err != nil {
		return info
	}

	for _, def := range patchDefs {
		ps := PatchStatus{ID: def.ID, Name: def.Name, State: "unknown"}
		offset, ok := rvaToFileOffset(data, def.RVA)
		if !ok || int(offset)+def.PatchSize > len(data) {
			info.Patches = append(info.Patches, ps)
			continue
		}
		target := data[offset : offset+uint32(def.PatchSize)]
		ps.CurrentBytes = bytesToHex(target)

		if bytesEqual(target, def.OrigBytes) {
			ps.State = "original"
		} else if def.NeedCave {
			// 跳板补丁：检查是否为 JMP rel32 + NOPs
			if target[0] == 0xE9 && allNop(target[5:]) {
				ps.State = "patched"
				// 读取跳板中的值
				ps.CurrentValue = readCaveValue(data, offset, def)
			}
		} else {
			// 直接补丁：检查 B8 xx xx xx xx + NOP 填充
			if target[0] == 0xB8 && isNopFill(target[5:]) {
				ps.State = "patched"
				ps.CurrentValue = binary.LittleEndian.Uint32(target[1:5])
			}
		}
		info.Patches = append(info.Patches, ps)
	}
	return info
}

// PatchFile 对指定补丁点应用补丁
func (a *App) PatchFile(patchID string, value uint32) error {
	if a.exePath == "" {
		return fmt.Errorf("未选择文件")
	}

	def := findPatchDef(patchID)
	if def == nil {
		return fmt.Errorf("未知补丁: %s", patchID)
	}

	data, err := os.ReadFile(a.exePath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	offset, ok := rvaToFileOffset(data, def.RVA)
	if !ok {
		return fmt.Errorf("无法定位 RVA 0x%X", def.RVA)
	}
	if int(offset)+def.PatchSize > len(data) {
		return fmt.Errorf("补丁超出文件范围")
	}

	target := data[offset : offset+uint32(def.PatchSize)]

	// 校验：必须是原始字节或已补丁状态
	isOrig := bytesEqual(target, def.OrigBytes)
	isPatched := false
	if def.NeedCave {
		isPatched = target[0] == 0xE9 && allNop(target[5:])
	} else {
		isPatched = target[0] == 0xB8 && isNopFill(target[5:])
	}
	if !isOrig && !isPatched {
		return fmt.Errorf("目标字节异常，拒绝补丁\n当前: %s", bytesToHex(target))
	}

	if def.NeedCave {
		err = applyCavePatch(data, offset, *def, value, isPatched)
	} else {
		err = applyDirectPatch(data, offset, *def, value)
	}
	if err != nil {
		return err
	}

	return os.WriteFile(a.exePath, data, 0644)
}

// BackupFile 创建备份
func (a *App) BackupFile(force bool) error {
	if a.exePath == "" {
		return fmt.Errorf("未选择文件")
	}
	bakPath := a.exePath + ".bak"
	if _, err := os.Stat(a.exePath); os.IsNotExist(err) {
		return fmt.Errorf("目标文件不存在")
	}
	if !force {
		if _, err := os.Stat(bakPath); err == nil {
			return fmt.Errorf("备份已存在，使用强制覆盖选项")
		}
	}
	data, err := os.ReadFile(a.exePath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}
	return os.WriteFile(bakPath, data, 0644)
}

// RestoreFile 从备份恢复
func (a *App) RestoreFile() error {
	if a.exePath == "" {
		return fmt.Errorf("未选择文件")
	}
	bakPath := a.exePath + ".bak"
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在")
	}
	data, err := os.ReadFile(bakPath)
	if err != nil {
		return fmt.Errorf("读取备份失败: %w", err)
	}
	return os.WriteFile(a.exePath, data, 0644)
}

// ── 补丁实现 ──

// applyDirectPatch 直接替换字节（mov eax,imm32 + NOP 填充）
func applyDirectPatch(data []byte, offset uint32, def PatchDef, value uint32) error {
	patch := make([]byte, def.PatchSize)
	patch[0] = 0xB8
	binary.LittleEndian.PutUint32(patch[1:5], value)
	// 剩余字节填 NOP
	switch def.PatchSize - 5 {
	case 4: // 9 字节: mov eax,imm32 + 4-byte NOP (0F 1F 40 00)
		patch[5] = 0x0F
		patch[6] = 0x1F
		patch[7] = 0x40
		patch[8] = 0x00
	case 3: // 8 字节: mov eax,imm32 + 3-byte NOP (0F 1F 00)
		patch[5] = 0x0F
		patch[6] = 0x1F
		patch[7] = 0x00
	default: // 其他情况用单字节 NOP 填充
		for i := 5; i < def.PatchSize; i++ {
			patch[i] = 0x90
		}
	}
	copy(data[offset:], patch)
	return nil
}

// applyCavePatch 使用代码跳板（用于 likes 类型）
func applyCavePatch(data []byte, offset uint32, def PatchDef, value uint32, alreadyPatched bool) error {
	// 跳板代码布局（17 字节）:
	//   B8 xx xx xx xx    ; mov eax, <value>
	//   89 01             ; mov [rcx], eax
	//   E8 yy yy yy yy   ; call <target>
	//   E9 zz zz zz zz   ; jmp back
	const caveSize = 17

	var caveOffset uint32
	var caveRVA uint32

	if alreadyPatched {
		// 已有跳板，读取 JMP 目标找到 cave 位置
		jmpRel := int32(binary.LittleEndian.Uint32(data[offset+1 : offset+5]))
		jmpNextRVA := def.RVA + 5
		caveRVA = uint32(int32(jmpNextRVA) + jmpRel)
		var ok bool
		caveOffset, ok = rvaToFileOffset(data, caveRVA)
		if !ok {
			return fmt.Errorf("无法定位已有跳板")
		}
	} else {
		// 首次补丁：在 .text 段末尾找空间
		var ok bool
		caveRVA, caveOffset, ok = findCaveSpace(data, caveSize)
		if !ok {
			return fmt.Errorf("找不到可用的代码空间")
		}
	}

	// 写跳板代码
	cave := make([]byte, caveSize)
	cave[0] = 0xB8
	binary.LittleEndian.PutUint32(cave[1:5], value)
	cave[5] = 0x89
	cave[6] = 0x01 // mov [rcx], eax

	// call <target>: E8 rel32, rel32 = target - (cave_call_rva + 5)
	cave[7] = 0xE8
	callRVA := caveRVA + 7
	callRel := int32(def.CallTarget) - int32(callRVA+5)
	binary.LittleEndian.PutUint32(cave[8:12], uint32(callRel))

	// jmp back: E9 rel32, rel32 = return_rva - (cave_jmp_rva + 5)
	cave[12] = 0xE9
	returnRVA := def.RVA + uint32(def.PatchSize)
	jmpRVA := caveRVA + 12
	jmpRel := int32(returnRVA) - int32(jmpRVA+5)
	binary.LittleEndian.PutUint32(cave[13:17], uint32(jmpRel))

	copy(data[caveOffset:], cave)

	// 写原始位置的 JMP + NOPs
	patch := make([]byte, def.PatchSize)
	patch[0] = 0xE9
	origJmpRel := int32(caveRVA) - int32(def.RVA+5)
	binary.LittleEndian.PutUint32(patch[1:5], uint32(origJmpRel))
	for i := 5; i < def.PatchSize; i++ {
		patch[i] = 0x90 // NOP
	}
	copy(data[offset:], patch)

	return nil
}

// findCaveSpace 在 PE 段的 rawData 末尾找零填充区，
// 并扩展 VirtualSize + SizeOfImage 确保运行时该区域被映射到内存。
func findCaveSpace(data []byte, size int) (rva uint32, fileOffset uint32, ok bool) {
	peOffset := binary.LittleEndian.Uint32(data[0x3C:0x40])
	coffHeader := peOffset + 4
	numSections := binary.LittleEndian.Uint16(data[coffHeader+2 : coffHeader+4])
	optHeaderSize := binary.LittleEndian.Uint16(data[coffHeader+16 : coffHeader+18])
	sectionStart := coffHeader + 20 + uint32(optHeaderSize)
	optHeader := coffHeader + 20

	// SizeOfImage 在 optional header offset 56 (PE32+)
	sizeOfImageOff := optHeader + 56
	// SectionAlignment 在 optional header offset 32
	sectionAlignment := binary.LittleEndian.Uint32(data[optHeader+32 : optHeader+36])

	for i := uint16(0); i < numSections; i++ {
		off := sectionStart + uint32(i)*40
		if int(off)+40 > len(data) {
			continue
		}
		virtualSize := binary.LittleEndian.Uint32(data[off+8 : off+12])
		virtualAddr := binary.LittleEndian.Uint32(data[off+12 : off+16])
		rawSize := binary.LittleEndian.Uint32(data[off+16 : off+20])
		rawPtr := binary.LittleEndian.Uint32(data[off+20 : off+24])
		characteristics := binary.LittleEndian.Uint32(data[off+36 : off+40])

		isExecutable := (characteristics & 0x20000020) != 0
		if !isExecutable || rawSize == 0 || rawPtr == 0 {
			continue
		}

		rawEnd := rawPtr + rawSize
		if rawEnd > uint32(len(data)) {
			rawEnd = uint32(len(data))
		}

		// 从段 raw 末尾往前找连续零字节
		zeroCount := 0
		for pos := int(rawEnd) - 1; pos >= int(rawPtr) && pos >= 0; pos-- {
			if data[pos] == 0 {
				zeroCount++
			} else {
				break
			}
		}
		if zeroCount < size+16 {
			continue
		}

		caveFileOff := rawEnd - uint32(size) - 8
		caveRVA := virtualAddr + (caveFileOff - rawPtr)

		// 关键：如果 cave 超出 virtualSize，扩展 VirtualSize 使其被映射到内存
		caveEnd := caveRVA - virtualAddr + uint32(size) + 8
		if caveEnd > virtualSize {
			// 对齐到 SectionAlignment
			newVirtualSize := alignUp(caveEnd, sectionAlignment)
			binary.LittleEndian.PutUint32(data[off+8:off+12], newVirtualSize)

			// 更新 SizeOfImage = 最后一个段的 VirtualAddress + 对齐后的 VirtualSize
			// 找最后一个段来计算
			newSizeOfImage := uint32(0)
			for j := uint16(0); j < numSections; j++ {
				soff := sectionStart + uint32(j)*40
				va := binary.LittleEndian.Uint32(data[soff+12 : soff+16])
				vs := binary.LittleEndian.Uint32(data[soff+8 : soff+12])
				end := va + alignUp(vs, sectionAlignment)
				if end > newSizeOfImage {
					newSizeOfImage = end
				}
			}
			binary.LittleEndian.PutUint32(data[sizeOfImageOff:sizeOfImageOff+4], newSizeOfImage)
		}

		return caveRVA, caveFileOff, true
	}
	return 0, 0, false
}

func alignUp(value, alignment uint32) uint32 {
	if alignment == 0 {
		return value
	}
	return (value + alignment - 1) & ^(alignment - 1)
}

// readCaveValue 从跳板中读取当前值
func readCaveValue(data []byte, offset uint32, def PatchDef) uint32 {
	if data[offset] != 0xE9 {
		return 0
	}
	jmpRel := int32(binary.LittleEndian.Uint32(data[offset+1 : offset+5]))
	caveRVA := uint32(int32(def.RVA+5) + jmpRel)
	caveOffset, ok := rvaToFileOffset(data, caveRVA)
	if !ok || int(caveOffset)+5 > len(data) {
		return 0
	}
	if data[caveOffset] != 0xB8 {
		return 0
	}
	return binary.LittleEndian.Uint32(data[caveOffset+1 : caveOffset+5])
}

func allNop(b []byte) bool {
	for _, v := range b {
		if v != 0x90 {
			return false
		}
	}
	return true
}

// isNopFill 检查字节是否为已知的多字节 NOP 填充
func isNopFill(b []byte) bool {
	switch len(b) {
	case 4: // 0F 1F 40 00
		return b[0] == 0x0F && b[1] == 0x1F && b[2] == 0x40 && b[3] == 0x00
	case 3: // 0F 1F 00
		return b[0] == 0x0F && b[1] == 0x1F && b[2] == 0x00
	default:
		return allNop(b)
	}
}

func findPatchDef(id string) *PatchDef {
	for i := range patchDefs {
		if patchDefs[i].ID == id {
			return &patchDefs[i]
		}
	}
	return nil
}

// ── PE / 工具函数 ──

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func bytesToHex(b []byte) string {
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = fmt.Sprintf("%02X", v)
	}
	return strings.Join(parts, " ")
}

func rvaToFileOffset(data []byte, rva uint32) (uint32, bool) {
	if len(data) < 64 {
		return 0, false
	}
	if data[0] != 'M' || data[1] != 'Z' {
		return 0, false
	}
	peOffset := binary.LittleEndian.Uint32(data[0x3C:0x40])
	if int(peOffset)+24 > len(data) {
		return 0, false
	}
	if data[peOffset] != 'P' || data[peOffset+1] != 'E' || data[peOffset+2] != 0 || data[peOffset+3] != 0 {
		return 0, false
	}
	coffHeader := peOffset + 4
	numSections := binary.LittleEndian.Uint16(data[coffHeader+2 : coffHeader+4])
	optHeaderSize := binary.LittleEndian.Uint16(data[coffHeader+16 : coffHeader+18])
	optHeader := coffHeader + 20
	if int(optHeader)+2 > len(data) {
		return 0, false
	}
	magic := binary.LittleEndian.Uint16(data[optHeader : optHeader+2])
	if magic != 0x020B {
		return 0, false
	}
	sectionStart := optHeader + uint32(optHeaderSize)
	for i := uint16(0); i < numSections; i++ {
		off := sectionStart + uint32(i)*40
		if int(off)+40 > len(data) {
			return 0, false
		}
		virtualSize := binary.LittleEndian.Uint32(data[off+8 : off+12])
		virtualAddr := binary.LittleEndian.Uint32(data[off+12 : off+16])
		rawSize := binary.LittleEndian.Uint32(data[off+16 : off+20])
		rawPtr := binary.LittleEndian.Uint32(data[off+20 : off+24])
		span := rawSize
		if virtualSize > span {
			span = virtualSize
		}
		if rva >= virtualAddr && rva < virtualAddr+span {
			return rawPtr + (rva - virtualAddr), true
		}
	}
	return 0, false
}

// ── Steam 路径扫描 ──

func findSteamLibraryFolders() []string {
	var dirs []string
	steamPath := ""
	for _, keyPath := range []string{`SOFTWARE\Valve\Steam`, `SOFTWARE\WOW6432Node\Valve\Steam`} {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		val, _, err := k.GetStringValue("InstallPath")
		k.Close()
		if err == nil && val != "" {
			steamPath = val
			dirs = append(dirs, val)
			break
		}
	}
	if steamPath == "" {
		k, err := registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Valve\Steam`, registry.QUERY_VALUE)
		if err == nil {
			val, _, err := k.GetStringValue("SteamPath")
			k.Close()
			if err == nil && val != "" {
				steamPath = filepath.FromSlash(val)
				dirs = append(dirs, steamPath)
			}
		}
	}
	if steamPath != "" {
		vdfPath := filepath.Join(steamPath, "steamapps", "libraryfolders.vdf")
		if data, err := os.ReadFile(vdfPath); err == nil {
			dirs = append(dirs, parseLibraryPaths(string(data))...)
		}
	}
	for _, fb := range []string{
		`C:\Program Files (x86)\Steam`, `C:\Program Files\Steam`,
		`D:\Steam`, `D:\SteamLibrary`, `E:\Steam`, `E:\SteamLibrary`,
	} {
		found := false
		for _, d := range dirs {
			if strings.EqualFold(d, fb) {
				found = true
				break
			}
		}
		if !found {
			dirs = append(dirs, fb)
		}
	}
	return dirs
}

func parseLibraryPaths(content string) []string {
	var paths []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `"path"`) {
			parts := strings.SplitN(line, `"path"`, 2)
			if len(parts) < 2 {
				continue
			}
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"`)
			val = strings.ReplaceAll(val, `\\`, `\`)
			if val != "" {
				paths = append(paths, val)
			}
		}
	}
	return paths
}

// ── 角色使用次数 (运行时内存读写) ──

const (
	charaProcessName = "granblue_fantasy_relink.exe"
	managerPtrRVA    = 0x68CFBB8
	charalistOffset  = 0xD80
	countOffset      = 0x3114
	charaStride      = 0x3120
	maxCharacters    = 40
)

var charaNames = [maxCharacters]string{
	"古兰", "姬塔", "卡塔莉娜", "拉卡姆", "伊欧", "欧根",
	"", "萝赛塔", "冈达葛萨", "菲莉", "兰斯洛特", "巴恩", "珀西瓦尔",
	"", "齐格飞", "夏洛特", "索恩", "尤达拉哈", "娜露梅",
	"", "塞达", "伊德", "巴萨拉卡",
	"", "卡莉奥丝特罗",
	"", "", "圣德芬", "希耶提",
	"", "", "", "", "", "", "", "", "", "", "",
}

type CharaProcessInfo struct {
	PID        uint32 `json:"pid"`
	ModuleBase uint64 `json:"moduleBase"`
	Manager    uint64 `json:"manager"`
	Connected  bool   `json:"connected"`
}

type CharaInfo struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Count int32  `json:"count"`
}

// CharaAttach finds the game process, opens a handle, reads module base and manager pointer.
func (a *App) CharaAttach() (CharaProcessInfo, error) {
	// Close existing handle if any
	if a.hProcess != 0 {
		windows.CloseHandle(a.hProcess)
		a.hProcess = 0
	}

	pid, err := findProcessByName(charaProcessName)
	if err != nil {
		return CharaProcessInfo{}, fmt.Errorf("未找到游戏进程，请先启动游戏")
	}

	h, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, pid)
	if err != nil {
		return CharaProcessInfo{}, fmt.Errorf("无法打开进程 (错误 %v)，请以管理员身份运行", err)
	}

	modBase, err := getModuleBase(h)
	if err != nil {
		windows.CloseHandle(h)
		return CharaProcessInfo{}, fmt.Errorf("无法获取模块基址 (ptrSize=%d): %v", unsafe.Sizeof(uintptr(0)), err)
	}

	// Read manager pointer
	ptrAddr := modBase + managerPtrRVA
	var manager uintptr
	err = readProcessMemory(h, ptrAddr, unsafe.Pointer(&manager), unsafe.Sizeof(manager))
	if err != nil || manager == 0 {
		windows.CloseHandle(h)
		return CharaProcessInfo{}, fmt.Errorf("管理器指针为空，请确保已进入游戏存档")
	}

	a.hProcess = h
	a.moduleBase = modBase
	a.managerPtr = manager
	a.charaPID = pid

	return CharaProcessInfo{
		PID:        pid,
		ModuleBase: uint64(modBase),
		Manager:    uint64(manager),
		Connected:  true,
	}, nil
}

// CharaDetach closes the process handle.
func (a *App) CharaDetach() {
	if a.hProcess != 0 {
		windows.CloseHandle(a.hProcess)
		a.hProcess = 0
	}
	a.moduleBase = 0
	a.managerPtr = 0
	a.charaPID = 0
	a.countdownAddr = 0
	a.faceAccessoryAddr = 0
}

// CharaGetAll reads all character counts, returns valid characters (skipping empty slots).
func (a *App) CharaGetAll() ([]CharaInfo, error) {
	if a.hProcess == 0 {
		return nil, fmt.Errorf("未连接游戏进程")
	}

	// Re-read manager pointer each time (handles game restart)
	var manager uintptr
	ptrAddr := a.moduleBase + managerPtrRVA
	err := readProcessMemory(a.hProcess, ptrAddr, unsafe.Pointer(&manager), unsafe.Sizeof(manager))
	if err != nil || manager == 0 {
		return nil, fmt.Errorf("管理器指针无效，请确保在游戏存档中")
	}
	a.managerPtr = manager

	var result []CharaInfo
	for i := 0; i < maxCharacters; i++ {
		countAddr := manager + charalistOffset + uintptr(i)*charaStride + countOffset
		var val int32
		err := readProcessMemory(a.hProcess, countAddr, unsafe.Pointer(&val), unsafe.Sizeof(val))
		if err != nil {
			continue
		}
		if charaNames[i] == "" && val == 0 {
			continue // skip empty slots
		}
		if val == -1 {
			continue // skip uninitialized slots
		}
		name := charaNames[i]
		if name == "" {
			name = fmt.Sprintf("槽位 %d", i)
		}
		result = append(result, CharaInfo{Index: i, Name: name, Count: val})
	}
	return result, nil
}

// CharaSetOne sets a single character's count by slot index.
func (a *App) CharaSetOne(index int, value int) error {
	if a.hProcess == 0 {
		return fmt.Errorf("未连接游戏进程")
	}
	if index < 0 || index >= maxCharacters {
		return fmt.Errorf("无效的角色索引: %d", index)
	}

	// Re-read manager pointer
	var manager uintptr
	ptrAddr := a.moduleBase + managerPtrRVA
	err := readProcessMemory(a.hProcess, ptrAddr, unsafe.Pointer(&manager), unsafe.Sizeof(manager))
	if err != nil || manager == 0 {
		return fmt.Errorf("管理器指针无效")
	}

	countAddr := manager + charalistOffset + uintptr(index)*charaStride + countOffset
	val := int32(value)
	return writeProcessMemory(a.hProcess, countAddr, unsafe.Pointer(&val), unsafe.Sizeof(val))
}

// CharaSetAll sets all valid character counts to the given value, returns number modified.
func (a *App) CharaSetAll(value int) (int, error) {
	if a.hProcess == 0 {
		return 0, fmt.Errorf("未连接游戏进程")
	}

	// Re-read manager pointer
	var manager uintptr
	ptrAddr := a.moduleBase + managerPtrRVA
	err := readProcessMemory(a.hProcess, ptrAddr, unsafe.Pointer(&manager), unsafe.Sizeof(manager))
	if err != nil || manager == 0 {
		return 0, fmt.Errorf("管理器指针无效")
	}

	modified := 0
	newVal := int32(value)
	for i := 0; i < maxCharacters; i++ {
		countAddr := manager + charalistOffset + uintptr(i)*charaStride + countOffset
		var cur int32
		err := readProcessMemory(a.hProcess, countAddr, unsafe.Pointer(&cur), unsafe.Sizeof(cur))
		if err != nil {
			continue
		}
		if cur == -1 {
			continue // skip empty slots
		}
		err = writeProcessMemory(a.hProcess, countAddr, unsafe.Pointer(&newVal), unsafe.Sizeof(newVal))
		if err == nil {
			modified++
		}
	}
	return modified, nil
}

// ── 角色脸部符文显示 (运行时 JE/JNE 切换) ──

var faceAccessoryPattern = []byte{
	0x49, 0x8B, 0x45, 0,
	0x4C, 0x39, 0xF0,
	0x0F, 0, 0, 0, 0, 0,
	0x4C, 0x89, 0xE9,
}

var faceAccessoryMask = []bool{
	true, true, true, false,
	true, true, true,
	true, false, false, false, false, false,
	true, true, true,
}

type FaceAccessoryStatus struct {
	Found        bool   `json:"found"`
	Address      uint64 `json:"address"`
	RVA          uint64 `json:"rva"`
	Hidden       bool   `json:"hidden"`
	JumpOpcode   string `json:"jumpOpcode"`
	CurrentBytes string `json:"currentBytes"`
}

func (a *App) FaceAccessoryScan() (FaceAccessoryStatus, error) {
	if err := a.ensureGameProcess(); err != nil {
		return FaceAccessoryStatus{}, err
	}
	addr, err := a.scanPatternUnique(faceAccessoryPattern, faceAccessoryMask, "脸部符文特征")
	if err != nil {
		a.faceAccessoryAddr = 0
		return FaceAccessoryStatus{}, err
	}
	a.faceAccessoryAddr = addr
	return a.readFaceAccessoryStatus(addr)
}

func (a *App) FaceAccessoryGetStatus() (FaceAccessoryStatus, error) {
	if err := a.ensureGameProcess(); err != nil {
		return FaceAccessoryStatus{}, err
	}
	if a.faceAccessoryAddr == 0 {
		return a.FaceAccessoryScan()
	}
	status, err := a.readFaceAccessoryStatus(a.faceAccessoryAddr)
	if err != nil {
		a.faceAccessoryAddr = 0
		return a.FaceAccessoryScan()
	}
	return status, nil
}

func (a *App) FaceAccessorySetHidden(hidden bool) (FaceAccessoryStatus, error) {
	status, err := a.FaceAccessoryGetStatus()
	if err != nil {
		return FaceAccessoryStatus{}, err
	}
	if !status.Found || a.faceAccessoryAddr == 0 {
		return FaceAccessoryStatus{}, fmt.Errorf("未定位脸部符文指令")
	}
	opcode := byte(0x84)
	if hidden {
		opcode = 0x85
	}
	if err := writeCodeMemory(a.hProcess, a.faceAccessoryAddr+8, []byte{opcode}); err != nil {
		return FaceAccessoryStatus{}, fmt.Errorf("写入脸部符文显示开关失败: %w", err)
	}
	return a.readFaceAccessoryStatus(a.faceAccessoryAddr)
}

func (a *App) readFaceAccessoryStatus(addr uintptr) (FaceAccessoryStatus, error) {
	buf := make([]byte, len(faceAccessoryPattern))
	if err := readProcessMemory(a.hProcess, addr, unsafe.Pointer(&buf[0]), uintptr(len(buf))); err != nil {
		return FaceAccessoryStatus{}, fmt.Errorf("读取脸部符文指令失败: %w", err)
	}
	if !matchPattern(buf, faceAccessoryPattern, faceAccessoryMask) {
		return FaceAccessoryStatus{}, fmt.Errorf("脸部符文指令字节已变化，请重新扫描")
	}
	if buf[8] != 0x84 && buf[8] != 0x85 {
		return FaceAccessoryStatus{}, fmt.Errorf("脸部符文跳转 opcode 异常: 0x%02X", buf[8])
	}
	jumpOpcode := "JE"
	if buf[8] == 0x85 {
		jumpOpcode = "JNE"
	}
	return FaceAccessoryStatus{
		Found:        true,
		Address:      uint64(addr),
		RVA:          uint64(addr - a.moduleBase),
		Hidden:       buf[8] == 0x85,
		JumpOpcode:   jumpOpcode,
		CurrentBytes: bytesToHex(buf),
	}, nil
}

// ── 固定倒计时 (运行时指令立即数修改) ──

var countdownPattern = []byte{
	0x48, 0xB8, 0, 0, 0, 0, 0, 0, 0, 0,
	0x48, 0x89, 0x87, 0, 0, 0, 0,
	0xC5, 0xFA, 0x10, 0x05,
}

var countdownMask = []bool{
	true, true, false, false, false, false, false, false, false, false,
	true, true, true, false, false, false, false,
	true, true, true, true,
}

type CountdownStatus struct {
	Found        bool    `json:"found"`
	Address      uint64  `json:"address"`
	RVA          uint64  `json:"rva"`
	Value1       float32 `json:"value1"`
	Value2       float32 `json:"value2"`
	CurrentBytes string  `json:"currentBytes"`
}

func (a *App) CountdownScan() (CountdownStatus, error) {
	if err := a.ensureGameProcess(); err != nil {
		return CountdownStatus{}, err
	}

	addr, err := a.scanCountdownPattern()
	if err != nil {
		a.countdownAddr = 0
		return CountdownStatus{}, err
	}
	a.countdownAddr = addr
	return a.readCountdownStatus(addr)
}

func (a *App) CountdownGetStatus() (CountdownStatus, error) {
	if err := a.ensureGameProcess(); err != nil {
		return CountdownStatus{}, err
	}
	if a.countdownAddr == 0 {
		return a.CountdownScan()
	}
	status, err := a.readCountdownStatus(a.countdownAddr)
	if err != nil {
		a.countdownAddr = 0
		return a.CountdownScan()
	}
	return status, nil
}

func (a *App) CountdownSet(value float64) (CountdownStatus, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 9999 {
		return CountdownStatus{}, fmt.Errorf("请输入 0 到 9999 之间的有效倒计时数值")
	}
	status, err := a.CountdownGetStatus()
	if err != nil {
		return CountdownStatus{}, err
	}
	if !status.Found || a.countdownAddr == 0 {
		return CountdownStatus{}, fmt.Errorf("未定位倒计时指令")
	}

	val := float32(value)
	bits := math.Float32bits(val)
	patch := make([]byte, 8)
	binary.LittleEndian.PutUint32(patch[0:4], bits)
	binary.LittleEndian.PutUint32(patch[4:8], bits)

	if err := writeCodeMemory(a.hProcess, a.countdownAddr+2, patch); err != nil {
		return CountdownStatus{}, fmt.Errorf("写入倒计时失败: %w", err)
	}
	return a.readCountdownStatus(a.countdownAddr)
}

func (a *App) ensureGameProcess() error {
	if a.hProcess != 0 && a.moduleBase != 0 {
		return nil
	}
	pid, err := findProcessByName(charaProcessName)
	if err != nil {
		return fmt.Errorf("未找到游戏进程，请先启动游戏")
	}
	h, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, pid)
	if err != nil {
		return fmt.Errorf("无法打开进程 (错误 %v)，请以管理员身份运行", err)
	}
	modBase, err := getModuleBase(h)
	if err != nil {
		windows.CloseHandle(h)
		return fmt.Errorf("无法获取模块基址: %v", err)
	}
	a.hProcess = h
	a.moduleBase = modBase
	a.charaPID = pid
	return nil
}

func (a *App) scanCountdownPattern() (uintptr, error) {
	return a.scanPatternUnique(countdownPattern, countdownMask, "倒计时特征")
}

func (a *App) scanPatternUnique(pattern []byte, mask []bool, label string) (uintptr, error) {
	moduleSize, err := getRemoteModuleSize(a.hProcess, a.moduleBase)
	if err != nil {
		return 0, err
	}
	const chunkSize uintptr = 0x10000
	patternLen := len(pattern)
	var matches []uintptr
	var carry []byte
	var carryBase uintptr

	for off := uintptr(0); off < moduleSize; off += chunkSize {
		size := chunkSize
		if off+size > moduleSize {
			size = moduleSize - off
		}
		buf := make([]byte, int(size))
		addr := a.moduleBase + off
		if err := readProcessMemory(a.hProcess, addr, unsafe.Pointer(&buf[0]), uintptr(len(buf))); err != nil {
			carry = nil
			continue
		}

		scanBuf := buf
		scanBase := addr
		if len(carry) > 0 {
			scanBuf = append(append([]byte{}, carry...), buf...)
			scanBase = carryBase
		}
		matches = append(matches, findPatternMatches(scanBuf, scanBase, pattern, mask)...)
		if len(matches) > 1 {
			return 0, fmt.Errorf("%s命中多个位置: %d", label, len(matches))
		}

		if len(buf) >= patternLen-1 {
			carry = append([]byte{}, buf[len(buf)-patternLen+1:]...)
			carryBase = addr + uintptr(len(buf)-patternLen+1)
		} else {
			carry = append(append([]byte{}, carry...), buf...)
			if len(carry) > patternLen-1 {
				carry = carry[len(carry)-patternLen+1:]
				carryBase = addr + uintptr(len(buf)-len(carry))
			}
		}
	}

	if len(matches) == 0 {
		return 0, fmt.Errorf("未找到%s码", label)
	}
	return matches[0], nil
}

func (a *App) readCountdownStatus(addr uintptr) (CountdownStatus, error) {
	buf := make([]byte, len(countdownPattern))
	if err := readProcessMemory(a.hProcess, addr, unsafe.Pointer(&buf[0]), uintptr(len(buf))); err != nil {
		return CountdownStatus{}, fmt.Errorf("读取倒计时指令失败: %w", err)
	}
	if !matchPattern(buf, countdownPattern, countdownMask) {
		return CountdownStatus{}, fmt.Errorf("倒计时指令字节已变化，请重新扫描")
	}
	v1 := math.Float32frombits(binary.LittleEndian.Uint32(buf[2:6]))
	v2 := math.Float32frombits(binary.LittleEndian.Uint32(buf[6:10]))
	return CountdownStatus{
		Found:        true,
		Address:      uint64(addr),
		RVA:          uint64(addr - a.moduleBase),
		Value1:       v1,
		Value2:       v2,
		CurrentBytes: bytesToHex(buf),
	}, nil
}

func findPatternMatches(buf []byte, base uintptr, pattern []byte, mask []bool) []uintptr {
	if len(buf) < len(pattern) {
		return nil
	}
	var matches []uintptr
	for i := 0; i <= len(buf)-len(pattern); i++ {
		if matchPattern(buf[i:i+len(pattern)], pattern, mask) {
			matches = append(matches, base+uintptr(i))
		}
	}
	return matches
}

func matchPattern(buf []byte, pattern []byte, mask []bool) bool {
	if len(buf) < len(pattern) {
		return false
	}
	for i := range pattern {
		if mask[i] && buf[i] != pattern[i] {
			return false
		}
	}
	return true
}

func getRemoteModuleSize(h windows.Handle, moduleBase uintptr) (uintptr, error) {
	headers := make([]byte, 0x400)
	if err := readProcessMemory(h, moduleBase, unsafe.Pointer(&headers[0]), uintptr(len(headers))); err != nil {
		return 0, fmt.Errorf("读取模块头失败: %w", err)
	}
	if headers[0] != 'M' || headers[1] != 'Z' {
		return 0, fmt.Errorf("模块 DOS 头无效")
	}
	peOff := int(binary.LittleEndian.Uint32(headers[0x3C:0x40]))
	if peOff <= 0 || peOff+0x5C > len(headers) {
		return 0, fmt.Errorf("模块 PE 头偏移无效")
	}
	if headers[peOff] != 'P' || headers[peOff+1] != 'E' || headers[peOff+2] != 0 || headers[peOff+3] != 0 {
		return 0, fmt.Errorf("模块 PE 头无效")
	}
	sizeOfImage := binary.LittleEndian.Uint32(headers[peOff+0x18+0x38 : peOff+0x18+0x3C])
	if sizeOfImage == 0 {
		return 0, fmt.Errorf("模块 SizeOfImage 无效")
	}
	return uintptr(sizeOfImage), nil
}

// ── Windows 进程操作辅助函数 ──

func findProcessByName(name string) (uint32, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	err = windows.Process32First(snap, &pe)
	if err != nil {
		return 0, err
	}
	for {
		exeName := windows.UTF16ToString(pe.ExeFile[:])
		if strings.EqualFold(exeName, name) {
			return pe.ProcessID, nil
		}
		err = windows.Process32Next(snap, &pe)
		if err != nil {
			break
		}
	}
	return 0, fmt.Errorf("进程未找到: %s", name)
}

var (
	modNtdll                      = windows.NewLazySystemDLL("ntdll.dll")
	procNtQueryInformationProcess = modNtdll.NewProc("NtQueryInformationProcess")
)

// getModuleBase reads the image base address from the remote process's PEB.
// This avoids module enumeration APIs which can fail with ERROR_PARTIAL_COPY.
func getModuleBase(hProcess windows.Handle) (uintptr, error) {
	// PROCESS_BASIC_INFORMATION (64-bit layout):
	//   ExitStatus          uintptr  (offset 0)
	//   PebBaseAddress      uintptr  (offset 8)
	//   AffinityMask        uintptr  (offset 16)
	//   BasePriority        uintptr  (offset 24)
	//   UniqueProcessId     uintptr  (offset 32)
	//   InheritedFromUnique uintptr  (offset 40)
	type processBasicInformation struct {
		ExitStatus                   uintptr
		PebBaseAddress               uintptr
		AffinityMask                 uintptr
		BasePriority                 uintptr
		UniqueProcessId              uintptr
		InheritedFromUniqueProcessId uintptr
	}

	var pbi processBasicInformation
	var retLen uint32
	r1, _, _ := procNtQueryInformationProcess.Call(
		uintptr(hProcess),
		0, // ProcessBasicInformation
		uintptr(unsafe.Pointer(&pbi)),
		unsafe.Sizeof(pbi),
		uintptr(unsafe.Pointer(&retLen)),
	)
	if r1 != 0 {
		return 0, fmt.Errorf("NtQueryInformationProcess 失败: NTSTATUS 0x%X", r1)
	}
	if pbi.PebBaseAddress == 0 {
		return 0, fmt.Errorf("PEB 地址为空")
	}

	// Read ImageBaseAddress from PEB (offset 0x10 in 64-bit PEB)
	var imageBase uintptr
	err := readProcessMemory(hProcess, pbi.PebBaseAddress+0x10, unsafe.Pointer(&imageBase), unsafe.Sizeof(imageBase))
	if err != nil {
		return 0, fmt.Errorf("读取 PEB.ImageBaseAddress 失败: %v", err)
	}
	if imageBase == 0 {
		return 0, fmt.Errorf("ImageBaseAddress 为空")
	}
	return imageBase, nil
}

func readProcessMemory(h windows.Handle, addr uintptr, buf unsafe.Pointer, size uintptr) error {
	var read uintptr
	return windows.ReadProcessMemory(h, addr, (*byte)(buf), size, &read)
}

func writeProcessMemory(h windows.Handle, addr uintptr, buf unsafe.Pointer, size uintptr) error {
	var written uintptr
	return windows.WriteProcessMemory(h, addr, (*byte)(buf), size, &written)
}

func writeCodeMemory(h windows.Handle, addr uintptr, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var oldProtect uint32
	if err := windows.VirtualProtectEx(h, addr, uintptr(len(data)), windows.PAGE_EXECUTE_READWRITE, &oldProtect); err != nil {
		return err
	}
	writeErr := writeProcessMemory(h, addr, unsafe.Pointer(&data[0]), uintptr(len(data)))
	var restoreProtect uint32
	_ = windows.VirtualProtectEx(h, addr, uintptr(len(data)), oldProtect, &restoreProtect)
	return writeErr
}

var (
	modKernel32        = windows.NewLazySystemDLL("kernel32.dll")
	procVirtualAllocEx = modKernel32.NewProc("VirtualAllocEx")
	procVirtualFreeEx  = modKernel32.NewProc("VirtualFreeEx")
	procVirtualQueryEx = modKernel32.NewProc("VirtualQueryEx")
)

type memoryBasicInformation struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionId       uint16
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

func virtualAllocRemoteNear(h windows.Handle, nearAddr uintptr, size uintptr) (uintptr, error) {
	const (
		memCommit                  = 0x1000
		memReserve                 = 0x2000
		memFree                    = 0x10000
		pageExecuteReadWrite       = 0x40
		allocGranularity           = uintptr(0x10000)
		maxRel32Distance     int64 = 0x7FFFFFFF
	)

	alignDown := func(v uintptr) uintptr { return v &^ (allocGranularity - 1) }
	tryAlloc := func(addr uintptr) uintptr {
		ret, _, _ := procVirtualAllocEx.Call(
			uintptr(h),
			addr,
			size,
			uintptr(memCommit|memReserve),
			uintptr(pageExecuteReadWrite),
		)
		return ret
	}
	isReachable := func(addr uintptr) bool {
		delta := int64(addr) - int64(nearAddr)
		if delta < 0 {
			delta = -delta
		}
		return delta <= maxRel32Distance
	}

	if addr := tryAlloc(nearAddr); addr != 0 && isReachable(addr) {
		return addr, nil
	}

	base := alignDown(nearAddr)
	for step := uintptr(0); step <= uintptr(maxRel32Distance); step += allocGranularity {
		candidates := [2]uintptr{}
		count := 0
		if step == 0 {
			candidates[count] = base
			count++
		} else {
			if base >= step {
				candidates[count] = base - step
				count++
			}
			if base <= ^uintptr(0)-step {
				candidates[count] = base + step
				count++
			}
		}

		for i := 0; i < count; i++ {
			candidate := candidates[i]
			if !isReachable(candidate) {
				continue
			}

			var mbi memoryBasicInformation
			ret, _, _ := procVirtualQueryEx.Call(
				uintptr(h),
				candidate,
				uintptr(unsafe.Pointer(&mbi)),
				unsafe.Sizeof(mbi),
			)
			if ret == 0 {
				continue
			}
			if mbi.State != memFree || mbi.RegionSize < size {
				continue
			}
			allocBase := alignDown(mbi.BaseAddress)
			if !isReachable(allocBase) {
				continue
			}
			if addr := tryAlloc(allocBase); addr != 0 && isReachable(addr) {
				return addr, nil
			}
		}
	}

	return 0, fmt.Errorf("VirtualAllocEx 附近分配失败")
}

func virtualFreeRemote(h windows.Handle, addr uintptr) error {
	ret, _, _ := procVirtualFreeEx.Call(
		uintptr(h),
		addr,
		0,
		uintptr(0x8000), // MEM_RELEASE
	)
	if ret == 0 {
		return fmt.Errorf("VirtualFreeEx 失败")
	}
	return nil
}
