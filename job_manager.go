package chronometer

// NOTE: ALL TIMES ARE IN UTC. JUST USE UTC.

import (
	"sync"
	"time"

	"github.com/blendlabs/go-exception"
)

const (
	HEARTBEAT_INTERVAL = 250 * time.Millisecond
)

func NewJobManager() *JobManager {
	jm := JobManager{}
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

	go func() {
		defer jm.cleanupTask(taskName)

		t.OnStart()
		if ct.ShouldCancel {
			t.OnCancellation()
			return
		}
		result := t.Execute(ct)
		if ct.ShouldCancel {
			t.OnCancellation()
			return
		}
		t.OnComplete(result)
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
			token.Cancel()
		} else {
			return exception.Newf("Cancellation token for job name `%s` not found.", taskName)
		}
	}
	return exception.Newf("Job name `%s` not found.", taskName)
}

func (jm *JobManager) Start() {
	ct := jm.createCancellationToken()
	jm.schedulerToken = ct
	go jm.schedule()
	jm.isRunning = true
}

func (jm *JobManager) Stop() {
	if !jm.isRunning {
		return
	}
	jm.schedulerToken.Cancel()
	jm.isRunning = false
}

func (jm *JobManager) schedule(ct *CancellationToken) {
	for !ct.ShouldCancel {
		now := time.Now().UTC()

		for jobName, nextRunTime := range jm.nextRunTimes {
			if nextRunTime.Sub(now) < HEARTBEAT_INTERVAL {
				jm.RunJob(jobName)
				jm.nextRunTimes[jobName] = jm.Schedules[jobName].GetNextRunTime(now)
			}
		}

		time.Sleep(HEARTBEAT_INTERVAL)
	}
}
