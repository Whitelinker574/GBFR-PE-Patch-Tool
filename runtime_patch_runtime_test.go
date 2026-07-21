package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

type runtimePatchFakeWrite struct {
	addr uintptr
	data []byte
}

type runtimePatchFakeMemory struct {
	data           map[uintptr][]byte
	writes         []runtimePatchFakeWrite
	reads          int
	writeCalls     int
	readErrAt      map[int]error
	readValueAt    map[int][]byte
	writeErrAt     map[int]error
	afterWriteCall func(call int, addr uintptr, data []byte)
}

func newRuntimePatchFakeMemory(values map[uintptr][]byte) *runtimePatchFakeMemory {
	cloned := make(map[uintptr][]byte, len(values))
	for addr, value := range values {
		cloned[addr] = append([]byte(nil), value...)
	}
	return &runtimePatchFakeMemory{
		data:        cloned,
		readErrAt:   make(map[int]error),
		readValueAt: make(map[int][]byte),
		writeErrAt:  make(map[int]error),
	}
}

func (m *runtimePatchFakeMemory) ReadCode(addr uintptr, size int) ([]byte, error) {
	m.reads++
	if err := m.readErrAt[m.reads]; err != nil {
		return nil, err
	}
	if value, ok := m.readValueAt[m.reads]; ok {
		return append([]byte(nil), value...), nil
	}
	value, ok := m.data[addr]
	if !ok || len(value) != size {
		return nil, fmt.Errorf("read %#x/%d", addr, size)
	}
	return append([]byte(nil), value...), nil
}

func (m *runtimePatchFakeMemory) WriteCode(addr uintptr, data []byte) error {
	m.writeCalls++
	m.writes = append(m.writes, runtimePatchFakeWrite{addr: addr, data: append([]byte(nil), data...)})
	if err := m.writeErrAt[m.writeCalls]; err != nil {
		return err
	}
	m.data[addr] = append([]byte(nil), data...)
	if m.afterWriteCall != nil {
		m.afterWriteCall(m.writeCalls, addr, data)
	}
	return nil
}

func runtimePatchTestSites() []runtimePatchPatchSiteLease {
	return []runtimePatchPatchSiteLease{
		{Address: 0x1000, Original: []byte{0x74}, Patch: []byte{0xEB}},
		{Address: 0x2000, Original: []byte{0x75, 0x01}, Patch: []byte{0x90, 0x90}},
		{Address: 0x3000, Original: []byte{0x7E}, Patch: []byte{0x7F}},
	}
}

func TestRuntimePatchPatchSingleSiteEnableDisable(t *testing.T) {
	sites := runtimePatchTestSites()[:1]
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{sites[0].Address: sites[0].Original})

	if err := installRuntimePatchSites(memory, sites); err != nil {
		t.Fatalf("installRuntimePatchSites() error = %v", err)
	}
	if !bytes.Equal(memory.data[sites[0].Address], sites[0].Patch) {
		t.Fatalf("enabled bytes = % X, want % X", memory.data[sites[0].Address], sites[0].Patch)
	}
	if err := restoreRuntimePatchSites(memory, sites); err != nil {
		t.Fatalf("restoreRuntimePatchSites() error = %v", err)
	}
	if !bytes.Equal(memory.data[sites[0].Address], sites[0].Original) {
		t.Fatalf("restored bytes = % X, want % X", memory.data[sites[0].Address], sites[0].Original)
	}
}

func TestRuntimePatchPatchThreeSiteEnable(t *testing.T) {
	sites := runtimePatchTestSites()
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{
		sites[0].Address: sites[0].Original,
		sites[1].Address: sites[1].Original,
		sites[2].Address: sites[2].Original,
	})

	if err := installRuntimePatchSites(memory, sites); err != nil {
		t.Fatalf("installRuntimePatchSites() error = %v", err)
	}
	for _, site := range sites {
		if !bytes.Equal(memory.data[site.Address], site.Patch) {
			t.Errorf("bytes at %#x = % X, want % X", site.Address, memory.data[site.Address], site.Patch)
		}
	}
}

func TestRuntimePatchPatchPreflightRejectsLaterForeignBytesBeforeFirstWrite(t *testing.T) {
	sites := runtimePatchTestSites()[:2]
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{
		sites[0].Address: sites[0].Original,
		sites[1].Address: {0xCC, 0xCC},
	})

	if err := installRuntimePatchSites(memory, sites); err == nil {
		t.Fatal("installRuntimePatchSites() error = nil, want preflight rejection")
	}
	if len(memory.writes) != 0 {
		t.Fatalf("writes = %v, want none", memory.writes)
	}
}

