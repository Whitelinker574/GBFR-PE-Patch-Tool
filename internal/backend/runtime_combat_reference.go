package backend

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type RuntimeCombatCurvePoint struct {
	Interpolation string  `json:"interpolation"`
	X             float64 `json:"x"`
	Y             float64 `json:"y"`
	LeftTangent   float64 `json:"leftTangent"`
	RightTangent  float64 `json:"rightTangent"`
}

type RuntimeDamageLimitNode struct {
	AttackRate float64 `json:"attackRate"`
	DamageCap  float64 `json:"damageCap"`
}

type RuntimeDamageCalculateReference struct {
	CriticalDamageUpperRate          float64 `json:"criticalDamageUpperRate"`
	SuperArmorDamageRate             float64 `json:"superArmorDamageRate"`
	AttackTypeDamageLimitNormal      float64 `json:"atkTypeDamageLimit_Normal"`
	AttackTypeDamageLimitAbility     float64 `json:"atkTypeDamageLimit_Ability"`
	AttackTypeDamageLimitSpecialArts float64 `json:"atkTypeDamageLimit_SpArts"`
	ChainBurstDamageLimit            float64 `json:"chainBurstDamageLimit"`
	WeakElementAddDamageRate         float64 `json:"weakElementAddDamageRate"`
	AdditionalDamageLimitStatusRate  float64 `json:"addDamageLimitBonusStatusRate"`
	AutoReviveMaxCount               float64 `json:"autoReviveMaxCount"`
	AutoReviveHPRate                 float64 `json:"autoReviveHpRate"`
	AutoReviveCooldown               float64 `json:"autoReviveCoolTime"`
	GutsMaxCount                     float64 `json:"gutsMaxCount"`
	GutsInvincibleSeconds            float64 `json:"gutsInvinsibleSec"`
	GutsCooldown                     float64 `json:"gutsCoolTime"`
}

type RuntimeGuardReference struct {
	GaugeMax                  float64 `json:"GuardGageMax"`
	GuardStopHealSeconds      float64 `json:"GuardStopHealSec"`
	DamageStopHealSeconds     float64 `json:"DamageStopHealSec"`
	GuardBreakSeconds         float64 `json:"GuardBreakSec"`
	GuardAutoHealValue        float64 `json:"GuardAutoHealValue"`
	GuardDamageCutRate        float64 `json:"GuardDamageCutRate"`
	GuardBreakDamageCutRate   float64 `json:"GuardBreakDamageCutRate"`
	JustGuardAcceptFrames     float64 `json:"JustGuardAcceptFrame"`
	ChargeParryAcceptFrames   float64 `json:"ChargeParryAcceptFrame"`
	ChargeParryInvincibleTime float64 `json:"ChargeParryInvinsbleTime"`
	GuardFailurePenaltyTime   float64 `json:"guardFailedPenaltyTime"`
}

type RuntimeCombatReference struct {
	DataVersion       string                               `json:"dataVersion"`
	CharacterCode     string                               `json:"characterCode,omitempty"`
	DamageCalculate   RuntimeDamageCalculateReference      `json:"damageCalculate"`
	Guard             RuntimeGuardReference                `json:"guard"`
	ConditionalCurves map[string][]RuntimeCombatCurvePoint `json:"conditionalCurves"`
	NormalCurve       []RuntimeDamageLimitNode             `json:"normalCurve,omitempty"`
	ArtsCurve         []RuntimeDamageLimitNode             `json:"artsCurve,omitempty"`
	Evidence          string                               `json:"evidence"`
	InterpolationNote string                               `json:"interpolationNote"`
}

type runtimeDamageLimitCatalog struct {
	NormalRowCount int                                 `json:"normalRowCount"`
	ArtsRowCount   int                                 `json:"artsRowCount"`
	Normal         map[string][]RuntimeDamageLimitNode `json:"normal"`
	Arts           map[string][]RuntimeDamageLimitNode `json:"arts"`
}

type runtimeCombatReferenceCatalog struct {
	SchemaVersion     int                                  `json:"schemaVersion"`
	DataVersion       string                               `json:"dataVersion"`
	DamageCalculate   RuntimeDamageCalculateReference      `json:"damageCalculate"`
	Guard             RuntimeGuardReference                `json:"guard"`
	ConditionalCurves map[string][]RuntimeCombatCurvePoint `json:"conditionalCurves"`
	DamageLimits      runtimeDamageLimitCatalog            `json:"damageLimits"`
}

//go:embed data/runtime_combat_reference_202.json
var runtimeCombatReferenceJSON []byte

var (
	runtimeCombatReferenceOnce sync.Once
	runtimeCombatReferenceData runtimeCombatReferenceCatalog
	runtimeCombatReferenceErr  error
)

func loadRuntimeCombatReference() (*runtimeCombatReferenceCatalog, error) {
	runtimeCombatReferenceOnce.Do(func() {
		if err := json.Unmarshal(runtimeCombatReferenceJSON, &runtimeCombatReferenceData); err != nil {
			runtimeCombatReferenceErr = fmt.Errorf("解析 2.0.2 战斗参考目录失败: %w", err)
			return
		}
		data := &runtimeCombatReferenceData
		if data.SchemaVersion != 1 || data.DataVersion != "2.0.2" ||
			data.DamageLimits.NormalRowCount != 975 || data.DamageLimits.ArtsRowCount != 930 ||
			len(data.ConditionalCurves) != 7 {
			runtimeCombatReferenceErr = fmt.Errorf("2.0.2 战斗参考目录版本或记录数量无效")
			return
		}
		for name, rows := range data.ConditionalCurves {
			if name == "" || len(rows) == 0 {
				runtimeCombatReferenceErr = fmt.Errorf("2.0.2 战斗条件曲线 %q 为空", name)
				return
			}
		}
	})
	if runtimeCombatReferenceErr != nil {
		return nil, runtimeCombatReferenceErr
	}
	return &runtimeCombatReferenceData, nil
}

func selectedRuntimeCombatReference(characterCode string) (*RuntimeCombatReference, error) {
	catalog, err := loadRuntimeCombatReference()
	if err != nil {
		return nil, err
	}
	code := strings.ToUpper(strings.TrimSpace(characterCode))
	return &RuntimeCombatReference{
		DataVersion:       catalog.DataVersion,
		CharacterCode:     code,
		DamageCalculate:   catalog.DamageCalculate,
		Guard:             catalog.Guard,
		ConditionalCurves: catalog.ConditionalCurves,
		NormalCurve:       catalog.DamageLimits.Normal[code],
		ArtsCurve:         catalog.DamageLimits.Arts[code],
		Evidence:          "GBFR 2.0.2 damagecalcparam.msg、guardparam.msg、战斗曲线与角色上限表（输入 SHA-256 随目录保存）",
		InterpolationNote: "仅显示表中原始节点；Smooth / SmoothSide 的运行时插值尚未闭环，不做线性冒充。",
	}, nil
}
