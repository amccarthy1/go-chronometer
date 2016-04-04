package chronometer

import (
	"sync"

	"github.com/blendlabs/go-exception"
)

// NewCancellationToken returns a new CancellationToken instance.
func NewCancellationToken() *CancellationToken {
	return &CancellationToken{
		shouldCancel:     false,
		shouldCancelLock: sync.RWMutex{},
	}
}

// CancellationPanic is the panic that gets raised when tasks are canceled.
type CancellationPanic error

// NewCancellationPanic returns a new cancellation exception.
func NewCancellationPanic() error {
	return CancellationPanic(exception.New("Cancellation grace period expired."))
}

// CancellationToken are the signalling mechanism chronometer uses to tell tasks that they should stop work.
type CancellationToken struct {
	shouldCancel     bool
	shouldCancelLock sync.RWMutex
}

// Cancel signals cancellation.
func (ct *CancellationToken) Cancel() {
	ct.shouldCancelLock.Lock()
	defer ct.shouldCancelLock.Unlock()
	ct.shouldCancel = true
}

func (ct *CancellationToken) didCancel() bool {
	ct.shouldCancelLock.RLock()
	defer ct.shouldCancelLock.RUnlock()

	return ct.shouldCancel
}

// CheckCancellation indicates if a token has been signaled to cancel.
func (ct *CancellationToken) CheckCancellation() {
	ct.shouldCancelLock.RLock()
	defer ct.shouldCancelLock.RUnlock()
	panic(NewCancellationPanic())
}
