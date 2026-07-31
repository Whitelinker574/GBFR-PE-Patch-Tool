package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	loadoutShareServiceURL         = "https://share.whitelinker.top"
	loadoutShareOnlineMaxFrameSize = 8 * 1024
	loadoutSharePreviewMaxBytes    = 12 * 1024
	loadoutShareTitleMaxRunes      = 80
	loadoutShareRequestAttempts    = 3
)

var (
	loadoutShareShortCodePattern    = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{16,24}$`)
	loadoutShareLegacyPrefixPattern = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{12}$`)
)

type LoadoutPublishedShare struct {
	Code          string `json:"code"`
	CompactCode   string `json:"compactCode"`
	URL           string `json:"url"`
	DownloadURL   string `json:"downloadUrl"`
	Bytes         int    `json:"bytes"`
	Reused        bool   `json:"reused"`
	Title         string `json:"title,omitempty"`
	CharacterName string `json:"characterName,omitempty"`
}

type loadoutShareOnlineError struct {
	Error string `json:"error"`
}

func loadoutShareHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ForceAttemptHTTP2:     false,
			DisableKeepAlives:     true,
			MaxIdleConns:          2,
			MaxIdleConnsPerHost:   1,
			IdleConnTimeout:       5 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

func loadoutShareDo(ctx context.Context, client *http.Client, method, target string, body []byte, headers map[string]string) (*http.Response, error) {
	if client == nil {
		client = loadoutShareHTTPClient()
	}
	var lastErr error
	for attempt := 0; attempt < loadoutShareRequestAttempts; attempt++ {
		request, err := http.NewRequestWithContext(loadoutShareRequestContext(ctx), method, target, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		response, err := client.Do(request)
		if err == nil {
			return response, nil
		}
		lastErr = err
		if request.Context().Err() != nil {
			return nil, request.Context().Err()
		}
		if attempt+1 < loadoutShareRequestAttempts {
			delay := time.Duration(attempt+1) * 180 * time.Millisecond
			timer := time.NewTimer(delay)
			select {
			case <-request.Context().Done():
				timer.Stop()
				return nil, request.Context().Err()
			case <-timer.C:
			}
		}
	}
	return nil, fmt.Errorf("连接被远端中断，请重试；也可使用离线长码或下载文件: %w", lastErr)
}

func loadoutShareRequestContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

func loadoutShareFrameFromCompatibilityCode(code string) ([]byte, error) {
	code = normalizeLoadoutShareCode(code)
	if !strings.HasPrefix(code, loadoutShareCodePrefix) {
		return nil, fmt.Errorf("本地配装帧前缀无效")
	}
	frame, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(code, loadoutShareCodePrefix))
	if err != nil {
		return nil, fmt.Errorf("解析本地配装帧失败: %w", err)
	}
	if len(frame) == 0 || len(frame) > loadoutShareOnlineMaxFrameSize {
		return nil, fmt.Errorf("线上分享只接受不超过 %d KB 的配装帧", loadoutShareOnlineMaxFrameSize/1024)
	}
	return frame, nil
}

func normalizeLoadoutShareShortCode(input string) (string, error) {
	value := strings.TrimSpace(input)
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		path := strings.Trim(parsed.EscapedPath(), "/")
		switch {
		case strings.HasPrefix(path, "s/"):
			value = strings.TrimPrefix(path, "s/")
		case strings.HasPrefix(path, "api/v1/loadouts/"):
			value = strings.TrimPrefix(path, "api/v1/loadouts/")
		case strings.HasPrefix(path, "download/") && strings.HasSuffix(strings.ToLower(path), ".gbfr-loadout"):
			value = strings.TrimSuffix(strings.TrimPrefix(path, "download/"), ".gbfr-loadout")
		default:
			return "", fmt.Errorf("链接中没有可识别的配装短码")
		}
		if decoded, decodeErr := url.PathUnescape(value); decodeErr == nil {
			value = decoded
		}
	}
	value = strings.ToUpper(value)
	value = strings.NewReplacer("-", "", " ", "", "\t", "", "\r", "", "\n", "").Replace(value)
	if !loadoutShareShortCodePattern.MatchString(value) && !loadoutShareLegacyPrefixPattern.MatchString(value) {
		return "", fmt.Errorf("短码格式无效；请输入 12 位旧码、16 位短码或完整分享链接")
	}
	return value, nil
}

