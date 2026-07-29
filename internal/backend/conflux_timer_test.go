package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

type fakeConfluxTimerMemory struct {
	data      map[uintptr][]byte
	failWrite uintptr
	failNext  map[uintptr]int
}

type decrementingConfluxTimerMemory struct {
	*fakeConfluxTimerMemory
	activeAddress uintptr
	step          float32
}

func (memory *decrementingConfluxTimerMemory) Read(addr uintptr, size int) ([]byte, error) {
	value, err := memory.fakeConfluxTimerMemory.Read(addr, size)
	if err != nil || addr != memory.activeAddress || size != confluxTimerActiveBytes {
		return value, err
	}
	initial, current, err := decodeConfluxTimerActive(value)
	if err != nil {
		return nil, err
	}
	current = max(0, current-memory.step)
	decremented := encodeConfluxActive(initial, current)
	memory.data[addr] = append([]byte(nil), decremented...)
	return decremented, nil
}

func (memory *fakeConfluxTimerMemory) Read(addr uintptr, size int) ([]byte, error) {
	value, ok := memory.data[addr]
	if !ok || len(value) != size {
		return nil, errors.New("missing fake timer memory")
	}
	return append([]byte(nil), value...), nil
}

func (memory *fakeConfluxTimerMemory) Write(addr uintptr, data []byte) error {
	if memory.failNext[addr] > 0 {
		memory.failNext[addr]--
		return errors.New("injected one-shot timer write failure")
	}
	if addr == memory.failWrite {
		return errors.New("injected timer write failure")
	}
	memory.data[addr] = append([]byte(nil), data...)
	return nil
}

func TestInstallConfluxTimerLeaseRollsBackBothFieldsOnSecondWriteFailure(t *testing.T) {
	sites := confluxTimerSites{Config: 0x1100, Active: 0x1300}
	originalConfig := encodeConfluxTimerValues(confluxTimerOriginalValues)
	originalActive := encodeConfluxActive(60, 13.5)
	shortened := encodeConfluxActive(2, 2)
	memory := &fakeConfluxTimerMemory{
		data:     map[uintptr][]byte{sites.Config: append([]byte(nil), originalConfig...), sites.Active: append([]byte(nil), originalActive...)},
		failNext: map[uintptr]int{sites.Active: 1},
	}
	lease := &confluxTimerLease{
		Sites: sites, State: confluxTimerLeaseRecovery,
		Original: originalConfig, PreviousActive: originalActive, WrittenActive: shortened,
	}
	err := installConfluxTimerLease(memory, lease)
	if err == nil || errors.Is(err, errLiveMemoryRollbackUnproven) {
		t.Fatalf("second write error was not cleanly rolled back: %v", err)
	}
	if !bytes.Equal(memory.data[sites.Config], originalConfig) || !bytes.Equal(memory.data[sites.Active], originalActive) {
		t.Fatal("failed install left timer memory modified")
	}
}

func TestInstallConfluxTimerLeaseAcceptsNaturallyDecrementingReadback(t *testing.T) {
	sites := confluxTimerSites{Config: 0x1100, Active: 0x1300}
	base := &fakeConfluxTimerMemory{data: map[uintptr][]byte{
		sites.Config: encodeConfluxTimerValues(confluxTimerOriginalValues),
		sites.Active: encodeConfluxActive(60, 13.5),
	}}
	memory := &decrementingConfluxTimerMemory{fakeConfluxTimerMemory: base, activeAddress: sites.Active, step: 0.1}
	lease := &confluxTimerLease{
		Sites: sites, State: confluxTimerLeaseRecovery,
		Original: encodeConfluxTimerValues(confluxTimerOriginalValues), PreviousActive: encodeConfluxActive(60, 13.5), WrittenActive: encodeConfluxActive(2, 2),
	}
	if err := installConfluxTimerLease(memory, lease); err != nil {
		t.Fatalf("decrementing timer readback was rejected: %v", err)
	}
}

func encodeConfluxActive(initial, current float32) []byte {
	result := make([]byte, confluxTimerActiveBytes)
	binary.LittleEndian.PutUint32(result[:4], math.Float32bits(initial))
	binary.LittleEndian.PutUint32(result[4:], math.Float32bits(current))
	return result
}

