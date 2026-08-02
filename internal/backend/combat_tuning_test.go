package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestCombatTuningRequestBounds(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func() error
		ok   bool
	}{
		{name: "cooldown disabled ignores multiplier", run: func() error {
			return validateCombatCooldownRequest(CombatCooldownRequest{SpeedMultiplier: math.NaN()})
		}, ok: true},
		{name: "no cooldown ignores multiplier", run: func() error {
			return validateCombatCooldownRequest(CombatCooldownRequest{Enabled: true, NoCooldown: true, SpeedMultiplier: math.Inf(1)})
		}, ok: true},
		{name: "cooldown minimum", run: func() error {
			return validateCombatCooldownRequest(CombatCooldownRequest{Enabled: true, SpeedMultiplier: 0.1})
		}, ok: true},
		{name: "cooldown maximum", run: func() error {
			return validateCombatCooldownRequest(CombatCooldownRequest{Enabled: true, SpeedMultiplier: 100})
		}, ok: true},
		{name: "cooldown below minimum", run: func() error {
			return validateCombatCooldownRequest(CombatCooldownRequest{Enabled: true, SpeedMultiplier: math.Nextafter(0.1, 0)})
		}},
		{name: "cooldown above maximum", run: func() error {
			return validateCombatCooldownRequest(CombatCooldownRequest{Enabled: true, SpeedMultiplier: math.Nextafter(100, 101)})
		}},
		{name: "cooldown NaN", run: func() error {
			return validateCombatCooldownRequest(CombatCooldownRequest{Enabled: true, SpeedMultiplier: math.NaN()})
		}},
		{name: "cooldown infinity", run: func() error {
			return validateCombatCooldownRequest(CombatCooldownRequest{Enabled: true, SpeedMultiplier: math.Inf(1)})
		}},
		{name: "charge disabled ignores multiplier", run: func() error {
			return validateCombatChargeRequest(CombatChargeRequest{SpeedMultiplier: math.NaN()})
		}, ok: true},
		{name: "instant charge ignores multiplier", run: func() error {
			return validateCombatChargeRequest(CombatChargeRequest{Enabled: true, Instant: true, SpeedMultiplier: math.Inf(-1)})
		}, ok: true},
		{name: "charge minimum", run: func() error {
			return validateCombatChargeRequest(CombatChargeRequest{Enabled: true, SpeedMultiplier: 0.1})
		}, ok: true},
		{name: "charge maximum", run: func() error {
			return validateCombatChargeRequest(CombatChargeRequest{Enabled: true, SpeedMultiplier: 100})
		}, ok: true},
		{name: "charge below minimum", run: func() error {
			return validateCombatChargeRequest(CombatChargeRequest{Enabled: true, SpeedMultiplier: math.Nextafter(0.1, 0)})
		}},
		{name: "charge above maximum", run: func() error {
			return validateCombatChargeRequest(CombatChargeRequest{Enabled: true, SpeedMultiplier: math.Nextafter(100, 101)})
		}},
		{name: "charge NaN", run: func() error {
			return validateCombatChargeRequest(CombatChargeRequest{Enabled: true, SpeedMultiplier: math.NaN()})
		}},
		{name: "charge infinity", run: func() error {
			return validateCombatChargeRequest(CombatChargeRequest{Enabled: true, SpeedMultiplier: math.Inf(1)})
		}},
		{name: "action speed disabled ignores multiplier", run: func() error {
			return validateCombatActionSpeedRequest(CombatActionSpeedRequest{SpeedMultiplier: math.NaN()})
		}, ok: true},
		{name: "action speed minimum", run: func() error {
			return validateCombatActionSpeedRequest(CombatActionSpeedRequest{Enabled: true, SpeedMultiplier: 0.1})
		}, ok: true},
		{name: "action speed maximum", run: func() error {
			return validateCombatActionSpeedRequest(CombatActionSpeedRequest{Enabled: true, SpeedMultiplier: 5})
		}, ok: true},
		{name: "action speed zero", run: func() error {
			return validateCombatActionSpeedRequest(CombatActionSpeedRequest{Enabled: true, SpeedMultiplier: 0})
		}},
		{name: "action speed below minimum", run: func() error {
			return validateCombatActionSpeedRequest(CombatActionSpeedRequest{Enabled: true, SpeedMultiplier: math.Nextafter(0.1, 0)})
		}},
		{name: "action speed above maximum", run: func() error {
			return validateCombatActionSpeedRequest(CombatActionSpeedRequest{Enabled: true, SpeedMultiplier: math.Nextafter(5, 6)})
		}},
		{name: "action speed NaN", run: func() error {
			return validateCombatActionSpeedRequest(CombatActionSpeedRequest{Enabled: true, SpeedMultiplier: math.NaN()})
		}},
		{name: "action speed infinity", run: func() error {
			return validateCombatActionSpeedRequest(CombatActionSpeedRequest{Enabled: true, SpeedMultiplier: math.Inf(1)})
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.run()
			if test.ok && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
			if !test.ok && err == nil {
				t.Fatal("invalid request was accepted")
			}
		})
	}
}