func displayLoadoutShareShortCode(code string) string {
	var groups []string
	for len(code) > 0 {
		size := min(4, len(code))
		groups = append(groups, code[:size])
		code = code[size:]
	}
	return strings.Join(groups, "-")
}

func loadoutShareOnlineErrorMessage(response *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4*1024))
	var payload loadoutShareOnlineError
	if json.Unmarshal(body, &payload) == nil && strings.TrimSpace(payload.Error) != "" {
		return strings.TrimSpace(payload.Error)
	}
	return http.StatusText(response.StatusCode)
}

func publishLoadoutShareFrame(ctx context.Context, client *http.Client, endpoint string, frame []byte) (*LoadoutPublishedShare, error) {
	return publishLoadoutShareFrameWithMetadata(ctx, client, endpoint, frame, nil)
}

type loadoutSharePreview struct {
	Title          string                          `json:"title,omitempty"`
	CharacterHash  string                          `json:"characterHash,omitempty"`
	CharacterName  string                          `json:"characterName,omitempty"`
	WeaponHash     string                          `json:"weaponHash,omitempty"`
	WeaponName     string                          `json:"weaponName,omitempty"`
	Sigils         []loadoutSharePreviewSigil      `json:"sigils,omitempty"`
	Abilities      []loadoutSharePreviewSkill      `json:"abilities,omitempty"`
	WeaponSkills   []loadoutSharePreviewSkill      `json:"weaponSkills,omitempty"`
	Wrightstone    *loadoutSharePreviewWrightstone `json:"wrightstone,omitempty"`
	Summons        []loadoutSharePreviewSummon     `json:"summons,omitempty"`
	MasteryCount   int                             `json:"masteryCount,omitempty"`
	MasteryCat     string                          `json:"masteryCat,omitempty"`
	MasteryLabel   string                          `json:"masteryLabel,omitempty"`
	MasterySkills  []loadoutSharePreviewMastery    `json:"masterySkills,omitempty"`
	OverLimit      []loadoutSharePreviewOverLimit  `json:"overLimit,omitempty"`
	CombinedSkills []loadoutSharePreviewTrait      `json:"combinedSkills,omitempty"`
}

type loadoutSharePreviewSigil struct {
	Hash           string `json:"hash,omitempty"`
	Name           string `json:"name,omitempty"`
	Level          int    `json:"level,omitempty"`
	PrimaryHash    string `json:"primaryHash,omitempty"`
	Primary        string `json:"primary,omitempty"`
	PrimaryLevel   int    `json:"primaryLevel,omitempty"`
	SecondaryHash  string `json:"secondaryHash,omitempty"`
	Secondary      string `json:"secondary,omitempty"`
	SecondaryLevel int    `json:"secondaryLevel,omitempty"`
}

type loadoutSharePreviewSkill struct {
	Hash   string `json:"hash,omitempty"`
	Key    string `json:"key,omitempty"`
	Name   string `json:"name,omitempty"`
	Level  int    `json:"level,omitempty"`
	Effect string `json:"effect,omitempty"`
}

type loadoutSharePreviewSummon struct {
	TypeHash       string  `json:"typeHash,omitempty"`
	Name           string  `json:"name,omitempty"`
	Rank           int     `json:"rank,omitempty"`
	MainTraitHash  string  `json:"mainTraitHash,omitempty"`
	MainTrait      string  `json:"mainTrait,omitempty"`
	MainTraitLevel int     `json:"mainTraitLevel,omitempty"`
	SubParamHash   string  `json:"subParamHash,omitempty"`
	SubParam       string  `json:"subParam,omitempty"`
	SubParamLevel  int     `json:"subParamLevel,omitempty"`
	SubParamValue  float64 `json:"subParamValue,omitempty"`
	SubParamUnit   string  `json:"subParamUnit,omitempty"`
}

type loadoutSharePreviewWrightstone struct {
	Hash   string                     `json:"hash,omitempty"`
	Name   string                     `json:"name,omitempty"`
	Traits []loadoutSharePreviewSkill `json:"traits,omitempty"`
}

