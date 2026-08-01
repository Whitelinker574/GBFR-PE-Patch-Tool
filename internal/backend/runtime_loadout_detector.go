package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	runtimeLoadoutDetectorHistoryVersion = 1
	runtimeLoadoutDetectorPollInterval   = 2500 * time.Millisecond
	runtimeLoadoutDetectorStablePolls    = 2
	runtimeLoadoutDetectorAbsentPolls    = 2
	runtimeLoadoutDetectorMaximumHistory = 500
	runtimeLoadoutDetectorStatusEvent    = "runtime-loadout-detector:status"
)

type RuntimeLoadoutDetectorMember struct {
	Role          string                   `json:"role"`
	CharacterHash string                   `json:"characterHash"`
	CharacterName string                   `json:"characterName"`
	Loadout       RuntimePatchPartyLoadout `json:"loadout"`
}

type RuntimeLoadoutDetectorRecord struct {
	ID         string                         `json:"id"`
	Sequence   int                            `json:"sequence"`
	CapturedAt int64                          `json:"capturedAt"`
	SessionID  string                         `json:"sessionId"`
	Members    []RuntimeLoadoutDetectorMember `json:"members"`
}

type RuntimeLoadoutDetectorStatus struct {
	Enabled         bool   `json:"enabled"`
	State           string `json:"state"`
	StartedAt       int64  `json:"startedAt"`
	LastPollAt      int64  `json:"lastPollAt"`
	LastCaptureAt   int64  `json:"lastCaptureAt"`
	CurrentTeamSize int    `json:"currentTeamSize"`
	SessionCaptured int    `json:"sessionCaptured"`
	HistoryCount    int    `json:"historyCount"`
	LastError       string `json:"lastError,omitempty"`
}

type runtimeLoadoutDetectorHistory struct {
	Version           int                            `json:"version"`
	ActiveFingerprint string                         `json:"activeFingerprint,omitempty"`
	Records           []RuntimeLoadoutDetectorRecord `json:"records"`
}

type runtimeLoadoutDetectorSession struct {
	mu      sync.Mutex
	app     *App
	process *readOnlyGameProcess
	cancel  context.CancelFunc
	done    chan struct{}

	historyPath string
	history     []RuntimeLoadoutDetectorRecord
	startedAt   time.Time
	sessionID   string
	state       string
	lastPollAt  time.Time
	lastCapture time.Time
	lastError   string

	pendingFingerprint  string
	pendingMembers      []RuntimeLoadoutDetectorMember
	pendingPolls        int
	activeFingerprint   string
	restoredFingerprint string
	activeRecordID      string
	absentPolls         int
	currentTeamSize     int
	sessionCaptured     int
	nextSequence        int
	networkSequence     uint64
	networkTracker      *runtimePartyNetworkProfileTracker
	emitStatus          func(RuntimeLoadoutDetectorStatus)
}

func (a *App) runtimeLoadoutDetectorHistoryPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "gbfr-player-info-edit", "runtime-loadout-history.json"), nil
}

func loadRuntimeLoadoutDetectorHistory(path string) ([]RuntimeLoadoutDetectorRecord, int, string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 1, "", nil
		}
		return nil, 0, "", err
	}
	var stored runtimeLoadoutDetectorHistory
	if err := json.Unmarshal(payload, &stored); err != nil {
		return nil, 0, "", fmt.Errorf("解析本地配装检测记录失败: %w", err)
	}
	if stored.Version != runtimeLoadoutDetectorHistoryVersion {
		return nil, 0, "", fmt.Errorf("不支持的本地配装检测记录版本 %d", stored.Version)
	}
	next := 1
	for _, record := range stored.Records {
		if record.Sequence >= next {
			next = record.Sequence + 1
		}
	}
	stored.Records = boundedRuntimeLoadoutDetectorHistory(stored.Records)
	for recordIndex := range stored.Records {
		for memberIndex := range stored.Records[recordIndex].Members {
			loadout := &stored.Records[recordIndex].Members[memberIndex].Loadout
			loadout.CombinedSkills = runtimePatchPartyCombinedSkills(*loadout)
		}
	}
	return stored.Records, next, stored.ActiveFingerprint, nil
}

func boundedRuntimeLoadoutDetectorHistory(records []RuntimeLoadoutDetectorRecord) []RuntimeLoadoutDetectorRecord {
	if len(records) <= runtimeLoadoutDetectorMaximumHistory {
		return records
	}
	start := len(records) - runtimeLoadoutDetectorMaximumHistory
	trimmed := make([]RuntimeLoadoutDetectorRecord, runtimeLoadoutDetectorMaximumHistory)
	copy(trimmed, records[start:])
	return trimmed
}

func runtimeLoadoutDetectorLastCapture(records []RuntimeLoadoutDetectorRecord) time.Time {
	var latest time.Time
	for _, record := range records {
		if record.CapturedAt <= 0 {
			continue
		}
		capturedAt := time.UnixMilli(record.CapturedAt)
		if latest.IsZero() || capturedAt.After(latest) {
			latest = capturedAt
		}
	}
	return latest
}

