package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type ct084Memory interface {
	ReadCode(addr uintptr, size int) ([]byte, error)
	WriteCode(addr uintptr, data []byte) error
}

type ct084PatchState uint8

const (
	ct084PatchEnabled ct084PatchState = iota + 1
	ct084PatchRecovery
)

type ct084PatchSiteLease struct {
	Address  uintptr
	RVA      uint64
	Original []byte
	Patch    []byte
}

type ct084PatchLease struct {
	FeatureID  string
	OwnerToken string
	Process    processInstanceID
	State      ct084PatchState
	Sites      []ct084PatchSiteLease
}

// CT084FeatureStatus is the live state of one independently implemented CT
// 0.8.4 direct patch. Arrays are always non-nil for stable Wails JSON shapes.
type CT084FeatureStatus struct {
	ID           string   `json:"id"`
	Enabled      bool     `json:"enabled"`
	Available    bool     `json:"available"`
	RVAs         []uint64 `json:"rvas"`
	CurrentBytes []string `json:"currentBytes"`
	Error        string   `json:"error"`
}

func cloneCT084PatchSiteLease(site ct084PatchSiteLease) ct084PatchSiteLease {
	return ct084PatchSiteLease{
		Address:  site.Address,
		RVA:      site.RVA,
		Original: append([]byte(nil), site.Original...),
		Patch:    append([]byte(nil), site.Patch...),
	}
}

func validateCT084OwnedLease(lease ct084PatchLease, token string, current processInstanceID) error {
	if !runtimeOwnerTokenMatches(lease.OwnerToken, token) {
		return errRuntimeOwnerLeaseStale
	}
	if !sameProcessInstance(lease.Process, current) {
		return fmt.Errorf("CT084 patch belongs to a replaced game process")
	}
	return nil
}

func findCT084CatalogConflict(feature CT084Feature, leases map[string]ct084PatchLease) string {
	for _, conflictID := range feature.Conflicts {
		if _, active := leases[conflictID]; active {
			return conflictID
		}
	}
	if feature.ConflictGroup == "" {
		return ""
	}
	catalog, err := loadCT084Catalog()
	if err != nil {
		return ""
	}
	for _, other := range catalog.Features {
		if other.ID == feature.ID || other.ConflictGroup != feature.ConflictGroup {
			continue
		}
		if _, active := leases[other.ID]; active {
			return other.ID
		}
	}
	return ""
}

func findCT084ActiveAddressOverlap(sites []ct084PatchSiteLease, leases map[string]ct084PatchLease, skipFeatureID string) string {
	for _, site := range sites {
		end := site.Address + uintptr(len(site.Original))
		for featureID, lease := range leases {
			if featureID == skipFeatureID {
				continue
			}
			for _, active := range lease.Sites {
				activeEnd := active.Address + uintptr(len(active.Original))
				if site.Address < activeEnd && active.Address < end {
					return featureID
				}
			}
		}
	}
	return ""
}

func validateCT084PatchSiteRanges(sites []ct084PatchSiteLease) error {
	for index, site := range sites {
		if site.Address == 0 || len(site.Original) == 0 || len(site.Original) != len(site.Patch) {
			return fmt.Errorf("CT084 site[%d] has invalid address or byte lengths", index)
		}
		if bytes.Equal(site.Original, site.Patch) {
			return fmt.Errorf("CT084 site[%d] original bytes already equal the enable patch", index)
		}
		end := site.Address + uintptr(len(site.Original))
		if end < site.Address {
			return fmt.Errorf("CT084 site[%d] address range overflows", index)
		}
		for previous := 0; previous < index; previous++ {
			other := sites[previous]
			otherEnd := other.Address + uintptr(len(other.Original))
			if site.Address < otherEnd && other.Address < end {
				return fmt.Errorf("CT084 site[%d] overlaps site[%d]", index, previous)
			}
		}
	}
	return nil
}