type loadoutSharePreviewMastery struct {
	Hash   string `json:"hash,omitempty"`
	Rank   string `json:"rank,omitempty"`
	Name   string `json:"name,omitempty"`
	Effect string `json:"effect,omitempty"`
	Count  int    `json:"count,omitempty"`
}

type loadoutSharePreviewOverLimit struct {
	Index         int     `json:"index"`
	AttributeHash string  `json:"attributeHash,omitempty"`
	Name          string  `json:"name,omitempty"`
	Level         int     `json:"level,omitempty"`
	Value         float64 `json:"value,omitempty"`
	Unit          string  `json:"unit,omitempty"`
}

type loadoutSharePreviewTrait struct {
	Hash     string   `json:"hash,omitempty"`
	Name     string   `json:"name,omitempty"`
	Level    int      `json:"level,omitempty"`
	RawLevel int      `json:"rawLevel,omitempty"`
	MaxLevel int      `json:"maxLevel,omitempty"`
	Effect   string   `json:"effect,omitempty"`
	Sources  []string `json:"sources,omitempty"`
}

func previewChineseName(hashText, fallback string) string {
	hash, err := ParseHashHex(hashText)
	if err == nil {
		if name := strings.TrimSpace(runtimeNameCN[hash]); name != "" {
			return name
		}
	}
	if name := strings.TrimSpace(traitCN[fallback]); name != "" {
		return name
	}
	return fallback
}

func previewChineseSigilName(hashText, fallback string) string {
	hash, err := ParseHashHex(hashText)
	if err == nil {
		if cat, loadErr := LoadCatalog(); loadErr == nil {
			if sigil := cat.LookupSigilByHash(hash); sigil != nil {
				if name := strings.TrimSpace(sigilCN[sigil.DisplayName]); name != "" {
					return normalizeChineseSigilItemName(name)
				}
				if name := strings.TrimSpace(runtimeNameCN[hash]); name != "" {
					return normalizeChineseSigilItemName(name)
				}
			}
		}
	}
	if name := strings.TrimSpace(sigilCN[strings.TrimSpace(fallback)]); name != "" {
		return normalizeChineseSigilItemName(name)
	}
	return normalizeChineseSigilItemName(fallback)
}

func previewChineseSigilNameForTraits(hashText, fallback, primary, secondary string) string {
	if cat, loadErr := LoadCatalog(); loadErr == nil {
		var sigil *SigilDef
		if hash, err := ParseHashHex(hashText); err == nil {
			sigil = cat.LookupSigilByHash(hash)
			if sigil == nil {
				if name := strings.TrimSpace(runtimeNameCN[hash]); name != "" {
					return normalizeChineseSigilItemName(name)
				}
			}
		}
		if !sigilHasFixedCatalogTitle(sigil) {
			if name := synthesizeSigilNameForTraits(cat, primary, strings.TrimSpace(secondary) != "", true); name != "" {
				return name
			}
		}
	}
	return previewChineseSigilName(hashText, fallback)
}

func previewChineseWeaponName(hashText, fallback string) string {
	_, _ = loadProgressionCatalog()
	hash, err := ParseHashHex(hashText)
	if err == nil {
		if definition, ok := progressionWeaponDefForHash(hash); ok && strings.TrimSpace(definition.NameCN) != "" {
			return definition.NameCN
		}
	}
	return fallback
}

func previewChineseWrightstoneName(name string) string {
	if localized := strings.TrimSpace(wrightstoneCN[strings.TrimSpace(name)]); localized != "" {
		return localized
	}
	return name
}