func TestClassifyConfluxTimerConfigRejectsUnverifiedValues(t *testing.T) {
	original := encodeConfluxTimerValues(confluxTimerOriginalValues)
	fast := encodeConfluxTimerValues(confluxTimerFastValues)
	if classifyConfluxTimerConfig(original) != confluxTimerStateOff || classifyConfluxTimerConfig(fast) != confluxTimerStateOn {
		t.Fatal("pinned original and fast timer configurations were not recognized")
	}
	mixed := append([]byte(nil), original...)
	copy(mixed[:4], fast[:4])
	if classifyConfluxTimerConfig(mixed) != confluxTimerStateMixed {
		t.Fatal("mixed timer configuration was not rejected")
	}
	unknown := append([]byte(nil), original...)
	binary.LittleEndian.PutUint32(unknown[8:12], math.Float32bits(99))
	if classifyConfluxTimerConfig(unknown) != confluxTimerStateUnknown {
		t.Fatal("unknown timer configuration was not rejected")
	}
}

func TestShortenConfluxTimerActiveNeverExtendsCountdown(t *testing.T) {
	shortened, err := shortenConfluxTimerActive(encodeConfluxActive(60, 0.75))
	if err != nil {
		t.Fatal(err)
	}
	initial, current, err := decodeConfluxTimerActive(shortened)
	if err != nil {
		t.Fatal(err)
	}
	if initial != 2 || current != 0.75 {
		t.Fatalf("shortened active timer = %.2f/%.2f, want 2.00/0.75", initial, current)
	}
	if _, err := shortenConfluxTimerActive(encodeConfluxActive(float32(math.NaN()), 1)); err == nil {
		t.Fatal("NaN timer was accepted")
	}
}

func TestConfluxTimerActiveReadbackRejectsIncreases(t *testing.T) {
	limit := encodeConfluxActive(2, 2)
	if !confluxTimerActiveDidNotIncrease(limit, encodeConfluxActive(2, 1.9)) {
		t.Fatal("a naturally decreasing countdown was rejected")
	}
	if confluxTimerActiveDidNotIncrease(limit, encodeConfluxActive(2, 2.1)) || confluxTimerActiveDidNotIncrease(limit, encodeConfluxActive(3, 1)) {
		t.Fatal("an increased countdown was accepted")
	}
}

func TestConfluxTimerVerificationCacheRequiresExactProcessInstance(t *testing.T) {
	verified := processInstanceID{PID: 1234, Created: 100}
	if !sameProcessInstance(verified, processInstanceID{PID: 1234, Created: 100}) {
		t.Fatal("the verified process instance did not match itself")
	}
	if sameProcessInstance(verified, processInstanceID{PID: 1234, Created: 101}) || sameProcessInstance(verified, processInstanceID{PID: 4321, Created: 100}) {
		t.Fatal("verification cache survived a PID or creation-time change")
	}
}