// installCT084PatchSites performs a complete read-only preflight before the
// first write, then installs each site atomically. Any later failure restores
// every site already touched in reverse order.
func installCT084PatchSites(memory ct084Memory, sites []ct084PatchSiteLease) error {
	if memory == nil {
		return fmt.Errorf("CT084 memory is nil")
	}
	if len(sites) == 0 {
		return fmt.Errorf("CT084 patch sites are empty")
	}
	if err := validateCT084PatchSiteRanges(sites); err != nil {
		return err
	}
	for index, site := range sites {
		current, err := memory.ReadCode(site.Address, len(site.Original))
		if err != nil {
			return fmt.Errorf("preflight CT084 site[%d]: %w", index, err)
		}
		if !bytes.Equal(current, site.Original) {
			return fmt.Errorf("preflight CT084 site[%d] contains foreign bytes: %s", index, bytesToHex(current))
		}
	}

	for index, site := range sites {
		writer := func(data []byte) error { return memory.WriteCode(site.Address, data) }
		reader := func() ([]byte, error) { return memory.ReadCode(site.Address, len(site.Original)) }
		_, installErr := installCodeHookAtomic(site.Original, site.Patch, writer, reader)
		if installErr == nil {
			continue
		}
		rollbackErr := restoreCT084PatchSites(memory, sites[:index+1])
		return errors.Join(fmt.Errorf("install CT084 site[%d]: %w", index, installErr), rollbackErr)
	}

	for index, site := range sites {
		current, err := memory.ReadCode(site.Address, len(site.Patch))
		if err == nil && bytes.Equal(current, site.Patch) {
			continue
		}
		if err == nil {
			err = fmt.Errorf("current bytes are %s", bytesToHex(current))
		}
		rollbackErr := restoreCT084PatchSites(memory, sites)
		return errors.Join(fmt.Errorf("verify installed CT084 site[%d]: %w", index, err), rollbackErr)
	}
	return nil
}

// restoreCT084PatchSites restores in reverse order and never overwrites bytes
// that are neither this lease's complete patch nor its complete original.
func restoreCT084PatchSites(memory ct084Memory, sites []ct084PatchSiteLease) error {
	if memory == nil {
		return errors.Join(fmt.Errorf("CT084 memory is nil"), errLiveMemoryRollbackUnproven)
	}
	var restoreErr error
	for index := len(sites) - 1; index >= 0; index-- {
		site := sites[index]
		if site.Address == 0 || len(site.Original) == 0 || len(site.Original) != len(site.Patch) {
			restoreErr = errors.Join(restoreErr,
				fmt.Errorf("restore CT084 site[%d]: invalid recovery record", index),
				errLiveMemoryRollbackUnproven)
			continue
		}
		current, err := memory.ReadCode(site.Address, len(site.Original))
		if err != nil {
			restoreErr = errors.Join(restoreErr,
				fmt.Errorf("restore CT084 site[%d] preflight: %w", index, err),
				errLiveMemoryRollbackUnproven)
			continue
		}
		if bytes.Equal(current, site.Original) {
			continue
		}
		if !bytes.Equal(current, site.Patch) {
			restoreErr = errors.Join(restoreErr,
				fmt.Errorf("restore CT084 site[%d] refused foreign bytes: %s", index, bytesToHex(current)),
				errLiveMemoryRollbackUnproven)
			continue
		}

		writer := func(data []byte) error { return memory.WriteCode(site.Address, data) }
		reader := func() ([]byte, error) { return memory.ReadCode(site.Address, len(site.Original)) }
		_, err = installCodeHookAtomic(site.Patch, site.Original, writer, reader)
		if err == nil {
			continue
		}
		// installCodeHookAtomic's canFree result proves its "original" input
		// (the enabled patch), not the restoration target. A fresh read is the
		// only way to prove that this site's original bytes were restored.
		current, proofErr := memory.ReadCode(site.Address, len(site.Original))
		if proofErr == nil && bytes.Equal(current, site.Original) {
			continue
		}
		if proofErr == nil {
			proofErr = fmt.Errorf("current bytes are %s", bytesToHex(current))
		}
		restoreErr = errors.Join(restoreErr,
			fmt.Errorf("restore CT084 site[%d]: %w; proof failed: %v", index, err, proofErr),
			errLiveMemoryRollbackUnproven)
	}
	return restoreErr
}

