package chronometer

import logger "github.com/blendlabs/go-logger"

// Job is an interface structs can satisfy to be loaded into the JobManager.
type Job interface {
	Name() string
	Schedule() Schedule
	Execute(ct *CancellationToken) error
}

// BaseJob is a base class for jobs.
type BaseJob struct {
	name string
}

// Name returns the job name
func (bj BaseJob) Name() string {
	return bj.name
}

// SetName sets the job name
func (bj *BaseJob) SetName(name string) {
	bj.name = name
}

// OnStart logs the start event for the job.
func (bj BaseJob) OnStart() {
	logger.Diagnostics().Infof("Job `%s` starting.", bj.Name())
}

// Schedule returns the job schedule.
func (bj BaseJob) Schedule() Schedule {
	return OnDemand()
}

// Execute is the job body.
func (bj BaseJob) Execute(ct *CancellationToken) error {
	// no op
	return nil
}

// OnComplete logs success or failure on completion
func (bj BaseJob) OnComplete(err error) {
	if err == nil {
		logger.Diagnostics().Infof("Job `%s` complete.", bj.Name())
	} else {
		logger.Diagnostics().Infof("Job `%s` failed.", bj.Name())
		logger.Diagnostics().Error(err)
	}
}