func previewChineseSource(source string, preview *loadoutSharePreview, bonus TraitBonus) string {
	parts := strings.Split(strings.TrimSpace(source), " · ")
	if len(parts) == 0 {
		return ""
	}
	traitName := previewChineseName(bonus.TraitID, bonus.Name)
	levelSuffix := func(value string) string {
		if index := strings.LastIndex(value, " Lv"); index >= 0 {
			return value[index:]
		}
		return ""
	}
	first := strings.TrimSpace(parts[0])
	if strings.HasPrefix(first, "因子") || strings.HasPrefix(first, "Sigil ") {
		digits := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(first, "因子"), "Sigil"))
		return fmt.Sprintf("因子%02s · %s%s", digits, traitName, levelSuffix(parts[len(parts)-1]))
	}
	if strings.HasPrefix(first, "构造因子") || strings.HasPrefix(first, "Constructed Sigil ") {
		digits := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(first, "构造因子"), "Constructed Sigil"))
		return fmt.Sprintf("构造因子%02s · %s%s", digits, traitName, levelSuffix(parts[len(parts)-1]))
	}
	if first == "武器" || first == "Weapon" {
		name := preview.WeaponName
		if len(parts) >= 3 {
			return fmt.Sprintf("武器 · %s · %s%s", name, traitName, levelSuffix(parts[2]))
		}
		return "武器 · " + name
	}
	if first == "武炼结晶" || first == "武器祝福" || first == "Wrightstone" {
		name := "武器祝福"
		if preview.Wrightstone != nil {
			name = preview.Wrightstone.Name
		}
		if len(parts) >= 3 {
			return fmt.Sprintf("武器祝福 · %s · %s%s", name, traitName, levelSuffix(parts[2]))
		}
		return "武器祝福 · " + name
	}
	if first == "召唤石" {
		return strings.Join(parts, " · ")
	}
	if strings.HasPrefix(first, "Summon ") {
		var index int
		if _, err := fmt.Sscanf(first, "Summon %d", &index); err == nil && index > 0 && index <= len(preview.Summons) {
			return "召唤石 · " + preview.Summons[index-1].Name
		}
		return "召唤石"
	}
	return strings.Join(parts, " · ")
}

func setPreviewCombinedSkills(preview *loadoutSharePreview, bonuses []TraitBonus) {
	if preview == nil {
		return
	}
	preview.CombinedSkills = preview.CombinedSkills[:0]
	for _, bonus := range bonuses {
		sources := make([]string, 0, len(bonus.Sources))
		for _, source := range bonus.Sources {
			if localized := previewChineseSource(source, preview, bonus); localized != "" {
				sources = append(sources, localized)
			}
		}
		if len(sources) > 8 {
			sources = sources[:8]
		}
		preview.CombinedSkills = append(preview.CombinedSkills, loadoutSharePreviewTrait{
			Hash: bonus.TraitID, Name: previewChineseName(bonus.TraitID, bonus.Name), Level: bonus.Level, RawLevel: bonus.RawLevel, MaxLevel: bonus.MaxLevel,
			Effect: bonus.Effect, Sources: sources,
		})
	}
}

