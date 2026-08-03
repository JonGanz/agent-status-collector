// Package ratelimit persists account-level rate limit snapshots (e.g.
// Claude's 5h/7d usage windows), independently of any individual session.
// A provider adapter may only be able to observe these via a specific
// session's integration (e.g. Claude Code's statusline), but the values
// themselves apply to the whole account, so they're queryable directly
// (`agent-status rate-limits`) without needing to find which session last
// reported them.
package ratelimit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/JonGanz/agent-status-collector/internal/fsutil"
	"github.com/JonGanz/agent-status-collector/internal/status"
)

// Record is the persisted snapshot for one provider.
type Record struct {
	Provider    string                   `json:"provider"`
	Windows     []status.RateLimitWindow `json:"windows"`
	LastUpdated time.Time                `json:"last_updated"`
}

// Store persists one Record per provider as a flat JSON file under dir.
type Store struct {
	dir string
}

// New creates a Store rooted at dir. The directory is created lazily on
// first write.
func New(dir string) *Store { return &Store{dir: dir} }

func (s *Store) path(providerName string) string {
	return filepath.Join(s.dir, providerName+".json")
}

// SaveRateLimits writes the current snapshot for providerName, atomically.
// It satisfies provider.RateLimitStore.
func (s *Store) SaveRateLimits(providerName string, windows []status.RateLimitWindow, at time.Time) error {
	data, err := json.MarshalIndent(Record{Provider: providerName, Windows: windows, LastUpdated: at}, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(s.path(providerName), data, 0o600)
}

// LoadRateLimits reads the snapshot for providerName. ok is false if none
// has been recorded yet. It satisfies provider.RateLimitStore.
func (s *Store) LoadRateLimits(providerName string) (windows []status.RateLimitWindow, lastUpdated time.Time, ok bool, err error) {
	rec, ok, err := s.load(providerName)
	if err != nil || !ok {
		return nil, time.Time{}, ok, err
	}
	return rec.Windows, rec.LastUpdated, true, nil
}

func (s *Store) load(providerName string) (Record, bool, error) {
	data, err := os.ReadFile(s.path(providerName))
	if err != nil {
		if os.IsNotExist(err) {
			return Record{}, false, nil
		}
		return Record{}, false, err
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return Record{}, false, err
	}
	return rec, true, nil
}

// LoadAll returns every persisted provider snapshot, sorted by provider
// name.
func (s *Store) LoadAll() ([]Record, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Record
	for _, de := range entries {
		name := de.Name()
		if de.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		rec, ok, err := s.load(strings.TrimSuffix(name, ".json"))
		if err != nil || !ok {
			continue
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out, nil
}