type ct084ProcessMemory struct{ handle windows.Handle }

func (memory ct084ProcessMemory) ReadCode(addr uintptr, size int) ([]byte, error) {
	if memory.handle == 0 || addr == 0 || size <= 0 {
		return nil, fmt.Errorf("invalid CT084 process memory read")
	}
	buf := make([]byte, size)
	if err := readProcessMemory(memory.handle, addr, unsafe.Pointer(&buf[0]), uintptr(size)); err != nil {
		return nil, err
	}
	return buf, nil
}

func (memory ct084ProcessMemory) WriteCode(addr uintptr, data []byte) error {
	if memory.handle == 0 || addr == 0 || len(data) == 0 {
		return fmt.Errorf("invalid CT084 process memory write")
	}
	return writeCodeMemory(memory.handle, addr, data)
}

func removeCT084PatchOrderID(order []string, id string) []string {
	filtered := order[:0]
	for _, current := range order {
		if current != id {
			filtered = append(filtered, current)
		}
	}
	return filtered
}

// restoreAllCT084PatchLeases restores feature order in reverse; the site
// helper independently restores each feature's sites in reverse. Failed
// records remain in both map and order for a later recovery attempt.
func restoreAllCT084PatchLeases(memory ct084Memory, leases map[string]ct084PatchLease, order *[]string, process processInstanceID, owner string) error {
	if order == nil {
		return errors.Join(fmt.Errorf("CT084 patch order is nil"), errLiveMemoryRollbackUnproven)
	}
	seen := make(map[string]struct{}, len(*order))
	for _, id := range *order {
		if _, exists := leases[id]; !exists {
			return errors.Join(fmt.Errorf("CT084 patch order contains unknown feature %s", id), errLiveMemoryRollbackUnproven)
		}
		if _, duplicate := seen[id]; duplicate {
			return errors.Join(fmt.Errorf("CT084 patch order contains duplicate feature %s", id), errLiveMemoryRollbackUnproven)
		}
		seen[id] = struct{}{}
	}
	for id := range leases {
		if _, ordered := seen[id]; !ordered {
			return errors.Join(fmt.Errorf("CT084 recovery lease %s is missing from patch order", id), errLiveMemoryRollbackUnproven)
		}
	}
	var joined error
	for index := len(*order) - 1; index >= 0; index-- {
		id := (*order)[index]
		lease, exists := leases[id]
		if !exists || (owner != "" && lease.OwnerToken != owner) {
			continue
		}
		if !sameProcessInstance(lease.Process, process) {
			joined = errors.Join(joined,
				fmt.Errorf("CT084 %s belongs to a different process instance", id),
				errLiveMemoryRollbackUnproven)
			continue
		}
		lease.State = ct084PatchRecovery
		leases[id] = lease
		if err := restoreCT084PatchSites(memory, lease.Sites); err != nil {
			joined = errors.Join(joined, fmt.Errorf("restore CT084 %s: %w", id, err))
			continue
		}
		delete(leases, id)
		*order = removeCT084PatchOrderID(*order, id)
	}
	return joined
}

func appendCT084StatusError(current, next string) string {
	if current == "" {
		return next
	}
	return current + "; " + next
}

