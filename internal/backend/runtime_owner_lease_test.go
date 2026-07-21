package backend

import (
	"bytes"
	"errors"
	"testing"
)

func TestRuntimeOwnerTokensRotateAndMatchExactly(t *testing.T) {
	app := &App{}
	first := app.nextRuntimeOwnerToken("sigil")
	second := app.nextRuntimeOwnerToken("sigil")
	if first == "" || second == "" || first == second {
		t.Fatalf("owner tokens did not rotate: first=%q second=%q", first, second)
	}
	if !runtimeOwnerTokenMatches(second, second) || runtimeOwnerTokenMatches(second, first) || runtimeOwnerTokenMatches(second, "") {
		t.Fatal("owner-token matching did not fail closed")
	}
}

func TestRuntimeAcquireRequestIDsAreStrictlyMonotonic(t *testing.T) {
	app := &App{}
	if err := app.acceptRuntimeAcquireRequestLocked(0); !errors.Is(err, errRuntimeAcquireRequestStale) {
		t.Fatalf("zero request ID error = %v, want stale", err)
	}
	if app.latestRuntimeAcquireRequestID != 0 {
		t.Fatalf("zero request changed latest ID to %d", app.latestRuntimeAcquireRequestID)
	}
	if err := app.acceptRuntimeAcquireRequestLocked(2); err != nil {
		t.Fatal(err)
	}
	for _, requestID := range []uint64{2, 1} {
		if err := app.acceptRuntimeAcquireRequestLocked(requestID); !errors.Is(err, errRuntimeAcquireRequestStale) {
			t.Fatalf("request ID %d error = %v, want stale", requestID, err)
		}
	}
	if app.latestRuntimeAcquireRequestID != 2 {
		t.Fatalf("stale request changed latest ID to %d", app.latestRuntimeAcquireRequestID)
	}
}

func TestSameFeatureReverseAcquireCannotReplaceNewerOwnerOrResource(t *testing.T) {
	app := &App{}
	if err := app.acceptRuntimeAcquireRequestLocked(2); err != nil {
		t.Fatal(err)
	}
	newer := app.grantSigilMemoryOwner(SigilMemoryStatus{Hooked: true, Address: 0x140345157})
	app.sigilMemoryHookAddr = uintptr(newer.Address)
	app.sigilMemoryCaveAddr = 0x140500000

	if err := app.acceptRuntimeAcquireRequestLocked(1); err == nil {
		app.grantSigilMemoryOwner(SigilMemoryStatus{Hooked: true, Address: 0xDEADBEEF})
	}
	if app.sigilMemoryOwnerToken != newer.OwnerToken || app.sigilMemoryHookAddr != uintptr(newer.Address) || app.sigilMemoryCaveAddr != 0x140500000 {
		t.Fatalf("older same-feature acquire replaced newer state: owner=%q hook=0x%X cave=0x%X", app.sigilMemoryOwnerToken, app.sigilMemoryHookAddr, app.sigilMemoryCaveAddr)
	}

	if err := app.acceptRuntimeAcquireRequestLocked(2); err == nil {
		app.grantSigilMemoryOwner(SigilMemoryStatus{Hooked: true, Address: 0xBADF00D})
	}
	if app.sigilMemoryOwnerToken != newer.OwnerToken || app.sigilMemoryHookAddr != uintptr(newer.Address) {
		t.Fatal("duplicate request ID rotated the current owner or resource")
	}
}

