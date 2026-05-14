package torrstor

import (
	"sort"
	"sync"
	"time"

	"server/settings"
	"server/torr/storage"

	"github.com/anacrolix/torrent/metainfo"
	ts "github.com/anacrolix/torrent/storage"
)

type Storage struct {
	storage.Storage

	caches   map[metainfo.Hash]*Cache
	capacity int64
	mu       sync.Mutex
}

func NewStorage(capacity int64) *Storage {
	stor := new(Storage)
	stor.capacity = capacity
	stor.caches = make(map[metainfo.Hash]*Cache)
	go stor.globalCleanupWorker()
	return stor
}

func (s *Storage) OpenTorrent(info *metainfo.Info, infoHash metainfo.Hash) (ts.TorrentImpl, error) {
	// capFunc := func() (int64, bool) { //	NE
	// 	return s.capacity, true //	NE
	// } //	NE
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.caches[infoHash]; ok {
		return ch, nil
	}
	ch := NewCache(s.capacity, s)
	ch.Init(info, infoHash)
	s.caches[infoHash] = ch
	return ch, nil //	OE
	// return ts.TorrentImpl{ //	NE
	// 	Piece:    ch.Piece, //	NE
	// 	Close:    ch.Close, //	NE
	// 	Capacity: &capFunc, //	NE
	// }, nil //	NE
}

func (s *Storage) CloseHash(hash metainfo.Hash) {
	if s.caches == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.caches[hash]; ok {
		ch.Close()
		delete(s.caches, hash)
	}
}

func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range s.caches {
		ch.Close()
	}
	return nil
}

func (s *Storage) GetCache(hash metainfo.Hash) *Cache {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cache, ok := s.caches[hash]; ok {
		return cache
	}
	return nil
}

// globalCleanupWorker monitors total disk usage and evicts pieces when over the global CacheSize limit.
// Eviction order: LRU pieces from unpinned caches first, then from oldest pinned caches.
func (s *Storage) globalCleanupWorker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if !settings.BTsets.UseDisk || settings.BTsets.CacheSize <= 0 {
			continue
		}
		s.runGlobalCleanup()
	}
}

func (s *Storage) runGlobalCleanup() {
	s.mu.Lock()
	caches := make([]*Cache, 0, len(s.caches))
	for _, c := range s.caches {
		caches = append(caches, c)
	}
	s.mu.Unlock()

	// sum total filled bytes
	var total int64
	for _, c := range caches {
		total += c.filled
	}
	if total <= settings.BTsets.CacheSize {
		return
	}

	// split into unpinned (evict first) and pinned (evict last)
	type cacheEntry struct {
		cache       *Cache
		lastAccess  int64
	}
	var unpinned, pinned []cacheEntry
	for _, c := range caches {
		entry := cacheEntry{cache: c, lastAccess: c.lastAccessTime()}
		if c.keepFiles {
			pinned = append(pinned, entry)
		} else {
			unpinned = append(unpinned, entry)
		}
	}
	// oldest first
	sortByAccess := func(a []cacheEntry) {
		sort.Slice(a, func(i, j int) bool { return a[i].lastAccess < a[j].lastAccess })
	}
	sortByAccess(unpinned)
	sortByAccess(pinned)

	evict := func(entries []cacheEntry) bool {
		for _, e := range entries {
			e.cache.cleanPieces()
			var newTotal int64
			s.mu.Lock()
			for _, c := range s.caches {
				newTotal += c.filled
			}
			s.mu.Unlock()
			if newTotal <= settings.BTsets.CacheSize {
				return true
			}
		}
		return false
	}

	if evict(unpinned) {
		return
	}
	evict(pinned)
}
