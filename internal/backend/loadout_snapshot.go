package backend

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const loadoutSnapshotCacheSize = 3

var errLoadoutSnapshotChanged = errors.New("存档在读取期间发生变化")

type loadoutSnapshotIdentity struct {
	path     string
	size     int64
	modified int64
}

type loadoutReadSnapshot struct {
	data       []byte
	save       *SaveData
	parsedOnce sync.Once
	parsed     *SaveGameFile
	parsedErr  error
}

func (snapshot *loadoutReadSnapshot) parsedSave() (*SaveGameFile, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("配装存档快照为空")
	}
	snapshot.parsedOnce.Do(func() {
		snapshot.parsed, snapshot.parsedErr = ParseSaveData(snapshot.data)
	})
	return snapshot.parsed, snapshot.parsedErr
}

type loadoutSnapshotCacheEntry struct {
	identity loadoutSnapshotIdentity
	snapshot *loadoutReadSnapshot
	used     uint64
}

type loadoutSnapshotCache struct {
	mu       sync.Mutex
	capacity int
	sequence uint64
	entries  map[string]loadoutSnapshotCacheEntry
}

func newLoadoutSnapshotCache(capacity int) *loadoutSnapshotCache {
	if capacity < 1 {
		capacity = 1
	}
	return &loadoutSnapshotCache{capacity: capacity, entries: make(map[string]loadoutSnapshotCacheEntry)}
}

func (cache *loadoutSnapshotCache) get(identity loadoutSnapshotIdentity, loader func() (*loadoutReadSnapshot, error)) (*loadoutReadSnapshot, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.sequence++
	if entry, ok := cache.entries[identity.path]; ok && entry.identity == identity {
		entry.used = cache.sequence
		cache.entries[identity.path] = entry
		return entry.snapshot, nil
	}

	snapshot, err := loader()
	if err != nil {
		return nil, err
	}
	cache.entries[identity.path] = loadoutSnapshotCacheEntry{
		identity: identity,
		snapshot: snapshot,
		used:     cache.sequence,
	}
	for len(cache.entries) > cache.capacity {
		oldestPath := ""
		oldestUse := ^uint64(0)
		for path, entry := range cache.entries {
			if entry.used < oldestUse {
				oldestPath = path
				oldestUse = entry.used
			}
		}
		delete(cache.entries, oldestPath)
	}
	return snapshot, nil
}

var loadoutSnapshots = newLoadoutSnapshotCache(loadoutSnapshotCacheSize)

func loadoutSnapshotIdentityForPath(path string) (loadoutSnapshotIdentity, error) {
	absPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return loadoutSnapshotIdentity{}, err
	}
	absPath = filepath.Clean(absPath)
	info, err := os.Stat(absPath)
	if err != nil {
		return loadoutSnapshotIdentity{}, fmt.Errorf("读取存档信息失败: %w", err)
	}
	if info.IsDir() {
		return loadoutSnapshotIdentity{}, fmt.Errorf("存档路径不能是目录")
	}
	cachePath := absPath
	if runtime.GOOS == "windows" {
		cachePath = strings.ToLower(cachePath)
	}
	return loadoutSnapshotIdentity{path: cachePath, size: info.Size(), modified: info.ModTime().UnixNano()}, nil
}

func loadLoadoutReadSnapshot(path string) (*loadoutReadSnapshot, error) {
	for attempt := 0; attempt < 2; attempt++ {
		identity, err := loadoutSnapshotIdentityForPath(path)
		if err != nil {
			return nil, err
		}
		snapshot, err := loadoutSnapshots.get(identity, func() (*loadoutReadSnapshot, error) {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil, fmt.Errorf("读取存档失败: %w", readErr)
			}
			after, statErr := loadoutSnapshotIdentityForPath(path)
			if statErr != nil {
				return nil, statErr
			}
			if after != identity || int64(len(data)) != identity.size {
				return nil, errLoadoutSnapshotChanged
			}
			save, parseErr := newSaveData(path, data)
			if parseErr != nil {
				return nil, parseErr
			}
			return &loadoutReadSnapshot{data: data, save: save}, nil
		})
		if errors.Is(err, errLoadoutSnapshotChanged) {
			continue
		}
		return snapshot, err
	}
	return nil, fmt.Errorf("%w，请稍后重试", errLoadoutSnapshotChanged)
}
