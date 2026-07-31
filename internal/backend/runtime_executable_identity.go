package backend

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

const (
	runtimeGame203ErrorCode    = "GBFR_RUNTIME_GAME_203"
	runtimeUnknownExeErrorCode = "GBFR_RUNTIME_UNKNOWN_EXE"
)

func queryProcessImagePath(handle windows.Handle) (string, error) {
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return "", err
	}
	if size == 0 || int(size) > len(buffer) {
		return "", fmt.Errorf("游戏可执行文件路径为空")
	}
	return windows.UTF16ToString(buffer[:size]), nil
}

func hashFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%X", hasher.Sum(nil)), nil
}

func runtimeExecutableDigestForHandle(handle windows.Handle) (string, error) {
	path, err := queryProcessImagePath(handle)
	if err != nil {
		return "", fmt.Errorf("读取游戏可执行文件路径失败: %w", err)
	}
	digest, err := hashFileSHA256(path)
	if err != nil {
		return "", fmt.Errorf("校验游戏可执行文件失败: %w", err)
	}
	return digest, nil
}

func legacyRuntimeExecutableError(featureName, digest string) error {
	return fmt.Errorf("[%s] %s仅支持已识别的游戏 2.0.2 / 2.0.3 可执行文件；当前游戏版本不会连接或写入", runtimeUnknownExeErrorCode, featureName)
}

func isSupportedRuntimeExecutableDigest(digest string) bool {
	return strings.EqualFold(digest, runtimePatchCatalogGameSHA256) ||
		strings.EqualFold(digest, game203ExecutableSHA256)
}

func verifyLegacyRuntimeExecutableHandle(handle windows.Handle, featureName string) error {
	digest, err := runtimeExecutableDigestForHandle(handle)
	if err != nil {
		return err
	}
	if !isSupportedRuntimeExecutableDigest(digest) {
		return legacyRuntimeExecutableError(featureName, digest)
	}
	return nil
}
