package backend

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const screenshotTargetSaveQAEnv = "GBFR_SCREENSHOT_TARGET_SAVE_QA"

type screenshotTarget struct {
	Name  string `json:"name"`
	Level int    `json:"level"`
}

var fedielScreenshotTargets = []screenshotTarget{
	{Name: "因子强化", Level: 1},
	{Name: "体力", Level: 52},
	{Name: "昏厥", Level: 48},
	{Name: "天星之止息", Level: 16},
	{Name: "伤害上限", Level: 63},
	{Name: "追击", Level: 48},
	{Name: "狂战士", Level: 15},
	{Name: "斯巴达", Level: 15},
	{Name: "天星之雪", Level: 16},
	{Name: "浪迹天涯", Level: 16},
	{Name: "属性克制转换", Level: 15},
	{Name: "浩劫新星", Level: 35},
	{Name: "超凡破限", Level: 55},
	{Name: "HP吸收", Level: 25},
	{Name: "迅捷能力", Level: 16},
	{Name: "怒涛", Level: 16},
	{Name: "明镜止水", Level: 16},
	{Name: "金刚", Level: 10},
	{Name: "坚持", Level: 15},
	{Name: "躲避性能", Level: 16},
	{Name: "不动", Level: 16},
	{Name: "可怕的漆黑钳蟹因子", Level: 21},
	{Name: "霸体", Level: 15},
	{Name: "黑龙的咒印", Level: 16},
	{Name: "黑龙的折跃", Level: 16},
	{Name: "黑龙的战气", Level: 16},
}

type screenshotSolverBundle struct {
	Atlas    *SigilAtlasIndex                `json:"atlas"`
	Context  *LoadoutEditContext             `json:"context"`
	Snapshot *LoadoutOptimizerDomainSnapshot `json:"snapshot"`
	Loadout  LoadoutEntry                    `json:"loadout"`
	Targets  []screenshotTarget              `json:"targets"`
}

type screenshotPlanCandidate struct {
	ID                      string `json:"id"`
	Source                  string `json:"source"`
	Name                    string `json:"name"`
	SlotID                  uint32 `json:"slotId"`
	SigilID                 string `json:"sigilId"`
	SigilLevel              int    `json:"sigilLevel"`
	PrimaryTraitID          string `json:"primaryTraitId"`
	PrimaryTraitName        string `json:"primaryTraitName"`
	PrimaryLevel            int    `json:"primaryLevel"`
	SecondaryTraitID        string `json:"secondaryTraitId"`
	SecondaryTraitName      string `json:"secondaryTraitName"`
	SecondaryLevel          int    `json:"secondaryLevel"`
	ExactSigilHash          string `json:"exactSigilHash"`
	ExactPrimaryTraitHash   string `json:"exactPrimaryTraitHash"`
	ExactSecondaryTraitHash string `json:"exactSecondaryTraitHash"`
}

type screenshotTargetResult struct {
	TraitID   string `json:"traitId"`
	Name      string `json:"name"`
	Requested int    `json:"requested"`
	Achieved  int    `json:"achieved"`
	Effective int    `json:"effective"`
	Met       bool   `json:"met"`
}

type screenshotSolverResult struct {
	Plan struct {
		Picked       []screenshotPlanCandidate `json:"picked"`
		ApplyPayload struct {
			Equipment map[string][]map[string]any `json:"equipment"`
		} `json:"applyPayload"`
	} `json:"plan"`
	TargetResults   []screenshotTargetResult `json:"targetResults"`
	CompletedPrefix int                      `json:"completedPrefix"`
}

func fedielScreenshotLoadout(t *testing.T, app *App, path string) (CharacterLoadouts, LoadoutEntry) {
	t.Helper()
	groups, err := app.LoadoutList(path)
	if err != nil {
		t.Fatal(err)
	}
	var selectedGroup CharacterLoadouts
	var selected LoadoutEntry
	best := -1
	for _, group := range groups {
		if !strings.EqualFold(group.CharaHash, "74DD4C79") {
			continue
		}
		selectedGroup = group
		for _, loadout := range group.Loadouts {
			if loadout.IsParty {
				continue
			}
			score := len(loadout.Sigils)*100 + len(loadout.Mastery)
			if loadout.WeaponSlotID != 0 {
				score += 1000
			}
			if score > best {
				best, selected = score, loadout
			}
		}
	}
	if selected.UnitID == 0 {
		t.Fatal("SaveData2 has no saved Fediel loadout")
	}
	return selectedGroup, selected
}

