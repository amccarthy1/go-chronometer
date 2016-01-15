package chronometer

import "github.com/blendlabs/go-exception"

func NewCancellationToken() *CancellationToken {
	return &CancellationToken{ShouldCancel: false, didCancel: false}
}

type CancellationToken struct {
	ShouldCancel bool
	didCancel    bool
}

func (ct *CancellationToken) signalCancellation() {
	ct.ShouldCancel = true
	ct.didCancel = false
}

func (ct *CancellationToken) Cancel() error {
	ct.didCancel = true
	return exception.New("Task Cancellation")
}