func TestRuntimePatchPatchPreflightRejectsActualAddressOverlap(t *testing.T) {
	sites := []runtimePatchPatchSiteLease{
		{Address: 0x1000, Original: []byte{1, 2}, Patch: []byte{3, 4}},
		{Address: 0x1001, Original: []byte{2, 5}, Patch: []byte{4, 6}},
	}
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{0x1000: {1, 2}, 0x1001: {2, 5}})

	if err := installRuntimePatchSites(memory, sites); err == nil {
		t.Fatal("installRuntimePatchSites() error = nil, want overlap rejection")
	}
	if len(memory.writes) != 0 {
		t.Fatalf("writes = %v, want none", memory.writes)
	}
}

func TestRuntimePatchPatchRejectsAlreadyEnabledExternalBytesBeforeWrite(t *testing.T) {
	site := runtimePatchPatchSiteLease{
		Address:  0x1000,
		Original: []byte{0x90, 0x90},
		Patch:    []byte{0x90, 0x90},
	}
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{site.Address: site.Patch})

	if err := installRuntimePatchSites(memory, []runtimePatchPatchSiteLease{site}); err == nil {
		t.Fatal("installRuntimePatchSites() error = nil, want already-enabled ownership rejection")
	}
	if memory.reads != 0 || len(memory.writes) != 0 {
		t.Fatalf("already-enabled patch reached memory: reads=%d writes=%v", memory.reads, memory.writes)
	}
}

func TestRuntimePatchPatchPreparationRejectsAlreadyEnabledExternalPatchBeforeLease(t *testing.T) {
	catalog, err := loadRuntimePatchCatalog()
	if err != nil {
		t.Fatal(err)
	}
	feature := findRuntimePatchCatalogFeature(catalog, "runtime-patch-023")
	if feature == nil || len(feature.Sites) != 1 || feature.Sites[0].Symbol != "GBFR_PATCH_023_1" {
		t.Fatalf("catalog fixture for wildcard patch changed: %+v", feature)
	}
	definition := feature.Sites[0]

	lease, err := prepareRuntimePatchSiteLease(
		0x140000000, 0x141000000, 0x140001000,
		definition, append([]byte(nil), definition.EnableBytes...),
	)
	if err == nil {
		t.Fatalf("prepareRuntimePatchSiteLease() = %+v, nil; want external-patch rejection", lease)
	}
	if lease.Address != 0 || lease.RVA != 0 || lease.Original != nil || lease.Patch != nil {
		t.Fatalf("rejected preparation returned an ownable lease: %+v", lease)
	}
}

func TestRuntimePatchPatchPreparationRejectsForeignWildcardBytesBeforeLease(t *testing.T) {
	catalog, err := loadRuntimePatchCatalog()
	if err != nil {
		t.Fatal(err)
	}
	feature := findRuntimePatchCatalogFeature(catalog, "runtime-patch-023")
	if feature == nil || len(feature.Sites) != 1 || feature.Sites[0].Symbol != "GBFR_PATCH_023_1" {
		t.Fatalf("catalog fixture for wildcard patch changed: %+v", feature)
	}
	definition := feature.Sites[0]
	if len(definition.EnableBytes) != 1 {
		t.Fatalf("enable bytes=% X, want a one-byte patch", definition.EnableBytes)
	}

	foreign := []byte{definition.EnableBytes[0] ^ 0xff}
	lease, err := prepareRuntimePatchSiteLease(
		0x140000000, 0x141000000, 0x140001000,
		definition, foreign,
	)
	if err == nil {
		t.Fatalf("prepareRuntimePatchSiteLease() = %+v, nil; want unproven-original rejection", lease)
	}
	if lease.Address != 0 || lease.RVA != 0 || lease.Original != nil || lease.Patch != nil {
		t.Fatalf("rejected preparation returned an ownable lease: %+v", lease)
	}
}

