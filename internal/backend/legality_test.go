package backend

import (
	"strings"
	"testing"
)

func TestSigilLegalityAllowsSynthesisTableSecondaryWithWarning(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sg := &SigilGen{catalog: catalog}

	var sigil *SigilDef
	for i := range catalog.Sigils {
		candidate := &catalog.Sigils[i]
		if supportsGeneratedPlusSigil(candidate) && !requiresFixedSigilSecondary(candidate) {
			sigil = candidate
			break
		}
	}
	illegal := catalog.LookupTraitByHash(0xF26BAEA5) // Divergence is present in the 2.0.3 synthesis trait table.
	if sigil == nil || illegal == nil {
		t.Fatal("test catalog has no normal plus sigil or summon-only trait")
	}
	if strings.HasPrefix(illegal.InternalID, "SKILL_") {
		t.Fatal("test summon-only trait unexpectedly uses a sigil skill identity")
	}
	sigilLevels, _ := catalog.RequireSigilLevels(sigil)
	primaryLevels, _ := catalog.RequirePrimaryTraitLevels(sigil)
	secondaryLevels, _ := requireTraitLevels(illegal, "test summon-only trait")
	item := QueueItem{
		SigilID: sigil.InternalID, Level: sigilLevels[0], PrimaryLevel: primaryLevels[0],
		SecondaryTraitID: illegal.InternalID, SecondaryLevel: secondaryLevels[0], Quantity: 1,
	}
	report, err := sg.CheckLegality(item)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityForced || !report.Writable {
		t.Fatalf("expected forced+writable, got %+v", report)
	}
	if err := sg.AddToQueue(item); err != nil {
		t.Fatalf("synthesis-table trait should enter the sigil write queue: %v", err)
	}
}

func TestSigilLegalityAllowsCataloguedNonNaturalSecondary(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sigil, err := catalog.RequireSigil("GEEN_096_24")
	if err != nil {
		t.Fatal(err)
	}
	item := QueueItem{
		SigilID: sigil.InternalID, Level: 15, PrimaryLevel: 15,
		SecondaryTraitID: "SKILL_036_00", SecondaryLevel: 15, Quantity: 1,
	}
	report, err := (&SigilGen{catalog: catalog}).CheckLegality(item)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityForced || !report.Writable {
		t.Fatalf("catalogued non-natural secondary should be forced+writable, got %+v", report)
	}
}

func TestSigilLegalityRejectsFreeformPrimaryTrait(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sigil := &catalog.Sigils[0]
	var primary *TraitDef
	for index := range catalog.Traits {
		candidate := &catalog.Traits[index]
		if isSelectableTrait(candidate) && candidate.InternalID != sigil.PrimaryTraitID {
			primary = candidate
			break
		}
	}
	if primary == nil {
		t.Fatal("test catalog has no alternate primary trait")
	}
	item := QueueItem{SigilID: sigil.InternalID, Level: 15, PrimaryTraitID: primary.InternalID, PrimaryLevel: 15, Quantity: 1}
	_, report, err := (&SigilGen{catalog: catalog}).normalizeQueueItem(item)
	if err != nil {
		t.Fatal(err)
	}
	if report.Writable || report.Status != LegalityImpossible {
		t.Fatalf("alternate primary must be rejected: %+v", report)
	}
}

func TestSigilLegalityRejectsSecondaryOnNaturalSingleSlot(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sg := &SigilGen{catalog: catalog}
	var sigil *SigilDef
	for i := range catalog.Sigils {
		if !supportsGeneratedPlusSigil(&catalog.Sigils[i]) {
			sigil = &catalog.Sigils[i]
			break
		}
	}
	if sigil == nil {
		t.Fatal("test catalog has no non-plus sigil")
	}
	sigilLevels, _ := catalog.RequireSigilLevels(sigil)
	primaryLevels, _ := catalog.RequirePrimaryTraitLevels(sigil)
	trait := &catalog.Traits[0]
	item := QueueItem{SigilID: sigil.InternalID, Level: sigilLevels[0], PrimaryLevel: primaryLevels[0], SecondaryTraitID: trait.InternalID, SecondaryLevel: 1, Quantity: 1}
	report, err := sg.CheckLegality(item)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityImpossible || report.Writable {
		t.Fatalf("non-natural secondary must be rejected, got %+v", report)
	}
}

