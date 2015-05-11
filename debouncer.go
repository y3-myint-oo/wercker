package main

import "time"

// Debouncer silences repeated triggers for settlePeriod
// and sends the current time on first trigger to C
// C is the public read only channel, c is the private r/w chan
type Debouncer struct {
	C            <-chan time.Time
	c            chan time.Time
	settlePeriod time.Duration
	settling     bool
}

func NewDebouncer(d time.Duration) *Debouncer {
	c := make(chan time.Time, 1)
	return &Debouncer{
		C:            c,
		c:            c,
		settlePeriod: d,
		settling:     false,
	}
}

func (d *Debouncer) Trigger() {
	if d.settling {
		return
	}
	d.settling = true
	time.AfterFunc(d.settlePeriod, func() {
		d.settling = false
	})
	// Non-blocking send of time on c.
	select {
	case d.c <- time.Now():
	default:
	}
}