func TestCombatTuningSiteSpecificationsAreComplete(t *testing.T) {
	specs := append(append(append([]combatTuningSiteSpec(nil), combatCooldownSpecs...), combatChargeSpec), combatActionSpeedSpec)
	for _, spec := range specs {
		t.Run(spec.Label, func(t *testing.T) {
			if len(spec.Original) != combatTuningHookSize {
				t.Fatalf("original length=%d, want hook size %d", len(spec.Original), combatTuningHookSize)
			}
			if len(spec.Pattern) < len(spec.Original) || len(spec.Mask) != len(spec.Pattern) {
				t.Fatalf("pattern/mask lengths original=%d pattern=%d mask=%d", len(spec.Original), len(spec.Pattern), len(spec.Mask))
			}
			if !bytes.Equal(spec.Pattern[:len(spec.Original)], spec.Original) {
				t.Fatalf("pattern does not begin with guarded original: pattern=% X original=% X", spec.Pattern, spec.Original)
			}
			for index := range spec.Original {
				if !spec.Mask[index] {
					t.Fatalf("original byte %d is wildcarded in the locator", index)
				}
			}
		})
	}
	if len(combatCooldownMarker) != 8 || len(combatChargeMarker) != 8 || len(combatActionSpeedMarker) != 8 ||
		bytes.Equal(combatCooldownMarker, combatChargeMarker) || bytes.Equal(combatCooldownMarker, combatActionSpeedMarker) ||
		bytes.Equal(combatChargeMarker, combatActionSpeedMarker) {
		t.Fatalf("ownership markers must be distinct fixed 8-byte values: cooldown=%q charge=%q action=%q", combatCooldownMarker, combatChargeMarker, combatActionSpeedMarker)
	}
}

