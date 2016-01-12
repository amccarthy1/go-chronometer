package chronometer

func NewCancellationToken() *CancellationToken {
	return &CancellationToken{ShouldCancel: false}
}

func NewCancellationTokenWithReceiver(receiver CancellationSignalReciever) *CancellationToken {
	return &CancellationToken{ShouldCancel: false, onCancellation: receiver}
}

type CancellationToken struct {
	ShouldCancel   bool
	onCancellation CancellationSignalReciever
}

func (ct *CancellationToken) signalCancellation() {
	ct.ShouldCancel = true
}

func (ct *CancellationToken) Cancel() error {
	if ct.onCancellation != nil {
		ct.onCancellation()
	}
	return nil
}
