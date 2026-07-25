package backend

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoadoutSnapshotCacheReusesUnchangedIdentityAndInvalidatesChanges(t *testing.T) {
	cache := newLoadoutSnapshotCache(3)
	identity := loadoutSnapshotIdentity{path: "save.dat", size: 100, modified: 10}
	var loads int
	loader := func() (*loadoutReadSnapshot, error) {
		loads++
		return &loadoutReadSnapshot{}, nil
	}

	first, err := cache.get(identity, loader)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.get(identity, loader)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || loads != 1 {
		t.Fatalf("unchanged snapshot was not reused: same=%v loads=%d", first == second, loads)
	}

	changed := identity
	changed.modified++
	third, err := cache.get(changed, loader)
	if err != nil {
		t.Fatal(err)
	}
	if third == first || loads != 2 {
		t.Fatalf("changed identity did not invalidate cache: same=%v loads=%d", third == first, loads)
	}
}

func TestLoadoutSnapshotCacheCoalescesConcurrentLoads(t *testing.T) {
	cache := newLoadoutSnapshotCache(3)
	identity := loadoutSnapshotIdentity{path: "save.dat", size: 100, modified: 10}
	var loads atomic.Int32
	loader := func() (*loadoutReadSnapshot, error) {
		loads.Add(1)
		time.Sleep(15 * time.Millisecond)
		return &loadoutReadSnapshot{}, nil
	}

	const workers = 8
	results := make([]*loadoutReadSnapshot, workers)
	errorsSeen := make([]error, workers)
	var wg sync.WaitGroup
	for index := 0; index < workers; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errorsSeen[index] = cache.get(identity, loader)
		}(index)
	}
	wg.Wait()

	if loads.Load() != 1 {
		t.Fatalf("concurrent snapshot loads=%d, want 1", loads.Load())
	}
	for index := range results {
		if errorsSeen[index] != nil {
			t.Fatalf("worker %d: %v", index, errorsSeen[index])
		}
		if results[index] != results[0] {
			t.Fatalf("worker %d received a different snapshot", index)
		}
	}
}

func TestLoadoutSnapshotCacheDoesNotRememberFailures(t *testing.T) {
	cache := newLoadoutSnapshotCache(3)
	identity := loadoutSnapshotIdentity{path: "save.dat", size: 100, modified: 10}
	wantErr := errors.New("parse failed")
	loads := 0
	loader := func() (*loadoutReadSnapshot, error) {
		loads++
		if loads == 1 {
			return nil, wantErr
		}
		return &loadoutReadSnapshot{}, nil
	}

	if _, err := cache.get(identity, loader); !errors.Is(err, wantErr) {
		t.Fatalf("first load error=%v, want %v", err, wantErr)
	}
	if _, err := cache.get(identity, loader); err != nil {
		t.Fatal(err)
	}
	if loads != 2 {
		t.Fatalf("failed result was cached: loads=%d", loads)
	}
}

func TestLoadoutSnapshotCacheEvictsLeastRecentlyUsedPath(t *testing.T) {
	cache := newLoadoutSnapshotCache(2)
	loads := map[string]int{}
	load := func(identity loadoutSnapshotIdentity) {
		t.Helper()
		_, err := cache.get(identity, func() (*loadoutReadSnapshot, error) {
			loads[identity.path]++
			return &loadoutReadSnapshot{}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	a := loadoutSnapshotIdentity{path: "a.dat", size: 1, modified: 1}
	b := loadoutSnapshotIdentity{path: "b.dat", size: 1, modified: 1}
	c := loadoutSnapshotIdentity{path: "c.dat", size: 1, modified: 1}
	load(a)
	load(b)
	load(a)
	load(c)
	load(b)
	if loads["a.dat"] != 1 || loads["b.dat"] != 2 || loads["c.dat"] != 1 {
		t.Fatalf("unexpected LRU loads: %+v", loads)
	}
}