func TestResolveConfluxTimerSitesUsesPinnedManagerPointer(t *testing.T) {
	moduleBase := uintptr(0x140000000)
	manager := uintptr(0x200000000)
	pointer := make([]byte, 8)
	binary.LittleEndian.PutUint64(pointer, uint64(manager))
	memory := &fakeConfluxTimerMemory{data: map[uintptr][]byte{
		moduleBase + confluxTimerManagerPointerRVA: pointer,
	}}
	sites, err := resolveConfluxTimerSites(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	if sites.Manager != manager || sites.Config != manager+confluxTimerConfigOffset || sites.Mode != manager+confluxTimerModeOffset || sites.Active != manager+confluxTimerActiveOffset {
		t.Fatalf("unexpected timer sites: %+v", sites)
	}
}

func TestReconcileConfluxTimerLeaseHandlesManagerReplacement(t *testing.T) {
	moduleBase := uintptr(0x140000000)
	oldManager := uintptr(0x200000000)
	newManager := uintptr(0x300000000)
	pointer := make([]byte, 8)
	binary.LittleEndian.PutUint64(pointer, uint64(newManager))
	newSites := confluxTimerSites{Manager: newManager, Config: newManager + confluxTimerConfigOffset, Mode: newManager + confluxTimerModeOffset, Active: newManager + confluxTimerActiveOffset}
	memory := &fakeConfluxTimerMemory{data: map[uintptr][]byte{
		moduleBase + confluxTimerManagerPointerRVA: pointer,
		newSites.Config: encodeConfluxTimerValues(confluxTimerOriginalValues),
	}}
	lease := &confluxTimerLease{Sites: confluxTimerSites{Manager: oldManager}, State: confluxTimerLeaseEnabled}
	reconciled, sites, retired, err := reconcileConfluxTimerLease(memory, moduleBase, lease)
	if err != nil || !retired || reconciled != nil || sites.Manager != newManager {
		t.Fatalf("default replacement manager was not retired safely: lease=%+v sites=%+v retired=%v err=%v", reconciled, sites, retired, err)
	}

	memory.data[newSites.Config] = encodeConfluxTimerValues(confluxTimerFastValues)
	reconciled, _, retired, err = reconcileConfluxTimerLease(memory, moduleBase, lease)
	if err != nil || retired || reconciled == nil || reconciled.Sites.Manager != newManager || reconciled.State != confluxTimerLeaseEnabled {
		t.Fatalf("fast replacement manager was not rebound safely: lease=%+v retired=%v err=%v", reconciled, retired, err)
	}

	mixed := encodeConfluxTimerValues(confluxTimerOriginalValues)
	fast := encodeConfluxTimerValues(confluxTimerFastValues)
	copy(mixed[:4], fast[:4])
	memory.data[newSites.Config] = mixed
	if _, _, _, err := reconcileConfluxTimerLease(memory, moduleBase, lease); !errors.Is(err, errLiveMemoryRollbackUnproven) {
		t.Fatalf("mixed replacement manager was not rejected: %v", err)
	}
}

func TestReadConfluxTimerStatusRequiresEndlessModeAndOwnership(t *testing.T) {
	sites := confluxTimerSites{Manager: 0x1000, Config: 0x1100, Mode: 0x1200, Active: 0x1300}
	mode := make([]byte, 4)
	binary.LittleEndian.PutUint32(mode, confluxTimerEndlessMode)
	memory := &fakeConfluxTimerMemory{data: map[uintptr][]byte{
		sites.Config: encodeConfluxTimerValues(confluxTimerFastValues),
		sites.Mode:   mode,
		sites.Active: encodeConfluxActive(2, 1),
	}}
	unowned, err := readConfluxTimerStatus(memory, sites, false)
	if err != nil {
		t.Fatal(err)
	}
	if !unowned.Enabled || unowned.Available || unowned.Error == "" {
		t.Fatalf("unowned fast timer status is not fail-closed: %+v", unowned)
	}
	owned, err := readConfluxTimerStatus(memory, sites, true)
	if err != nil {
		t.Fatal(err)
	}
	if !owned.Enabled || !owned.Available || !owned.Owned || owned.Error != "" {
		t.Fatalf("owned fast timer status is incorrect: %+v", owned)
	}
}

func TestRestoreConfluxTimerEnabledLeaseDoesNotExtendActiveCountdown(t *testing.T) {
	sites := confluxTimerSites{Config: 0x1100, Active: 0x1300}
	originalActive := encodeConfluxActive(60, 30)
	currentActive := encodeConfluxActive(2, 0.5)
	memory := &fakeConfluxTimerMemory{data: map[uintptr][]byte{
		sites.Config: encodeConfluxTimerValues(confluxTimerFastValues),
		sites.Active: currentActive,
	}}
	lease := &confluxTimerLease{
		Sites: sites, State: confluxTimerLeaseEnabled,
		Original: encodeConfluxTimerValues(confluxTimerOriginalValues), PreviousActive: originalActive, WrittenActive: encodeConfluxActive(2, 2),
	}
	if err := restoreConfluxTimerLease(memory, lease); err != nil {
		t.Fatal(err)
	}
	if classifyConfluxTimerConfig(memory.data[sites.Config]) != confluxTimerStateOff {
		t.Fatal("timer defaults were not restored")
	}
	if !bytes.Equal(memory.data[sites.Active], currentActive) {
		t.Fatal("normal disable extended or rewound the active countdown")
	}
}

func TestRestoreConfluxTimerRecoveryLeaseRestoresBothWrites(t *testing.T) {
	sites := confluxTimerSites{Config: 0x1100, Active: 0x1300}
	originalActive := encodeConfluxActive(60, 13.5)
	memory := &fakeConfluxTimerMemory{data: map[uintptr][]byte{
		sites.Config: encodeConfluxTimerValues(confluxTimerFastValues),
		sites.Active: encodeConfluxActive(2, 2),
	}}
	lease := &confluxTimerLease{
		Sites: sites, State: confluxTimerLeaseRecovery,
		Original: encodeConfluxTimerValues(confluxTimerOriginalValues), PreviousActive: originalActive, WrittenActive: encodeConfluxActive(2, 2),
	}
	if err := restoreConfluxTimerLease(memory, lease); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(memory.data[sites.Active], originalActive) {
		t.Fatal("failed enable did not restore the previous active countdown")
	}
}

func TestRestoreConfluxTimerRecoveryLeaseAcceptsDecrementAfterRollback(t *testing.T) {
	sites := confluxTimerSites{Config: 0x1100, Active: 0x1300}
	base := &fakeConfluxTimerMemory{data: map[uintptr][]byte{
		sites.Config: encodeConfluxTimerValues(confluxTimerFastValues),
		sites.Active: encodeConfluxActive(2, 2),
	}}
	memory := &decrementingConfluxTimerMemory{fakeConfluxTimerMemory: base, activeAddress: sites.Active, step: 0.1}
	lease := &confluxTimerLease{
		Sites: sites, State: confluxTimerLeaseRecovery,
		Original: encodeConfluxTimerValues(confluxTimerOriginalValues), PreviousActive: encodeConfluxActive(60, 13.5), WrittenActive: encodeConfluxActive(2, 2),
	}
	if err := restoreConfluxTimerLease(memory, lease); err != nil {
		t.Fatalf("decrement after rollback was rejected: %v", err)
	}
}

func TestRestoreConfluxTimerRefusesUnknownThirdPartyState(t *testing.T) {
	sites := confluxTimerSites{Config: 0x1100, Active: 0x1300}
	unknown := encodeConfluxTimerValues(confluxTimerOriginalValues)
	binary.LittleEndian.PutUint32(unknown[:4], math.Float32bits(99))
	memory := &fakeConfluxTimerMemory{data: map[uintptr][]byte{
		sites.Config: unknown,
		sites.Active: encodeConfluxActive(1, 1),
	}}
	lease := &confluxTimerLease{Sites: sites, State: confluxTimerLeaseEnabled, Original: encodeConfluxTimerValues(confluxTimerOriginalValues)}
	err := restoreConfluxTimerLease(memory, lease)
	if !errors.Is(err, errLiveMemoryRollbackUnproven) {
		t.Fatalf("unknown state restore error = %v", err)
	}
	if !bytes.Equal(memory.data[sites.Config], unknown) {
		t.Fatal("unknown third-party timer state was overwritten")
	}
}

func TestRestoreConfluxTimerEnabledLeaseRefusesMixedThirdPartyState(t *testing.T) {
	sites := confluxTimerSites{Config: 0x1100, Active: 0x1300}
	mixed := encodeConfluxTimerValues(confluxTimerOriginalValues)
	fast := encodeConfluxTimerValues(confluxTimerFastValues)
	copy(mixed[:4], fast[:4])
	memory := &fakeConfluxTimerMemory{data: map[uintptr][]byte{
		sites.Config: mixed,
		sites.Active: encodeConfluxActive(1, 1),
	}}
	lease := &confluxTimerLease{Sites: sites, State: confluxTimerLeaseEnabled, Original: encodeConfluxTimerValues(confluxTimerOriginalValues)}
	err := restoreConfluxTimerLease(memory, lease)
	if !errors.Is(err, errLiveMemoryRollbackUnproven) {
		t.Fatalf("mixed third-party state error = %v", err)
	}
	if !bytes.Equal(memory.data[sites.Config], mixed) {
		t.Fatal("mixed third-party timer state was overwritten")
	}
}
