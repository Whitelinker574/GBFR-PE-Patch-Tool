package main

import (
	"encoding/binary"
	"fmt"
	"unsafe"
)

const (
	currencyHookSize       = 5
	currencyCaveDataOffset = uintptr(0x40)
)

// DLC 2.0.2 CT v0.7.4 captures RCX at this instruction and then reads the
// resource fields at +30/+34/+98/+9C. The wildcards are relocation- and
// register-dependent bytes; fixed bytes identify the current code path.
var currencyCapturePattern = []byte{
	0xBA, 0, 0, 0, 0,
	0x41, 0x89, 0,
	0xE8, 0, 0, 0, 0,
	0x48, 0x8B, 0, 0, 0, 0, 0,
	0x8B, 0, 0, 0, 0, 0,
	0x83,
}

var currencyCaptureMask = []bool{
	true, false, false, false, false,
	true, true, false,
	true, false, false, false, false,
	true, true, false, false, false, false, false,
	true, false, false, false, false, false,
	true,
}

func (a *App) currencyRoot() (uintptr, error) {
	if err := a.ensureCurrencyHook(); err != nil {
		return 0, err
	}
	var root uintptr
	if err := readProcessMemory(a.hProcess, a.currencyCaveAddr+currencyCaveDataOffset, unsafe.Pointer(&root), unsafe.Sizeof(root)); err != nil {
		return 0, fmt.Errorf("读取实时资源指针失败: %w", err)
	}
	if root == 0 {
		return 0, fmt.Errorf("实时资源定位已启用；请在游戏内打开主菜单或让金币/MSP发生一次刷新，然后点击“刷新货币”")
	}
	if err := a.validateCurrencyRoot(root); err != nil {
		return 0, err
	}
	return root, nil
}

func (a *App) validateCurrencyRoot(root uintptr) error {
	for _, def := range currencyDefs {
		var value int32
		if err := readProcessMemory(a.hProcess, root+def.Offset, unsafe.Pointer(&value), unsafe.Sizeof(value)); err != nil {
			return fmt.Errorf("实时资源结构校验失败（%s）: %w", def.Name, err)
		}
		if value < 0 {
			return fmt.Errorf("实时资源结构校验失败：%s 读取到异常负值 %d，已拒绝写入", def.Name, value)
		}
	}
	return nil
}

func (a *App) ensureCurrencyHook() error {
	if a.hProcess == 0 || a.moduleBase == 0 {
		return fmt.Errorf("未连接游戏进程")
	}
	if a.currencyHookAddr == 0 {
		addr, err := a.scanPatternUnique(currencyCapturePattern, currencyCaptureMask, "DLC 2.0.2 玩家资源结构")
		if err != nil {
			return err
		}
		a.currencyHookAddr = addr
	}
	current := make([]byte, currencyHookSize)
	if err := readProcessMemory(a.hProcess, a.currencyHookAddr, unsafe.Pointer(&current[0]), uintptr(len(current))); err != nil {
		return fmt.Errorf("读取实时资源定位指令失败: %w", err)
	}
	if isCurrencyCaptureJump(current) {
		if a.currencyCaveAddr == 0 {
			return fmt.Errorf("检测到未知来源的资源定位 Hook；请完全退出游戏后重试")
		}
		return nil
	}
	if !isCurrencyCaptureOriginal(current) {
		return fmt.Errorf("实时资源定位指令不匹配 DLC 2.0.2: %s", bytesToHex(current))
	}
	return a.installCurrencyHook(current)
}

func (a *App) installCurrencyHook(original []byte) error {
	cave, err := virtualAllocRemoteNear(a.hProcess, a.currencyHookAddr, 0x1000)
	if err != nil {
		return fmt.Errorf("分配实时资源定位代码洞失败: %w", err)
	}
	code, err := buildCurrencyCaptureCave(cave, a.currencyHookAddr+currencyHookSize, original)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return err
	}
	if err := writeProcessMemory(a.hProcess, cave, unsafe.Pointer(&code[0]), uintptr(len(code))); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return fmt.Errorf("写入实时资源定位代码洞失败: %w", err)
	}
	patch, err := makeRelJump(a.currencyHookAddr, cave, currencyHookSize)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return err
	}
	if err := writeCodeMemory(a.hProcess, a.currencyHookAddr, patch); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return fmt.Errorf("写入实时资源定位 Hook 失败: %w", err)
	}
	a.currencyCaveAddr = cave
	a.currencyOriginal = append(a.currencyOriginal[:0], original...)
	return nil
}

func (a *App) releaseCurrencyHook() error {
	if a.hProcess == 0 || a.currencyHookAddr == 0 {
		return nil
	}
	current := make([]byte, currencyHookSize)
	if err := readProcessMemory(a.hProcess, a.currencyHookAddr, unsafe.Pointer(&current[0]), uintptr(len(current))); err != nil {
		return err
	}
	if !isCurrencyCaptureJump(current) {
		return nil
	}
	if len(a.currencyOriginal) != currencyHookSize || !isCurrencyCaptureOriginal(a.currencyOriginal) {
		return fmt.Errorf("缺少可验证的资源定位原始指令，拒绝恢复")
	}
	if err := writeCodeMemory(a.hProcess, a.currencyHookAddr, a.currencyOriginal); err != nil {
		return fmt.Errorf("恢复实时资源定位指令失败: %w", err)
	}
	a.currencyHookAddr = 0
	a.currencyCaveAddr = 0
	a.currencyOriginal = nil
	return nil
}

func isCurrencyCaptureOriginal(buf []byte) bool {
	return len(buf) >= currencyHookSize && buf[0] == 0xBA
}

func isCurrencyCaptureJump(buf []byte) bool {
	return len(buf) >= currencyHookSize && buf[0] == 0xE9
}

func buildCurrencyCaptureCave(cave, returnAddr uintptr, original []byte) ([]byte, error) {
	if len(original) != currencyHookSize || !isCurrencyCaptureOriginal(original) {
		return nil, fmt.Errorf("实时资源定位原始指令长度或签名异常")
	}
	code := make([]byte, 0, currencyCaveDataOffset+8)
	// The displaced instruction is `mov edx, imm32`, so RDX is about to be
	// overwritten by the original code. Use RDX as scratch and avoid clobbering
	// another volatile register that the surrounding function may still need.
	code = append(code, 0x48, 0xBA) // mov rdx, data address
	code = binary.LittleEndian.AppendUint64(code, uint64(cave+currencyCaveDataOffset))
	code = append(code, 0x48, 0x89, 0x0A) // mov [rdx], rcx
	code = append(code, original...)
	jmp, err := makeRelJump(cave+uintptr(len(code)), returnAddr, 5)
	if err != nil {
		return nil, err
	}
	code = append(code, jmp...)
	for len(code) < int(currencyCaveDataOffset)+8 {
		code = append(code, 0)
	}
	return code, nil
}