func buildCT084FeatureStatuses(features []CT084Feature, memory ct084Memory, connected bool, owner string, process processInstanceID, leases map[string]ct084PatchLease) []CT084FeatureStatus {
	statuses := make([]CT084FeatureStatus, 0, len(features))
	for _, feature := range features {
		status := CT084FeatureStatus{
			ID:           feature.ID,
			Available:    connected,
			RVAs:         make([]uint64, 0),
			CurrentBytes: make([]string, 0),
		}
		lease, active := leases[feature.ID]
		if !active {
			if conflict := findCT084CatalogConflict(feature, leases); conflict != "" {
				status.Available = false
				status.Error = fmt.Sprintf("conflicts with active CT084 feature %s", conflict)
			}
			statuses = append(statuses, status)
			continue
		}

		status.RVAs = make([]uint64, len(lease.Sites))
		status.CurrentBytes = make([]string, len(lease.Sites))
		for index, site := range lease.Sites {
			status.RVAs[index] = site.RVA
		}
		if err := validateCT084OwnedLease(lease, owner, process); err != nil {
			status.Available = false
			status.Error = err.Error()
			statuses = append(statuses, status)
			continue
		}
		if lease.State != ct084PatchEnabled {
			status.Error = appendCT084StatusError(status.Error, "recovery is required")
		}
		allPatched := len(lease.Sites) != 0
		for index, site := range lease.Sites {
			current, err := memory.ReadCode(site.Address, len(site.Patch))
			if err != nil {
				allPatched = false
				status.Error = appendCT084StatusError(status.Error, fmt.Sprintf("site[%d] read failed: %v", index, err))
				continue
			}
			status.CurrentBytes[index] = bytesToHex(current)
			if !bytes.Equal(current, site.Patch) {
				allPatched = false
				status.Error = appendCT084StatusError(status.Error, fmt.Sprintf("site[%d] contains foreign or restored bytes", index))
			}
		}
		status.Enabled = lease.State == ct084PatchEnabled && allPatched && status.Error == ""
		statuses = append(statuses, status)
	}
	return statuses
}

func findCT084CatalogFeature(catalog *CT084Catalog, id string) *CT084Feature {
	if catalog == nil {
		return nil
	}
	for index := range catalog.Features {
		if catalog.Features[index].ID == strings.TrimSpace(id) {
			return &catalog.Features[index]
		}
	}
	return nil
}

func cloneCT084PatchLease(lease ct084PatchLease) ct084PatchLease {
	cloned := lease
	cloned.Sites = make([]ct084PatchSiteLease, len(lease.Sites))
	for index, site := range lease.Sites {
		cloned.Sites[index] = cloneCT084PatchSiteLease(site)
	}
	return cloned
}

// restoreAllCT084PatchesLocked is called with procMu and runtimePatchMu held.
// It deliberately does not acquire liveMemoryWriteMu because charaDetachLocked
// already runs inside the process lifecycle critical section.
func (a *App) restoreAllCT084PatchesLocked(owner string) error {
	if len(a.ct084PatchLeases) == 0 && len(a.ct084PatchOrder) == 0 {
		return nil
	}
	err := restoreAllCT084PatchLeases(
		ct084ProcessMemory{handle: a.hProcess},
		a.ct084PatchLeases,
		&a.ct084PatchOrder,
		a.currentProcessInstance(),
		owner,
	)
	if len(a.ct084PatchLeases) == 0 {
		a.ct084PatchLeases = nil
		a.ct084PatchOrder = nil
	}
	if errors.Is(err, errLiveMemoryRollbackUnproven) {
		a.poisonCurrentLiveMemoryWrites()
	}
	return err
}

func (a *App) dropCT084PatchesForOwnerLocked(owner string) {
	if owner == "" {
		return
	}
	for id, lease := range a.ct084PatchLeases {
		if lease.OwnerToken == owner {
			delete(a.ct084PatchLeases, id)
			a.ct084PatchOrder = removeCT084PatchOrderID(a.ct084PatchOrder, id)
		}
	}
	if len(a.ct084PatchLeases) == 0 {
		a.ct084PatchLeases = nil
		a.ct084PatchOrder = nil
	}
}

