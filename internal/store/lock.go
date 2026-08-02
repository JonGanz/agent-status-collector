package store

import (
	"github.com/gofrs/flock"
)

// withLock acquires an exclusive lock on a sidecar lockfile for the given
// session id and runs fn while holding it. Locking the sidecar file (rather
// than the session JSON file itself) means readers of the JSON file never
// block on the lock, since writes go through atomic rename (see atomic.go).
func withLock(lockPath string, fn func() error) error {
	fl := flock.New(lockPath)
	if err := fl.Lock(); err != nil {
		return err
	}
	defer fl.Unlock()
	return fn()
}