func previewForLoadout(share *LoadoutShare, entry *LoadoutEntry, context *LoadoutStatContext, simulation *LoadoutSimulation) *loadoutSharePreview {
	if share == nil {
		return nil
	}
	preview := &loadoutSharePreview{Title: share.Name, CharacterHash: share.CharaHash, CharacterName: share.CharaName, WeaponHash: share.WeaponHash, WeaponName: previewChineseWeaponName(share.WeaponHash, share.WeaponName), MasteryCount: len(share.MasteryHashes)}
	entrySigils := make(map[int]LoadoutSigil)
	if entry != nil {
		for _, sigil := range entry.Sigils {
			entrySigils[sigil.Index] = sigil
		}
	}
	for index, sigil := range share.Sigils {
		primary, secondary, name := sigil.PrimaryTraitHash, sigil.SecondaryTraitHash, sigil.Name
		slotIndex := index
		if sigil.Index != nil {
			slotIndex = *sigil.Index
		}
		if resolved, ok := entrySigils[slotIndex]; ok {
			if value := strings.TrimSpace(resolved.Name); value != "" {
				name = value
			}
			if value := strings.TrimSpace(resolved.PrimaryTraitName); value != "" {
				primary = value
			}
			if value := strings.TrimSpace(resolved.SecondaryTraitName); value != "" {
				secondary = value
			}
		}
		primary = previewChineseName(sigil.PrimaryTraitHash, primary)
		secondary = previewChineseName(sigil.SecondaryTraitHash, secondary)
		name = previewChineseSigilNameForTraits(sigil.Hash, name, primary, secondary)
		secondaryLevel := sigil.SecondaryTraitLevel
		if strings.TrimSpace(secondary) == "" {
			secondaryLevel = 0
		}
		preview.Sigils = append(preview.Sigils, loadoutSharePreviewSigil{
			Hash: sigil.Hash, Name: name, Level: sigil.Level,
			PrimaryHash: sigil.PrimaryTraitHash, Primary: primary, PrimaryLevel: sigil.PrimaryTraitLevel,
			SecondaryHash: sigil.SecondaryTraitHash, Secondary: secondary, SecondaryLevel: secondaryLevel,
		})
	}
	for _, skill := range share.Skills {
		preview.Abilities = append(preview.Abilities, loadoutSharePreviewSkill{Hash: skill.Hash, Key: skill.Key, Name: previewChineseName(skill.Hash, skill.Name)})
	}
	if entry != nil && entry.Weapon != nil {
		for _, skill := range entry.Weapon.Skills {
			preview.WeaponSkills = append(preview.WeaponSkills, loadoutSharePreviewSkill{Hash: skill.TraitHash, Name: previewChineseName(skill.TraitHash, skill.Name), Level: skill.Level, Effect: skill.Effect})
		}
		if entry.Weapon.Wrightstone != nil {
			wrightstone := &loadoutSharePreviewWrightstone{Hash: entry.Weapon.Wrightstone.Hash, Name: previewChineseWrightstoneName(entry.Weapon.Wrightstone.Name)}
			for _, trait := range entry.Weapon.Wrightstone.Traits {
				wrightstone.Traits = append(wrightstone.Traits, loadoutSharePreviewSkill{Hash: trait.Hash, Name: previewChineseName(trait.Hash, trait.Name), Level: trait.Level})
			}
			preview.Wrightstone = wrightstone
		}
	}
	for index, summon := range share.Summons {
		item := loadoutSharePreviewSummon{TypeHash: summon.TypeHash, Name: summon.Name, Rank: summon.Rank, MainTraitHash: summon.MainTraitHash, MainTraitLevel: summon.MainTraitLevel, SubParamHash: summon.SubParamHash, SubParamLevel: summon.SubParamLevel}
		if context != nil && index < len(context.EquippedSummons) {
			resolved := context.EquippedSummons[index]
			item.TypeHash = resolved.TypeHash
			item.Name = resolved.Name
			item.MainTraitHash = resolved.MainTraitHash
			item.MainTrait = resolved.MainTraitName
			item.MainTraitLevel = resolved.MainTraitLevel
			item.SubParamHash = resolved.SubParamHash
			item.SubParam = summonSubParamLabel(resolved.SubParamName)
			if item.SubParam == "" {
				item.SubParam = resolved.SubParamName
			}
			item.SubParamLevel = resolved.SubParamLevel
			item.SubParamValue = resolved.SubParamValue
			item.SubParamUnit = resolved.SubParamUnit
		}
		preview.Summons = append(preview.Summons, item)
	}
	if context != nil && len(context.OverLimit) == 4 {
		for _, bonus := range context.OverLimit {
			preview.OverLimit = append(preview.OverLimit, loadoutSharePreviewOverLimit{
				Index: bonus.Index, AttributeHash: bonus.AttributeHash, Name: bonus.Name,
				Level: bonus.Level, Value: bonus.Value, Unit: bonus.Unit,
			})
		}
	} else {
		for _, slot := range share.OverLimit {
			item := loadoutSharePreviewOverLimit{Index: slot.Index, AttributeHash: slot.AttributeHash, Level: slot.Level}
			if hash, err := ParseHashHex(slot.AttributeHash); err == nil {
				if definition, ok := overLimitCatalog[hash]; ok {
					item.Name, item.Unit = definition.name, definition.unit
					if slot.Level >= 1 && slot.Level <= len(definition.values) {
						item.Value = definition.values[slot.Level-1]
						if definition.name == "昏厥值" {
							item.Value *= legacyMasteryStunPanelScale
						}
					}
				}
			}
			preview.OverLimit = append(preview.OverLimit, item)
		}
	}
	if entry != nil {
		masteryNodes := entry.Mastery
		if len(masteryNodes) != len(share.MasteryHashes) {
			masteryNodes = make([]LoadoutMasteryNode, 0, len(share.MasteryHashes))
			for _, value := range share.MasteryHashes {
				hash, err := ParseHashHex(value)
				if err != nil {
					continue
				}
				if node, ok := loadoutMasteryNodeForHash(hash); ok {
					masteryNodes = append(masteryNodes, node)
				}
			}
		}
		masteryIndex := make(map[string]int)
		for _, node := range masteryNodes {
			key := node.Rank + "\x00" + node.Name + "\x00" + node.Desc
			if existing, ok := masteryIndex[key]; ok {
				preview.MasterySkills[existing].Count++
				continue
			}
			masteryIndex[key] = len(preview.MasterySkills)
			preview.MasterySkills = append(preview.MasterySkills, loadoutSharePreviewMastery{Hash: node.Hash, Rank: node.RankLabel, Name: node.Name, Effect: node.Desc, Count: 1})
		}
	}
	if simulation != nil {
		setPreviewCombinedSkills(preview, simulation.Bonuses)
	}
	mastery := make([]uint32, 0, len(share.MasteryHashes))
	for _, value := range share.MasteryHashes {
		if hash, err := ParseHashHex(value); err == nil {
			mastery = append(mastery, hash)
		}
	}
	if summary, err := summarizeMasteryHashes(share.OwnerCode, mastery); err == nil {
		preview.MasteryCount = summary.Total
		preview.MasteryCat = summary.PrimaryCat
		preview.MasteryLabel = summary.PrimaryLabel
	}
	return preview
}