func TestSigilLevelsRespectEffectCurveAndNaturalDefault(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var sigil *SigilDef
	for i := range catalog.Sigils {
		if !supportsGeneratedPlusSigil(&catalog.Sigils[i]) {
			sigil = &catalog.Sigils[i]
			break
		}
	}
	if sigil == nil {
		t.Fatal("test catalog has no sigil without secondary slot")
	}
	sg := &SigilGen{catalog: catalog}
	base := QueueItem{SigilID: sigil.InternalID, Level: 15, PrimaryLevel: 15, Quantity: 1}
	report, err := sg.CheckLegality(base)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityLegal || !report.Writable {
		t.Fatalf("level 15 must be legal+writable, got %+v", report)
	}
	base.Level = 16
	report, err = sg.CheckLegality(base)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityForced || !report.Writable {
		t.Fatalf("level 16 must be forced+writable, got %+v", report)
	}
	base.Level = sigilWritableLevelMax + 1
	report, err = sg.CheckLegality(base)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityForced || !report.Writable {
		t.Fatalf("factor level above the usual reference must warn but remain writable, got %+v", report)
	}
}

func TestSigilSecondaryLevelsRespectEffectCurve(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var sigil *SigilDef
	var secondary *TraitDef
	for i := range catalog.Sigils {
		candidate := &catalog.Sigils[i]
		if !supportsGeneratedPlusSigil(candidate) {
			continue
		}
		allowed, _ := catalog.GetAllowedSecondaryTraits(candidate)
		for _, trait := range allowed {
			if trait.InternalID != candidate.PrimaryTraitID {
				sigil, secondary = candidate, trait
				break
			}
		}
		if sigil != nil {
			break
		}
	}
	if sigil == nil || secondary == nil {
		t.Fatal("test catalog has no natural plus-sigil combination")
	}
	sg := &SigilGen{catalog: catalog}
	base := QueueItem{SigilID: sigil.InternalID, Level: 15, PrimaryLevel: 15, SecondaryTraitID: secondary.InternalID, SecondaryLevel: 15, Quantity: 1}
	report, err := sg.CheckLegality(base)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityLegal || !report.Writable {
		t.Fatalf("secondary level 15 must be legal+writable, got %+v", report)
	}
	levels, err := catalog.RequireSecondaryTraitLevels(sigil, secondary)
	if err != nil {
		t.Fatal(err)
	}
	if effectCurveMax(levels, 15) <= 15 {
		t.Skip("selected secondary trait has no curve levels above natural 15")
	}
	base.SecondaryLevel = 16
	report, err = sg.CheckLegality(base)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityForced || !report.Writable {
		t.Fatalf("secondary level 16 must be forced+writable, got %+v", report)
	}
}

