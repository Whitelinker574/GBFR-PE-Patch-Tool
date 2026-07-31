package backend

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type InfinityRuleEffect struct {
	Key   string  `json:"key"`
	ID    int     `json:"id"`
	Value float64 `json:"value"`
}

type InfinityRuleEntry struct {
	QuestID        string               `json:"questId"`
	NameKey        string               `json:"nameKey"`
	DescriptionKey string               `json:"descriptionKey"`
	NameZh         string               `json:"nameZh"`
	NameEn         string               `json:"nameEn"`
	DescriptionZh  string               `json:"descriptionZh"`
	DescriptionEn  string               `json:"descriptionEn"`
	Effects        []InfinityRuleEffect `json:"effects"`
}

type InfinityDifficultyEntry struct {
	Key              string  `json:"Unk1"`
	SecondaryKey     string  `json:"Unk2"`
	TertiaryKey      string  `json:"Unk3"`
	SortOrder        int     `json:"SortOrder"`
	EnemyMinLevel    int     `json:"EnemyMinLevel"`
	EnemyLevelOffset int     `json:"Unk6"`
	EnemyMaxLevel    int     `json:"EnemyMaxLevel"`
	Power            int     `json:"Power"`
	RawValue9        float64 `json:"Unk9"`
	RawValue10       float64 `json:"Unk10"`
}

type InfinityRuleCatalogData struct {
	SchemaVersion  int                       `json:"schemaVersion"`
	DataVersion    string                    `json:"dataVersion"`
	Rules          []InfinityRuleEntry       `json:"rules"`
	Difficulties   []InfinityDifficultyEntry `json:"difficulties"`
	Interpretation string                    `json:"interpretation"`
}

//go:embed data/infinity_rule_catalog_202.json
var infinityRuleCatalogJSON []byte

var (
	infinityRuleCatalogOnce sync.Once
	infinityRuleCatalogData InfinityRuleCatalogData
	infinityRuleCatalogErr  error
)

func loadInfinityRuleCatalog() (*InfinityRuleCatalogData, error) {
	infinityRuleCatalogOnce.Do(func() {
		if err := json.Unmarshal(infinityRuleCatalogJSON, &infinityRuleCatalogData); err != nil {
			infinityRuleCatalogErr = fmt.Errorf("解析无尽模式 2.0.2 规则目录失败: %w", err)
			return
		}
		data := &infinityRuleCatalogData
		if data.SchemaVersion != 1 || data.DataVersion != "2.0.2" || len(data.Rules) != 25 || len(data.Difficulties) != 5 {
			infinityRuleCatalogErr = fmt.Errorf("无尽模式规则目录版本或记录数量无效")
			return
		}
		for _, rule := range data.Rules {
			if rule.QuestID == "" || rule.NameZh == "" || rule.NameEn == "" ||
				strings.Contains(rule.DescriptionZh, "{") || strings.Contains(rule.DescriptionEn, "{") {
				infinityRuleCatalogErr = fmt.Errorf("无尽模式规则 %q 的本地化或参数回填不完整", rule.NameKey)
				return
			}
		}
	})
	if infinityRuleCatalogErr != nil {
		return nil, infinityRuleCatalogErr
	}
	return &infinityRuleCatalogData, nil
}

func (a *App) InfinityRuleCatalog() (*InfinityRuleCatalogData, error) {
	return loadInfinityRuleCatalog()
}