func TestRuntimePatchPatchPreparationOwnsOnlyLockedExpectedOriginal(t *testing.T) {
	catalog, err := loadRuntimePatchCatalog()
	if err != nil {
		t.Fatal(err)
	}
	feature := findRuntimePatchCatalogFeature(catalog, "runtime-patch-023")
	if feature == nil || len(feature.Sites) != 1 {
		t.Fatalf("catalog fixture changed: %+v", feature)
	}
	definition := feature.Sites[0]
	address := uintptr(0x140001000)
	lease, err := prepareRuntimePatchSiteLease(
		0x140000000, 0x141000000, address,
		definition, append([]byte(nil), definition.ExpectedOriginalBytes...),
	)
	if err != nil {
		t.Fatal(err)
	}
	if lease.Address != address || lease.RVA != 0x1000 || !bytes.Equal(lease.Original, definition.ExpectedOriginalBytes) || !bytes.Equal(lease.Patch, definition.EnableBytes) {
		t.Fatalf("prepared lease=%+v", lease)
	}
	definition.ExpectedOriginalBytes[0] ^= 0xff
	if bytes.Equal(lease.Original, definition.ExpectedOriginalBytes) {
		t.Fatal("prepared lease aliases mutable catalog expected-original bytes")
	}
}

func TestRuntimePatchPatchSecondWriteFailureRollsBackInReverse(t *testing.T) {
	sites := runtimePatchTestSites()[:2]
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{
		sites[0].Address: sites[0].Original,
		sites[1].Address: sites[1].Original,
	})
	memory.writeErrAt[2] = errors.New("second site write failed")

	err := installRuntimePatchSites(memory, sites)
	if err == nil {
		t.Fatal("installRuntimePatchSites() error = nil")
	}
	if errors.Is(err, errLiveMemoryRollbackUnproven) {
		t.Fatalf("rollback was proven but error contains sentinel: %v", err)
	}
	for _, site := range sites {
		if !bytes.Equal(memory.data[site.Address], site.Original) {
			t.Errorf("bytes at %#x = % X, want original % X", site.Address, memory.data[site.Address], site.Original)
		}
	}
	if got := memory.writes[len(memory.writes)-1].addr; got != sites[0].Address {
		t.Fatalf("last rollback address = %#x, want first site %#x", got, sites[0].Address)
	}
}

func TestRuntimePatchPatchWriteReadbackMismatchRollsBackAllSites(t *testing.T) {
	sites := runtimePatchTestSites()[:2]
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{
		sites[0].Address: sites[0].Original,
		sites[1].Address: sites[1].Original,
	})
	// Two preflight reads, two reads for site one, then pre/post reads for site two.
	memory.readValueAt[6] = []byte{0xCC, 0xCC}

	err := installRuntimePatchSites(memory, sites)
	if err == nil {
		t.Fatal("installRuntimePatchSites() error = nil")
	}
	if errors.Is(err, errLiveMemoryRollbackUnproven) {
		t.Fatalf("rollback was proven but error contains sentinel: %v", err)
	}
	for _, site := range sites {
		if !bytes.Equal(memory.data[site.Address], site.Original) {
			t.Errorf("bytes at %#x = % X, want original % X", site.Address, memory.data[site.Address], site.Original)
		}
	}
}

func TestRuntimePatchPatchFinalReadbackFailureRollsBackAllSites(t *testing.T) {
	sites := runtimePatchTestSites()[:2]
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{
		sites[0].Address: sites[0].Original,
		sites[1].Address: sites[1].Original,
	})
	// Reads 1-2 preflight, 3-6 per-site atomic installs, 7 is final verification.
	memory.readErrAt[7] = errors.New("final verification failed")

	err := installRuntimePatchSites(memory, sites)
	if err == nil {
		t.Fatal("installRuntimePatchSites() error = nil")
	}
	for _, site := range sites {
		if !bytes.Equal(memory.data[site.Address], site.Original) {
			t.Errorf("bytes at %#x = % X, want original % X", site.Address, memory.data[site.Address], site.Original)
		}
	}
}

func TestRuntimePatchPatchRestoreForeignBytesNeverOverwrites(t *testing.T) {
	site := runtimePatchTestSites()[0]
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{site.Address: {0xCC}})

	err := restoreRuntimePatchSites(memory, []runtimePatchPatchSiteLease{site})
	if !errors.Is(err, errLiveMemoryRollbackUnproven) {
		t.Fatalf("restore error = %v, want errLiveMemoryRollbackUnproven", err)
	}
	if len(memory.writes) != 0 {
		t.Fatalf("writes = %v, want none", memory.writes)
	}
}

