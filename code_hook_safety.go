package main

import (
	"bytes"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type codeHookWriter func([]byte) error
type codeHookReader func() ([]byte, error)

var errRuntimeHookRollbackUnproven = errors.New("运行时 Hook 原始入口未能确认恢复")

func runtimeHookInstallFailure(label string, canFree bool, cause error, free, retain, poison func()) error {
	installErr := fmt.Errorf("%s安装失败: %w", label, cause)
	if canFree {
		if free != nil {
			free()
		}
		return installErr
	}
	if retain != nil {
		retain()
	}
	if poison != nil {
		poison()
	}
	return errors.Join(installErr, errRuntimeHookRollbackUnproven)
}

func finalizeRuntimeHookEnable[T any](label string, read func() (T, error), rollback func() error, poison func()) (T, error) {
	var zero T
	if read == nil || rollback == nil {
		if poison != nil {
			poison()
		}
		return zero, errors.Join(fmt.Errorf("%s安装后验证参数无效", label), errRuntimeHookRollbackUnproven)
	}
	status, readErr := read()
	if readErr == nil {
		return status, nil
	}
	verificationErr := fmt.Errorf("%s安装后验证失败: %w", label, readErr)
	if rollbackErr := rollback(); rollbackErr != nil {
		if poison != nil {
			poison()
		}
		return zero, errors.Join(
			verificationErr,
			errRuntimeHookRollbackUnproven,
			fmt.Errorf("%s验证失败后的恢复也失败，已保留恢复租约: %w", label, rollbackErr),
		)
	}
	return zero, fmt.Errorf("%s安装后验证失败，原始入口已恢复: %w", label, readErr)
}

// installCodeHookAtomic returns canFree=true only when the entry point is
// proven to contain its complete original byte sequence. Callers must keep the
// remote cave mapped on every other failure path.
func installCodeHookAtomic(original, patch []byte, writer codeHookWriter, reader codeHookReader) (canFree bool, err error) {
	if len(original) == 0 || len(original) != len(patch) || writer == nil || reader == nil {
		return true, fmt.Errorf("Hook 安装参数无效")
	}
	current, err := reader()
	if err != nil {
		return true, fmt.Errorf("读取 Hook 写入前指令失败: %w", err)
	}
	if !bytes.Equal(current, original) {
		return true, fmt.Errorf("Hook 写入前指令已变化: %s", bytesToHex(current))
	}

	installErr := writer(patch)
	if installErr == nil {
		current, installErr = reader()
		if installErr == nil && bytes.Equal(current, patch) {
			return false, nil
		}
		if installErr == nil {
			installErr = fmt.Errorf("Hook 写后回读不一致: %s", bytesToHex(current))
		} else {
			installErr = fmt.Errorf("Hook 写后回读失败: %w", installErr)
		}
	}

	restoreErr := writer(original)
	if restoreErr == nil {
		var restored []byte
		restored, restoreErr = reader()
		if restoreErr == nil && !bytes.Equal(restored, original) {
			restoreErr = fmt.Errorf("Hook 恢复后回读不一致: %s", bytesToHex(restored))
		}
	}
	if restoreErr == nil {
		return true, installErr
	}
	return false, errors.Join(installErr, fmt.Errorf("Hook 原始指令无法确认恢复，代码洞必须保留: %w", restoreErr))
}

func installRemoteCodeHook(h windows.Handle, addr uintptr, original, patch []byte) (bool, error) {
	writer := func(data []byte) error {
		return writeCodeMemory(h, addr, data)
	}
	reader := func() ([]byte, error) {
		data := make([]byte, len(original))
		if err := readProcessMemory(h, addr, unsafe.Pointer(&data[0]), uintptr(len(data))); err != nil {
			return nil, err
		}
		return data, nil
	}
	return installCodeHookAtomic(original, patch, writer, reader)
}
