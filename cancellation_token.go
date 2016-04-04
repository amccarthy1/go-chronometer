package chronometer

import (
	"sync"

	"github.com/blendlabs/go-exception"
)

// NewCancellationToken returns a new CancellationToken instance.
func NewCancellationToken() *CancellationToken {
	return &CancellationToken{
		shouldCancel:       false,
		shouldCancelLock:   sync.RWMutex{},
		didCancel:          false,
		didCancelLock:      sync.RWMutex{},
		cancellationSignal: make(chan bool, 1),
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
	cancellationSignal chan bool

	shouldCancel     bool
	shouldCancelLock sync.RWMutex

	didCancel     bool
	didCancelLock sync.RWMutex
}

// Cancel signals cancellation.
func (ct *CancellationToken) Cancel() {
	ct.shouldCancelLock.Lock()
	defer ct.shouldCancelLock.Unlock()
	ct.shouldCancel = true
	ct.cancellationSignal <- true
}

// ShouldCancel indicates if a token has been signaled to cancel.
func (ct *CancellationToken) ShouldCancel() bool {
	ct.shouldCancelLock.RLock()
	defer ct.shouldCancelLock.RUnlock()
	return ct.shouldCancel
}

// DidCancel indicates if the token canceled gracefully or not.
func (ct *CancellationToken) DidCancel() bool {
	ct.didCancelLock.RLock()
	defer ct.didCancelLock.RUnlock()
	return ct.didCancel
}

// CanceledGracefully should be called on task return to indicate that the
// Task was cancelled gracefully.
func (ct *CancellationToken) CanceledGracefully() error {
	ct.didCancelLock.Lock()
	defer ct.didCancelLock.Unlock()
	ct.didCancel = true
	return nil
}