func (a *App) validateCT084StatusOwnerLocked(token string) error {
	if !runtimeOwnerTokenMatches(a.charaOwnerToken, token) {
		return errRuntimeOwnerLeaseStale
	}
	if a.hProcess == 0 || a.moduleBase == 0 || a.charaPID == 0 || a.charaCreated == 0 || !processHandleAlive(a.hProcess) {
		return fmt.Errorf("game process connection is no longer live")
	}
	return nil
}

// CT084GetStatusesOwned reads only already-owned sites. Inactive features are
// never pattern-scanned while serving a status request.
func (a *App) CT084GetStatusesOwned(token string) ([]CT084FeatureStatus, error) {
	a.procMu.Lock()
	defer a.procMu.Unlock()
	if err := a.validateCT084StatusOwnerLocked(token); err != nil {
		return nil, err
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	catalog, err := loadCT084Catalog()
	if err != nil {
		return nil, err
	}
	return buildCT084FeatureStatuses(
		catalog.Features,
		ct084ProcessMemory{handle: a.hProcess},
		true,
		token,
		a.currentProcessInstance(),
		a.ct084PatchLeases,
	), nil
}

func prepareCT084PatchSiteLease(moduleBase, moduleEnd, address uintptr, definition CT084PatchSite, original []byte) (ct084PatchSiteLease, error) {
	if moduleBase == 0 || moduleEnd <= moduleBase || address < moduleBase || len(definition.EnableBytes) == 0 {
		return ct084PatchSiteLease{}, fmt.Errorf("CT084 patch site has invalid module bounds, address, or patch bytes")
	}
	end := address + uintptr(len(definition.EnableBytes))
	if end < address || end > moduleEnd {
		return ct084PatchSiteLease{}, fmt.Errorf("CT084 patch site target is outside module bounds")
	}
	if len(original) != len(definition.EnableBytes) {
		return ct084PatchSiteLease{}, fmt.Errorf("CT084 captured original length does not match enable patch")
	}
	if len(definition.DisableBytes) != 0 && !bytes.Equal(original, definition.DisableBytes) {
		return ct084PatchSiteLease{}, fmt.Errorf("CT084 explicit disable bytes do not match runtime bytes")
	}
	if bytes.Equal(original, definition.EnableBytes) {
		return ct084PatchSiteLease{}, fmt.Errorf("CT084 runtime bytes already equal the enable patch; ownership cannot be claimed")
	}
	return ct084PatchSiteLease{
		Address:  address,
		RVA:      uint64(address - moduleBase),
		Original: append([]byte(nil), original...),
		Patch:    append([]byte(nil), definition.EnableBytes...),
	}, nil
}

func (a *App) prepareCT084PatchSitesLocked(feature CT084Feature, memory ct084Memory) ([]ct084PatchSiteLease, error) {
	moduleSize, err := getRemoteModuleSize(a.hProcess, a.moduleBase)
	if err != nil {
		return nil, fmt.Errorf("read CT084 module bounds: %w", err)
	}
	moduleEnd := a.moduleBase + moduleSize
	if moduleEnd < a.moduleBase {
		return nil, fmt.Errorf("CT084 module range overflows")
	}
	sites := make([]ct084PatchSiteLease, 0, len(feature.Sites))
	matchBySignature := make(map[string]uintptr, len(feature.Sites))
	for index, definition := range feature.Sites {
		signatureKey := definition.Module + "\x00" + definition.Symbol + "\x00" + definition.AOB
		match, cached := matchBySignature[signatureKey]
		if !cached {
			pattern := ct084Pattern{
				Values: append([]byte(nil), definition.PatternValues...),
				Mask:   append([]byte(nil), definition.PatternMasks...),
			}
			match, err = a.scanCT084PatternUnique(pattern, fmt.Sprintf("%s site[%d] %s", feature.ID, index, definition.Symbol))
			if err != nil {
				return nil, err
			}
			matchBySignature[signatureKey] = match
		}
		offset := uintptr(definition.Offset)
		address := match + offset
		if address < match {
			return nil, fmt.Errorf("%s site[%d] target address overflows", feature.ID, index)
		}
		end := address + uintptr(len(definition.EnableBytes))
		if end < address || address < a.moduleBase || end > moduleEnd {
			return nil, fmt.Errorf("%s site[%d] target is outside module bounds", feature.ID, index)
		}
		original, err := memory.ReadCode(address, len(definition.EnableBytes))
		if err != nil {
			return nil, fmt.Errorf("read %s site[%d] original bytes: %w", feature.ID, index, err)
		}
		lease, err := prepareCT084PatchSiteLease(a.moduleBase, moduleEnd, address, definition, original)
		if err != nil {
			return nil, fmt.Errorf("prepare %s site[%d]: %w", feature.ID, index, err)
		}
		sites = append(sites, lease)
	}
	if err := validateCT084PatchSiteRanges(sites); err != nil {
		return nil, err
	}
	return sites, nil
}

func (a *App) ct084StatusForFeatureLocked(feature CT084Feature, token string, memory ct084Memory) CT084FeatureStatus {
	return buildCT084FeatureStatuses(
		[]CT084Feature{feature}, memory, true, token,
		a.currentProcessInstance(), a.ct084PatchLeases,
	)[0]
}

// CT084SetEnabledOwned atomically enables or restores every site belonging to
// a catalog feature while holding global -> process -> runtime patch locks.
func (a *App) CT084SetEnabledOwned(token, id string, enabled bool) (CT084FeatureStatus, error) {
	empty := CT084FeatureStatus{
		ID:           strings.TrimSpace(id),
		RVAs:         make([]uint64, 0),
		CurrentBytes: make([]string, 0),
	}
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return empty, err
	}
	defer a.procMu.Unlock()
	if enabled {
		if err := a.ensureLiveMemoryWritesSafe(); err != nil {
			return empty, err
		}
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()

	catalog, err := loadCT084Catalog()
	if err != nil {
		return empty, err
	}
	feature := findCT084CatalogFeature(catalog, id)
	if feature == nil {
		return empty, fmt.Errorf("unknown CT084 feature: %s", strings.TrimSpace(id))
	}
	memory := ct084ProcessMemory{handle: a.hProcess}
	process := a.currentProcessInstance()
	lease, active := a.ct084PatchLeases[feature.ID]

	if !enabled {
		if !active {
			return a.ct084StatusForFeatureLocked(*feature, token, memory), nil
		}
		if err := validateCT084OwnedLease(lease, token, process); err != nil {
			return a.ct084StatusForFeatureLocked(*feature, token, memory), err
		}
		lease.State = ct084PatchRecovery
		a.ct084PatchLeases[feature.ID] = cloneCT084PatchLease(lease)
		if err := restoreCT084PatchSites(memory, lease.Sites); err != nil {
			if errors.Is(err, errLiveMemoryRollbackUnproven) {
				a.poisonCurrentLiveMemoryWrites()
			}
			return a.ct084StatusForFeatureLocked(*feature, token, memory), err
		}
		delete(a.ct084PatchLeases, feature.ID)
		a.ct084PatchOrder = removeCT084PatchOrderID(a.ct084PatchOrder, feature.ID)
		if len(a.ct084PatchLeases) == 0 {
			a.ct084PatchLeases = nil
			a.ct084PatchOrder = nil
		}
		return a.ct084StatusForFeatureLocked(*feature, token, memory), nil
	}

	if active {
		if err := validateCT084OwnedLease(lease, token, process); err != nil {
			return a.ct084StatusForFeatureLocked(*feature, token, memory), err
		}
		status := a.ct084StatusForFeatureLocked(*feature, token, memory)
		if lease.State == ct084PatchEnabled && status.Enabled {
			return status, nil
		}
		lease.State = ct084PatchRecovery
		a.ct084PatchLeases[feature.ID] = cloneCT084PatchLease(lease)
		a.poisonCurrentLiveMemoryWrites()
		return status, errors.Join(fmt.Errorf("CT084 feature requires recovery before it can be enabled again"), errLiveMemoryRollbackUnproven)
	}
	for activeID, activeLease := range a.ct084PatchLeases {
		if !runtimeOwnerTokenMatches(activeLease.OwnerToken, token) {
			return a.ct084StatusForFeatureLocked(*feature, token, memory), fmt.Errorf("CT084 feature %s is owned by another page", activeID)
		}
		if !sameProcessInstance(activeLease.Process, process) {
			return a.ct084StatusForFeatureLocked(*feature, token, memory), fmt.Errorf("CT084 feature %s belongs to a replaced game process", activeID)
		}
		if activeLease.State != ct084PatchEnabled {
			return a.ct084StatusForFeatureLocked(*feature, token, memory), fmt.Errorf("CT084 feature %s requires recovery before another feature can be enabled", activeID)
		}
	}
	if conflict := findCT084CatalogConflict(*feature, a.ct084PatchLeases); conflict != "" {
		return a.ct084StatusForFeatureLocked(*feature, token, memory), fmt.Errorf("CT084 feature conflicts with active feature %s", conflict)
	}

	sites, err := a.prepareCT084PatchSitesLocked(*feature, memory)
	if err != nil {
		return a.ct084StatusForFeatureLocked(*feature, token, memory), err
	}
	if overlap := findCT084ActiveAddressOverlap(sites, a.ct084PatchLeases, feature.ID); overlap != "" {
		return a.ct084StatusForFeatureLocked(*feature, token, memory), fmt.Errorf("CT084 feature overlaps active feature %s", overlap)
	}
	if a.ct084PatchLeases == nil {
		a.ct084PatchLeases = make(map[string]ct084PatchLease)
	}
	candidate := ct084PatchLease{
		FeatureID:  feature.ID,
		OwnerToken: token,
		Process:    process,
		State:      ct084PatchRecovery,
		Sites:      sites,
	}
	a.ct084PatchLeases[feature.ID] = cloneCT084PatchLease(candidate)
	a.ct084PatchOrder = append(a.ct084PatchOrder, feature.ID)
	if err := installCT084PatchSites(memory, sites); err != nil {
		if errors.Is(err, errLiveMemoryRollbackUnproven) {
			a.poisonCurrentLiveMemoryWrites()
		} else {
			delete(a.ct084PatchLeases, feature.ID)
			a.ct084PatchOrder = removeCT084PatchOrderID(a.ct084PatchOrder, feature.ID)
			if len(a.ct084PatchLeases) == 0 {
				a.ct084PatchLeases = nil
				a.ct084PatchOrder = nil
			}
		}
		return a.ct084StatusForFeatureLocked(*feature, token, memory), err
	}
	candidate.State = ct084PatchEnabled
	a.ct084PatchLeases[feature.ID] = cloneCT084PatchLease(candidate)
	return a.ct084StatusForFeatureLocked(*feature, token, memory), nil
}

// CT084ReleaseOwned restores all patches owned by the presented Chara token.
// Poison never prevents this recovery path.
func (a *App) CT084ReleaseOwned(token string) error {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	a.procMu.Lock()
	defer a.procMu.Unlock()
	if strings.TrimSpace(token) == "" {
		return nil
	}
	hasOwnedLease := false
	for _, lease := range a.ct084PatchLeases {
		if lease.OwnerToken == token {
			hasOwnedLease = true
			break
		}
	}
	if !hasOwnedLease {
		return nil
	}
	if a.hProcess == 0 || !processHandleAlive(a.hProcess) {
		a.dropCT084PatchesForOwnerLocked(token)
		return nil
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	return a.restoreAllCT084PatchesLocked(token)
}