func loadoutPreviewEvidence(app *App, path string, share *LoadoutShare, entry *LoadoutEntry) (*LoadoutStatContext, *LoadoutSimulation) {
	if app == nil || share == nil || entry == nil {
		return nil, nil
	}
	cat, err := LoadCatalog()
	if err != nil {
		return nil, nil
	}
	save, err := LoadSave(path)
	if err != nil {
		return nil, nil
	}
	parsed, err := LoadSaveFile(path)
	if err != nil {
		return nil, nil
	}
	context, err := app.loadoutStatContextFromLoaded(path, share.CharaHash, parsed, save, false)
	if err != nil {
		return nil, nil
	}
	sigilSlots := make([]uint32, loadoutMaxSigils)
	for _, sigil := range entry.Sigils {
		if sigil.Index >= 0 && sigil.Index < len(sigilSlots) {
			sigilSlots[sigil.Index] = sigil.SlotID
		}
	}
	mastery := append([]string(nil), share.MasteryHashes...)
	simulation, err := app.loadoutSimulateBuildFromLoaded(path, share.CharaHash, entry.WeaponSlotID, sigilSlots, nil, mastery, context.EquippedSummonSlotIDs, cat, save, context, false)
	if err != nil {
		return context, nil
	}
	return context, simulation
}

func encodeLoadoutSharePreview(preview *loadoutSharePreview) string {
	if preview == nil {
		return ""
	}
	encode := func(value *loadoutSharePreview) ([]byte, bool) {
		payload, err := json.Marshal(value)
		if err != nil || len(payload) > loadoutSharePreviewMaxBytes {
			return nil, false
		}
		return payload, true
	}
	if payload, ok := encode(preview); ok {
		return base64.RawURLEncoding.EncodeToString(payload)
	}
	// Source labels are useful drill-down context, but are the least important
	// part of a public preview. Keep the complete localized skill ledger first.
	compact := *preview
	compact.CombinedSkills = append([]loadoutSharePreviewTrait(nil), preview.CombinedSkills...)
	for index := range compact.CombinedSkills {
		compact.CombinedSkills[index].Sources = nil
	}
	if payload, ok := encode(&compact); ok {
		return base64.RawURLEncoding.EncodeToString(payload)
	}
	// Effects remain available in the desktop app; names and effective levels
	// are more valuable than silently dropping the entire web preview.
	for index := range compact.CombinedSkills {
		compact.CombinedSkills[index].Effect = ""
	}
	for index := range compact.WeaponSkills {
		compact.WeaponSkills[index].Effect = ""
	}
	for index := range compact.MasterySkills {
		compact.MasterySkills[index].Effect = ""
	}
	if payload, ok := encode(&compact); ok {
		return base64.RawURLEncoding.EncodeToString(payload)
	}
	core := compact
	core.MasterySkills = nil
	core.CombinedSkills = nil
	if payload, ok := encode(&core); ok {
		return base64.RawURLEncoding.EncodeToString(payload)
	}
	return ""
}

