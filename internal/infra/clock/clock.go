package clock

import "time"

type Clock struct{}

func (Clock) Now() time.Time { return time.Now() }

type Ticker struct {
	*time.Ticker
}

func NewTicker(interval time.Duration) *Ticker {
	return &Ticker{Ticker: time.NewTicker(interval)}
}

func (t *Ticker) Ticks() <-chan time.Time { return t.C }
