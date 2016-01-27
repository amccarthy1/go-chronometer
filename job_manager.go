package chronometer

// NOTE: ALL TIMES ARE IN UTC. JUST USE UTC.

import (
	"fmt"
	"sync"
	"time"

	"github.com/blendlabs/go-exception"
)

const (
	HEARTBEAT_INTERVAL = 250 * time.Millisecond
	STATE_RUNNING      = "running"
	STATE_STOPPED      = "stopped"
)

func NewJobManager() *JobManager {
	jm := JobManager{}

	jm.LoadedJobs = map[string]Job{}
	jm.RunningTasks = map[string]Task{}
	jm.Schedules = map[string]Schedule{}

	jm.CancellationTokens = map[string]*CancellationToken{}
	jm.RunningTaskStartTimes = map[string]time.Time{}
	jm.LastRunTimes = map[string]time.Time{}
	jm.NextRunTimes = map[string]time.Time{}

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

	CancellationTokens    map[string]*CancellationToken
	RunningTaskStartTimes map[string]time.Time
	LastRunTimes          map[string]time.Time
	NextRunTimes          map[string]time.Time

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
	jm.NextRunTimes[jobName] = jobSchedule.GetNextRunTime(nil)

	return nil
}

func (jm *JobManager) RunJob(jobName string) error {
	if job, hasJob := jm.LoadedJobs[jobName]; hasJob {
		now := time.Now().UTC()

		jm.LastRunTimes[jobName] = now

		return jm.RunTask(job)
	}
	return exception.Newf("Job name `%s` not found.", jobName)
}

func (jm *JobManager) RunAllJobs() error {
	now := time.Now().UTC()
	for jobName, job := range jm.LoadedJobs {
		jm.LastRunTimes[jobName] = now
		job_err := jm.RunTask(job)
		if job_err != nil {
			return job_err
		}
	}
	return nil
}

func (jm *JobManager) RunTask(t Task) error {
	jm.metaLock.Lock()
	defer jm.metaLock.Unlock()

	taskName := t.Name()
	ct := jm.createCancellationToken()

	jm.RunningTasks[taskName] = t

	jm.CancellationTokens[taskName] = ct
	jm.RunningTaskStartTimes[taskName] = time.Now().UTC()

	go func() {
		defer jm.cleanupTask(taskName)

		if receiver, isReceiver := t.(OnStartReceiver); isReceiver {
			receiver.OnStart()
		}

		if ct.ShouldCancel {
			if receiver, isReceiver := t.(OnCancellationReceiver); isReceiver {
				receiver.OnCancellation()
			}
			return
		}
		result := t.Execute(ct)
		if ct.ShouldCancel {
			if receiver, isReceiver := t.(OnCancellationReceiver); isReceiver {
				receiver.OnCancellation()
			}
			return
		}
		if receiver, isReceiver := t.(OnCompleteReceiver); isReceiver {
			receiver.OnComplete(result)
		}
	}()
	return nil
}

func (jm *JobManager) cleanupTask(taskName string) {
	jm.metaLock.Lock()
	defer jm.metaLock.Unlock()

	delete(jm.RunningTaskStartTimes, taskName)
	delete(jm.RunningTasks, taskName)
	delete(jm.CancellationTokens, taskName)
}

func (jm *JobManager) CancelTask(taskName string) error {
	if task, hasTask := jm.RunningTasks[taskName]; hasTask {
		if token, hasCancellationToken := jm.CancellationTokens[taskName]; hasCancellationToken {
			jm.cleanupTask(taskName)
			if receiver, isReceiver := task.(OnCancellationReceiver); isReceiver {
				receiver.OnCancellation()
			}
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
	go jm.runDueJobs(ct)
	go jm.killHangingJobs(ct)
	jm.isRunning = true
}

func (jm *JobManager) Stop() {
	if !jm.isRunning {
		return
	}
	jm.schedulerToken.signalCancellation()
	jm.isRunning = false
}

func (jm *JobManager) runDueJobs(ct *CancellationToken) {
	for !ct.ShouldCancel {
		now := time.Now().UTC()

		for jobName, nextRunTime := range jm.NextRunTimes {
			if nextRunTime.Before(now) {
				jm.metaLock.Lock()
				jm.NextRunTimes[jobName] = jm.Schedules[jobName].GetNextRunTime(&now)
				jm.metaLock.Unlock()

				jm.RunJob(jobName)
			}
		}

		time.Sleep(HEARTBEAT_INTERVAL)
	}
}

func (jm *JobManager) killHangingJobs(ct *CancellationToken) {
	for !ct.ShouldCancel {
		now := time.Now().UTC()

		for taskName, startedTime := range jm.RunningTaskStartTimes {
			if task, hasTask := jm.RunningTasks[taskName]; hasTask {
				if timeoutProvider, isTimeoutProvder := task.(TimeoutProvider); isTimeoutProvder {
					timeout := timeoutProvider.Timeout()
					if now.Sub(startedTime) >= timeout {
						jm.CancelTask(taskName)
						if receiver, isReceiver := task.(OnCompleteReceiver); isReceiver {
							receiver.OnComplete(exception.New("Timeout Reached."))
						}
					}
				}
			}
		}
		time.Sleep(HEARTBEAT_INTERVAL)
	}
}

func (jm *JobManager) Status() []TaskStatus {
	statuses := []TaskStatus{}
	now := time.Now().UTC()
	for jobName, job := range jm.LoadedJobs {
		status := TaskStatus{}
		status.Name = jobName

		if runningSince, isRunning := jm.RunningTaskStartTimes[jobName]; isRunning {
			status.State = STATE_RUNNING
			status.RunningFor = fmt.Sprintf("%v", now.Sub(runningSince))
		} else {
			status.State = STATE_STOPPED
		}

		if statusProvider, isStatusProvider := job.(StatusProvider); isStatusProvider {
			status.Status = statusProvider.Status()
		}

		statuses = append(statuses, status)
	}

	for taskName, task := range jm.RunningTasks {
		if _, isJob := jm.LoadedJobs[taskName]; !isJob {
			status := TaskStatus{}
			status.Name = taskName
			status.State = STATE_RUNNING
			if runningSince, isRunning := jm.RunningTaskStartTimes[taskName]; isRunning {
				status.RunningFor = fmt.Sprintf("%v", now.Sub(runningSince))
			}
			if statusProvider, isStatusProvider := task.(StatusProvider); isStatusProvider {
				status.Status = statusProvider.Status()
			}
			statuses = append(statuses, status)
		}
	}
	return statuses
}

func (jm *JobManager) TaskStatus(taskName string) *TaskStatus {
	if task, isRunning := jm.RunningTasks[taskName]; isRunning {
		now := time.Now().UTC()
		status := TaskStatus{}
		status.Name = taskName
		status.State = STATE_RUNNING
		if runningSince, isRunning := jm.RunningTaskStartTimes[taskName]; isRunning {
			status.RunningFor = fmt.Sprintf("%v", now.Sub(runningSince))
		}
		if statusProvider, isStatusProvider := task.(StatusProvider); isStatusProvider {
			status.Status = statusProvider.Status()
		}
		return &status
	}
	return nil
}
