package backend

import "testing"

func TestRuntimeCombatReferenceSelectsExactCharacterCurves(t *testing.T) {
	reference, err := selectedRuntimeCombatReference("pl0400")
	if err != nil {
		t.Fatal(err)
	}
	if reference.DataVersion != "2.0.2" || reference.CharacterCode != "PL0400" {
		t.Fatalf("unexpected reference identity: %+v", reference)
	}
	if len(reference.NormalCurve) != 30 || len(reference.ArtsCurve) != 30 {
		t.Fatalf("Io curves are incomplete: normal=%d arts=%d", len(reference.NormalCurve), len(reference.ArtsCurve))
	}
	if reference.DamageCalculate.AttackTypeDamageLimitNormal != 9999 ||
		reference.DamageCalculate.AttackTypeDamageLimitAbility != 14999 ||
		reference.DamageCalculate.AttackTypeDamageLimitSpecialArts != 19999 {
		t.Fatalf("global damage baselines changed: %+v", reference.DamageCalculate)
	}
	if reference.Guard.JustGuardAcceptFrames != 5 || reference.Guard.GaugeMax != 40 {
		t.Fatalf("guard reference changed: %+v", reference.Guard)
	}
	if len(reference.ConditionalCurves["garrison"]) != 6 || len(reference.ConditionalCurves["enmity"]) != 8 {
		t.Fatalf("conditional curves are incomplete: %+v", reference.ConditionalCurves)
	}
}

func TestRuntimeCombatReferenceDoesNotInventUnknownCharacterCurves(t *testing.T) {
	reference, err := selectedRuntimeCombatReference("PL9999")
	if err != nil {
		t.Fatal(err)
	}
	if len(reference.NormalCurve) != 0 || len(reference.ArtsCurve) != 0 {
		t.Fatalf("unknown character unexpectedly received a curve: %+v", reference)
	}
}