func TestCrossFeatureReverseAcquireCannotReplaceNewerOwnerOrResource(t *testing.T) {
	app := &App{}
	if err := app.acceptRuntimeAcquireRequestLocked(20); err != nil {
		t.Fatal(err)
	}
	newer := app.grantWrightstoneMemoryOwner(WrightstoneMemoryStatus{Hooked: true, Address: 0x14034B4EC})
	app.wrightstoneMemoryHookAddr = uintptr(newer.Address)
	app.wrightstoneMemoryCaveAddr = 0x140510000

	if err := app.acceptRuntimeAcquireRequestLocked(19); err == nil {
		app.grantOverLimitOwner(OverLimitStatus{Hooked: true, Address: 0x14037EAA0})
	}
	if app.wrightstoneMemoryOwnerToken != newer.OwnerToken || app.wrightstoneMemoryHookAddr != uintptr(newer.Address) || app.wrightstoneMemoryCaveAddr != 0x140510000 {
		t.Fatalf("older cross-feature acquire disturbed newer state: owner=%q hook=0x%X cave=0x%X", app.wrightstoneMemoryOwnerToken, app.wrightstoneMemoryHookAddr, app.wrightstoneMemoryCaveAddr)
	}
	if app.overLimitOwnerToken != "" || app.overLimitHookAddr != 0 {
		t.Fatalf("stale cross-feature request created an owner/resource: owner=%q hook=0x%X", app.overLimitOwnerToken, app.overLimitHookAddr)
	}
}

func TestCharaReleaseIgnoresStaleOwnerAndCurrentOwnerCanDetach(t *testing.T) {
	app := &App{
		moduleBase:      0x140000000,
		charaPID:        42,
		charaCreated:    100,
		charaOwnerToken: "chara-current",
	}
	if err := app.CharaRelease("chara-stale"); err != nil {
		t.Fatal(err)
	}
	if app.moduleBase == 0 || app.charaOwnerToken != "chara-current" {
		t.Fatalf("stale release tore down current owner: module=0x%X owner=%q", app.moduleBase, app.charaOwnerToken)
	}
	if err := app.CharaRelease("chara-current"); err != nil {
		t.Fatal(err)
	}
	if app.moduleBase != 0 || app.charaPID != 0 || app.charaOwnerToken != "" {
		t.Fatalf("current owner did not detach cleanly: module=0x%X pid=%d owner=%q", app.moduleBase, app.charaPID, app.charaOwnerToken)
	}
}

func TestSigilMemoryReleaseIgnoresStaleOwnerLease(t *testing.T) {
	original := append([]byte(nil), sigilMemoryOriginalBytes...)
	app := &App{
		sigilMemoryHookAddr:   0x140345157,
		sigilMemoryCaveAddr:   0x140500000,
		sigilMemoryOriginal:   append([]byte(nil), original...),
		sigilMemoryOwnerToken: "sigil-current",
	}
	if _, err := app.SigilMemoryRelease("sigil-stale"); err != nil {
		t.Fatal(err)
	}
	if app.sigilMemoryHookAddr == 0 || app.sigilMemoryCaveAddr == 0 || !bytes.Equal(app.sigilMemoryOriginal, original) || app.sigilMemoryOwnerToken != "sigil-current" {
		t.Fatalf("stale release discarded the current hook lease: hook=0x%X cave=0x%X owner=%q", app.sigilMemoryHookAddr, app.sigilMemoryCaveAddr, app.sigilMemoryOwnerToken)
	}
}

func TestOwnedRuntimeAcquireHelpersReturnRotatedLease(t *testing.T) {
	app := &App{}
	sigilFirst := app.grantSigilMemoryOwner(SigilMemoryStatus{Hooked: true})
	sigilSecond := app.grantSigilMemoryOwner(SigilMemoryStatus{Hooked: true})
	if sigilFirst.OwnerToken == "" || sigilSecond.OwnerToken == "" || sigilFirst.OwnerToken == sigilSecond.OwnerToken || app.sigilMemoryOwnerToken != sigilSecond.OwnerToken {
		t.Fatalf("sigil lease did not rotate: first=%q second=%q current=%q", sigilFirst.OwnerToken, sigilSecond.OwnerToken, app.sigilMemoryOwnerToken)
	}
	chara := app.grantCharaOwner(CharaProcessInfo{PID: 42, Connected: true})
	if chara.OwnerToken == "" || app.charaOwnerToken != chara.OwnerToken {
		t.Fatalf("chara lease was not returned: info=%+v current=%q", chara, app.charaOwnerToken)
	}
	wrightstone := app.grantWrightstoneMemoryOwner(WrightstoneMemoryStatus{Hooked: true})
	if wrightstone.OwnerToken == "" || app.wrightstoneMemoryOwnerToken != wrightstone.OwnerToken {
		t.Fatalf("wrightstone lease was not returned: status=%+v current=%q", wrightstone, app.wrightstoneMemoryOwnerToken)
	}
	overLimit := app.grantOverLimitOwner(OverLimitStatus{Hooked: true})
	if overLimit.OwnerToken == "" || app.overLimitOwnerToken != overLimit.OwnerToken {
		t.Fatalf("overlimit lease was not returned: status=%+v current=%q", overLimit, app.overLimitOwnerToken)
	}
}

