package ratelimit

import (
	"testing"
	"time"

	"github.com/JonGanz/agent-status-collector/internal/status"
)

func TestSaveLoadRateLimits_RoundTrip(t *testing.T) {
	s := New(t.TempDir())
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	windows := []status.RateLimitWindow{{Label: "5h", PercentUsed: 12.5}}

	if err := s.SaveRateLimits("claudecode", windows, now); err != nil {
		t.Fatalf("SaveRateLimits: %v", err)
	}

	got, lastUpdated, ok, err := s.LoadRateLimits("claudecode")
	if err != nil {
		t.Fatalf("LoadRateLimits: %v", err)
	}
	if !ok {
		t.Fatalf("LoadRateLimits: ok = false, want true")
	}
	if len(got) != 1 || got[0].Label != "5h" || got[0].PercentUsed != 12.5 {
		t.Fatalf("LoadRateLimits windows = %+v", got)
	}
	if !lastUpdated.Equal(now) {
		t.Fatalf("LoadRateLimits lastUpdated = %v, want %v", lastUpdated, now)
	}
}

func TestLoadRateLimits_NotFound(t *testing.T) {
	s := New(t.TempDir())
	_, _, ok, err := s.LoadRateLimits("nope")
	if err != nil {
		t.Fatalf("LoadRateLimits: %v", err)
	}
	if ok {
		t.Fatalf("ok = true for a provider that was never saved")
	}
}

func TestSaveRateLimits_OverwritesPreviousSnapshot(t *testing.T) {
	s := New(t.TempDir())
	if err := s.SaveRateLimits("claudecode", []status.RateLimitWindow{{Label: "5h", PercentUsed: 10}}, time.Now()); err != nil {
		t.Fatalf("SaveRateLimits: %v", err)
	}
	if err := s.SaveRateLimits("claudecode", []status.RateLimitWindow{{Label: "5h", PercentUsed: 90}}, time.Now()); err != nil {
		t.Fatalf("SaveRateLimits: %v", err)
	}
	windows, _, _, err := s.LoadRateLimits("claudecode")
	if err != nil {
		t.Fatalf("LoadRateLimits: %v", err)
	}
	if len(windows) != 1 || windows[0].PercentUsed != 90 {
		t.Fatalf("windows = %+v, want overwritten to PercentUsed=90", windows)
	}
}

func TestLoadAll_SortedByProvider(t *testing.T) {
	s := New(t.TempDir())
	now := time.Now()
	if err := s.SaveRateLimits("zzz-provider-b", []status.RateLimitWindow{{Label: "5h"}}, now); err != nil {
		t.Fatalf("SaveRateLimits: %v", err)
	}
	if err := s.SaveRateLimits("zzz-provider-a", []status.RateLimitWindow{{Label: "5h"}}, now); err != nil {
		t.Fatalf("SaveRateLimits: %v", err)
	}

	all, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("LoadAll returned %d records, want 2", len(all))
	}
	if all[0].Provider != "zzz-provider-a" || all[1].Provider != "zzz-provider-b" {
		t.Fatalf("LoadAll not sorted: %+v", all)
	}
}

func TestLoadAll_EmptyDirNoError(t *testing.T) {
	s := New(t.TempDir())
	all, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("LoadAll = %+v, want empty", all)
	}
}

func TestLoadAll_MissingDirNoError(t *testing.T) {
	s := New(t.TempDir() + "/does-not-exist")
	all, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("LoadAll = %+v, want empty", all)
	}
}