func TestRuntimePatchPatchRestoreContinuesAfterOneSiteFailure(t *testing.T) {
	sites := runtimePatchTestSites()[:2]
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{
		sites[0].Address: sites[0].Patch,
		sites[1].Address: sites[1].Patch,
	})
	memory.writeErrAt[1] = errors.New("last site restore failed")

	err := restoreRuntimePatchSites(memory, sites)
	if !errors.Is(err, errLiveMemoryRollbackUnproven) {
		t.Fatalf("restore error = %v, want errLiveMemoryRollbackUnproven", err)
	}
	if !bytes.Equal(memory.data[sites[0].Address], sites[0].Original) {
		t.Fatalf("first site was not attempted after later failure: % X", memory.data[sites[0].Address])
	}
	if got := []uintptr{memory.writes[0].addr, memory.writes[len(memory.writes)-1].addr}; !reflect.DeepEqual(got, []uintptr{sites[1].Address, sites[0].Address}) {
		t.Fatalf("restore endpoints = %#x, want reverse feature site order", got)
	}
}

func TestRuntimePatchSiteCopiesAreDefensive(t *testing.T) {
	site := runtimePatchPatchSiteLease{Address: 0x1000, Original: []byte{1}, Patch: []byte{2}}
	copySite := cloneRuntimePatchSiteLease(site)
	copySite.Original[0] = 9
	copySite.Patch[0] = 8
	if site.Original[0] != 1 || site.Patch[0] != 2 {
		t.Fatalf("clone aliases input: original=%v patch=%v", site.Original, site.Patch)
	}
}

func TestRuntimePatchPatchOwnedLeaseRejectsStaleEmptyAndReplacedProcessBeforeIO(t *testing.T) {
	lease := runtimePatchPatchLease{
		FeatureID:  "runtime-patch-1",
		OwnerToken: "current",
		Process:    processInstanceID{PID: 42, Created: 100},
	}
	tests := []struct {
		name    string
		token   string
		process processInstanceID
	}{
		{name: "empty", token: "", process: lease.Process},
		{name: "stale", token: "stale", process: lease.Process},
		{name: "reused PID", token: "current", process: processInstanceID{PID: 42, Created: 101}},
		{name: "different PID", token: "current", process: processInstanceID{PID: 43, Created: 100}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateRuntimePatchOwnedLease(lease, test.token, test.process); err == nil {
				t.Fatal("validateRuntimePatchOwnedLease() error = nil")
			}
		})
	}
}

func TestRuntimePatchPatchConflictAndActiveAddressOverlap(t *testing.T) {
	feature := RuntimePatchFeature{ID: "runtime-patch-new", Conflicts: []string{"runtime-patch-old"}}
	leases := map[string]runtimePatchPatchLease{
		"runtime-patch-old": {
			FeatureID: "runtime-patch-old",
			Sites:     []runtimePatchPatchSiteLease{{Address: 0x2000, Original: []byte{1, 2}, Patch: []byte{3, 4}}},
		},
	}
	if conflict := findRuntimePatchCatalogConflict(feature, leases); conflict != "runtime-patch-old" {
		t.Fatalf("conflict = %q, want runtime-patch-old", conflict)
	}
	if overlap := findRuntimePatchActiveAddressOverlap([]runtimePatchPatchSiteLease{{Address: 0x2001, Original: []byte{2}, Patch: []byte{4}}}, leases, ""); overlap == "" {
		t.Fatal("actual address overlap was not detected")
	}
}

func TestRuntimePatchPatchRestoreOwnedLeasesUsesReverseFeatureAndSiteOrder(t *testing.T) {
	firstSites := []runtimePatchPatchSiteLease{
		{Address: 0x1000, Original: []byte{1}, Patch: []byte{11}},
		{Address: 0x1100, Original: []byte{2}, Patch: []byte{12}},
	}
	secondSites := []runtimePatchPatchSiteLease{
		{Address: 0x2000, Original: []byte{3}, Patch: []byte{13}},
		{Address: 0x2100, Original: []byte{4}, Patch: []byte{14}},
	}
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{
		0x1000: {11}, 0x1100: {12}, 0x2000: {13}, 0x2100: {14},
	})
	process := processInstanceID{PID: 42, Created: 100}
	leases := map[string]runtimePatchPatchLease{
		"first":  {FeatureID: "first", OwnerToken: "owner", Process: process, State: runtimePatchPatchEnabled, Sites: firstSites},
		"second": {FeatureID: "second", OwnerToken: "owner", Process: process, State: runtimePatchPatchEnabled, Sites: secondSites},
	}
	order := []string{"first", "second"}

	if err := restoreAllRuntimePatchPatchLeases(memory, leases, &order, process, "owner"); err != nil {
		t.Fatalf("restoreAllRuntimePatchPatchLeases() error = %v", err)
	}
	if len(leases) != 0 || len(order) != 0 {
		t.Fatalf("successful restore retained leases=%v order=%v", leases, order)
	}
	got := make([]uintptr, 0, len(memory.writes))
	for _, write := range memory.writes {
		got = append(got, write.addr)
	}
	want := []uintptr{0x2100, 0x2000, 0x1100, 0x1000}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("write order = %#x, want %#x", got, want)
	}
}