func TestCombatCooldownCavesPreserveOriginalAndReturnToNextInstruction(t *testing.T) {
	const cave = uintptr(0x180000000)
	const returnAddr = uintptr(0x180001000)
	for index, spec := range combatCooldownSpecs {
		t.Run(spec.Label, func(t *testing.T) {
			code, err := buildCombatCooldownCave(index, cave, returnAddr, CombatCooldownRequest{
				Enabled: true, SpeedMultiplier: 2,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := bytes.Count(code, spec.Original); got != 1 {
				t.Fatalf("original instruction count=%d, want 1 in % X", got, code)
			}
			assertCombatCaveReturn(t, code, cave, returnAddr, combatCooldownMarker)
			assertCombatFactor(t, code, cave, combatCooldownMarker, 2)
		})
	}
}

func TestCombatCooldownNoCooldownCavesZeroTheCorrectRegisterWithoutFactor(t *testing.T) {
	const cave = uintptr(0x180000000)
	const returnAddr = uintptr(0x180001000)
	wantZero := [][]byte{
		{0x0F, 0x57, 0xF6}, // xorps xmm6,xmm6
		{0x0F, 0x57, 0xC0}, // xorps xmm0,xmm0
		{0x0F, 0x57, 0xC0}, // xorps xmm0,xmm0
	}
	for index, spec := range combatCooldownSpecs {
		t.Run(spec.Label, func(t *testing.T) {
			code, err := buildCombatCooldownCave(index, cave, returnAddr, CombatCooldownRequest{
				Enabled: true, NoCooldown: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(code, wantZero[index]) {
				t.Fatalf("no-cooldown cave does not zero the path register: % X", code)
			}
			assertCombatCaveReturn(t, code, cave, returnAddr, combatCooldownMarker)
			markerOffset := bytes.Index(code, combatCooldownMarker)
			if markerOffset+len(combatCooldownMarker) != len(code) {
				t.Fatalf("no-cooldown cave unexpectedly carries factor data: % X", code)
			}
		})
	}
}

func TestCombatCooldownSelfAndWholePartyBranches(t *testing.T) {
	const cave = uintptr(0x180000000)
	const returnAddr = uintptr(0x180001000)
	for index, spec := range combatCooldownSpecs {
		t.Run(spec.Label, func(t *testing.T) {
			self, err := buildCombatCooldownCave(index, cave, returnAddr, CombatCooldownRequest{
				Enabled: true, SpeedMultiplier: 2,
			})
			if err != nil {
				t.Fatal(err)
			}
			wholeParty, err := buildCombatCooldownCave(index, cave, returnAddr, CombatCooldownRequest{
				Enabled: true, SpeedMultiplier: 2, ApplyWholeParty: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(self, []byte{0x10, 0x02, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x75}) {
				t.Fatalf("self-only cave has no local-player discriminator: % X", self)
			}
			if index == 0 && !bytes.Contains(self, []byte{0x48, 0x85, 0xDB, 0x74}) {
				t.Fatalf("path A dereferences its owner pointer without a null guard: % X", self)
			}
			jumpOpcode := bytes.Index(self, []byte{0x75})
			if jumpOpcode < 0 || jumpOpcode+1 >= len(self) {
				t.Fatalf("self-only conditional jump is missing: % X", self)
			}
			jumpTarget := jumpOpcode + 2 + int(int8(self[jumpOpcode+1]))
			originalOffset := bytes.Index(self, spec.Original)
			if index == 0 {
				if jumpTarget >= len(self) || self[jumpTarget] != 0x5B || jumpTarget+1 != originalOffset {
					t.Fatalf("path A non-local jump target=%d, want pop+original at %d", jumpTarget, originalOffset)
				}
			} else if jumpTarget != originalOffset {
				t.Fatalf("non-local jump target=%d, want original at %d", jumpTarget, originalOffset)
			}
			hasWholePartyDiscriminator := bytes.Contains(wholeParty, []byte{0x10, 0x02, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x75})
			if index == 0 && !hasWholePartyDiscriminator {
				t.Fatalf("path A must retain its verified local-player discriminator in party mode: % X", wholeParty)
			}
			if index > 0 && hasWholePartyDiscriminator {
				t.Fatalf("party array path retained the self-only discriminator: % X", wholeParty)
			}
		})
	}
}

func TestCombatChargeCavesScaleFrameIncrementAndReturn(t *testing.T) {
	const cave = uintptr(0x180000000)
	const returnAddr = uintptr(0x180001000)

	scaled, err := buildCombatChargeCave(cave, returnAddr, CombatChargeRequest{
		Enabled: true, SpeedMultiplier: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(scaled, combatChargeSpec.Original) {
		t.Fatalf("charge cave does not begin with displaced original: % X", scaled)
	}
	// The displaced vmovss loads accumulated charge into xmm1. The game later
	// adds the scaled current-frame increment from xmm0, so speed-up must
	// multiply xmm0 and leave xmm1 intact.
	if !bytes.Contains(scaled, []byte{0xF3, 0x0F, 0x59, 0x05}) {
		t.Fatalf("charge multiplier does not scale the xmm0 frame increment: % X", scaled)
	}
	assertCombatCaveReturn(t, scaled, cave, returnAddr, combatChargeMarker)
	assertCombatFactor(t, scaled, cave, combatChargeMarker, 4)

	instant, err := buildCombatChargeCave(cave, returnAddr, CombatChargeRequest{
		Enabled: true, Instant: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(instant, combatChargeSpec.Original) ||
		!bytes.Contains(instant, []byte{0x0F, 0x57, 0xC9}) {
		t.Fatalf("instant cave does not zero the loaded xmm1 duration: % X", instant)
	}
	assertCombatCaveReturn(t, instant, cave, returnAddr, combatChargeMarker)
	if got := len(instant) - (bytes.Index(instant, combatChargeMarker) + len(combatChargeMarker)); got != 0 {
		t.Fatalf("instant cave unexpectedly carries factor data: % X", instant)
	}
}

func TestCombatActionSpeedCaveScopesActorAndReturns(t *testing.T) {
	const cave = uintptr(0x180000000)
	const returnAddr = uintptr(0x180001000)
	self, err := buildCombatActionSpeedCave(cave, returnAddr, CombatActionSpeedRequest{
		Enabled: true, SpeedMultiplier: 1.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	party, err := buildCombatActionSpeedCave(cave, returnAddr, CombatActionSpeedRequest{
		Enabled: true, SpeedMultiplier: 2, ApplyWholeParty: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	contextGuard := []byte{0x80, 0xB9, 0xFE, 0x01, 0, 0, 1}
	localGuard := []byte{0x83, 0xB9, 0x10, 0x02, 0, 0, 1}
	writeSpeed := []byte{0xF3, 0x0F, 0x11, 0x40, 0x18}
	if !bytes.Contains(self, contextGuard) || !bytes.Contains(self, localGuard) || !bytes.Contains(self, writeSpeed) {
		t.Fatalf("self-only action-speed cave is missing scope/write guards: % X", self)
	}
	if !bytes.Contains(party, contextGuard) || bytes.Contains(party, localGuard) || !bytes.Contains(party, writeSpeed) {
		t.Fatalf("party action-speed cave has the wrong scope guards: % X", party)
	}
	for name, code := range map[string][]byte{"self": self, "party": party} {
		t.Run(name, func(t *testing.T) {
			if got := bytes.Count(code, combatActionSpeedSpec.Original); got != 1 {
				t.Fatalf("displaced original count=%d, want 1: % X", got, code)
			}
			originalOffset := bytes.Index(code, combatActionSpeedSpec.Original)
			if originalOffset < 1 || code[originalOffset-1] != 0x59 {
				t.Fatalf("original is not preceded by pop rcx: % X", code)
			}
			assertCombatCaveReturn(t, code, cave, returnAddr, combatActionSpeedMarker)
			assertCombatFactor(t, code, cave, combatActionSpeedMarker, map[string]float32{"self": 1.5, "party": 2}[name])
		})
	}
}

func assertCombatCaveReturn(t *testing.T, code []byte, cave, returnAddr uintptr, marker []byte) {
	t.Helper()
	markerOffset := bytes.Index(code, marker)
	if markerOffset < 5 {
		t.Fatalf("marker %q missing or has no return jump: % X", marker, code)
	}
	jumpOffset := markerOffset - 5
	if code[jumpOffset] != 0xE9 {
		t.Fatalf("return opcode at %d=%02X, want E9", jumpOffset, code[jumpOffset])
	}
	if got := relJumpTarget(cave+uintptr(jumpOffset), code[jumpOffset:markerOffset]); got != returnAddr {
		t.Fatalf("return target=0x%X, want 0x%X", got, returnAddr)
	}
}

func assertCombatFactor(t *testing.T, code []byte, cave uintptr, marker []byte, want float32) {
	t.Helper()
	markerOffset := bytes.Index(code, marker)
	factorOffset := markerOffset + len(marker)
	if factorOffset+4 != len(code) {
		t.Fatalf("factor layout is not marker+float32: % X", code)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(code[factorOffset:])); got != want {
		t.Fatalf("factor=%v, want %v", got, want)
	}

	dispOffset := -1
	for index := 0; index+8 <= markerOffset; index++ {
		if code[index] == 0xF3 && code[index+1] == 0x0F &&
			(code[index+2] == 0x10 || code[index+2] == 0x5E || code[index+2] == 0x59) &&
			(code[index+3] == 0x05 || code[index+3] == 0x0D || code[index+3] == 0x35) {
			dispOffset = index + 4
		}
	}
	if dispOffset < 0 {
		t.Fatalf("factor instruction missing: % X", code)
	}
	delta := int64(int32(binary.LittleEndian.Uint32(code[dispOffset : dispOffset+4])))
	target := uintptr(int64(cave+uintptr(dispOffset+4)) + delta)
	if wantTarget := cave + uintptr(factorOffset); target != wantTarget {
		t.Fatalf("factor target=0x%X, want 0x%X", target, wantTarget)
	}
}

type combatTuningProcessFixture struct {
	app      *App
	page     uintptr
	original map[uintptr][]byte
}

func newCombatTuningProcessFixture(t *testing.T) *combatTuningProcessFixture {
	t.Helper()
	handle := windows.CurrentProcess()
	page, err := virtualAllocRemote(handle, 0x2000, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		t.Fatal(err)
	}
	created, err := processCreationTime(handle)
	if err != nil {
		_ = virtualFreeRemote(handle, page)
		t.Fatal(err)
	}
	fixture := &combatTuningProcessFixture{
		page:     page,
		original: make(map[uintptr][]byte),
		app: &App{
			hProcess:                    handle,
			moduleBase:                  page,
			charaPID:                    uint32(os.Getpid()),
			charaCreated:                created,
			charaOwnerToken:             "combat-owner",
			combatTuningCooldownAddrs:   []uintptr{page + 0x100, page + 0x200, page + 0x300},
			combatTuningChargeAddr:      page + 0x400,
			combatTuningActionSpeedAddr: page + 0x500,
		},
	}
	for index, spec := range combatCooldownSpecs {
		fixture.writeOriginal(t, fixture.app.combatTuningCooldownAddrs[index], spec.Original)
	}
	fixture.writeOriginal(t, fixture.app.combatTuningChargeAddr, combatChargeSpec.Original)
	fixture.writeOriginal(t, fixture.app.combatTuningActionSpeedAddr, combatActionSpeedSpec.Original)
	t.Cleanup(func() {
		caves := make(map[uintptr]struct{})
		for _, lease := range []*combatTuningLease{
			fixture.app.combatTuningCooldownLease,
			fixture.app.combatTuningChargeLease,
			fixture.app.combatTuningActionSpeedLease,
		} {
			if lease == nil {
				continue
			}
			for _, site := range lease.Sites {
				caves[site.CaveAddr] = struct{}{}
			}
		}
		for _, retired := range fixture.app.retiredRuntimeCaves {
			caves[retired.Address] = struct{}{}
		}
		for addr, original := range fixture.original {
			_ = writeCodeMemory(handle, addr, original)
		}
		for cave := range caves {
			if cave != 0 {
				_ = virtualFreeRemote(handle, cave)
			}
		}
		if err := virtualFreeRemote(handle, page); err != nil {
			t.Errorf("free combat tuning test page: %v", err)
		}
	})
	return fixture
}

func (fixture *combatTuningProcessFixture) writeOriginal(t *testing.T, addr uintptr, original []byte) {
	t.Helper()
	fixture.original[addr] = append([]byte(nil), original...)
	if err := writeCodeMemory(fixture.app.hProcess, addr, original); err != nil {
		t.Fatal(err)
	}
}

func (fixture *combatTuningProcessFixture) read(t *testing.T, addr uintptr, size int) []byte {
	t.Helper()
	data := make([]byte, size)
	handle := fixture.app.hProcess
	if handle == 0 {
		handle = windows.CurrentProcess()
	}
	if err := readProcessMemory(handle, addr, unsafe.Pointer(&data[0]), uintptr(size)); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCombatTuningCooldownInstallReadbackAndRestore(t *testing.T) {
	fixture := newCombatTuningProcessFixture(t)
	request := CombatCooldownRequest{Enabled: true, SpeedMultiplier: 3, ApplyWholeParty: true}
	lease, err := fixture.app.installCombatCooldownLocked("combat-owner", request)
	if err != nil {
		t.Fatal(err)
	}
	if lease != fixture.app.combatTuningCooldownLease || len(lease.Sites) != len(combatCooldownSpecs) {
		t.Fatalf("cooldown lease was not retained: %+v", lease)
	}
	for _, site := range lease.Sites {
		entry := fixture.read(t, site.EntryAddr, len(site.Installed))
		if !bytes.Equal(entry, site.Installed) || relJumpTarget(site.EntryAddr, entry) != site.CaveAddr {
			t.Fatalf("%s entry is not the owned cave jump: % X", site.Label, entry)
		}
		if cave := fixture.read(t, site.CaveAddr, len(site.CaveCode)); !bytes.Equal(cave, site.CaveCode) {
			t.Fatalf("%s cave readback differs", site.Label)
		}
	}
	status, err := fixture.app.readCombatTuningFeatureLocked("combat-owner", combatTuningKindCooldown)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || !status.Available || !status.ApplyWholeParty || status.SpeedMultiplier != 3 ||
		len(status.RVAs) != len(combatCooldownSpecs) {
		t.Fatalf("unexpected installed cooldown status: %+v", status)
	}
	if err := fixture.app.restoreCombatTuningOwnedLocked("combat-owner", false); err != nil {
		t.Fatal(err)
	}
	if fixture.app.combatTuningCooldownLease != nil {
		t.Fatal("successful cooldown restore retained its active lease")
	}
	for index, addr := range fixture.app.combatTuningCooldownAddrs {
		if got := fixture.read(t, addr, len(combatCooldownSpecs[index].Original)); !bytes.Equal(got, combatCooldownSpecs[index].Original) {
			t.Fatalf("%s entry after restore=% X", combatCooldownSpecs[index].Label, got)
		}
	}
	if len(fixture.app.retiredRuntimeCaves) != len(combatCooldownSpecs) {
		t.Fatalf("restored caves=%d, want %d retired until detach", len(fixture.app.retiredRuntimeCaves), len(combatCooldownSpecs))
	}
}

func TestCombatTuningChargeInstallReadbackAndRestore(t *testing.T) {
	fixture := newCombatTuningProcessFixture(t)
	request := CombatChargeRequest{Enabled: true, SpeedMultiplier: 2.5}
	lease, err := fixture.app.installCombatChargeLocked("combat-owner", request)
	if err != nil {
		t.Fatal(err)
	}
	entry := fixture.read(t, lease.Sites[0].EntryAddr, combatTuningHookSize)
	if !bytes.Equal(entry, lease.Sites[0].Installed) || relJumpTarget(lease.Sites[0].EntryAddr, entry) != lease.Sites[0].CaveAddr {
		t.Fatalf("charge entry is not the owned cave jump: % X", entry)
	}
	status, err := fixture.app.readCombatTuningFeatureLocked("combat-owner", combatTuningKindCharge)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || !status.Available || status.Instant || status.SpeedMultiplier != 2.5 || len(status.RVAs) != 1 {
		t.Fatalf("unexpected installed charge status: %+v", status)
	}
	if err := fixture.app.restoreCombatTuningOwnedLocked("combat-owner", false); err != nil {
		t.Fatal(err)
	}
	if fixture.app.combatTuningChargeLease != nil {
		t.Fatal("successful charge restore retained its active lease")
	}
	if got := fixture.read(t, fixture.app.combatTuningChargeAddr, len(combatChargeSpec.Original)); !bytes.Equal(got, combatChargeSpec.Original) {
		t.Fatalf("charge entry after restore=% X", got)
	}
}

func TestCombatTuningActionSpeedInstallReadbackAndRestore(t *testing.T) {
	fixture := newCombatTuningProcessFixture(t)
	request := CombatActionSpeedRequest{Enabled: true, SpeedMultiplier: 1.75, ApplyWholeParty: true}
	lease, err := fixture.app.installCombatActionSpeedLocked("combat-owner", request)
	if err != nil {
		t.Fatal(err)
	}
	entry := fixture.read(t, lease.Sites[0].EntryAddr, combatTuningHookSize)
	if !bytes.Equal(entry, lease.Sites[0].Installed) || relJumpTarget(lease.Sites[0].EntryAddr, entry) != lease.Sites[0].CaveAddr {
		t.Fatalf("action-speed entry is not the owned cave jump: % X", entry)
	}
	status, err := fixture.app.readCombatTuningFeatureLocked("combat-owner", combatTuningKindActionSpeed)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || !status.Available || !status.ApplyWholeParty || status.SpeedMultiplier != 1.75 || len(status.RVAs) != 1 {
		t.Fatalf("unexpected installed action-speed status: %+v", status)
	}
	if err := fixture.app.restoreCombatTuningOwnedLocked("combat-owner", false); err != nil {
		t.Fatal(err)
	}
	if fixture.app.combatTuningActionSpeedLease != nil {
		t.Fatal("successful action-speed restore retained its active lease")
	}
	if got := fixture.read(t, fixture.app.combatTuningActionSpeedAddr, len(combatActionSpeedSpec.Original)); !bytes.Equal(got, combatActionSpeedSpec.Original) {
		t.Fatalf("action-speed entry after restore=% X", got)
	}
}

func TestCombatTuningRestoreRejectsWrongOwnerWithoutChangingEntry(t *testing.T) {
	fixture := newCombatTuningProcessFixture(t)
	lease, err := fixture.app.installCombatChargeLocked("combat-owner", CombatChargeRequest{Enabled: true, Instant: true})
	if err != nil {
		t.Fatal(err)
	}
	before := fixture.read(t, lease.Sites[0].EntryAddr, combatTuningHookSize)
	err = fixture.app.restoreCombatTuningOwnedLocked("other-owner", false)
	if !errors.Is(err, errRuntimeOwnerLeaseStale) {
		t.Fatalf("wrong-owner restore error=%v, want stale-owner error", err)
	}
	if got := fixture.read(t, lease.Sites[0].EntryAddr, combatTuningHookSize); !bytes.Equal(got, before) {
		t.Fatalf("wrong-owner restore changed entry: got % X want % X", got, before)
	}
	if fixture.app.combatTuningChargeLease != lease {
		t.Fatal("wrong-owner restore discarded the recovery lease")
	}
}

func TestCombatTuningRestoreRejectsDifferentProcessInstance(t *testing.T) {
	fixture := newCombatTuningProcessFixture(t)
	lease, err := fixture.app.installCombatChargeLocked("combat-owner", CombatChargeRequest{Enabled: true, Instant: true})
	if err != nil {
		t.Fatal(err)
	}
	before := fixture.read(t, lease.Sites[0].EntryAddr, combatTuningHookSize)
	lease.Process.Created++
	err = fixture.app.restoreCombatTuningOwnedLocked("combat-owner", false)
	if err == nil || !strings.Contains(err.Error(), "另一个游戏进程实例") {
		t.Fatalf("different-PID-instance restore error=%v", err)
	}
	if got := fixture.read(t, lease.Sites[0].EntryAddr, combatTuningHookSize); !bytes.Equal(got, before) {
		t.Fatalf("different-instance restore changed entry: got % X want % X", got, before)
	}
	if fixture.app.combatTuningChargeLease != lease {
		t.Fatal("different-instance restore discarded the recovery lease")
	}
}

func TestCombatTuningRestoreFailsClosedOnForeignEntryBytes(t *testing.T) {
	fixture := newCombatTuningProcessFixture(t)
	lease, err := fixture.app.installCombatChargeLocked("combat-owner", CombatChargeRequest{Enabled: true, Instant: true})
	if err != nil {
		t.Fatal(err)
	}
	foreign := []byte{0xCC, 0x90, 0x90, 0x90, 0x90}
	if err := writeCodeMemory(fixture.app.hProcess, lease.Sites[0].EntryAddr, foreign); err != nil {
		t.Fatal(err)
	}
	err = fixture.app.restoreCombatTuningOwnedLocked("combat-owner", false)
	if err == nil || !strings.Contains(err.Error(), "既不是自有跳转也不是原始指令") {
		t.Fatalf("foreign-entry restore error=%v", err)
	}
	if got := fixture.read(t, lease.Sites[0].EntryAddr, len(foreign)); !bytes.Equal(got, foreign) {
		t.Fatalf("fail-closed restore overwrote foreign bytes: got % X want % X", got, foreign)
	}
	if fixture.app.combatTuningChargeLease != lease {
		t.Fatal("foreign-entry restore discarded the recovery lease")
	}
}

func TestCombatTuningRestoreFailsClosedOnCorruptOwnedCave(t *testing.T) {
	fixture := newCombatTuningProcessFixture(t)
	lease, err := fixture.app.installCombatChargeLocked("combat-owner", CombatChargeRequest{Enabled: true, Instant: true})
	if err != nil {
		t.Fatal(err)
	}
	site := lease.Sites[0]
	corrupt := []byte{0xCC}
	if err := writeCodeMemory(fixture.app.hProcess, site.CaveAddr, corrupt); err != nil {
		t.Fatal(err)
	}
	before := fixture.read(t, site.EntryAddr, combatTuningHookSize)
	err = fixture.app.restoreCombatTuningOwnedLocked("combat-owner", false)
	if err == nil || !strings.Contains(err.Error(), "代码洞所有权校验失败") {
		t.Fatalf("corrupt-cave restore error=%v", err)
	}
	if got := fixture.read(t, site.EntryAddr, combatTuningHookSize); !bytes.Equal(got, before) {
		t.Fatalf("corrupt-cave restore changed entry: got % X want % X", got, before)
	}
	if fixture.app.combatTuningChargeLease != lease {
		t.Fatal("corrupt-cave restore discarded the recovery lease")
	}
}

func TestCombatTuningCooldownRestoreContinuesAfterOneCorruptCave(t *testing.T) {
	fixture := newCombatTuningProcessFixture(t)
	lease, err := fixture.app.installCombatCooldownLocked("combat-owner", CombatCooldownRequest{
		Enabled: true, SpeedMultiplier: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	failed := lease.Sites[1]
	if err := writeCodeMemory(fixture.app.hProcess, failed.CaveAddr, []byte{0xCC}); err != nil {
		t.Fatal(err)
	}
	err = fixture.app.restoreCombatTuningOwnedLocked("combat-owner", false)
	if err == nil || !strings.Contains(err.Error(), failed.Label) {
		t.Fatalf("partial cooldown restore error=%v", err)
	}
	if fixture.app.combatTuningCooldownLease != lease || len(lease.Sites) != 1 ||
		lease.Sites[0].EntryAddr != failed.EntryAddr {
		t.Fatalf("partial restore did not retain only the failed site: %+v", lease.Sites)
	}
	for index, addr := range fixture.app.combatTuningCooldownAddrs {
		got := fixture.read(t, addr, len(combatCooldownSpecs[index].Original))
		if index == 1 {
			if bytes.Equal(got, combatCooldownSpecs[index].Original) {
				t.Fatalf("%s unexpectedly restored without proving cave ownership", combatCooldownSpecs[index].Label)
			}
			continue
		}
		if !bytes.Equal(got, combatCooldownSpecs[index].Original) {
			t.Fatalf("%s was not restored after peer failure: % X", combatCooldownSpecs[index].Label, got)
		}
	}
	if len(fixture.app.retiredRuntimeCaves) != len(combatCooldownSpecs)-1 {
		t.Fatalf("retired caves=%d, want %d successful restores", len(fixture.app.retiredRuntimeCaves), len(combatCooldownSpecs)-1)
	}
}

func TestCharaReleaseRestoresCombatTuningOwnedByThePage(t *testing.T) {
	fixture := newCombatTuningProcessFixture(t)
	entry := fixture.app.combatTuningChargeAddr
	actionEntry := fixture.app.combatTuningActionSpeedAddr
	if _, err := fixture.app.installCombatChargeLocked("combat-owner", CombatChargeRequest{Enabled: true, Instant: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.app.installCombatActionSpeedLocked("combat-owner", CombatActionSpeedRequest{Enabled: true, SpeedMultiplier: 1.5}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.app.CharaRelease("combat-owner"); err != nil {
		t.Fatal(err)
	}
	if got := fixture.read(t, entry, len(combatChargeSpec.Original)); !bytes.Equal(got, combatChargeSpec.Original) {
		t.Fatalf("owned page release did not restore charge entry: % X", got)
	}
	if got := fixture.read(t, actionEntry, len(combatActionSpeedSpec.Original)); !bytes.Equal(got, combatActionSpeedSpec.Original) {
		t.Fatalf("owned page release did not restore action-speed entry: % X", got)
	}
	if fixture.app.hProcess != 0 || fixture.app.charaOwnerToken != "" ||
		fixture.app.combatTuningChargeLease != nil || fixture.app.combatTuningActionSpeedLease != nil || len(fixture.app.retiredRuntimeCaves) != 0 {
		t.Fatalf("owned page release retained runtime state: handle=%v owner=%q charge=%+v action=%+v retired=%+v",
			fixture.app.hProcess, fixture.app.charaOwnerToken, fixture.app.combatTuningChargeLease,
			fixture.app.combatTuningActionSpeedLease, fixture.app.retiredRuntimeCaves)
	}
}

func TestCombatTuningPreparationFailsClosedOnChangedOriginal(t *testing.T) {
	fixture := newCombatTuningProcessFixture(t)
	changed := []byte{0xCC, 0xCC, 0xCC, 0xCC, 0xCC}
	if err := writeCodeMemory(fixture.app.hProcess, fixture.app.combatTuningCooldownAddrs[1], changed); err != nil {
		t.Fatal(err)
	}
	_, err := fixture.app.installCombatCooldownLocked("combat-owner", CombatCooldownRequest{Enabled: true, SpeedMultiplier: 2})
	if err == nil || !strings.Contains(err.Error(), "原始指令已变化") {
		t.Fatalf("changed-original install error=%v", err)
	}
	if fixture.app.combatTuningCooldownLease != nil {
		t.Fatal("preparation failure retained an active cooldown lease")
	}
	if got := fixture.read(t, fixture.app.combatTuningCooldownAddrs[0], len(combatCooldownSpecs[0].Original)); !bytes.Equal(got, combatCooldownSpecs[0].Original) {
		t.Fatalf("preparation failure changed an earlier entry: % X", got)
	}
	if got := fixture.read(t, fixture.app.combatTuningCooldownAddrs[1], len(changed)); !bytes.Equal(got, changed) {
		t.Fatalf("preparation failure overwrote foreign bytes: % X", got)
	}
}

func TestCombatChargeAndGhandagozaInstantPunchConflictBothDirections(t *testing.T) {
	t.Run("charge rejects active catalog patch", func(t *testing.T) {
		leases := map[string]runtimePatchPatchLease{
			"runtime-patch-017": {
				FeatureID:  "runtime-patch-017",
				OwnerToken: "combat-owner",
				State:      runtimePatchPatchEnabled,
			},
		}
		err := validateCombatChargeCatalogConflict(leases)
		if err == nil || !strings.Contains(err.Error(), "瞬间直冲拳") {
			t.Fatalf("charge/catalog conflict error=%v", err)
		}
		delete(leases, "runtime-patch-017")
		if err := validateCombatChargeCatalogConflict(leases); err != nil {
			t.Fatalf("inactive catalog conflict error=%v", err)
		}
	})

	t.Run("catalog patch rejects active charge", func(t *testing.T) {
		lease := &combatTuningLease{Kind: combatTuningKindCharge}
		err := validateRuntimePatchCombatChargeConflict("runtime-patch-017", lease)
		if err == nil || !strings.Contains(err.Error(), "共享蓄力调整") {
			t.Fatalf("catalog/charge conflict error=%v", err)
		}
		if err := validateRuntimePatchCombatChargeConflict("runtime-patch-018", lease); err != nil {
			t.Fatalf("unrelated catalog feature conflict error=%v", err)
		}
		if err := validateRuntimePatchCombatChargeConflict("runtime-patch-017", nil); err != nil {
			t.Fatalf("inactive charge conflict error=%v", err)
		}
	})
}

func TestCombatTuningPublishFailureRollsBackEarlierInstalledSites(t *testing.T) {
	fixture := newCombatTuningProcessFixture(t)
	injected := errors.New("injected second-site install failure")
	previous := combatTuningInstallRemoteCodeHook
	calls := 0
	combatTuningInstallRemoteCodeHook = func(handle windows.Handle, addr uintptr, oldBytes, patch []byte) (codeHookInstallResult, error) {
		calls++
		if calls == 2 {
			return codeHookInstallResult{State: codeHookEntryNeverPublished}, injected
		}
		return previous(handle, addr, oldBytes, patch)
	}
	t.Cleanup(func() { combatTuningInstallRemoteCodeHook = previous })

	_, err := fixture.app.installCombatCooldownLocked("combat-owner", CombatCooldownRequest{Enabled: true, SpeedMultiplier: 2})
	if !errors.Is(err, injected) {
		t.Fatalf("publish error=%v, want injected failure", err)
	}
	if fixture.app.combatTuningCooldownLease != nil {
		t.Fatal("proven publish rollback retained an active lease")
	}
	for index, addr := range fixture.app.combatTuningCooldownAddrs {
		if got := fixture.read(t, addr, len(combatCooldownSpecs[index].Original)); !bytes.Equal(got, combatCooldownSpecs[index].Original) {
			t.Fatalf("%s entry after publish rollback=% X", combatCooldownSpecs[index].Label, got)
		}
	}
	if len(fixture.app.retiredRuntimeCaves) != len(combatCooldownSpecs) {
		t.Fatalf("publish rollback retired caves=%d, want %d", len(fixture.app.retiredRuntimeCaves), len(combatCooldownSpecs))
	}
}

func TestCombatTuningPatternsMatchLocalGame202(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_TEST to verify the local game 2.0.2 executable")
	}
	if err := verifyRuntimePatchLocalGameIdentity(path); err != nil {
		t.Fatalf("verify local game 2.0.2 identity: %v", err)
	}
	sections, err := readRuntimePatchLocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		spec combatTuningSiteSpec
		rva  uint32
	}{
		{spec: combatCooldownSpecs[0], rva: 0x21EEF5D},
		{spec: combatCooldownSpecs[1], rva: 0x28037AE},
		{spec: combatCooldownSpecs[2], rva: 0x33D64FA},
		{spec: combatChargeSpec, rva: 0x27A39D0},
	}
	for _, check := range checks {
		t.Run(check.spec.Label, func(t *testing.T) {
			pattern := runtimePatchPattern{
				Values: append([]byte(nil), check.spec.Pattern...),
				Mask:   make([]byte, len(check.spec.Mask)),
			}
			for index, exact := range check.spec.Mask {
				if exact {
					pattern.Mask[index] = 0xFF
				}
			}
			matches := findRuntimePatchLocalPatternMatches(sections, pattern)
			if len(matches) != 1 || matches[0].rva != check.rva {
				t.Fatalf("matches=%s, want one match at RVA 0x%X", formatRuntimePatchLocalMatchLocations(matches), check.rva)
			}
			got, err := readPEImageRVA(path, uintptr(check.rva), len(check.spec.Original))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, check.spec.Original) {
				t.Fatalf("RVA 0x%X original=% X, want % X", check.rva, got, check.spec.Original)
			}
		})
	}
}

func TestCombatActionSpeedPatternMatchesLocalGame203(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_203_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_203_TEST to verify the local game 2.0.3 executable")
	}
	if err := verifyRuntimePatchLocalGameIdentityExact(path, runtimePatchLocalGame203Size, runtimePatchLocalGame203SHA256); err != nil {
		t.Fatalf("verify local game 2.0.3 identity: %v", err)
	}
	sections, err := readRuntimePatchLocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	pattern := runtimePatchPattern{
		Values: append([]byte(nil), combatActionSpeedSpec.Pattern...),
		Mask:   make([]byte, len(combatActionSpeedSpec.Mask)),
	}
	for index, exact := range combatActionSpeedSpec.Mask {
		if exact {
			pattern.Mask[index] = 0xFF
		}
	}
	matches := findRuntimePatchLocalPatternMatches(sections, pattern)
	const wantRVA = uint32(0xBB0918)
	if len(matches) != 1 || matches[0].rva != wantRVA {
		t.Fatalf("matches=%s, want one match at RVA 0x%X", formatRuntimePatchLocalMatchLocations(matches), wantRVA)
	}
	got, err := readPEImageRVA(path, uintptr(wantRVA), len(combatActionSpeedSpec.Original))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, combatActionSpeedSpec.Original) {
		t.Fatalf("RVA 0x%X original=% X, want % X", wantRVA, got, combatActionSpeedSpec.Original)
	}
}
