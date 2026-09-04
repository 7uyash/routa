package recorder

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
)

// Recorder is a thread-safe in-memory ring buffer for captured HTTP exchanges.
type Recorder struct {
	mu       sync.RWMutex
	entries  []*Entry
	maxSize  int
	onChange func(*Entry) // callback for live dashboard updates
}

// New creates a Recorder with the given maximum capacity.
func New(maxSize int) *Recorder {
	if maxSize <= 0 {
		maxSize = 500
	}
	return &Recorder{
		entries: make([]*Entry, 0, maxSize),
		maxSize: maxSize,
	}
}

// OnChange registers a callback that fires each time a new entry is recorded.
func (r *Recorder) OnChange(fn func(*Entry)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onChange = fn
}

// Record adds a new entry to the recorder. If the buffer is full, the oldest
// entry is evicted.
func (r *Recorder) Record(entry *Entry) {
	r.mu.Lock()

	if entry.ID == "" {
		entry.ID = generateID()
	}

	if len(r.entries) >= r.maxSize {
		// Drop oldest entry
		r.entries = r.entries[1:]
	}
	r.entries = append(r.entries, entry)

	cb := r.onChange
	r.mu.Unlock()

	// Fire callback outside the lock to prevent deadlocks.
	if cb != nil {
		cb(entry)
	}
}

// List returns entries matching the given filter. Results are returned in
// reverse chronological order (newest first).
func (r *Recorder) List(f Filter) []EntrySummary {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	var results []EntrySummary
	// Iterate in reverse for newest-first ordering.
	for i := len(r.entries) - 1; i >= 0; i-- {
		e := r.entries[i]
		if !matchesFilter(e, f) {
			continue
		}
		if offset > 0 {
			offset--
			continue
		}
		results = append(results, e.Summary())
		if len(results) >= limit {
			break
		}
	}
	return results
}

// Get returns the full entry by ID, or nil if not found.
func (r *Recorder) Get(id string) *Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, e := range r.entries {
		if e.ID == id {
			return e
		}
	}
	return nil
}

// All returns all entries (newest first). Used for session saving.
func (r *Recorder) All() []*Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Entry, len(r.entries))
	for i, e := range r.entries {
		result[len(r.entries)-1-i] = e
	}
	return result
}

// Count returns the current number of recorded entries.
func (r *Recorder) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// Clear removes all recorded entries.
func (r *Recorder) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = r.entries[:0]
}

// matchesFilter checks if an entry matches the given filter criteria.
func matchesFilter(e *Entry, f Filter) bool {
	if f.Method != "" && !strings.EqualFold(e.Method, f.Method) {
		return false
	}
	if f.Path != "" && !strings.Contains(e.Path, f.Path) {
		return false
	}
	if f.StatusCode != 0 && e.StatusCode != f.StatusCode {
		return false
	}
	if f.StatusMin > 0 && e.StatusCode < f.StatusMin {
		return false
	}
	if f.StatusMax > 0 && e.StatusCode > f.StatusMax {
		return false
	}
	if f.Source != "" && e.Source != f.Source {
		return false
	}
	if f.IsReplay != nil && e.IsReplay != *f.IsReplay {
		return false
	}
	if f.Search != "" {
		search := strings.ToLower(f.Search)
		if !strings.Contains(strings.ToLower(e.Path), search) &&
			!strings.Contains(strings.ToLower(e.Method), search) &&
			!strings.Contains(strings.ToLower(e.Host), search) {
			return false
		}
	}
	return true
}

// generateID creates a short random hex ID for entries.
func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
