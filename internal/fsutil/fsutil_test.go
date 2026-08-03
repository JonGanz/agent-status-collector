package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomic_NoLeftoverTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	if err := WriteAtomic(path, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "x.json" {
			t.Fatalf("unexpected leftover file %q in %s", e.Name(), dir)
		}
	}
}

func TestWriteAtomic_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deeper", "x.json")
	if err := WriteAtomic(path, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

func TestWriteAtomic_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	if err := WriteAtomic(path, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	if err := WriteAtomic(path, []byte(`{"a":2}`), 0o600); err != nil {
		t.Fatalf("WriteAtomic (overwrite): %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != `{"a":2}` {
		t.Fatalf("content = %q, want {\"a\":2}", data)
	}
}