func TestOwnedAcquireMethodSignaturesCarryGlobalRequestID(t *testing.T) {
	var chara func(*App, uint64) (CharaProcessInfo, error) = (*App).CharaAcquire
	var sigil func(*App, uint64) (SigilMemoryStatus, error) = (*App).SigilMemoryAcquire
	var wrightstone func(*App, uint64) (WrightstoneMemoryStatus, error) = (*App).WrightstoneMemoryAcquire
	var overLimit func(*App, uint64) (OverLimitStatus, error) = (*App).OverLimitAcquire
	if chara == nil || sigil == nil || wrightstone == nil || overLimit == nil {
		t.Fatal("owned acquire method is missing")
	}
}

func TestWrightstoneAndOverLimitReleaseIgnoreStaleOwners(t *testing.T) {
	wrightstoneOriginal := []byte{1, 2, 3}
	app := &App{
		wrightstoneMemoryHookAddr:   0x14034B4EC,
		wrightstoneMemoryCaveAddr:   0x140510000,
		wrightstoneMemoryOriginal:   append([]byte(nil), wrightstoneOriginal...),
		wrightstoneMemoryOwnerToken: "wrightstone-current",
		overLimitHookAddr:           0x14037EAA0,
		overLimitCaveAddr:           0x140520000,
		overLimitOwnerToken:         "overlimit-current",
	}
	if _, err := app.WrightstoneMemoryRelease("wrightstone-stale"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.OverLimitRelease("overlimit-stale"); err != nil {
		t.Fatal(err)
	}
	if app.wrightstoneMemoryHookAddr == 0 || app.wrightstoneMemoryCaveAddr == 0 || !bytes.Equal(app.wrightstoneMemoryOriginal, wrightstoneOriginal) || app.wrightstoneMemoryOwnerToken != "wrightstone-current" {
		t.Fatal("stale wrightstone release discarded the current hook lease")
	}
	if app.overLimitHookAddr == 0 || app.overLimitCaveAddr == 0 || app.overLimitOwnerToken != "overlimit-current" {
		t.Fatal("stale overlimit release discarded the current hook lease")
	}
}

func TestOwnedIdleHookReleaseConsumesOnlyMatchingToken(t *testing.T) {
	app := &App{
		wrightstoneMemoryOwnerToken: "wrightstone-current",
		overLimitOwnerToken:         "overlimit-current",
	}
	if _, err := app.WrightstoneMemoryRelease("wrightstone-current"); err != nil {
		t.Fatal(err)
	}
	if app.wrightstoneMemoryOwnerToken != "" || app.overLimitOwnerToken != "overlimit-current" {
		t.Fatal("wrightstone idle release consumed the wrong owner")
	}
	if _, err := app.OverLimitRelease("overlimit-current"); err != nil {
		t.Fatal(err)
	}
	if app.overLimitOwnerToken != "" {
		t.Fatal("overlimit idle release did not consume its owner")
	}
}

func TestLegacyHookMutationsRejectActiveOwnedLeasesBeforeProcessIO(t *testing.T) {
	const selected = uint64(0x140700000)
	overLimitUpdates := []OverLimitUpdate{
		{Index: 0, ExpectedSelectedAddr: selected, Attribute: 0xC4925BD7, Level: 1, Value: 100},
		{Index: 1, ExpectedSelectedAddr: selected, Attribute: 0x52A207B5, Level: 2, Value: 200},
		{Index: 2, ExpectedSelectedAddr: selected, Attribute: 0x45C65767, Level: 4, Value: 2},
		{Index: 3, ExpectedSelectedAddr: selected, Attribute: 0x6CB38EF3, Level: 8, Value: 0.4},
	}
	tests := []struct {
		name       string
		owner      func(*App) string
		current    func(*App) string
		operations []func(*App) error
		release    func(*App, string) error
	}{
		{
			name: "sigil",
			owner: func(app *App) string {
				return app.grantSigilMemoryOwner(SigilMemoryStatus{Hooked: true}).OwnerToken
			},
			current: func(app *App) string { return app.sigilMemoryOwnerToken },
			operations: []func(*App) error{
				func(app *App) error { _, err := app.SigilMemoryEnable(); return err },
				func(app *App) error { _, err := app.SigilMemoryUpdate(SigilMemoryUpdate{}); return err },
				func(app *App) error { _, err := app.SigilMemoryDisable(); return err },
			},
			release: func(app *App, token string) error { _, err := app.SigilMemoryRelease(token); return err },
		},
		{
			name: "wrightstone",
			owner: func(app *App) string {
				return app.grantWrightstoneMemoryOwner(WrightstoneMemoryStatus{Hooked: true}).OwnerToken
			},
			current: func(app *App) string { return app.wrightstoneMemoryOwnerToken },
			operations: []func(*App) error{
				func(app *App) error { _, err := app.WrightstoneMemoryEnable(); return err },
				func(app *App) error { _, err := app.WrightstoneMemoryUpdate(WrightstoneMemoryUpdate{}); return err },
				func(app *App) error { _, err := app.WrightstoneMemoryDisable(); return err },
			},
			release: func(app *App, token string) error { _, err := app.WrightstoneMemoryRelease(token); return err },
		},
		{
			name: "overlimit",
			owner: func(app *App) string {
				return app.grantOverLimitOwner(OverLimitStatus{Hooked: true}).OwnerToken
			},
			current: func(app *App) string { return app.overLimitOwnerToken },
			operations: []func(*App) error{
				func(app *App) error { _, err := app.OverLimitEnable(); return err },
				func(app *App) error { _, err := app.OverLimitSetSlot(overLimitUpdates[0]); return err },
				func(app *App) error { _, err := app.OverLimitSetAll(overLimitUpdates); return err },
				func(app *App) error { _, err := app.OverLimitDisable(); return err },
			},
			release: func(app *App, token string) error { _, err := app.OverLimitRelease(token); return err },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{}
			token := tt.owner(app)
			for index, operation := range tt.operations {
				if err := operation(app); !errors.Is(err, errRuntimeOwnerLeaseStale) {
					t.Fatalf("legacy operation %d error = %v, want stale owner", index, err)
				}
				if tt.current(app) != token || app.hProcess != 0 || app.moduleBase != 0 {
					t.Fatalf("legacy operation %d changed owner or process state: owner=%q process=%v module=0x%X", index, tt.current(app), app.hProcess, app.moduleBase)
				}
			}
			if err := tt.release(app, token); err != nil {
				t.Fatalf("owned release after rejected legacy calls: %v", err)
			}
			if tt.current(app) != "" {
				t.Fatalf("owned release retained token %q", tt.current(app))
			}
		})
	}
}