func TestGeneratedPlusSigilAllowsEmptySecondarySlot(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var sigil *SigilDef
	for i := range catalog.Sigils {
		if supportsGeneratedPlusSigil(&catalog.Sigils[i]) {
			sigil = &catalog.Sigils[i]
			break
		}
	}
	if sigil == nil {
		t.Fatal("test catalog has no generated plus sigil")
	}
	sg := &SigilGen{catalog: catalog}
	report, err := sg.CheckLegality(QueueItem{
		SigilID: sigil.InternalID, Level: 15, PrimaryLevel: 15,
		SecondaryTraitID: "", SecondaryLevel: 0, Quantity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityLegal || !report.Writable {
		t.Fatalf("empty secondary slot must be legal+writable, got %+v", report)
	}
}

func TestSupplementaryDamageAllowsPreciseWrathSecondary(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sigil, err := catalog.RequireSigil("GEEN_151_24")
	if err != nil {
		t.Fatal(err)
	}
	allowed, err := catalog.GetAllowedSecondaryTraits(sigil)
	if err != nil {
		t.Fatal(err)
	}
	for _, trait := range allowed {
		if trait.InternalID == "SKILL_109_00" {
			return
		}
	}
	t.Fatal("Supplementary Damage V+ must accept Precise Wrath as a natural secondary trait")
}

func TestWrightstoneRegenWritableCapIsFortyFive(t *testing.T) {
	catalog, err := LoadWrightstoneCatalog()
	if err != nil {
		t.Fatal(err)
	}
	trait, err := catalog.RequireTrait("SKILL_066_00")
	if err != nil {
		t.Fatal(err)
	}
	levels, err := requireWrightstoneTraitLevels(trait)
	if err != nil {
		t.Fatal(err)
	}
	if got := highestLevel(levels, 0); got != 45 {
		t.Fatalf("Regen blessing writable cap = %d, want 45", got)
	}
}

func TestWrightstoneLegalityRejectsNonFixedFirstTrait(t *testing.T) {
	catalog, err := LoadWrightstoneCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Wrightstones) == 0 || len(catalog.Traits) < 3 {
		t.Fatal("wrightstone test catalog is incomplete")
	}
	w := &catalog.Wrightstones[0]
	traits := make([]*WrightstoneTraitDef, 0, 3)
	for i := range catalog.Traits {
		trait := &catalog.Traits[i]
		levels, levelErr := requireWrightstoneTraitLevels(trait)
		if levelErr == nil && len(levels) > 0 && (len(traits) > 0 || trait.InternalID != w.DefaultTraitID) {
			traits = append(traits, trait)
		}
		if len(traits) == 3 {
			break
		}
	}
	if len(traits) < 3 {
		t.Fatal("wrightstone test catalog has too few usable traits")
	}
	levels0, _ := requireWrightstoneTraitLevels(traits[0])
	levels1, _ := requireWrightstoneTraitLevels(traits[1])
	levels2, _ := requireWrightstoneTraitLevels(traits[2])
	item := WrightstoneQueueItem{
		WrightstoneID: w.InternalID,
		FirstTraitID:  traits[0].InternalID, FirstLevel: levels0[0],
		SecondTraitID: traits[1].InternalID, SecondLevel: levels1[0],
		ThirdTraitID: traits[2].InternalID, ThirdLevel: levels2[0], Quantity: 1,
	}
	wg := &WrightstoneGen{catalog: catalog}
	report, err := wg.CheckLegality(item)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityImpossible || report.Writable {
		t.Fatalf("expected impossible+not writable, got %+v", report)
	}
	if err := wg.AddToQueue(item); err == nil {
		t.Fatal("wrightstone with a non-fixed first trait must not enter the queue")
	}
}

func TestWrightstoneSlotNaturalCapsRemainWritable(t *testing.T) {
	catalog, err := LoadWrightstoneCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Wrightstones) == 0 {
		t.Fatal("wrightstone catalog is empty")
	}
	w := &catalog.Wrightstones[0]
	first, err := catalog.RequireTrait(w.DefaultTraitID)
	if err != nil {
		t.Fatal(err)
	}
	picked := []*WrightstoneTraitDef{first}
	for i := range catalog.Traits {
		trait := &catalog.Traits[i]
		if trait.InternalID != first.InternalID {
			if _, levelErr := requireWrightstoneTraitLevels(trait); levelErr != nil {
				continue
			}
			picked = append(picked, trait)
		}
		if len(picked) == 3 {
			break
		}
	}
	if len(picked) < 3 {
		t.Fatal("not enough traits")
	}
	item := WrightstoneQueueItem{
		WrightstoneID: w.InternalID,
		FirstTraitID:  picked[0].InternalID, FirstLevel: 20,
		SecondTraitID: picked[1].InternalID, SecondLevel: 15,
		ThirdTraitID: picked[2].InternalID, ThirdLevel: 10,
		Quantity: 1,
	}
	wg := &WrightstoneGen{catalog: catalog}
	report, err := wg.CheckLegality(item)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Writable {
		t.Fatalf("natural caps must remain writable: %+v", report)
	}
	item.FirstLevel = 21
	item.SecondLevel = 16
	report, err = wg.CheckLegality(item)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityForced || !report.Writable {
		t.Fatalf("over-cap levels must be forced+writable, got %+v", report)
	}
	levels, _ := requireWrightstoneTraitLevels(picked[0])
	item.FirstLevel = effectCurveMax(levels, 20) + 1
	report, err = wg.CheckLegality(item)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityForced || !report.Writable {
		t.Fatalf("level above the skill effect curve must warn but remain writable, got %+v", report)
	}
}

