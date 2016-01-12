package chronometer

// NOTE: ALL TIMES ARE IN UTC. JUST USE UTC.

import (
	"sync"
	"time"

	"github.com/blendlabs/go-exception"
)

const (
	HEARTBEAT_INTERVAL = 50 * time.Millisecond
)

func NewJobManager() *JobManager {
	jm := JobManager{}

	jm.LoadedJobs = map[string]Job{}
	jm.RunningTasks = map[string]Task{}
	jm.Schedules = map[string]Schedule{}

	jm.cancellationTokens = map[string]*CancellationToken{}
	jm.runningTaskStartTimes = map[string]time.Time{}
	jm.lastRunTimes = map[string]time.Time{}
	jm.nextRunTimes = map[string]time.Time{}

	jm.metaLock = &sync.Mutex{}
	return &jm
}

var _default *JobManager
var _defaultLock = &sync.Mutex{}

func Default() *JobManager {
	if _default == nil {
		_defaultLock.Lock()
		defer _defaultLock.Unlock()

		if _default == nil {
			_default = NewJobManager()
		}
	}
	return _default
}

type JobManager struct {
	LoadedJobs   map[string]Job
	RunningTasks map[string]Task
	Schedules    map[string]Schedule

	cancellationTokens    map[string]*CancellationToken
	runningTaskStartTimes map[string]time.Time
	lastRunTimes          map[string]time.Time
	nextRunTimes          map[string]time.Time

	isRunning      bool
	schedulerToken *CancellationToken
	metaLock       *sync.Mutex
}

func (jm *JobManager) createCancellationToken() *CancellationToken {
	return &CancellationToken{}
}

func (jm *JobManager) LoadJob(j Job) error {
	if _, hasJob := jm.LoadedJobs[j.Name()]; hasJob {
		return exception.New("Job name `%s` already loaded.", j.Name())
	}

	jm.metaLock.Lock()
	defer jm.metaLock.Unlock()

	jobName := j.Name()
	jobSchedule := j.Schedule()

	jm.LoadedJobs[jobName] = j
	jm.Schedules[jobName] = jobSchedule
	jm.nextRunTimes[jobName] = jobSchedule.GetNextRunTime(nil)

	return nil
}

func (jm *JobManager) RunJob(jobName string) error {
	if job, hasJob := jm.LoadedJobs[jobName]; hasJob {
		now := time.Now().UTC()

		jm.lastRunTimes[jobName] = now

		return jm.RunTask(job)
	}
	return exception.Newf("Job name `%s` not found.", jobName)
}

func (jm *JobManager) RunTask(t Task) error {
	jm.metaLock.Lock()
	defer jm.metaLock.Unlock()

	taskName := t.Name()
	ct := jm.createCancellationToken()

	jm.RunningTasks[taskName] = t

	jm.cancellationTokens[taskName] = ct
	jm.runningTaskStartTimes[taskName] = time.Now().UTC()

	go func() {
		defer jm.cleanupTask(taskName)

		if signalReceiver, isSignalReceiver := t.(OnStartSignalReceiver); isSignalReceiver {
			signalReceiver.OnStart()
		}

		if ct.ShouldCancel {
			if signalReceiver, isSignalReceiver := t.(OnCancellationSignalReceiver); isSignalReceiver {
				signalReceiver.OnCancellation()
			}
			return
		}
		result := t.Execute(ct)
		if ct.ShouldCancel {
			if signalReceiver, isSignalReceiver := t.(OnCancellationSignalReceiver); isSignalReceiver {
				signalReceiver.OnCancellation()
			}
			return
		}
		if signalReceiver, isSignalReceiver := t.(OnCompleteSignalReceiver); isSignalReceiver {
			signalReceiver.OnComplete(result)
		}
	}()
	return nil
}

func (jm *JobManager) cleanupTask(taskName string) {
	jm.metaLock.Lock()
	defer jm.metaLock.Unlock()

	delete(jm.runningTaskStartTimes, taskName)
	delete(jm.RunningTasks, taskName)
	delete(jm.cancellationTokens, taskName)
}

func (jm *JobManager) CancelTask(taskName string) error {
	if _, hasJob := jm.LoadedJobs[taskName]; hasJob {
		if token, hasCancellationToken := jm.cancellationTokens[taskName]; hasCancellationToken {
			token.signalCancellation()
		} else {
			return exception.Newf("Cancellation token for job name `%s` not found.", taskName)
		}
	}
	return exception.Newf("Job name `%s` not found.", taskName)
}

func (jm *JobManager) Start() {
	ct := jm.createCancellationToken()
	jm.schedulerToken = ct
	go jm.schedule(ct)
	jm.isRunning = true
}

func (jm *JobManager) Stop() {
	if !jm.isRunning {
		return
	}
	jm.schedulerToken.signalCancellation()
	jm.isRunning = false
}

func (jm *JobManager) schedule(ct *CancellationToken) {
	for !ct.ShouldCancel {
		now := time.Now().UTC()

		for jobName, nextRunTime := range jm.nextRunTimes {
			if nextRunTime.Sub(now) < HEARTBEAT_INTERVAL {
				jm.RunJob(jobName)
				jm.nextRunTimes[jobName] = jm.Schedules[jobName].GetNextRunTime(&now)
			}
		}

		time.Sleep(HEARTBEAT_INTERVAL)
	}
}