func TestOwnedWritesRejectEmptyAndStaleTokensBeforeProcessIO(t *testing.T) {
	const selected = uint64(0x140700000)
	overLimitUpdates := []OverLimitUpdate{
		{Index: 0, ExpectedSelectedAddr: selected, Attribute: 0xC4925BD7, Level: 1, Value: 100},
		{Index: 1, ExpectedSelectedAddr: selected, Attribute: 0x52A207B5, Level: 2, Value: 200},
		{Index: 2, ExpectedSelectedAddr: selected, Attribute: 0x45C65767, Level: 4, Value: 2},
		{Index: 3, ExpectedSelectedAddr: selected, Attribute: 0x6CB38EF3, Level: 8, Value: 0.4},
	}
	_, summonUpdate := validSummonMemoryUpdate(t)

	for _, presented := range []string{"", "stale-owner"} {
		t.Run(map[bool]string{true: "empty", false: "stale"}[presented == ""], func(t *testing.T) {
			app := &App{
				charaOwnerToken:               "chara-current",
				sigilMemoryOwnerToken:         "sigil-current",
				wrightstoneMemoryOwnerToken:   "wrightstone-current",
				overLimitOwnerToken:           "overlimit-current",
				sigilMemoryHookAddr:           0x140345157,
				wrightstoneMemoryHookAddr:     0x14034B4EC,
				overLimitHookAddr:             0x14037EAA0,
				latestRuntimeAcquireRequestID: 9,
			}
			before := struct {
				hProcess                                  uintptr
				moduleBase                                uintptr
				chara, sigil, wrightstone, overLimit      string
				sigilHook, wrightstoneHook, overLimitHook uintptr
				latest                                    uint64
			}{
				hProcess: uintptr(app.hProcess), moduleBase: app.moduleBase,
				chara: app.charaOwnerToken, sigil: app.sigilMemoryOwnerToken,
				wrightstone: app.wrightstoneMemoryOwnerToken, overLimit: app.overLimitOwnerToken,
				sigilHook: app.sigilMemoryHookAddr, wrightstoneHook: app.wrightstoneMemoryHookAddr,
				overLimitHook: app.overLimitHookAddr, latest: app.latestRuntimeAcquireRequestID,
			}

			_, sigilErr := app.SigilMemoryUpdateOwned(presented, SigilMemoryUpdate{})
			_, wrightstoneErr := app.WrightstoneMemoryUpdateOwned(presented, WrightstoneMemoryUpdate{})
			_, overLimitErr := app.OverLimitSetAllOwned(presented, overLimitUpdates)
			_, summonErr := app.SummonUpdateOwned(presented, summonUpdate)
			_, currencyErr := app.CurrencySetOneOwned(presented, "rupies", 1)
			_, potionErr := app.PotionSetOneOwned(presented, "revive", 1)
			_, materialErr := app.MaterialConsumeSetEnabledOwned(presented, true)
			_, collectibleErr := app.CollectibleTaskCompleteOwned(presented)
			_, terminusErr := app.TerminusDropSetEnabledOwned(presented, true)
			_, monsterErr := app.MonsterEnhanceSetPatchValueEnabledOwned(presented, "inventory_set_45", true, 45)
			for name, err := range map[string]error{
				"sigil": sigilErr, "wrightstone": wrightstoneErr, "overlimit": overLimitErr, "summon": summonErr,
				"currency": currencyErr, "potion": potionErr, "material": materialErr,
				"collectible": collectibleErr, "terminus": terminusErr, "monster": monsterErr,
			} {
				if !errors.Is(err, errRuntimeOwnerLeaseStale) {
					t.Fatalf("%s owned write error = %v, want stale owner before process IO", name, err)
				}
			}
			if uintptr(app.hProcess) != before.hProcess || app.moduleBase != before.moduleBase ||
				app.charaOwnerToken != before.chara || app.sigilMemoryOwnerToken != before.sigil ||
				app.wrightstoneMemoryOwnerToken != before.wrightstone || app.overLimitOwnerToken != before.overLimit ||
				app.sigilMemoryHookAddr != before.sigilHook || app.wrightstoneMemoryHookAddr != before.wrightstoneHook ||
				app.overLimitHookAddr != before.overLimitHook || app.latestRuntimeAcquireRequestID != before.latest {
				t.Fatal("stale owned write changed owner, process, generation, or runtime resource state")
			}
		})
	}
}