func TestSigilLevelsBeyondUsualReferenceRemainWritable(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var sigil *SigilDef
	for index := range catalog.Sigils {
		candidate := &catalog.Sigils[index]
		if catalog.IsSigilConstructible(candidate) && !requiresFixedSigilSecondary(candidate) {
			sigil = candidate
			break
		}
	}
	if sigil == nil {
		t.Fatal("test catalog has no writable factor")
	}
	item := QueueItem{
		SigilID: sigil.InternalID, Level: sigilWritableLevelMax + 1,
		PrimaryTraitID: sigil.PrimaryTraitID, PrimaryLevel: 15, Quantity: 1,
	}
	report, err := (&SigilGen{catalog: catalog}).CheckLegality(item)
	if err != nil || !report.Writable || report.Status != LegalityForced {
		t.Fatalf("factor level above 50 must warn but remain writable: report=%+v err=%v", report, err)
	}
}

func TestSigilTraitBelowNaturalReferenceUsesItsEffectCurveCap(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sigil, err := catalog.RequireSigil("B289A9AD")
	if err != nil {
		t.Fatal(err)
	}
	item := QueueItem{SigilID: sigil.InternalID, Level: 5, PrimaryLevel: 5, Quantity: 1}
	report, err := (&SigilGen{catalog: catalog}).CheckLegality(item)
	if err != nil || !report.Writable {
		t.Fatalf("level 5 special trait should be writable: report=%+v err=%v", report, err)
	}
	item.PrimaryLevel = 6
	report, err = (&SigilGen{catalog: catalog}).CheckLegality(item)
	if err != nil || !report.Writable || report.Status != LegalityForced {
		t.Fatalf("level 6 must warn but remain writable above the five-level curve: report=%+v err=%v", report, err)
	}
}

func TestWrightstoneLevelsRejectValuesBeyondSkillCurve(t *testing.T) {
	catalog, err := LoadWrightstoneCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Wrightstones) == 0 || len(catalog.Traits) < 3 {
		t.Fatal("wrightstone catalog is incomplete")
	}
	item := WrightstoneQueueItem{
		WrightstoneID: catalog.Wrightstones[0].InternalID,
		FirstTraitID:  catalog.Wrightstones[0].DefaultTraitID, FirstLevel: 1,
		SecondTraitID: catalog.Traits[1].InternalID, SecondLevel: 1,
		ThirdTraitID: catalog.Traits[2].InternalID, ThirdLevel: 1,
		Quantity: 1,
	}
	first, err := catalog.RequireTrait(item.FirstTraitID)
	if err != nil {
		t.Fatal(err)
	}
	levels, _ := requireWrightstoneTraitLevels(first)
	item.FirstLevel = effectCurveMax(levels, 20) + 1
	report, err := (&WrightstoneGen{catalog: catalog}).CheckLegality(item)
	if err != nil || !report.Writable || report.Status != LegalityForced {
		t.Fatalf("wrightstone level above the effect curve must warn but remain writable: report=%+v err=%v", report, err)
	}
}
