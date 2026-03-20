package app

import "sync/atomic"

type Lifecycle struct {
	live         atomic.Bool
	ready        atomic.Bool
	shuttingDown atomic.Bool
}

func NewLifecycle() *Lifecycle {
	l := &Lifecycle{}
	l.live.Store(true)
	return l
}

func (l *Lifecycle) SetReady(ready bool) {
	l.ready.Store(ready)
}

func (l *Lifecycle) StartShutdown() {
	l.ready.Store(false)
	l.shuttingDown.Store(true)
}

func (l *Lifecycle) IsLive() bool {
	return l.live.Load()
}

func (l *Lifecycle) IsReady() bool {
	return l.ready.Load() && !l.shuttingDown.Load()
}
