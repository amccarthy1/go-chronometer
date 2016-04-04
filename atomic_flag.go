package chronometer

import "sync"

// AtomicFlag is a boolean value that is syncronized.
type AtomicFlag struct {
	lock  sync.RWMutex
	value bool
}

// Set the flag value.
func (af *AtomicFlag) Set(value bool) {
	af.lock.Lock()
	defer af.lock.Unlock()
	af.value = value
}

// Get the flag value.
func (af *AtomicFlag) Get() bool {
	af.lock.RLock()
	defer af.lock.RUnlock()
	return af.value
}
