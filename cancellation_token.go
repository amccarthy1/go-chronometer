package chronometer

func NewCancellationToken() *CancellationToken {
	return &CancellationToken{ShouldCancel: false}
}

type CancellationToken struct {
	ShouldCancel bool
}

func (ct *CancellationToken) Cancel() {
	ct.ShouldCancel = true
}
