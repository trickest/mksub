package roundChan

import (
	"sync/atomic"
)

// RoundRobin is an interface for representing round-robin balancing.
type RoundRobin interface {
	Next() *chan string
	Add(*chan string)
}

type roundRobin struct {
	chs  []*chan string
	next uint32
}

// New returns RoundRobin implementation(*roundRobin).
func New(chs ...*chan string) RoundRobin {
	return &roundRobin{
		chs: chs,
	}
}

// Next returns next channel
func (r *roundRobin) Next() *chan string {
	n := atomic.AddUint32(&r.next, 1)
	return r.chs[(int(n)-1)%len(r.chs)]
}

// Add adds a channel
func (r *roundRobin) Add(ch *chan string) {
	r.chs = append(r.chs, ch)
}
