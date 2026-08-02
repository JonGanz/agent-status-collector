// Package eventlog appends raw hook/statusline payloads to a debug log, so
// that when the collector loses track of an agent, the exact inputs it
// received are still on disk to inspect.
package eventlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// MaxSizeBytes is the size threshold past which Append rotates the log file
// (renaming it to a .1 suffix) before writing further. This is a basic
// growth guard, not a full rotation subsystem.
const MaxSizeBytes = 10 * 1024 * 1024 // 10MB

type entry struct {
	Timestamp time.Time       `json:"timestamp"`
	Provider  string          `json:"provider"`
	Event     string          `json:"event"`
	Payload   json.RawMessage `json:"payload"`
}

// Append writes one JSON line describing a hook/statusline invocation to
// path, creating parent directories as needed. It takes an exclusive lock
// around the rotate-check+append to keep concurrent writers from
// interleaving.
func Append(path, provider, event string, rawPayload []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	fl := flock.New(path + ".lock")
	if err := fl.Lock(); err != nil {
		return err
	}
	defer fl.Unlock()

	if fi, err := os.Stat(path); err == nil && fi.Size() > MaxSizeBytes {
		if err := os.Rename(path, path+".1"); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	line, err := json.Marshal(entry{
		Timestamp: time.Now(),
		Provider:  provider,
		Event:     event,
		Payload:   json.RawMessage(rawPayload),
	})
	if err != nil {
		return err
	}
	line = append(line, '\n')
	_, err = f.Write(line)
	return err
}
