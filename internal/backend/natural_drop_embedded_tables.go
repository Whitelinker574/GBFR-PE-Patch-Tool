package backend

import (
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/klauspost/compress/zstd"
)

const (
	naturalDropBundledSourceID = "builtin://dlc-2.0.2"
	naturalDropBundledRawSize  = 1_935_344
)

//go:embed data/natural_drop_tables_202.zstd.b64
var naturalDropBundledPayload string

var (
	naturalDropBundledOnce   sync.Once
	naturalDropBundledTables map[string][]byte
	naturalDropBundledErr    error
)

func naturalDropUsesBundledSource(source string) bool {
	source = strings.TrimSpace(source)
	return source == "" || strings.EqualFold(source, naturalDropBundledSourceID)
}

func naturalDropExpectedTable(name string) (int, string, bool) {
	for _, required := range naturalDropRequiredTables {
		if required.Name == name {
			return required.Size, required.SHA256, true
		}
	}
	for _, required := range naturalWrightstoneRequiredTables {
		if required.Name == name {
			return required.Size, required.SHA256, true
		}
	}
	for _, required := range naturalSigilRequiredTables {
		if required.Name == name {
			return required.Size, required.SHA256, true
		}
	}
	for _, required := range naturalDropItemRequiredTables {
		if required.Name == name {
			return required.Size, required.SHA256, true
		}
	}
	return 0, "", false
}

func decodeNaturalDropBundledTables() {
	encoded := strings.NewReplacer("\r", "", "\n", "", "\t", "", " ", "").Replace(naturalDropBundledPayload)
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		naturalDropBundledErr = fmt.Errorf("内置 2.0.2 原表编码损坏: %w", err)
		return
	}
	decoder, err := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(8<<20), zstd.WithDecoderMaxWindow(8<<20))
	if err != nil {
		naturalDropBundledErr = fmt.Errorf("初始化内置原表解码器失败: %w", err)
		return
	}
	raw, err := decoder.DecodeAll(compressed, nil)
	decoder.Close()
	if err != nil {
		naturalDropBundledErr = fmt.Errorf("解压内置 2.0.2 原表失败: %w", err)
		return
	}
	if len(raw) != naturalDropBundledRawSize {
		naturalDropBundledErr = fmt.Errorf("内置 2.0.2 原表总长度错误: %d", len(raw))
		return
	}

	order := []string{
		"summon.tbl",
		"summon_lot.tbl",
		"reward_summon_lot.tbl",
		"item_pendulum.tbl",
		"gacha_lot.tbl",
		"gacha_rate_group.tbl",
		"gacha.tbl",
		"gem.tbl",
		"reward.tbl",
		"reward_lot.tbl",
		"endlessmode_package.tbl",
	}
	result := make(map[string][]byte, len(order))
	offset := 0
	for _, name := range order {
		size, expected, ok := naturalDropExpectedTable(name)
		if !ok || offset+size > len(raw) {
			naturalDropBundledErr = fmt.Errorf("内置 2.0.2 原表目录损坏: %s", name)
			return
		}
		data := append([]byte(nil), raw[offset:offset+size]...)
		if digest := fileSHA256(data); !strings.EqualFold(digest, expected) {
			naturalDropBundledErr = fmt.Errorf("内置 %s 校验失败: %s", name, digest)
			return
		}
		result[name] = data
		offset += size
	}
	if offset != len(raw) {
		naturalDropBundledErr = errors.New("内置 2.0.2 原表存在未声明数据")
		return
	}
	naturalDropBundledTables = result
}

func naturalDropBundledTable(name string) ([]byte, error) {
	naturalDropBundledOnce.Do(decodeNaturalDropBundledTables)
	if naturalDropBundledErr != nil {
		return nil, naturalDropBundledErr
	}
	data, ok := naturalDropBundledTables[name]
	if !ok {
		return nil, fmt.Errorf("内置 2.0.2 原表缺少 %s", name)
	}
	return append([]byte(nil), data...), nil
}
