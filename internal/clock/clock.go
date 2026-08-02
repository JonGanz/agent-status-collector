// Package clock provides an injectable time source so staleness/TTL logic
// can be tested deterministically without sleeping in real time.
package clock

import "time"

// Clock provides the current time.
type Clock interface {
	Now() time.Time
}

// Real is the production Clock backed by time.Now.
type Real struct{}

func (Real) Now() time.Time { return time.Now() }

// Fake is a settable Clock for tests.
type Fake struct {
	t time.Time
}

// NewFake returns a Fake clock initialized to t.
func NewFake(t time.Time) *Fake { return &Fake{t: t} }

func (f *Fake) Now() time.Time { return f.t }

// Advance moves the fake clock forward by d.
func (f *Fake) Advance(d time.Duration) { f.t = f.t.Add(d) }

// Set moves the fake clock to an explicit time.
func (f *Fake) Set(t time.Time) { f.t = t }