func TestRuntimePatchPatchRestoreOwnedLeasesRetainsUnprovenLeaseAndOrder(t *testing.T) {
	site := runtimePatchPatchSiteLease{Address: 0x1000, Original: []byte{1}, Patch: []byte{2}}
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{site.Address: {0xCC}})
	process := processInstanceID{PID: 42, Created: 100}
	leases := map[string]runtimePatchPatchLease{
		"feature": {FeatureID: "feature", OwnerToken: "owner", Process: process, State: runtimePatchPatchRecovery, Sites: []runtimePatchPatchSiteLease{site}},
	}
	order := []string{"feature"}

	err := restoreAllRuntimePatchPatchLeases(memory, leases, &order, process, "owner")
	if !errors.Is(err, errLiveMemoryRollbackUnproven) {
		t.Fatalf("restore error = %v, want rollback sentinel", err)
	}
	if _, ok := leases["feature"]; !ok || !reflect.DeepEqual(order, []string{"feature"}) {
		t.Fatalf("unproven restore discarded recovery lease/order: leases=%v order=%v", leases, order)
	}
	if len(memory.writes) != 0 {
		t.Fatalf("foreign bytes were overwritten: %v", memory.writes)
	}
}

func TestRuntimePatchPatchRestoreOwnedLeasesRejectsMissingOrderEntryWithoutIO(t *testing.T) {
	process := processInstanceID{PID: 42, Created: 100}
	lease := runtimePatchPatchLease{
		FeatureID: "feature", OwnerToken: "owner", Process: process, State: runtimePatchPatchRecovery,
		Sites: []runtimePatchPatchSiteLease{{Address: 0x1000, Original: []byte{1}, Patch: []byte{2}}},
	}
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{0x1000: {2}})
	leases := map[string]runtimePatchPatchLease{"feature": lease}
	order := []string{}

	err := restoreAllRuntimePatchPatchLeases(memory, leases, &order, process, "owner")
	if !errors.Is(err, errLiveMemoryRollbackUnproven) {
		t.Fatalf("restore error = %v, want rollback sentinel", err)
	}
	if memory.reads != 0 || len(memory.writes) != 0 {
		t.Fatalf("invalid recovery metadata reached memory: reads=%d writes=%d", memory.reads, len(memory.writes))
	}
	if _, ok := leases["feature"]; !ok || len(order) != 0 {
		t.Fatalf("invalid recovery metadata was discarded: leases=%v order=%v", leases, order)
	}
}

func TestRuntimePatchPatchStatusesUseDefensiveNonNilArraysAndNeverReportForeignAsEnabled(t *testing.T) {
	feature := RuntimePatchFeature{ID: "feature", Sites: []RuntimePatchSite{{EnableBytes: []byte{2}}}}
	process := processInstanceID{PID: 42, Created: 100}
	lease := runtimePatchPatchLease{
		FeatureID: "feature", OwnerToken: "owner", Process: process, State: runtimePatchPatchEnabled,
		Sites: []runtimePatchPatchSiteLease{{Address: 0x140001000, RVA: 0x1000, Original: []byte{1}, Patch: []byte{2}}},
	}
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{0x140001000: {0xCC}})
	statuses := buildRuntimePatchFeatureStatuses(
		[]RuntimePatchFeature{feature}, memory, true, "owner", process,
		map[string]runtimePatchPatchLease{"feature": lease},
	)
	if len(statuses) != 1 {
		t.Fatalf("statuses = %d, want 1", len(statuses))
	}
	status := statuses[0]
	if status.RVAs == nil || status.CurrentBytes == nil {
		t.Fatalf("status arrays must be non-nil: %+v", status)
	}
	if status.Enabled || status.Error == "" {
		t.Fatalf("foreign active bytes reported optimistically: %+v", status)
	}
	status.RVAs[0] = 99
	status.CurrentBytes[0] = "changed"
	if lease.Sites[0].RVA != 0x1000 || memory.data[0x140001000][0] != 0xCC {
		t.Fatal("status result aliases lease or memory storage")
	}

	missing := buildRuntimePatchFeatureStatuses([]RuntimePatchFeature{{ID: "missing"}}, memory, true, "owner", process, nil)[0]
	if missing.RVAs == nil || missing.CurrentBytes == nil || len(missing.RVAs) != 0 || len(missing.CurrentBytes) != 0 {
		t.Fatalf("inactive arrays are not defensive empty arrays: %+v", missing)
	}
}