func TestOwnedWriteMethodSignaturesRequireOwnerToken(t *testing.T) {
	var sigil func(*App, string, SigilMemoryUpdate) (SigilMemoryStatus, error) = (*App).SigilMemoryUpdateOwned
	var wrightstone func(*App, string, WrightstoneMemoryUpdate) (WrightstoneMemoryStatus, error) = (*App).WrightstoneMemoryUpdateOwned
	var overLimit func(*App, string, []OverLimitUpdate) (OverLimitStatus, error) = (*App).OverLimitSetAllOwned
	var summon func(*App, string, SummonUpdate) (SummonInfo, error) = (*App).SummonUpdateOwned
	var currency func(*App, string, string, int) (CurrencyInfo, error) = (*App).CurrencySetOneOwned
	var potion func(*App, string, string, int) (PotionInfo, error) = (*App).PotionSetOneOwned
	var material func(*App, string, bool) (MaterialConsumeStatus, error) = (*App).MaterialConsumeSetEnabledOwned
	var collectible func(*App, string) (CollectibleTaskStatus, error) = (*App).CollectibleTaskCompleteOwned
	var terminus func(*App, string, bool) (TerminusDropStatus, error) = (*App).TerminusDropSetEnabledOwned
	var monster func(*App, string, string, bool, float64) (MonsterEnhanceResult, error) = (*App).MonsterEnhanceSetPatchValueEnabledOwned
	if sigil == nil || wrightstone == nil || overLimit == nil || summon == nil || currency == nil || potion == nil || material == nil || collectible == nil || terminus == nil || monster == nil {
		t.Fatal("owned write method is missing")
	}
}