func trimLoadoutShareTitle(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > loadoutShareTitleMaxRunes {
		runes = runes[:loadoutShareTitleMaxRunes]
	}
	return string(runes)
}

func publishLoadoutShareFrameWithMetadata(ctx context.Context, client *http.Client, endpoint string, frame []byte, preview *loadoutSharePreview) (*LoadoutPublishedShare, error) {
	if len(frame) == 0 || len(frame) > loadoutShareOnlineMaxFrameSize {
		return nil, fmt.Errorf("线上分享只接受不超过 %d KB 的配装帧", loadoutShareOnlineMaxFrameSize/1024)
	}
	ctx = loadoutShareRequestContext(ctx)
	endpoint = strings.TrimRight(endpoint, "/")
	headers := map[string]string{
		"Content-Type": "application/octet-stream",
		"Accept":       "application/json",
		"User-Agent":   repoName + "/" + appVersion,
	}
	if preview != nil {
		if encoded := encodeLoadoutSharePreview(preview); encoded != "" {
			headers["X-Loadout-Preview"] = encoded
		}
		headers["X-Loadout-Title-B64"] = base64.RawStdEncoding.EncodeToString([]byte(preview.Title))
		headers["X-Loadout-Character-B64"] = base64.RawStdEncoding.EncodeToString([]byte(preview.CharacterName))
		headers["X-Loadout-Character-Hash"] = preview.CharacterHash
	}
	var response *http.Response
	for attempt := 0; attempt < 2; attempt++ {
		var err error
		response, err = loadoutShareDo(ctx, client, http.MethodPost, endpoint+"/api/v1/loadouts", frame, headers)
		if err != nil {
			return nil, fmt.Errorf("连接配装分享服务失败: %w", err)
		}
		if response.StatusCode != http.StatusServiceUnavailable || attempt > 0 {
			break
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4*1024))
		_ = response.Body.Close()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("配装分享服务返回 %d: %s", response.StatusCode, loadoutShareOnlineErrorMessage(response))
	}
	var received LoadoutPublishedShare
	if err := json.NewDecoder(io.LimitReader(response.Body, 16*1024)).Decode(&received); err != nil {
		return nil, fmt.Errorf("解析配装分享服务响应失败: %w", err)
	}
	code, err := normalizeLoadoutShareShortCode(received.CompactCode)
	if err != nil {
		code, err = normalizeLoadoutShareShortCode(received.Code)
	}
	if err != nil {
		return nil, fmt.Errorf("配装分享服务返回了无效短码")
	}
	received.CompactCode = code
	received.Code = displayLoadoutShareShortCode(code)
	received.URL = endpoint + "/s/" + code
	received.DownloadURL = endpoint + "/download/" + code + ".gbfr-loadout"
	received.Bytes = len(frame)
	return &received, nil
}