func TestRuntimePatchPatchStatusesRejectForeignLeaseBeforeMemoryIO(t *testing.T) {
	process := processInstanceID{PID: 42, Created: 100}
	lease := runtimePatchPatchLease{
		FeatureID: "feature", OwnerToken: "other", Process: process, State: runtimePatchPatchEnabled,
		Sites: []runtimePatchPatchSiteLease{{Address: 0x1000, RVA: 0x1000, Original: []byte{1}, Patch: []byte{2}}},
	}
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{0x1000: {2}})

	status := buildRuntimePatchFeatureStatuses(
		[]RuntimePatchFeature{{ID: "feature"}}, memory, true, "presented", process,
		map[string]runtimePatchPatchLease{"feature": lease},
	)[0]
	if status.Error == "" || status.Enabled {
		t.Fatalf("foreign lease status = %+v", status)
	}
	if memory.reads != 0 || len(memory.writes) != 0 {
		t.Fatalf("foreign lease reached memory: reads=%d writes=%d", memory.reads, len(memory.writes))
	}
}

func TestCharaRuntimePatchLeaseBlocksAttachAcquireAndCountsAsActive(t *testing.T) {
	app := &App{
		runtimePatchPatchLeases: map[string]runtimePatchPatchLease{"feature": {FeatureID: "feature", OwnerToken: "owner"}},
		runtimePatchPatchOrder:  []string{"feature"},
	}
	if !app.hasActiveRuntimeHookLeaseLocked() {
		t.Fatal("RuntimePatch recovery lease is not included in active runtime hooks")
	}
	if _, err := app.CharaAttach(); err == nil {
		t.Fatal("CharaAttach rotated connection while RuntimePatch lease existed")
	}
	if _, err := app.CharaAcquire(1); err == nil {
		t.Fatal("CharaAcquire rotated owner while RuntimePatch lease existed")
	}
}

func TestCharaReleaseDeadProcessDropsOnlyPresentedRuntimePatchOwner(t *testing.T) {
	process := processInstanceID{PID: 42, Created: 100}
	app := &App{
		moduleBase:      0x140000000,
		charaPID:        process.PID,
		charaCreated:    process.Created,
		charaOwnerToken: "current",
		runtimePatchPatchLeases: map[string]runtimePatchPatchLease{
			"current-feature": {FeatureID: "current-feature", OwnerToken: "current", Process: process},
			"other-feature":   {FeatureID: "other-feature", OwnerToken: "other", Process: process},
		},
		runtimePatchPatchOrder: []string{"current-feature", "other-feature"},
	}

	if err := app.CharaRelease("current"); err != nil {
		t.Fatal(err)
	}
	if _, ok := app.runtimePatchPatchLeases["current-feature"]; ok {
		t.Fatal("dead-process release retained presented owner's lease")
	}
	if _, ok := app.runtimePatchPatchLeases["other-feature"]; !ok {
		t.Fatal("dead-process release discarded another owner's lease")
	}
	if app.charaOwnerToken != "" || app.moduleBase == 0 {
		t.Fatalf("release owner/process state = %q/%#x", app.charaOwnerToken, app.moduleBase)
	}
}

func TestCharaDetachLockedRestoresRuntimePatchWithoutTakingGlobalLockAgain(t *testing.T) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "app.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body *ast.BlockStmt
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "charaDetachLocked" {
			body = fn.Body
			break
		}
	}
	if body == nil {
		t.Fatal("charaDetachLocked not found")
	}
	if !blockCallsSelector(body, "a", "restoreAllRuntimePatchPatchesLocked") {
		t.Fatal("charaDetachLocked does not restore RuntimePatch leases")
	}
	if blockCallsSelector(body, "liveMemoryWriteMu", "Lock") {
		t.Fatal("charaDetachLocked recursively takes global live-memory lock")
	}
}