func screenshotJSON(t *testing.T, path string, value any) {
	t.Helper()
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
}

func screenshotMapUint32(value any, key string) uint32 {
	entry, _ := value.(map[string]any)
	number, _ := entry[key].(float64)
	return uint32(number)
}

func screenshotMapInt(value any, key string) int {
	entry, _ := value.(map[string]any)
	number, _ := entry[key].(float64)
	return int(number)
}

func screenshotMapString(value any, key string) string {
	entry, _ := value.(map[string]any)
	text, _ := entry[key].(string)
	return text
}

func screenshotMapStrings(value any, key string) []string {
	entry, _ := value.(map[string]any)
	values, _ := entry[key].([]any)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func loadoutSlotVectors(loadout LoadoutEntry) ([]uint32, []string, []string) {
	sigils := make([]uint32, loadoutMaxSigils)
	for _, sigil := range loadout.Sigils {
		if sigil.Index >= 0 && sigil.Index < len(sigils) {
			sigils[sigil.Index] = sigil.SlotID
		}
	}
	skills := make([]string, 0, len(loadout.Skills))
	for _, skill := range loadout.Skills {
		skills = append(skills, skill.Hash)
	}
	mastery := make([]string, 0, len(loadout.Mastery))
	for _, node := range loadout.Mastery {
		mastery = append(mastery, node.Hash)
	}
	return sigils, skills, mastery
}

func TestSaveData2SummonIdentityParity(t *testing.T) {
	source := strings.TrimSpace(os.Getenv(screenshotTargetSaveQAEnv))
	if source == "" {
		t.Skipf("set %s to a real SaveData2.dat", screenshotTargetSaveQAEnv)
	}
	app := &App{}
	groups, err := app.LoadoutList(source)
	if err != nil || len(groups) == 0 {
		t.Fatalf("load real-save character list: groups=%d err=%v", len(groups), err)
	}
	stats, err := app.LoadoutStatContext(source, groups[0].CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	raw := mustLoadSave(t, source)
	for _, summon := range stats.Summons {
		unitID, state, readErr := exactSummonStateForSlot(raw, summon.SlotID)
		if readErr != nil {
			t.Errorf("typed summon slot=%d unit=%d is absent from writable view: %v", summon.SlotID, summon.UnitID, readErr)
			continue
		}
		if unitID != summon.UnitID || hashText(state.TypeHash) != normalizedHashText(summon.TypeHash) {
			t.Errorf("summon identity mismatch slot=%d typedUnit=%d writableUnit=%d typedType=%s writableType=%s",
				summon.SlotID, summon.UnitID, unitID, summon.TypeHash, hashText(state.TypeHash))
		}
	}
}

func TestFedielScreenshotTargetsRoundTripOnRealSaveCopy(t *testing.T) {
	previousLanguage := getCurrentLanguage()
	setCurrentLanguage("zh")
	t.Cleanup(func() { setCurrentLanguage(previousLanguage) })
	source := strings.TrimSpace(os.Getenv(screenshotTargetSaveQAEnv))
	if source == "" {
		t.Skipf("set %s to a real SaveData2.dat; the test writes only an internal temp copy", screenshotTargetSaveQAEnv)
	}
	absSource, err := filepath.Abs(source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Base(absSource), "SaveData2.dat") {
		t.Fatalf("QA source must be named SaveData2.dat: %s", absSource)
	}
	sourceBytes, err := os.ReadFile(absSource)
	if err != nil {
		t.Fatal(err)
	}
	sourceDigest := sha256.Sum256(sourceBytes)
	workDir := t.TempDir()
	inputPath := filepath.Join(workDir, "input", "SaveData2.dat")
	outputPath := filepath.Join(workDir, "output", "SaveData2.dat")
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, sourceBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", filepath.Join(workDir, "appdata"))

	app := &App{}
	group, loadout := fedielScreenshotLoadout(t, app, inputPath)
	context, err := app.LoadoutEditContext(inputPath, group.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	if context.OwnerCode != "PL2900" {
		t.Fatalf("Fediel owner code=%q, want PL2900", context.OwnerCode)
	}
	snapshot, err := app.LoadoutOptimizerInventorySnapshot(inputPath, group.CharaHash, loadout.UnitID)
	if err != nil {
		t.Fatal(err)
	}
	atlas, err := NewSigilGen().GetSigilAtlasIndex()
	if err != nil {
		t.Fatal(err)
	}
	bundlePath := filepath.Join(workDir, "bundle.json")
	resultPath := filepath.Join(workDir, "solver-result.json")
	screenshotJSON(t, bundlePath, screenshotSolverBundle{Atlas: atlas, Context: context, Snapshot: snapshot, Loadout: loadout, Targets: fedielScreenshotTargets})
	scriptPath, err := filepath.Abs(filepath.Join("..", "..", "frontend", "scripts", "qa_real_save_target_solver.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("node", scriptPath, bundlePath, resultPath)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("real-save target solver failed: %v\n%s", err, output)
	}
	t.Logf("solver: %s", output)
	resultBytes, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	var solver screenshotSolverResult
	if err := json.Unmarshal(resultBytes, &solver); err != nil {
		t.Fatal(err)
	}
	if len(solver.TargetResults) != len(fedielScreenshotTargets) {
		t.Fatalf("solver returned %d targets, want %d", len(solver.TargetResults), len(fedielScreenshotTargets))
	}
	if solver.CompletedPrefix != len(fedielScreenshotTargets) {
		for index, candidate := range solver.Plan.Picked {
			t.Logf("picked %02d: %s | %s Lv%d + %s Lv%d | source=%s slot=%d", index+1, candidate.Name,
				candidate.PrimaryTraitName, candidate.PrimaryLevel, candidate.SecondaryTraitName, candidate.SecondaryLevel,
				candidate.Source, candidate.SlotID)
		}
		for _, target := range solver.TargetResults {
			t.Logf("target %s: %d/%d met=%v", target.Name, target.Effective, target.Requested, target.Met)
		}
		t.Fatalf("screenshot build completed only %d ordered targets, want all %d", solver.CompletedPrefix, len(fedielScreenshotTargets))
	}
	for index := 0; index < len(fedielScreenshotTargets); index++ {
		if !solver.TargetResults[index].Met {
			t.Fatalf("ordered screenshot target %d (%s) is inside completed prefix but not met", index+1, solver.TargetResults[index].Name)
		}
	}

	baseSigils, skills, mastery := loadoutSlotVectors(loadout)
	finalSigils := make([]uint32, loadoutMaxSigils)
	constructed := make([]LoadoutConstructedSigil, 0)
	used := make(map[uint32]bool)
	cursor := 0
	for _, candidate := range solver.Plan.Picked {
		if cursor >= loadoutMaxSigils {
			break
		}
		if candidate.Source == "inventory" {
			if candidate.SlotID == 0 || used[candidate.SlotID] {
				t.Fatalf("solver selected invalid/duplicate inventory factor: %+v", candidate)
			}
			finalSigils[cursor] = candidate.SlotID
			used[candidate.SlotID] = true
			cursor++
			continue
		}
		if candidate.SigilID == "" || candidate.PrimaryTraitID == "" {
			t.Fatalf("solver selected an incomplete constructed factor: %+v", candidate)
		}
		constructed = append(constructed, LoadoutConstructedSigil{
			Index: cursor, ExactSigilHash: candidate.ExactSigilHash,
			ExactPrimaryTraitHash:   candidate.ExactPrimaryTraitHash,
			ExactSecondaryTraitHash: candidate.ExactSecondaryTraitHash,
			Item: QueueItem{
				SigilID: candidate.SigilID, SigilName: candidate.Name, Level: candidate.SigilLevel,
				PrimaryTraitID: candidate.PrimaryTraitID, PrimaryTraitName: candidate.PrimaryTraitName, PrimaryLevel: candidate.PrimaryLevel,
				SecondaryTraitID: candidate.SecondaryTraitID, SecondaryTraitName: candidate.SecondaryTraitName, SecondaryLevel: candidate.SecondaryLevel,
				Quantity: 1,
			},
		})
		cursor++
	}
	for _, slotID := range baseSigils {
		if cursor >= loadoutMaxSigils {
			break
		}
		if slotID == 0 || used[slotID] {
			continue
		}
		finalSigils[cursor] = slotID
		used[slotID] = true
		cursor++
	}

	weaponSlotID := loadout.WeaponSlotID
	weaponSkillHashes := append([]string(nil), loadout.WeaponSkillHashes...)
	weaponTranscendence := 0
	if rows := solver.Plan.ApplyPayload.Equipment["weapon"]; len(rows) > 0 {
		if value := screenshotMapUint32(rows[0], "weaponSlotId"); value != 0 {
			weaponSlotID = value
		}
		if values := screenshotMapStrings(rows[0], "weaponSkillHashes"); len(values) == 5 {
			weaponSkillHashes = values
		}
		weaponTranscendence = screenshotMapInt(rows[0], "weaponTranscendence")
	}
	summonSlotIDs := make([]uint32, 0, 4)
	summonEdits := make([]LoadoutSummonInlineEdit, 0, 4)
	for _, row := range solver.Plan.ApplyPayload.Equipment["summons"] {
		if slotID := screenshotMapUint32(row, "slotId"); slotID != 0 {
			summonSlotIDs = append(summonSlotIDs, slotID)
			if mainHash := screenshotMapString(row, "mainTraitHash"); mainHash != "" {
				summonEdits = append(summonEdits, LoadoutSummonInlineEdit{
					SlotID: slotID, ExpectUnitID: screenshotMapUint32(row, "expectUnitId"),
					ExpectTypeHash: screenshotMapString(row, "expectTypeHash"),
					ExpectMainTraitHash: screenshotMapString(row, "expectMainTraitHash"),
					ExpectMainTraitLevel: screenshotMapInt(row, "expectMainTraitLevel"),
					ExpectSubParamHash: screenshotMapString(row, "expectSubParamHash"),
					ExpectSubParamLevel: screenshotMapInt(row, "expectSubParamLevel"), ExpectRank: screenshotMapInt(row, "expectRank"),
					MainTraitHash: mainHash, MainTraitLevel: screenshotMapInt(row, "mainTraitLevel"),
					SubParamHash: screenshotMapString(row, "subParamHash"), SubParamLevel: screenshotMapInt(row, "subParamLevel"), Rank: screenshotMapInt(row, "rank"),
				})
			}
		}
	}
	if len(summonSlotIDs) != 4 {
		stats, statsErr := app.LoadoutStatContext(inputPath, group.CharaHash)
		if statsErr != nil {
			t.Fatal(statsErr)
		}
		summonSlotIDs = append([]uint32(nil), stats.EquippedSummonSlotIDs...)
	}
	write := LoadoutWrite{
		UnitID: loadout.UnitID, ExpectCharaHash: group.CharaHash, Op: "write", Name: "截图联合求解回读",
		WeaponSlotID: weaponSlotID, WeaponSkillHashes: weaponSkillHashes,
		SigilSlotIDs: finalSigils, ConstructedSigils: constructed,
		SkillHashes: skills, MasteryHashes: mastery, SummonSlotIDs: summonSlotIDs,
	}
	weaponEdits := make([]LoadoutWeaponInlineEdit, 0, 1)
	weaponContext, contextErr := readLoadoutWeaponContext(mustLoadSave(t, inputPath), weaponSlotID)
	if contextErr != nil {
		t.Fatal(contextErr)
	}
	expectedWeaponSkills := make([]string, 5)
	for _, slot := range weaponContext.SkillSlots {
		if slot.Index >= 0 && slot.Index < len(expectedWeaponSkills) {
			expectedWeaponSkills[slot.Index] = slot.CurrentHash
		}
	}
	for index := range expectedWeaponSkills {
		if strings.TrimSpace(expectedWeaponSkills[index]) == "" {
			expectedWeaponSkills[index] = hashText(EmptyHash)
		}
	}
	if weaponTranscendence == 0 {
		weaponTranscendence = weaponContext.Transcendence
	}
	if len(weaponSkillHashes) == 5 && (weaponTranscendence != weaponContext.Transcendence || !reflect.DeepEqual(weaponSkillHashes, expectedWeaponSkills)) {
		weaponEdits = append(weaponEdits, LoadoutWeaponInlineEdit{
			SlotID: weaponSlotID, ExpectUnitID: weaponContext.UnitID, ExpectStoredHash: weaponContext.StoredHash,
			ExpectTranscendence: weaponContext.Transcendence, Transcendence: &weaponTranscendence,
			ExpectSkillHashes: expectedWeaponSkills, SkillHashes: weaponSkillHashes,
		})
	}
	applyResult, err := app.LoadoutApplyWithResources(inputPath, outputPath, LoadoutApplyRequest{
		Changes: []LoadoutWrite{write}, WeaponEdits: weaponEdits, SummonEdits: summonEdits,
	})
	if err != nil {
		t.Fatal(err)
	}
	if applyResult.SlotsWritten != 1 || applyResult.CreatedCount != len(constructed) || applyResult.VerifiedCount != len(constructed) {
		t.Fatalf("write/readback counts mismatch: %+v", applyResult)
	}

	_, readback := fedielScreenshotLoadout(t, app, outputPath)
	if readback.UnitID != loadout.UnitID {
		groups, listErr := app.LoadoutList(outputPath)
		if listErr != nil {
			t.Fatal(listErr)
		}
		found := false
		for _, afterGroup := range groups {
			if !strings.EqualFold(afterGroup.CharaHash, group.CharaHash) {
				continue
			}
			for _, candidate := range afterGroup.Loadouts {
				if candidate.UnitID == loadout.UnitID {
					readback, found = candidate, true
					break
				}
			}
		}
		if !found {
			t.Fatalf("written loadout %d disappeared after readback", loadout.UnitID)
		}
	}
	readbackSigils, _, readbackMastery := loadoutSlotVectors(readback)
	readbackSnapshot, err := loadLoadoutReadSnapshot(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	parsedReadback, err := readbackSnapshot.parsedSave()
	if err != nil {
		t.Fatal(err)
	}
	readbackStats, err := app.loadoutStatContextFromLoaded(outputPath, group.CharaHash, parsedReadback, readbackSnapshot.save, false)
	if err != nil {
		t.Fatal(err)
	}
	readbackCatalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	simulation, err := app.loadoutSimulateBuildFromLoaded(outputPath, group.CharaHash, readback.WeaponSlotID, readbackSigils, nil, readbackMastery, summonSlotIDs, readbackCatalog, readbackSnapshot.save, readbackStats, false)
	if err != nil {
		t.Fatal(err)
	}
	bonusByTrait := make(map[string]int, len(simulation.Bonuses))
	for _, bonus := range simulation.Bonuses {
		bonusByTrait[bonus.TraitID] = bonus.RawLevel
	}
	for _, target := range solver.TargetResults {
		actual := bonusByTrait[target.TraitID]
		if actual != target.Achieved {
			t.Errorf("%s readback level=%d, solver=%d, target=%d", target.Name, actual, target.Achieved, target.Requested)
		}
		if actual != target.Requested {
			t.Errorf("%s readback level=%d, screenshot requires exactly %d", target.Name, actual, target.Requested)
		}
		if target.Met && actual < target.Requested {
			t.Errorf("%s was marked met but readback level=%d < %d", target.Name, actual, target.Requested)
		}
	}
	if digest := sha256.Sum256(mustReadFile(t, absSource)); digest != sourceDigest {
		t.Fatal("the original SaveData2 changed during isolated-copy QA")
	}
	inputAfter := sha256.Sum256(mustReadFile(t, inputPath))
	if inputAfter != sourceDigest {
		t.Fatal("the read-only temp input changed during output-copy QA")
	}
	if reflect.DeepEqual(sourceBytes, mustReadFile(t, outputPath)) {
		t.Fatal("round-trip output did not contain the staged build")
	}
	if t.Failed() {
		t.FailNow()
	}
	t.Logf("real SaveData2 copy round-trip passed: source=%X output=%s created=%d verifiedFields=%d completedPrefix=%d/%d",
		sourceDigest, outputPath, applyResult.CreatedCount, applyResult.VerifiedFields, solver.CompletedPrefix, len(solver.TargetResults))
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(fmt.Errorf("read %s: %w", path, err))
	}
	return payload
}

func mustLoadSave(t *testing.T, path string) *SaveData {
	t.Helper()
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	return save
}