func fetchLoadoutShareFrame(ctx context.Context, client *http.Client, endpoint, input string) ([]byte, error) {
	code, err := normalizeLoadoutShareShortCode(input)
	if err != nil {
		return nil, err
	}
	endpoint = strings.TrimRight(endpoint, "/")
	response, err := loadoutShareDo(ctx, client, http.MethodGet, endpoint+"/api/v1/loadouts/"+code, nil, map[string]string{
		"Accept":     "application/vnd.gbfr.loadout",
		"User-Agent": repoName + "/" + appVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("连接配装分享服务失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("读取配装短码失败 (%d): %s", response.StatusCode, loadoutShareOnlineErrorMessage(response))
	}
	frame, err := io.ReadAll(io.LimitReader(response.Body, loadoutShareOnlineMaxFrameSize+1))
	if err != nil {
		return nil, fmt.Errorf("读取线上配装失败: %w", err)
	}
	if len(frame) == 0 || len(frame) > loadoutShareOnlineMaxFrameSize {
		return nil, fmt.Errorf("线上配装帧大小无效")
	}
	return frame, nil
}

func decodeLoadoutShareFrame(frame []byte) (*LoadoutShare, error) {
	code := loadoutShareCodePrefix + base64.RawURLEncoding.EncodeToString(frame)
	return decodeLoadoutShareCode(code)
}

func (a *App) PublishLoadoutShare(savePath string, unitID uint32) (*LoadoutPublishedShare, error) {
	return a.publishLoadoutShare(savePath, unitID, "")
}

func (a *App) PublishLoadoutShareNamed(savePath string, unitID uint32, title string) (*LoadoutPublishedShare, error) {
	return a.publishLoadoutShare(savePath, unitID, title)
}

func publishLogsLoadoutShareCandidate(ctx context.Context, client *http.Client, endpoint string, candidate LogsLoadoutShareCandidate, title string) (*LoadoutPublishedShare, error) {
	share, err := decodeLoadoutShareCode(candidate.CompatibilityCode)
	if err != nil {
		return nil, fmt.Errorf("Logs 配装完整性校验失败: %w", err)
	}
	if share.SourceKind != loadoutShareSourceLogsDB {
		return nil, fmt.Errorf("只允许上传由 Logs 解析得到的配装")
	}
	if candidate.Preview == nil {
		return nil, fmt.Errorf("Logs 配装缺少可发布预览")
	}
	if !strings.EqualFold(strings.TrimSpace(candidate.CharacterHash), strings.TrimSpace(share.CharaHash)) ||
		!strings.EqualFold(strings.TrimSpace(candidate.OwnerCode), strings.TrimSpace(share.OwnerCode)) ||
		!strings.EqualFold(strings.TrimSpace(candidate.Preview.CharacterHash), strings.TrimSpace(share.CharaHash)) ||
		!strings.EqualFold(strings.TrimSpace(candidate.Preview.CharacterCode), strings.TrimSpace(share.OwnerCode)) {
		return nil, fmt.Errorf("Logs 配装角色标识与分享内容不一致")
	}
	frame, err := loadoutShareFrameFromCompatibilityCode(candidate.CompatibilityCode)
	if err != nil {
		return nil, err
	}
	preview := previewForRuntimeLoadout(share, *candidate.Preview)
	if preview == nil {
		return nil, fmt.Errorf("无法生成 Logs 配装预览")
	}
	if value := trimLoadoutShareTitle(title); value != "" {
		preview.Title = value
	}
	return publishLoadoutShareFrameWithMetadata(ctx, client, endpoint, frame, preview)
}

func (a *App) PublishLogsLoadoutShare(candidate LogsLoadoutShareCandidate, title string) (*LoadoutPublishedShare, error) {
	return publishLogsLoadoutShareCandidate(a.ctx, loadoutShareHTTPClient(), loadoutShareServiceURL, candidate, title)
}

func (a *App) publishLoadoutShare(savePath string, unitID uint32, title string) (*LoadoutPublishedShare, error) {
	share, err := buildLoadoutShare(savePath, unitID)
	if err != nil {
		return nil, err
	}
	encoded, err := a.LoadoutShareCode(savePath, unitID)
	if err != nil {
		return nil, err
	}
	frame, err := loadoutShareFrameFromCompatibilityCode(encoded.CompatibilityCode)
	if err != nil {
		return nil, err
	}
	var source *LoadoutEntry
	if groups, listErr := a.LoadoutList(savePath); listErr == nil {
		for groupIndex := range groups {
			for entryIndex := range groups[groupIndex].Loadouts {
				if groups[groupIndex].Loadouts[entryIndex].UnitID == unitID {
					source = &groups[groupIndex].Loadouts[entryIndex]
					break
				}
			}
		}
	}
	context, simulation := loadoutPreviewEvidence(a, savePath, share, source)
	preview := previewForLoadout(share, source, context, simulation)
	if strings.TrimSpace(title) != "" {
		preview.Title = trimLoadoutShareTitle(title)
	}
	return publishLoadoutShareFrameWithMetadata(a.ctx, loadoutShareHTTPClient(), loadoutShareServiceURL, frame, preview)
}

func (a *App) LoadoutImportShortCode(savePath, expectCharaHash, input string) (*LoadoutImportDraft, error) {
	frame, err := fetchLoadoutShareFrame(a.ctx, loadoutShareHTTPClient(), loadoutShareServiceURL, input)
	if err != nil {
		return nil, err
	}
	share, err := decodeLoadoutShareFrame(frame)
	if err != nil {
		return nil, fmt.Errorf("线上配装完整性校验失败: %w", err)
	}
	return resolveLoadoutShare(savePath, expectCharaHash, share)
}
