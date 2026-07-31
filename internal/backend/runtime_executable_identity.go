package backend

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/windows"
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

func legacyRuntimeExecutableError(featureName, digest string) error {
	if strings.EqualFold(digest, game203ExecutableSHA256) {
		return fmt.Errorf("%s暂未支持游戏 2.0.3：静态目录、配装计算、分享与 Logs 数据已核对；离线存档的 2.0.3 游戏重启回读仍待验收，实时功能不会连接或写入", featureName)
	}
	return fmt.Errorf("%s仅支持已验证的游戏 2.0.2 可执行文件；当前游戏版本不会连接或写入", featureName)
}

func verifyLegacyRuntimeExecutableHandle(handle windows.Handle, featureName string) error {
	path, err := queryProcessImagePath(handle)
	if err != nil {
		return fmt.Errorf("读取游戏可执行文件路径失败: %w", err)
	}
	digest, err := hashFileSHA256(path)
	if err != nil {
		return fmt.Errorf("校验游戏可执行文件失败: %w", err)
	}
	if !strings.EqualFold(digest, runtimePatchCatalogGameSHA256) {
		return legacyRuntimeExecutableError(featureName, digest)
	}
	return nil
}