func runtimeLoadoutDetectorUnixMilli(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

func saveRuntimeLoadoutDetectorHistory(path string, records []RuntimeLoadoutDetectorRecord, activeFingerprint string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(runtimeLoadoutDetectorHistory{
		Version: runtimeLoadoutDetectorHistoryVersion, ActiveFingerprint: activeFingerprint,
		Records: boundedRuntimeLoadoutDetectorHistory(records),
	}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("创建本地配装检测记录临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	replaced := false
	defer func() {
		_ = tmp.Close()
		if !replaced {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(payload); err != nil {
		return fmt.Errorf("写入本地配装检测记录临时文件失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("同步本地配装检测记录临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭本地配装检测记录临时文件失败: %w", err)
	}
	if err := replaceFileAtomic(tmpPath, path); err != nil {
		return fmt.Errorf("保存本地配装检测记录失败: %w", err)
	}
	replaced = true
	return nil
}

func (a *App) startRuntimeLoadoutDetector(persistPreference bool) (RuntimeLoadoutDetectorStatus, error) {
	a.runtimeLoadoutDetectorMu.Lock()
	defer a.runtimeLoadoutDetectorMu.Unlock()
	if a.runtimeLoadoutDetector != nil {
		return a.runtimeLoadoutDetector.status(), nil
	}
	historyPath, err := a.runtimeLoadoutDetectorHistoryPath()
	if err != nil {
		return RuntimeLoadoutDetectorStatus{}, err
	}
	history, nextSequence, activeFingerprint, err := loadRuntimeLoadoutDetectorHistory(historyPath)
	if err != nil {
		return RuntimeLoadoutDetectorStatus{}, err
	}
	if err := writeRuntimePartyObserverConfig(true); err != nil {
		return RuntimeLoadoutDetectorStatus{}, err
	}
	startedAt := time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	session := &runtimeLoadoutDetectorSession{
		app: a, networkTracker: newRuntimePartyNetworkProfileTracker(),
		cancel: cancel, done: make(chan struct{}), historyPath: historyPath, history: history,
		startedAt: startedAt, sessionID: startedAt.Format("20060102-150405"), state: "waiting_game",
		lastCapture: runtimeLoadoutDetectorLastCapture(history), nextSequence: nextSequence,
		restoredFingerprint: activeFingerprint,
	}
	if a.ctx != nil {
		session.emitStatus = func(status RuntimeLoadoutDetectorStatus) {
			runtime.EventsEmit(a.ctx, runtimeLoadoutDetectorStatusEvent, status)
		}
	}
	if activeFingerprint != "" && len(history) > 0 {
		session.activeRecordID = history[len(history)-1].ID
	}
	a.runtimeLoadoutDetector = session
	if persistPreference {
		if err := a.setRuntimeLoadoutDetectorPreference(true); err != nil {
			a.runtimeLoadoutDetector = nil
			cancel()
			_ = writeRuntimePartyObserverConfig(false)
			return RuntimeLoadoutDetectorStatus{}, err
		}
	}
	go session.run(ctx)
	return session.status(), nil
}

func (a *App) RuntimeLoadoutDetectorStart() (RuntimeLoadoutDetectorStatus, error) {
	return a.startRuntimeLoadoutDetector(true)
}

func (a *App) closeRuntimeLoadoutDetector(clearPreference bool) error {
	a.runtimeLoadoutDetectorMu.Lock()
	session := a.runtimeLoadoutDetector
	a.runtimeLoadoutDetector = nil
	a.runtimeLoadoutDetectorMu.Unlock()
	var closeErr error
	if session != nil {
		closeErr = session.close(clearPreference)
		closeErr = errors.Join(closeErr, a.stopOwnedRuntimeCompanion("party-observer", func() error {
			return writeRuntimePartyObserverConfig(false)
		}))
	}
	if clearPreference {
		return errors.Join(closeErr, a.setRuntimeLoadoutDetectorPreference(false))
	}
	return closeErr
}

func (a *App) setRuntimeLoadoutDetectorPreference(active bool) error {
	return a.updateConfig(func(config *AppConfig) { config.RuntimeLoadoutDetectorActive = active })
}

func (a *App) RuntimeLoadoutDetectorStop() (RuntimeLoadoutDetectorStatus, error) {
	if err := a.closeRuntimeLoadoutDetector(true); err != nil {
		return RuntimeLoadoutDetectorStatus{}, err
	}
	records, _, _, err := loadRuntimeLoadoutDetectorHistoryFromApp(a)
	if err != nil {
		return RuntimeLoadoutDetectorStatus{}, err
	}
	status := RuntimeLoadoutDetectorStatus{State: "stopped", HistoryCount: len(records)}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, runtimeLoadoutDetectorStatusEvent, status)
	}
	return status, nil
}

func loadRuntimeLoadoutDetectorHistoryFromApp(a *App) ([]RuntimeLoadoutDetectorRecord, int, string, error) {
	path, err := a.runtimeLoadoutDetectorHistoryPath()
	if err != nil {
		return nil, 0, "", err
	}
	return loadRuntimeLoadoutDetectorHistory(path)
}

func (a *App) RuntimeLoadoutDetectorStatus() (RuntimeLoadoutDetectorStatus, error) {
	a.runtimeLoadoutDetectorMu.Lock()
	session := a.runtimeLoadoutDetector
	a.runtimeLoadoutDetectorMu.Unlock()
	if session != nil {
		return session.status(), nil
	}
	records, _, _, err := loadRuntimeLoadoutDetectorHistoryFromApp(a)
	if err != nil {
		return RuntimeLoadoutDetectorStatus{}, err
	}
	return RuntimeLoadoutDetectorStatus{State: "stopped", HistoryCount: len(records)}, nil
}

func (a *App) RuntimeLoadoutDetectorHistory() ([]RuntimeLoadoutDetectorRecord, error) {
	a.runtimeLoadoutDetectorMu.Lock()
	session := a.runtimeLoadoutDetector
	a.runtimeLoadoutDetectorMu.Unlock()
	var records []RuntimeLoadoutDetectorRecord
	if session != nil {
		records = session.records()
	} else {
		var err error
		records, _, _, err = loadRuntimeLoadoutDetectorHistoryFromApp(a)
		if err != nil {
			return nil, err
		}
	}
	sort.SliceStable(records, func(left, right int) bool {
		return records[left].CapturedAt > records[right].CapturedAt
	})
	return records, nil
}

func (session *runtimeLoadoutDetectorSession) run(ctx context.Context) {
	defer close(session.done)
	session.tick()
	session.publishStatus()
	ticker := time.NewTicker(runtimeLoadoutDetectorPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			session.mu.Lock()
			if session.process != nil {
				_ = session.process.Close()
				session.process = nil
			}
			session.mu.Unlock()
			return
		case <-ticker.C:
			session.tick()
			session.publishStatus()
		}
	}
}

func (session *runtimeLoadoutDetectorSession) publishStatus() {
	if session.emitStatus != nil {
		session.emitStatus(session.status())
	}
}

func (session *runtimeLoadoutDetectorSession) close(clearActiveFingerprint bool) error {
	if session == nil {
		return nil
	}
	session.cancel()
	<-session.done
	if !clearActiveFingerprint {
		return nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	session.activeFingerprint = ""
	session.restoredFingerprint = ""
	session.activeRecordID = ""
	return saveRuntimeLoadoutDetectorHistory(session.historyPath, session.history, "")
}

func (session *runtimeLoadoutDetectorSession) status() RuntimeLoadoutDetectorStatus {
	session.mu.Lock()
	defer session.mu.Unlock()
	return RuntimeLoadoutDetectorStatus{
		Enabled: true, State: session.state, StartedAt: session.startedAt.UnixMilli(),
		LastPollAt:      runtimeLoadoutDetectorUnixMilli(session.lastPollAt),
		LastCaptureAt:   runtimeLoadoutDetectorUnixMilli(session.lastCapture),
		CurrentTeamSize: session.currentTeamSize, SessionCaptured: session.sessionCaptured,
		HistoryCount: len(session.history), LastError: session.lastError,
	}
}

func (session *runtimeLoadoutDetectorSession) records() []RuntimeLoadoutDetectorRecord {
	session.mu.Lock()
	defer session.mu.Unlock()
	return append([]RuntimeLoadoutDetectorRecord(nil), session.history...)
}

func (session *runtimeLoadoutDetectorSession) tick() {
	observerErr := session.ensurePartyObserver()
	session.mu.Lock()
	defer session.mu.Unlock()
	session.lastPollAt = time.Now()
	var networkMembers []RuntimeLoadoutDetectorMember
	if observerErr == nil {
		events, nextSequence, err := readRuntimePartyObserverEvents(session.networkSequence)
		if err != nil {
			observerErr = err
		} else {
			session.networkSequence = nextSequence
			networkMembers, observerErr = session.observePartyNetworkEvents(events)
		}
	}
	var memoryMembers []RuntimeLoadoutDetectorMember
	var memoryErr error
	if session.process == nil {
		process, err := openReadOnlyGameProcessForLayouts(windowsReadOnlyProcessBackend{}, charaProcessName, runtimeCharacterPanelRuntimeLayouts)
		if err != nil {
			memoryErr = err
		} else {
			session.process = process
		}
	}
	if session.process != nil {
		if err := session.process.Validate(); err != nil {
			memoryErr = err
		}
	}
	if memoryErr != nil && session.process != nil {
		_ = session.process.Close()
		session.process = nil
	}
	if session.process != nil {
		snapshot, err := readStableRuntimePatchPartySnapshots(func() (runtimePatchPartySnapshot, error) {
			return readRuntimePatchPartySnapshot(session.process, session.process.moduleBase)
		})
		if err != nil {
			memoryErr = err
		} else {
			memoryMembers = runtimeLoadoutDetectorMembers(snapshot)
		}
	}
	members := mergeRuntimeLoadoutDetectorMembers(memoryMembers, networkMembers)
	if len(members) == 0 {
		if memoryErr != nil && session.process == nil {
			session.state = "waiting_game"
		} else {
			session.state = "waiting_task"
		}
		session.lastError = firstRuntimeLoadoutDetectorError(observerErr, memoryErr)
		session.observeAbsent()
		return
	}
	fingerprint, err := runtimeLoadoutDetectorFingerprint(members)
	if err != nil {
		session.state = "waiting_task"
		session.lastError = err.Error()
		return
	}
	session.lastError = firstRuntimeLoadoutDetectorError(observerErr, nil)
	session.observeTeam(fingerprint, members)
}

func firstRuntimeLoadoutDetectorError(errors ...error) string {
	for _, err := range errors {
		if err != nil {
			return err.Error()
		}
	}
	return ""
}

func (session *runtimeLoadoutDetectorSession) ensurePartyObserver() error {
	if session == nil || session.app == nil {
		return nil
	}
	process, err := findRuntimeProcessInstance()
	if err != nil {
		return err
	}
	if session.app.runtimeCompanionActive("party-observer") && runtimeCompanionMatchesProcess(readRuntimeCompanionStatus("party-observer"), process) {
		return nil
	}
	return session.app.startRuntimeCompanion("party-observer", "runtime_party_observer")
}

func (session *runtimeLoadoutDetectorSession) observePartyNetworkEvents(events []runtimePartyObserverEvent) ([]RuntimeLoadoutDetectorMember, error) {
	if session.networkTracker == nil {
		session.networkTracker = newRuntimePartyNetworkProfileTracker()
	}
	for _, event := range events {
		if event.Kind == runtimePartyObserverResetKind {
			session.networkTracker.Reset()
			continue
		}
		if _, _, err := session.networkTracker.Observe(event.Direction, event.Payload); err != nil {
			return nil, err
		}
	}
	return runtimeLoadoutDetectorNetworkMembers(session.networkTracker.StableRemoteProfiles())
}

func runtimeLoadoutDetectorNetworkMembers(profiles []runtimePartyNetworkProfile) ([]RuntimeLoadoutDetectorMember, error) {
	members := make([]RuntimeLoadoutDetectorMember, 0, len(profiles))
	for _, profile := range profiles {
		loadout, err := runtimePartyNetworkProfileLoadout(profile)
		if err != nil {
			return nil, err
		}
		members = append(members, RuntimeLoadoutDetectorMember{
			Role: fmt.Sprintf("party%d", profile.PartyIndex), CharacterHash: loadout.CharacterHash,
			CharacterName: loadout.CharacterName, Loadout: loadout,
		})
	}
	return members, nil
}

func runtimeLoadoutDetectorCoreMatches(left, right RuntimePatchPartyLoadout) bool {
	if !strings.EqualFold(left.CharacterHash, right.CharacterHash) || left.Weapon.Hash != right.Weapon.Hash || len(left.Sigils) != len(right.Sigils) {
		return false
	}
	leftByIndex := make(map[int]RuntimePatchPartySigil, len(left.Sigils))
	for _, sigil := range left.Sigils {
		leftByIndex[sigil.Index] = sigil
	}
	for _, expected := range right.Sigils {
		actual, ok := leftByIndex[expected.Index]
		if !ok || actual.Hash != expected.Hash || actual.SecondaryTraitHash != expected.SecondaryTraitHash || actual.Level != expected.Level {
			return false
		}
	}
	return true
}

func mergeRuntimeLoadoutDetectorMembers(memoryMembers, networkMembers []RuntimeLoadoutDetectorMember) []RuntimeLoadoutDetectorMember {
	if len(networkMembers) == 0 {
		return memoryMembers
	}
	result := make([]RuntimeLoadoutDetectorMember, 0, len(memoryMembers)+len(networkMembers))
	usedMemory := make([]bool, len(memoryMembers))
	for _, network := range networkMembers {
		merged := network
		for index, memory := range memoryMembers {
			if usedMemory[index] || !runtimeLoadoutDetectorCoreMatches(memory.Loadout, network.Loadout) {
				continue
			}
			usedMemory[index] = true
			merged = memory
			merged.Role = network.Role
			merged.Loadout.PartyIndex = network.Loadout.PartyIndex
			merged.Loadout.Online = true
			merged.Loadout.Stable = true
			merged.Loadout.Verification = "network_profile_core+memory_superset"
			merged.Loadout.Evidence = runtimePatchMonitorText(
				"Party 网络资料帧与只读内存配装核心一致；保留内存中更多已记录范围",
				"Party network profile matched the read-only memory core; additional recorded memory scopes were retained",
			)
			break
		}
		result = append(result, merged)
	}
	for index, member := range memoryMembers {
		if !usedMemory[index] {
			result = append(result, member)
		}
	}
	return result
}

func runtimeLoadoutDetectorMembers(snapshot RuntimePatchPartyMonitor) []RuntimeLoadoutDetectorMember {
	members := make([]RuntimeLoadoutDetectorMember, 0, 4)
	for _, entity := range snapshot.Entities {
		if entity.Role == "player" || entity.Role == "companion" || !entity.Present || entity.Loadout == nil || !entity.Loadout.Available || !entity.Loadout.Online {
			continue
		}
		candidate := *entity.Loadout
		candidate.Stable = true
		candidate.SnapshotCount = runtimePatchPartySnapshotCount
		members = append(members, RuntimeLoadoutDetectorMember{
			Role: entity.Role, CharacterHash: candidate.CharacterHash,
			CharacterName: candidate.CharacterName, Loadout: candidate,
		})
	}
	return members
}

func runtimeLoadoutDetectorFingerprint(members []RuntimeLoadoutDetectorMember) (string, error) {
	payload, err := json.Marshal(members)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func cloneRuntimeLoadoutDetectorMembers(source []RuntimeLoadoutDetectorMember) []RuntimeLoadoutDetectorMember {
	if len(source) == 0 {
		return nil
	}
	payload, _ := json.Marshal(source)
	var cloned []RuntimeLoadoutDetectorMember
	_ = json.Unmarshal(payload, &cloned)
	return cloned
}

func (session *runtimeLoadoutDetectorSession) observeAbsent() {
	session.currentTeamSize = 0
	session.pendingFingerprint = ""
	session.pendingMembers = nil
	session.pendingPolls = 0
	session.absentPolls++
	if session.absentPolls >= runtimeLoadoutDetectorAbsentPolls && (session.activeFingerprint != "" || session.restoredFingerprint != "") {
		session.activeFingerprint = ""
		session.restoredFingerprint = ""
		session.activeRecordID = ""
		if err := saveRuntimeLoadoutDetectorHistory(session.historyPath, session.history, ""); err != nil {
			session.lastError = err.Error()
		}
	}
}

func (session *runtimeLoadoutDetectorSession) observeTeam(fingerprint string, members []RuntimeLoadoutDetectorMember) {
	session.absentPolls = 0
	session.currentTeamSize = len(members)
	if fingerprint == session.activeFingerprint {
		session.state = "recording"
		session.clearPendingTeam()
		return
	}
	if session.activeFingerprint == "" && fingerprint == session.restoredFingerprint {
		session.activeFingerprint = fingerprint
		session.restoredFingerprint = ""
		if session.activeRecordID == "" && len(session.history) > 0 {
			session.activeRecordID = session.history[len(session.history)-1].ID
		}
		session.state = "recording"
		session.clearPendingTeam()
		return
	}
	if session.activeFingerprint != "" {
		session.observeActiveTeamChange(fingerprint, members)
		return
	}
	if fingerprint != session.pendingFingerprint {
		session.pendingFingerprint = fingerprint
		session.pendingMembers = cloneRuntimeLoadoutDetectorMembers(members)
		session.pendingPolls = 1
		session.state = "stabilizing"
		return
	}
	session.pendingPolls++
	session.state = "stabilizing"
	if session.pendingPolls < runtimeLoadoutDetectorStablePolls {
		return
	}
	now := time.Now()
	record := RuntimeLoadoutDetectorRecord{
		ID:       fmt.Sprintf("%s-%04d", session.sessionID, session.nextSequence),
		Sequence: session.nextSequence, CapturedAt: now.UnixMilli(), SessionID: session.sessionID,
		Members: cloneRuntimeLoadoutDetectorMembers(session.pendingMembers),
	}
	session.history = boundedRuntimeLoadoutDetectorHistory(append(session.history, record))
	session.nextSequence++
	session.sessionCaptured++
	session.lastCapture = now
	session.activeFingerprint = fingerprint
	session.restoredFingerprint = ""
	session.activeRecordID = record.ID
	session.clearPendingTeam()
	session.state = "recording"
	if err := saveRuntimeLoadoutDetectorHistory(session.historyPath, session.history, session.activeFingerprint); err != nil {
		session.lastError = err.Error()
	}
}

func (session *runtimeLoadoutDetectorSession) clearPendingTeam() {
	session.pendingFingerprint = ""
	session.pendingMembers = nil
	session.pendingPolls = 0
}

func (session *runtimeLoadoutDetectorSession) activeRecordIndex() int {
	if session.activeRecordID == "" {
		return -1
	}
	for index := len(session.history) - 1; index >= 0; index-- {
		if session.history[index].ID == session.activeRecordID {
			return index
		}
	}
	return -1
}

func (session *runtimeLoadoutDetectorSession) observeActiveTeamChange(fingerprint string, members []RuntimeLoadoutDetectorMember) {
	if fingerprint != session.pendingFingerprint {
		session.pendingFingerprint = fingerprint
		session.pendingMembers = cloneRuntimeLoadoutDetectorMembers(members)
		session.pendingPolls = 1
		session.state = "stabilizing"
		return
	}
	session.pendingPolls++
	session.state = "stabilizing"
	if session.pendingPolls < runtimeLoadoutDetectorStablePolls {
		return
	}
	index := session.activeRecordIndex()
	if index >= 0 && runtimeLoadoutDetectorMembersMoreComplete(session.pendingMembers, session.history[index].Members) {
		now := time.Now()
		session.history[index].Members = cloneRuntimeLoadoutDetectorMembers(session.pendingMembers)
		session.history[index].CapturedAt = now.UnixMilli()
		session.activeFingerprint = fingerprint
		session.lastCapture = now
		if err := saveRuntimeLoadoutDetectorHistory(session.historyPath, session.history, session.activeFingerprint); err != nil {
			session.lastError = err.Error()
		}
	}
	session.clearPendingTeam()
	session.state = "recording"
}

func runtimeLoadoutDetectorMembersMoreComplete(candidate, existing []RuntimeLoadoutDetectorMember) bool {
	if len(candidate) < len(existing) {
		return false
	}
	candidateByRole := make(map[string]RuntimeLoadoutDetectorMember, len(candidate))
	for _, member := range candidate {
		candidateByRole[member.Role] = member
	}
	improved := len(candidate) > len(existing)
	for _, previous := range existing {
		next, ok := candidateByRole[previous.Role]
		if !ok {
			return false
		}
		if !runtimeLoadoutDetectorLoadoutSuperset(next, previous) {
			return false
		}
		previousScopes := runtimeLoadoutDetectorCompleteness(previous.Loadout)
		nextScopes := runtimeLoadoutDetectorCompleteness(next.Loadout)
		for index := range previousScopes {
			if nextScopes[index] < previousScopes[index] {
				return false
			}
			if nextScopes[index] > previousScopes[index] {
				improved = true
			}
		}
	}
	return improved
}

func runtimeLoadoutDetectorLoadoutSuperset(candidate, existing RuntimeLoadoutDetectorMember) bool {
	if !runtimeLoadoutDetectorStringPreserved(existing.CharacterHash, candidate.CharacterHash) ||
		!runtimeLoadoutDetectorStringPreserved(existing.Loadout.CharacterCode, candidate.Loadout.CharacterCode) ||
		!runtimeLoadoutDetectorStringPreserved(existing.Loadout.CharacterHash, candidate.Loadout.CharacterHash) ||
		!runtimeLoadoutDetectorHashPreserved(existing.Loadout.Weapon.Hash, candidate.Loadout.Weapon.Hash) ||
		!runtimeLoadoutDetectorHashPreserved(existing.Loadout.Weapon.WrightstoneID, candidate.Loadout.Weapon.WrightstoneID) ||
		!runtimeLoadoutDetectorPanelPreserved(existing.Loadout.Stats, candidate.Loadout.Stats) ||
		!runtimeLoadoutDetectorWeaponScalarsPreserved(existing.Loadout.Weapon, candidate.Loadout.Weapon) ||
		!runtimeLoadoutDetectorU32Preserved(existing.Loadout.MasterLevel, candidate.Loadout.MasterLevel) ||
		(existing.Loadout.MasteryAvailable && !candidate.Loadout.MasteryAvailable) {
		return false
	}
	if !runtimeLoadoutDetectorIndexedTraitsPreserved(existing.Loadout.Weapon.Skills, candidate.Loadout.Weapon.Skills) ||
		!runtimeLoadoutDetectorIndexedTraitsPreserved(existing.Loadout.Weapon.Traits, candidate.Loadout.Weapon.Traits) ||
		!runtimeLoadoutDetectorAbilitiesPreserved(existing.Loadout.Abilities, candidate.Loadout.Abilities) ||
		!runtimeLoadoutDetectorMasteryPreserved(existing.Loadout.Mastery, candidate.Loadout.Mastery) ||
		!runtimeLoadoutDetectorSummonsPreserved(existing.Loadout.Summons, candidate.Loadout.Summons) ||
		!runtimeLoadoutDetectorSigilsPreserved(existing.Loadout.Sigils, candidate.Loadout.Sigils) ||
		!runtimeLoadoutDetectorOverLimitPreserved(existing.Loadout.OverLimit, candidate.Loadout.OverLimit) {
		return false
	}
	return true
}

func runtimeLoadoutDetectorStringPreserved(existing, candidate string) bool {
	existing = strings.TrimSpace(existing)
	return existing == "" || strings.EqualFold(existing, strings.TrimSpace(candidate))
}

func runtimeLoadoutDetectorHashPreserved(existing, candidate uint32) bool {
	return runtimePatchPartyEmptyHash(existing) || existing == candidate
}

func runtimeLoadoutDetectorU32Preserved(existing, candidate uint32) bool {
	return existing == 0 || existing == candidate
}

func runtimeLoadoutDetectorFloat32Preserved(existing, candidate float32) bool {
	return existing == 0 || existing == candidate
}

func runtimeLoadoutDetectorPanelPreserved(existing, candidate RuntimePatchPartyPanelStats) bool {
	return runtimeLoadoutDetectorU32Preserved(existing.Level, candidate.Level) &&
		runtimeLoadoutDetectorU32Preserved(existing.TotalHP, candidate.TotalHP) &&
		runtimeLoadoutDetectorU32Preserved(existing.TotalAttack, candidate.TotalAttack) &&
		runtimeLoadoutDetectorFloat32Preserved(existing.StunPower, candidate.StunPower) &&
		runtimeLoadoutDetectorFloat32Preserved(existing.CriticalRate, candidate.CriticalRate) &&
		runtimeLoadoutDetectorU32Preserved(existing.TotalPower, candidate.TotalPower)
}

func runtimeLoadoutDetectorWeaponScalarsPreserved(existing, candidate RuntimePatchPartyWeapon) bool {
	return runtimeLoadoutDetectorU32Preserved(existing.XP, candidate.XP) &&
		runtimeLoadoutDetectorU32Preserved(existing.Level, candidate.Level) &&
		runtimeLoadoutDetectorU32Preserved(existing.StarLevel, candidate.StarLevel) &&
		runtimeLoadoutDetectorU32Preserved(existing.PlusMarks, candidate.PlusMarks) &&
		runtimeLoadoutDetectorU32Preserved(existing.AwakeningLevel, candidate.AwakeningLevel) &&
		runtimeLoadoutDetectorU32Preserved(existing.HP, candidate.HP) &&
		runtimeLoadoutDetectorU32Preserved(existing.Attack, candidate.Attack)
}

func runtimeLoadoutDetectorIndexedTraitsPreserved(existing, candidate []RuntimePatchPartyTrait) bool {
	for index, previous := range existing {
		if runtimePatchPartyEmptyHash(previous.Hash) {
			continue
		}
		if index >= len(candidate) || candidate[index].Hash != previous.Hash ||
			!runtimeLoadoutDetectorU32Preserved(previous.Level, candidate[index].Level) {
			return false
		}
	}
	return true
}

func runtimeLoadoutDetectorAbilitiesPreserved(existing, candidate []RuntimePatchPartyAbility) bool {
	available := make(map[uint32]bool, len(candidate))
	for _, ability := range candidate {
		if !runtimePatchPartyEmptyHash(ability.Hash) {
			available[ability.Hash] = true
		}
	}
	for _, ability := range existing {
		if !runtimePatchPartyEmptyHash(ability.Hash) && !available[ability.Hash] {
			return false
		}
	}
	return true
}

func runtimeLoadoutDetectorMasteryPreserved(existing, candidate []LoadoutMasteryNode) bool {
	available := make(map[string]bool, len(candidate))
	for _, node := range candidate {
		available[strings.ToUpper(strings.TrimSpace(node.Hash))] = true
	}
	for _, node := range existing {
		hash := strings.ToUpper(strings.TrimSpace(node.Hash))
		if hash != "" && !available[hash] {
			return false
		}
	}
	return true
}

func runtimeLoadoutDetectorSummonsPreserved(existing, candidate []RuntimePatchPartySummon) bool {
	available := make(map[int]RuntimePatchPartySummon, len(candidate))
	for _, summon := range candidate {
		available[summon.Index] = summon
	}
	for _, previous := range existing {
		next, ok := available[previous.Index]
		if !ok || !runtimeLoadoutDetectorHashPreserved(previous.TypeHash, next.TypeHash) ||
			!runtimeLoadoutDetectorHashPreserved(previous.MainTraitHash, next.MainTraitHash) ||
			!runtimeLoadoutDetectorHashPreserved(previous.SubParamHash, next.SubParamHash) ||
			!runtimeLoadoutDetectorU32Preserved(previous.MainTraitLevel, next.MainTraitLevel) ||
			!runtimeLoadoutDetectorU32Preserved(previous.SubParamLevel, next.SubParamLevel) {
			return false
		}
	}
	return true
}

func runtimeLoadoutDetectorSigilsPreserved(existing, candidate []RuntimePatchPartySigil) bool {
	available := make(map[int]RuntimePatchPartySigil, len(candidate))
	for _, sigil := range candidate {
		available[sigil.Index] = sigil
	}
	for _, previous := range existing {
		next, ok := available[previous.Index]
		if !ok || !runtimeLoadoutDetectorHashPreserved(previous.Hash, next.Hash) ||
			!runtimeLoadoutDetectorHashPreserved(previous.PrimaryTraitHash, next.PrimaryTraitHash) ||
			!runtimeLoadoutDetectorHashPreserved(previous.SecondaryTraitHash, next.SecondaryTraitHash) ||
			!runtimeLoadoutDetectorU32Preserved(previous.Level, next.Level) ||
			!runtimeLoadoutDetectorU32Preserved(previous.PrimaryTraitLevel, next.PrimaryTraitLevel) ||
			!runtimeLoadoutDetectorU32Preserved(previous.SecondaryTraitLevel, next.SecondaryTraitLevel) {
			return false
		}
	}
	return true
}

func runtimeLoadoutDetectorOverLimitPreserved(existing, candidate []RuntimePatchPartyOverLimit) bool {
	available := make(map[int]RuntimePatchPartyOverLimit, len(candidate))
	for _, slot := range candidate {
		available[slot.Index] = slot
	}
	for _, previous := range existing {
		next, ok := available[previous.Index]
		if !ok || !runtimeLoadoutDetectorHashPreserved(previous.AttributeHash, next.AttributeHash) ||
			!runtimeLoadoutDetectorU32Preserved(previous.Flags, next.Flags) ||
			!runtimeLoadoutDetectorU32Preserved(previous.Level, next.Level) ||
			!runtimeLoadoutDetectorFloat32Preserved(previous.Value, next.Value) {
			return false
		}
	}
	return true
}

func runtimeLoadoutDetectorCompleteness(loadout RuntimePatchPartyLoadout) [16]int {
	identity := 0
	for _, value := range []string{loadout.CharacterCode, loadout.CharacterHash, loadout.CharacterName} {
		if strings.TrimSpace(value) != "" {
			identity++
		}
	}
	panel := 0
	for _, value := range []uint32{loadout.Stats.Level, loadout.Stats.TotalHP, loadout.Stats.TotalAttack, loadout.Stats.TotalPower} {
		if value != 0 {
			panel++
		}
	}
	for _, value := range []float32{loadout.Stats.StunPower, loadout.Stats.CriticalRate} {
		if value != 0 {
			panel++
		}
	}
	weapon := 0
	if !runtimePatchPartyEmptyHash(loadout.Weapon.Hash) {
		weapon++
	}
	if strings.TrimSpace(loadout.Weapon.Name) != "" {
		weapon++
	}
	wrightstone := len(loadout.Weapon.Traits)
	wrightstoneDetails := 0
	if !runtimePatchPartyEmptyHash(loadout.Weapon.WrightstoneID) {
		wrightstone++
	}
	for _, trait := range loadout.Weapon.Traits {
		if !runtimePatchPartyEmptyHash(trait.Hash) {
			wrightstoneDetails++
		}
		if trait.Level > 0 {
			wrightstoneDetails++
		}
	}
	weaponProgression := 0
	for _, value := range []uint32{loadout.Weapon.XP, loadout.Weapon.Level, loadout.Weapon.StarLevel, loadout.Weapon.PlusMarks, loadout.Weapon.AwakeningLevel, loadout.Weapon.HP, loadout.Weapon.Attack} {
		if value != 0 {
			weaponProgression++
		}
	}
	weaponSkillDetails := 0
	for _, skill := range loadout.Weapon.Skills {
		if !runtimePatchPartyEmptyHash(skill.Hash) {
			weaponSkillDetails++
		}
		if skill.Level > 0 {
			weaponSkillDetails++
		}
	}
	sigilTraits := 0
	for _, sigil := range loadout.Sigils {
		if !runtimePatchPartyEmptyHash(sigil.Hash) {
			sigilTraits++
		}
		if sigil.Level > 0 {
			sigilTraits++
		}
		if !runtimePatchPartyEmptyHash(sigil.PrimaryTraitHash) {
			sigilTraits++
		}
		if sigil.PrimaryTraitLevel > 0 {
			sigilTraits++
		}
		if !runtimePatchPartyEmptyHash(sigil.SecondaryTraitHash) {
			sigilTraits++
		}
		if sigil.SecondaryTraitLevel > 0 {
			sigilTraits++
		}
	}
	summonDetails := 0
	for _, summon := range loadout.Summons {
		for _, hash := range []uint32{summon.TypeHash, summon.MainTraitHash, summon.SubParamHash} {
			if !runtimePatchPartyEmptyHash(hash) {
				summonDetails++
			}
		}
		if summon.MainTraitLevel > 0 {
			summonDetails++
		}
		if summon.SubParamLevel > 0 {
			summonDetails++
		}
	}
	mastery := len(loadout.Mastery)
	if loadout.MasteryAvailable {
		mastery++
	}
	if loadout.MasterLevel > 0 {
		mastery++
	}
	overLimitValues := 0
	for _, slot := range loadout.OverLimit {
		if !runtimePatchPartyEmptyHash(slot.AttributeHash) {
			overLimitValues++
		}
		if slot.Flags > 0 {
			overLimitValues++
		}
		if slot.Level > 0 {
			overLimitValues++
		}
		if slot.Value != 0 {
			overLimitValues++
		}
	}
	return [16]int{
		identity, panel, weapon, weaponProgression, len(loadout.Weapon.Skills), weaponSkillDetails,
		wrightstone, wrightstoneDetails, len(loadout.Abilities), len(loadout.Summons), summonDetails,
		mastery, len(loadout.Sigils), sigilTraits, len(loadout.OverLimit), overLimitValues,
	}
}

func (a *App) runtimeLoadoutDetectorCandidate(recordID, role string) (*RuntimePatchPartyLoadout, error) {
	recordID = strings.TrimSpace(recordID)
	role = strings.TrimSpace(role)
	if recordID == "" || role == "" {
		return nil, fmt.Errorf("配装检测记录和角色槽位不能为空")
	}
	records, err := a.RuntimeLoadoutDetectorHistory()
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.ID != recordID {
			continue
		}
		for _, member := range record.Members {
			if member.Role == role {
				candidate := member.Loadout
				return &candidate, nil
			}
		}
		return nil, fmt.Errorf("场次 %s 中没有角色槽位 %s", recordID, role)
	}
	return nil, fmt.Errorf("找不到配装检测记录 %s", recordID)
}

func (a *App) runtimeLoadoutDetectorShare(recordID, role, title string) (*LoadoutShare, *RuntimePatchPartyLoadout, error) {
	candidate, err := a.runtimeLoadoutDetectorCandidate(recordID, role)
	if err != nil {
		return nil, nil, err
	}
	share, err := runtimeLoadoutShareFromCandidate(*candidate, title)
	return share, candidate, err
}

func (a *App) RuntimeLoadoutDetectorShare(recordID, role, title string) (*LoadoutShareCodeResult, error) {
	share, _, err := a.runtimeLoadoutDetectorShare(recordID, role, title)
	if err != nil {
		return nil, err
	}
	return encodeLoadoutShareCode(share)
}

func (a *App) RuntimeLoadoutDetectorExport(recordID, role, title string) (string, error) {
	share, _, err := a.runtimeLoadoutDetectorShare(recordID, role, title)
	if err != nil {
		return "", err
	}
	outputPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title: "导出检测到的角色配装", DefaultFilename: safeLoadoutFilename(share.Name) + ".gbfr-loadout.json",
		Filters: []runtime.FileFilter{{DisplayName: "GBFR 配装", Pattern: "*.gbfr-loadout.json"}},
	})
	if err != nil || outputPath == "" {
		return outputPath, err
	}
	payload, err := marshalLoadoutShare(share)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Clean(outputPath), payload, 0o600); err != nil {
		return "", fmt.Errorf("写出检测配装失败: %w", err)
	}
	return outputPath, nil
}

func (a *App) RuntimeLoadoutDetectorPublish(recordID, role, title string) (*LoadoutPublishedShare, error) {
	share, candidate, err := a.runtimeLoadoutDetectorShare(recordID, role, title)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeLoadoutShareCode(share)
	if err != nil {
		return nil, err
	}
	frame, err := loadoutShareFrameFromCompatibilityCode(encoded.CompatibilityCode)
	if err != nil {
		return nil, err
	}
	return publishLoadoutShareFrameWithMetadata(a.ctx, loadoutShareHTTPClient(), loadoutShareServiceURL, frame, previewForRuntimeLoadout(share, *candidate))
}

func (a *App) RuntimeLoadoutDetectorDelete(recordID string) error {
	recordID = strings.TrimSpace(recordID)
	if recordID == "" {
		return fmt.Errorf("配装检测记录不能为空")
	}
	a.runtimeLoadoutDetectorMu.Lock()
	session := a.runtimeLoadoutDetector
	a.runtimeLoadoutDetectorMu.Unlock()
	if session != nil {
		session.mu.Lock()
		defer session.mu.Unlock()
		return session.deleteRecord(recordID)
	}
	path, err := a.runtimeLoadoutDetectorHistoryPath()
	if err != nil {
		return err
	}
	records, _, activeFingerprint, err := loadRuntimeLoadoutDetectorHistory(path)
	if err != nil {
		return err
	}
	next := records[:0]
	for _, record := range records {
		if record.ID != recordID {
			next = append(next, record)
		}
	}
	if len(next) == len(records) {
		return fmt.Errorf("找不到配装检测记录 %s", recordID)
	}
	return saveRuntimeLoadoutDetectorHistory(path, next, activeFingerprint)
}

func (session *runtimeLoadoutDetectorSession) deleteRecord(recordID string) error {
	next := make([]RuntimeLoadoutDetectorRecord, 0, len(session.history))
	for _, record := range session.history {
		if record.ID != recordID {
			next = append(next, record)
		}
	}
	if len(next) == len(session.history) {
		return fmt.Errorf("找不到配装检测记录 %s", recordID)
	}
	if err := saveRuntimeLoadoutDetectorHistory(session.historyPath, next, session.activeFingerprint); err != nil {
		return err
	}
	session.history = next
	return nil
}