func TestNewSigilOwnerInvalidatesOlderCharaRelease(t *testing.T) {
	original := append([]byte(nil), sigilMemoryOriginalBytes...)
	app := &App{
		moduleBase:          0x140000000,
		charaPID:            42,
		charaCreated:        100,
		charaOwnerToken:     "summon-old",
		sigilMemoryHookAddr: 0x140345157,
		sigilMemoryCaveAddr: 0x140500000,
		sigilMemoryOriginal: original,
	}
	sigil := app.grantSigilMemoryOwner(SigilMemoryStatus{Hooked: true})
	if sigil.OwnerToken == "" || app.charaOwnerToken != "" {
		t.Fatalf("new sigil owner did not invalidate the older chara lease: sigil=%q chara=%q", sigil.OwnerToken, app.charaOwnerToken)
	}
	if err := app.CharaRelease("summon-old"); err != nil {
		t.Fatal(err)
	}
	if app.moduleBase == 0 || app.sigilMemoryHookAddr == 0 || app.sigilMemoryCaveAddr == 0 || !bytes.Equal(app.sigilMemoryOriginal, original) || app.sigilMemoryOwnerToken != sigil.OwnerToken {
		t.Fatalf("late summon cleanup tore down the newer sigil lease: module=0x%X hook=0x%X cave=0x%X sigil=%q", app.moduleBase, app.sigilMemoryHookAddr, app.sigilMemoryCaveAddr, app.sigilMemoryOwnerToken)
	}
}

