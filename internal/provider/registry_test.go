package provider

import (
	"context"
	"io"
	"testing"

	"github.com/JonGanz/agent-status-collector/internal/status"
)

type fakeProvider struct{ name string }

func (f fakeProvider) Name() string { return f.name }
func (f fakeProvider) HandleHook(ctx context.Context, event string, payload io.Reader) (status.Status, error) {
	return status.Status{Provider: f.name}, nil
}
func (f fakeProvider) Detect() (bool, string)      { return true, "fake" }
func (f fakeProvider) IsConfigured() (bool, error) { return true, nil }
func (f fakeProvider) Setup(dryRun bool) (SetupResult, error) {
	return SetupResult{}, nil
}

func TestRegisterAndGet(t *testing.T) {
	Register(fakeProvider{name: "zzz-test-provider"})
	p, ok := Get("zzz-test-provider")
	if !ok {
		t.Fatalf("Get: not found")
	}
	if p.Name() != "zzz-test-provider" {
		t.Fatalf("Name() = %q", p.Name())
	}
}

func TestAll_SortedByName(t *testing.T) {
	Register(fakeProvider{name: "zzz-b"})
	Register(fakeProvider{name: "zzz-a"})
	all := All()
	var lastRelevant []string
	for _, p := range all {
		if p.Name() == "zzz-a" || p.Name() == "zzz-b" {
			lastRelevant = append(lastRelevant, p.Name())
		}
	}
	if len(lastRelevant) != 2 || lastRelevant[0] != "zzz-a" || lastRelevant[1] != "zzz-b" {
		t.Fatalf("All() relevant subset = %v, want [zzz-a zzz-b]", lastRelevant)
	}
}

func TestGet_Unknown(t *testing.T) {
	_, ok := Get("does-not-exist")
	if ok {
		t.Fatalf("Get: found unexpected provider")
	}
}
