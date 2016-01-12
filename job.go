package chronometer

type Job interface {
	Name() string
	Schedule() Schedule
	Execute(ct *CancellationToken) error
}