func TestCharaReleaseConsumesCurrentOwnerWithoutDetachingActiveRuntimeHook(t *testing.T) {
	tests := []struct {
		name string
		mark func(*App)
	}{
		{name: "sigil hook", mark: func(app *App) { app.sigilMemoryHookAddr = 0x140345157 }},
		{name: "sigil cave recovery", mark: func(app *App) { app.sigilMemoryCaveAddr = 0x140500000 }},
		{name: "sigil original recovery", mark: func(app *App) { app.sigilMemoryOriginal = []byte{1} }},
		{name: "wrightstone hook", mark: func(app *App) { app.wrightstoneMemoryHookAddr = 0x14034B4EC }},
		{name: "wrightstone cave recovery", mark: func(app *App) { app.wrightstoneMemoryCaveAddr = 0x140510000 }},
		{name: "wrightstone original recovery", mark: func(app *App) { app.wrightstoneMemoryOriginal = []byte{1} }},
		{name: "overlimit hook", mark: func(app *App) { app.overLimitHookAddr = 0x14037EAA0 }},
		{name: "overlimit cave recovery", mark: func(app *App) { app.overLimitCaveAddr = 0x140520000 }},
		{name: "currency hook", mark: func(app *App) { app.currencyHookAddr = 0x140356621 }},
		{name: "currency cave recovery", mark: func(app *App) { app.currencyCaveAddr = 0x140530000 }},
		{name: "currency original recovery", mark: func(app *App) { app.currencyOriginal = []byte{1} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := &App{
				moduleBase:      0x140000000,
				charaPID:        42,
				charaCreated:    100,
				charaOwnerToken: "summon-current",
			}
			test.mark(app)
			if err := app.CharaRelease("summon-current"); err != nil {
				t.Fatal(err)
			}
			if app.moduleBase == 0 || app.charaPID == 0 {
				t.Fatal("owned chara release detached a process with an active runtime hook/recovery lease")
			}
			if app.charaOwnerToken != "" {
				t.Fatalf("guarded chara owner was not consumed: %q", app.charaOwnerToken)
			}
			if !app.hasActiveRuntimeHookLeaseLocked() {
				t.Fatal("guarded release discarded the active runtime hook/recovery lease")
			}
		})
	}
}

func TestNewCharaOwnerCannotDetachOlderActiveSigilOwner(t *testing.T) {
	app := &App{
		moduleBase:            0x140000000,
		charaPID:              42,
		charaCreated:          100,
		sigilMemoryHookAddr:   0x140345157,
		sigilMemoryCaveAddr:   0x140500000,
		sigilMemoryOwnerToken: "sigil-current",
	}
	chara := app.grantCharaOwner(CharaProcessInfo{PID: 42, Connected: true})
	if err := app.CharaRelease(chara.OwnerToken); err != nil {
		t.Fatal(err)
	}
	if app.moduleBase == 0 || app.sigilMemoryHookAddr == 0 || app.sigilMemoryOwnerToken != "sigil-current" || app.charaOwnerToken != "" {
		t.Fatalf("newer chara release disturbed the active sigil owner: module=0x%X hook=0x%X sigil=%q chara=%q", app.moduleBase, app.sigilMemoryHookAddr, app.sigilMemoryOwnerToken, app.charaOwnerToken)
	}
}

func TestForceCleanupStillIgnoresOwnedTokens(t *testing.T) {
	app := &App{
		moduleBase:                    0x140000000,
		charaPID:                      42,
		charaCreated:                  100,
		charaOwnerToken:               "chara-current",
		sigilMemoryOwnerToken:         "sigil-current",
		wrightstoneMemoryOwnerToken:   "wrightstone-current",
		overLimitOwnerToken:           "overlimit-current",
		latestRuntimeAcquireRequestID: 42,
	}
	if err := app.CharaDetach(); err != nil {
		t.Fatal(err)
	}
	if app.moduleBase != 0 || app.charaOwnerToken != "" || app.sigilMemoryOwnerToken != "" || app.wrightstoneMemoryOwnerToken != "" || app.overLimitOwnerToken != "" {
		t.Fatalf("force cleanup retained owned lifecycle state: module=0x%X chara=%q sigil=%q wrightstone=%q overlimit=%q", app.moduleBase, app.charaOwnerToken, app.sigilMemoryOwnerToken, app.wrightstoneMemoryOwnerToken, app.overLimitOwnerToken)
	}
	if app.latestRuntimeAcquireRequestID != 42 {
		t.Fatal("force cleanup reset the global monotonic request generation")
	}
}
