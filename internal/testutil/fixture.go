// Package testutil provides small shared helpers for tests across packages.
package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// LoadFixture reads a file under the caller package's testdata directory,
// e.g. testutil.LoadFixture(t, "hooks/session_start.json") reads
// <callingpkg>/testdata/hooks/session_start.json.
func LoadFixture(t *testing.T, relPath string) []byte {
	t.Helper()
	_, callerFile, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatalf("testutil.LoadFixture: could not determine caller")
	}
	path := filepath.Join(filepath.Dir(callerFile), "testdata", relPath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("LoadFixture(%q): %v", relPath, err)
	}
	return data
}
