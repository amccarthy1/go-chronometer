package chronometer

// NOTE: ALL TIMES ARE IN UTC. JUST USE UTC.

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/blendlabs/go-exception"
	logger "github.com/blendlabs/go-logger"
	"github.com/blendlabs/go-util/collections"
)

// State is a job state.
type State string

const (
	// HeartbeatInterval is the interval between schedule next run checks.
	HeartbeatInterval = 250 * time.Millisecond

	// HangingHeartbeatInterval is the interval between schedule next run checks.
	HangingHeartbeatInterval = 333 * time.Millisecond

	//StateRunning is the running state.
	StateRunning State = "running"

	// StateEnabled is the enabled state.
	StateEnabled State = "enabled"

	// StateDisabled is the disabled state.
	StateDisabled State = "disabled"
)

// New returns a new job manager.
func New() *JobManager {
	jm := JobManager{
		loadedJobs:            map[string]Job{},
		runningTasks:          map[string]Task{},
		schedules:             map[string]Schedule{},
		contexts:              map[string]context.Context{},
		cancels:               map[string]context.CancelFunc{},
		runningTaskStartTimes: map[string]time.Time{},
		lastRunTimes:          map[string]time.Time{},
		nextRunTimes:          map[string]*time.Time{},
		disabledJobs:          collections.SetOfString{},
		enabledProviders:      map[string]func() bool{},
	}

	return &jm
}

var _default *JobManager
var _defaultLock = &sync.Mutex{}

// Default returns a shared instance of a JobManager.
func Default() *JobManager {
	if _default == nil {
		_defaultLock.Lock()
		defer _defaultLock.Unlock()

		if _default == nil {
			_default = New()
		}
	}
	return _default
}

// JobManager is the main orchestration and job management object.
type JobManager struct {
	loadedJobsLock sync.Mutex
	loadedJobs     map[string]Job

	disabledJobsLock sync.Mutex
	disabledJobs     collections.SetOfString

	enabledProvidersLock sync.Mutex
	enabledProviders     map[string]func() bool

	runningTasksLock sync.Mutex
	runningTasks     map[string]Task

	schedulesLock sync.Mutex
	schedules     map[string]Schedule

	runningTaskStartTimesLock sync.Mutex
	runningTaskStartTimes     map[string]time.Time

	contextsLock sync.Mutex
	contexts     map[string]context.Context

	cancelsLock sync.Mutex
	cancels     map[string]context.CancelFunc

	lastRunTimesLock sync.Mutex
	lastRunTimes     map[string]time.Time

	nextRunTimesLock sync.Mutex
	nextRunTimes     map[string]*time.Time

	schedulerCancel context.CancelFunc
	isRunning       bool

	logger *logger.Agent
}

// Logger returns the diagnostics agent.
func (jm *JobManager) Logger() *logger.Agent {
	return jm.logger
}

// SetLogger sets the diagnostics agent.
func (jm *JobManager) SetLogger(agent *logger.Agent) {
	jm.logger = agent

	if jm.logger != nil {
		jm.logger.AddEventListener(EventTask, NewTaskListener(jm.taskListener))
		jm.logger.AddEventListener(EventTaskComplete, NewTaskCompleteListener(jm.taskCompleteListener))
	}
}

// ShouldShowMessagesFor is a helper function to determine if we should show messages for a
// given task name.
func (jm *JobManager) ShouldShowMessagesFor(taskName string) bool {
	jm.loadedJobsLock.Lock()
	defer jm.loadedJobsLock.Unlock()

	if job, hasJob := jm.loadedJobs[taskName]; hasJob {
		if typed, isTyped := job.(ShowMessagesProvider); isTyped {
			return typed.ShowMessages()
		}
	}

	return true
}

func (jm *JobManager) taskListener(wr *logger.Writer, ts logger.TimeSource, taskName string) {
	if jm.ShouldShowMessagesFor(taskName) {
		logger.WriteEventf(wr, ts, EventTask, logger.ColorBlue, "`%s` starting", taskName)
	}
}

func (jm *JobManager) taskCompleteListener(wr *logger.Writer, ts logger.TimeSource, taskName string, elapsed time.Duration, err error) {
	if jm.ShouldShowMessagesFor(taskName) {
		if err != nil {
			logger.WriteEventf(wr, ts, EventTaskComplete, logger.ColorRed, "`%s` failed %v", taskName, elapsed)
		} else {
			logger.WriteEventf(wr, ts, EventTaskComplete, logger.ColorBlue, "`%s` completed %v", taskName, elapsed)
		}
	}
}

// fireTaskListeners fires the currently configured task listeners.
func (jm *JobManager) fireTaskListeners(taskName string) {
	if jm.logger == nil {
		return
	}
	jm.logger.OnEvent(EventTask, taskName)
}

// fireTaskListeners fires the currently configured task listeners.
func (jm *JobManager) fireTaskCompleteListeners(taskName string, elapsed time.Duration, err error) {
	if jm.logger == nil {
		return
	}
	jm.logger.OnEvent(EventTaskComplete, taskName, elapsed, err)
	if err != nil {
		jm.logger.OnEvent(logger.EventError, err)
	}
}

// --------------------------------------------------------------------------------
// Informational Methods
// --------------------------------------------------------------------------------

// HasJob returns if a jobName is loaded or not.
func (jm *JobManager) HasJob(jobName string) bool {
	jm.loadedJobsLock.Lock()
	_, hasJob := jm.loadedJobs[jobName]
	jm.loadedJobsLock.Unlock()
	return hasJob
}

// IsDisabled returns if a job is disabled.
func (jm *JobManager) IsDisabled(jobName string) (value bool) {
	jm.disabledJobsLock.Lock()
	value = jm.disabledJobs.Contains(jobName)
	jm.disabledJobsLock.Unlock()

	jm.enabledProvidersLock.Lock()
	if provider, hasProvider := jm.enabledProviders[jobName]; hasProvider {
		value = value || !provider()
	}
	jm.enabledProvidersLock.Unlock()
	return
}

// IsRunning returns if a task is currently running.
func (jm *JobManager) IsRunning(taskName string) bool {
	jm.runningTasksLock.Lock()
	_, isRunning := jm.runningTasks[taskName]
	jm.runningTasksLock.Unlock()
	return isRunning
}

// ReadAllJobs allows the consumer to do something with the full job list, using a read lock.
func (jm *JobManager) ReadAllJobs(action func(jobs map[string]Job)) {
	jm.loadedJobsLock.Lock()
	defer jm.loadedJobsLock.Unlock()
	action(jm.loadedJobs)
}

// --------------------------------------------------------------------------------
// Core Methods
// --------------------------------------------------------------------------------

// LoadJob adds a job to the manager.
func (jm *JobManager) LoadJob(j Job) error {
	jobName := j.Name()

	if jm.HasJob(jobName) {
		return exception.Newf("Job name `%s` already loaded.", j.Name())
	}

	jm.setLoadedJob(jobName, j)
	jobSchedule := j.Schedule()
	jm.setSchedule(jobName, jobSchedule)
	jm.setNextRunTime(jobName, jobSchedule.GetNextRunTime(nil))
	jm.setEnabledProvider(jobName, j)
	return nil
}

// DisableJob stops a job from running but does not unload it.
func (jm *JobManager) DisableJob(jobName string) error {
	if !jm.HasJob(jobName) {
		return exception.Newf("Job name `%s` isn't loaded.", jobName)
	}

	jm.setDisabledJob(jobName)
	jm.deleteNextRunTime(jobName)
	return nil
}

// EnableJob enables a job that has been disabled.
func (jm *JobManager) EnableJob(jobName string) error {
	if !jm.HasJob(jobName) {
		return exception.Newf("Job name `%s` isn't loaded.", jobName)
	}

	jm.deleteDisabledJob(jobName)
	job := jm.getLoadedJob(jobName)
	jobSchedule := job.Schedule()
	jm.setNextRunTime(jobName, jobSchedule.GetNextRunTime(nil))

	return nil
}

func (jm *JobManager) showJobMessages(job Job) bool {
	hasDiagnostics := jm.logger != nil
	if showMessagesProvider, isShowMessagesProvider := job.(ShowMessagesProvider); isShowMessagesProvider {
		return hasDiagnostics && showMessagesProvider.ShowMessages()
	}
	return hasDiagnostics
}

// RunJob runs a job by jobName on demand.
func (jm *JobManager) RunJob(jobName string) error {
	jm.loadedJobsLock.Lock()
	defer jm.loadedJobsLock.Unlock()

	jm.disabledJobsLock.Lock()
	defer jm.disabledJobsLock.Unlock()

	if job, hasJob := jm.loadedJobs[jobName]; hasJob {
		if !jm.disabledJobs.Contains(jobName) {
			now := time.Now().UTC()
			jm.setLastRunTime(jobName, now)
			err := jm.RunTask(job)
			return err
		}
		return nil
	}
	return exception.Newf("Job name `%s` not found.", jobName)
}

// RunAllJobs runs every job that has been loaded in the JobManager at once.
func (jm *JobManager) RunAllJobs() error {
	jm.loadedJobsLock.Lock()
	defer jm.loadedJobsLock.Unlock()

	for jobName, job := range jm.loadedJobs {
		if !jm.IsDisabled(jobName) {
			jobErr := jm.RunTask(job)
			if jobErr != nil {
				return jobErr
			}
		}
	}
	return nil
}

func (jm *JobManager) createContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

// RunTask runs a task on demand.
func (jm *JobManager) RunTask(t Task) error {
	taskName := t.Name()
	start := Now()
	ctx, cancel := jm.createContext()

	jm.setRunningTask(taskName, t)
	jm.setContext(ctx, taskName)
	jm.setCancelFunc(taskName, cancel)
	jm.setRunningTaskStartTime(taskName, start)
	jm.setLastRunTime(taskName, start)

	// this is the main goroutine that runs the task
	go func() {
		var err error

		defer func() {
			jm.cleanupTask(taskName)
			jm.fireTaskCompleteListeners(taskName, Since(start), err)
		}()

		// panic recovery
		defer func() {
			if r := recover(); r != nil {
				err = exception.Newf("%v", r)
			}
		}()

		jm.onTaskStart(t)
		jm.fireTaskListeners(taskName)
		err = t.Execute(ctx)
		jm.onTaskComplete(t, err)
	}()

	return nil
}

func (jm *JobManager) onTaskStart(t Task) {
	if receiver, isReceiver := t.(OnStartReceiver); isReceiver {
		receiver.OnStart()
	}
}

func (jm *JobManager) onTaskComplete(t Task, result error) {
	if receiver, isReceiver := t.(OnCompleteReceiver); isReceiver {
		receiver.OnComplete(result)
	}
}

func (jm *JobManager) onTaskCancellation(t Task) {
	if receiver, isReceiver := t.(OnCancellationReceiver); isReceiver {
		receiver.OnCancellation()
	}
}

func (jm *JobManager) cleanupTask(taskName string) {
	jm.deleteRunningTaskStartTime(taskName)
	jm.deleteRunningTask(taskName)
	jm.deleteContext(taskName)
	jm.deleteCancelFunc(taskName)
}

// CancelTask cancels (sends the cancellation signal) to a running task.
func (jm *JobManager) CancelTask(taskName string) error {
	jm.runningTasksLock.Lock()
	defer jm.runningTasksLock.Unlock()

	if task, hasTask := jm.runningTasks[taskName]; hasTask {
		cancel := jm.getCancelFunc(taskName)
		jm.onTaskCancellation(task)
		cancel()
	}
	return exception.Newf("Task name `%s` not found.", taskName)
}

// Start begins the schedule runner for a JobManager.
func (jm *JobManager) Start() {
	ctx, cancel := jm.createContext()
	jm.schedulerCancel = cancel

	go jm.runDueJobs(ctx)
	go jm.killHangingJobs(ctx)

	jm.isRunning = true
}

// Stop stops the schedule runner for a JobManager.
func (jm *JobManager) Stop() {
	if !jm.isRunning {
		return
	}
	jm.schedulerCancel()
	jm.isRunning = false
}

func (jm *JobManager) runDueJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			jm.runDueJobsInner()
			time.Sleep(HeartbeatInterval)
		}
	}
}

func (jm *JobManager) runDueJobsInner() {
	jm.nextRunTimesLock.Lock()
	defer jm.nextRunTimesLock.Unlock()

	now := Now()

	for jobName, nextRunTime := range jm.nextRunTimes {
		if nextRunTime != nil {
			if !jm.IsDisabled(jobName) {
				if nextRunTime.Before(now) {
					jm.nextRunTimes[jobName] = jm.getSchedule(jobName).GetNextRunTime(&now)
					jm.RunJob(jobName)
				}
			}
		}
	}
}

func (jm *JobManager) killHangingJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			jm.killHangingJobsInner()
			time.Sleep(HangingHeartbeatInterval)
		}
	}
}

func (jm *JobManager) killHangingJobsInner() {
	jm.runningTasksLock.Lock()
	defer jm.runningTasksLock.Unlock()

	jm.runningTaskStartTimesLock.Lock()
	defer jm.runningTaskStartTimesLock.Unlock()

	jm.cancelsLock.Lock()
	defer jm.cancelsLock.Unlock()

	now := Now()
	for taskName, startedTime := range jm.runningTaskStartTimes {
		if task, hasTask := jm.runningTasks[taskName]; hasTask {
			if timeoutProvider, isTimeoutProvder := task.(TimeoutProvider); isTimeoutProvder {
				timeout := timeoutProvider.Timeout()
				if now.Sub(startedTime) >= timeout {
					jm.killHangingJob(taskName)
				}
			}
		}
	}
}

// killHangingJob cancels (sends the cancellation signal) to a running task that has exceeded its timeout.
// it assumes that the following locks are held:
// - runningTasksLock
// - runningTaskStartTimesLock
// - contextsLock
// otherwise, chaos, mayhem, deadlocks. You should *rarely* need to call this explicitly.
func (jm *JobManager) killHangingJob(taskName string) error {
	if _, hasTask := jm.runningTasks[taskName]; hasTask {
		if cancel, hasCancel := jm.cancels[taskName]; hasCancel {
			cancel()

			delete(jm.runningTasks, taskName)
			delete(jm.runningTaskStartTimes, taskName)
			delete(jm.contexts, taskName)
			delete(jm.cancels, taskName)
		}
	}
	return exception.Newf("Task name `%s` not found.", taskName)
}

// Status returns the status metadata for a JobManager
func (jm *JobManager) Status() []TaskStatus {

	jm.loadedJobsLock.Lock()
	defer jm.loadedJobsLock.Unlock()

	jm.runningTaskStartTimesLock.Lock()
	defer jm.runningTaskStartTimesLock.Unlock()

	jm.disabledJobsLock.Lock()
	defer jm.disabledJobsLock.Unlock()

	jm.runningTasksLock.Lock()
	defer jm.runningTasksLock.Unlock()

	jm.nextRunTimesLock.Lock()
	defer jm.nextRunTimesLock.Unlock()

	jm.lastRunTimesLock.Lock()
	defer jm.lastRunTimesLock.Unlock()

	var statuses []TaskStatus
	now := Now()
	for jobName, job := range jm.loadedJobs {
		status := TaskStatus{}
		status.Name = jobName

		if runningSince, isRunning := jm.runningTaskStartTimes[jobName]; isRunning {
			status.State = StateRunning
			status.RunningFor = fmt.Sprintf("%v", now.Sub(runningSince))
		} else if jm.disabledJobs.Contains(jobName) {
			status.State = StateDisabled
		} else {
			status.State = StateEnabled
		}

		if lastRunTime, hasLastRunTime := jm.lastRunTimes[jobName]; hasLastRunTime {
			status.LastRunTime = FormatTime(lastRunTime)
		}

		if nextRunTime, hasNextRunTime := jm.nextRunTimes[jobName]; hasNextRunTime {
			if nextRunTime != nil {
				status.NextRunTime = FormatTime(*nextRunTime)
			}
		}

		if statusProvider, isStatusProvider := job.(StatusProvider); isStatusProvider {
			providedStatus := statusProvider.Status()
			if len(providedStatus) > 0 {
				status.Status = providedStatus
			}
		}

		statuses = append(statuses, status)
	}

	for taskName, task := range jm.runningTasks {
		if _, isJob := jm.loadedJobs[taskName]; !isJob {
			status := TaskStatus{
				Name:  taskName,
				State: StateRunning,
			}
			if runningSince, isRunning := jm.runningTaskStartTimes[taskName]; isRunning {
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

// TaskStatus returns the status metadata for a given task.
func (jm *JobManager) TaskStatus(taskName string) *TaskStatus {
	jm.runningTaskStartTimesLock.Lock()
	defer jm.runningTaskStartTimesLock.Unlock()

	jm.runningTasksLock.Lock()
	defer jm.runningTasksLock.Unlock()

	if task, isRunning := jm.runningTasks[taskName]; isRunning {
		now := Now()
		status := TaskStatus{
			Name:  taskName,
			State: StateRunning,
		}
		if runningSince, isRunning := jm.runningTaskStartTimes[taskName]; isRunning {
			status.RunningFor = fmt.Sprintf("%v", now.Sub(runningSince))
		}
		if statusProvider, isStatusProvider := task.(StatusProvider); isStatusProvider {
			status.Status = statusProvider.Status()
		}
		return &status
	}
	return nil
}

// --------------------------------------------------------------------------------
// Atomic Methods
// --------------------------------------------------------------------------------

func (jm *JobManager) getContext(taskName string) (ctx context.Context) {
	jm.contextsLock.Lock()
	ctx = jm.contexts[taskName]
	jm.contextsLock.Unlock()
	return
}

// note; this setter is out of order because of the linter.
func (jm *JobManager) setContext(ctx context.Context, taskName string) {
	jm.contextsLock.Lock()
	jm.contexts[taskName] = ctx
	jm.contextsLock.Unlock()
}

func (jm *JobManager) deleteContext(taskName string) {
	jm.contextsLock.Lock()
	delete(jm.contexts, taskName)
	jm.contextsLock.Unlock()
}

func (jm *JobManager) getCancelFunc(taskName string) (cancel context.CancelFunc) {
	jm.cancelsLock.Lock()
	cancel = jm.cancels[taskName]
	jm.cancelsLock.Unlock()
	return
}

func (jm *JobManager) setCancelFunc(taskName string, cancel context.CancelFunc) {
	jm.cancelsLock.Lock()
	jm.cancels[taskName] = cancel
	jm.cancelsLock.Unlock()
}

func (jm *JobManager) deleteCancelFunc(taskName string) {
	jm.cancelsLock.Lock()
	delete(jm.cancels, taskName)
	jm.cancelsLock.Unlock()
}

func (jm *JobManager) setDisabledJob(jobName string) {
	jm.disabledJobsLock.Lock()
	jm.disabledJobs.Add(jobName)
	jm.disabledJobsLock.Unlock()
}

func (jm *JobManager) deleteDisabledJob(jobName string) {
	jm.disabledJobsLock.Lock()
	jm.disabledJobs.Remove(jobName)
	jm.disabledJobsLock.Unlock()
}

func (jm *JobManager) getNextRunTime(jobName string) (nextRunTime *time.Time) {
	jm.nextRunTimesLock.Lock()
	nextRunTime = jm.nextRunTimes[jobName]
	jm.nextRunTimesLock.Unlock()
	return
}

func (jm *JobManager) setNextRunTime(jobName string, t *time.Time) {
	jm.nextRunTimesLock.Lock()
	jm.nextRunTimes[jobName] = t
	jm.nextRunTimesLock.Unlock()
}

func (jm *JobManager) setEnabledProvider(jobName string, j Job) {
	if typed, isTyped := j.(EnabledProvider); isTyped {
		jm.enabledProvidersLock.Lock()
		jm.enabledProviders[jobName] = func() bool { return typed.Enabled() }
		jm.enabledProvidersLock.Unlock()
	}
}

func (jm *JobManager) deleteNextRunTime(jobName string) {
	jm.nextRunTimesLock.Lock()
	delete(jm.nextRunTimes, jobName)
	jm.nextRunTimesLock.Unlock()
}

func (jm *JobManager) getLastRunTime(taskName string) (lastRunTime time.Time) {
	jm.lastRunTimesLock.Lock()
	lastRunTime = jm.lastRunTimes[taskName]
	jm.lastRunTimesLock.Unlock()
	return
}

func (jm *JobManager) setLastRunTime(taskName string, t time.Time) {
	jm.lastRunTimesLock.Lock()
	jm.lastRunTimes[taskName] = t
	jm.lastRunTimesLock.Unlock()
}

func (jm *JobManager) deleteLastRunTime(taskName string) {
	jm.lastRunTimesLock.Lock()
	delete(jm.lastRunTimes, taskName)
	jm.lastRunTimesLock.Unlock()
}

func (jm *JobManager) getLoadedJob(jobName string) (job Job) {
	jm.loadedJobsLock.Lock()
	job = jm.loadedJobs[jobName]
	jm.loadedJobsLock.Unlock()
	return
}

func (jm *JobManager) setLoadedJob(jobName string, j Job) {
	jm.loadedJobsLock.Lock()
	jm.loadedJobs[jobName] = j
	jm.loadedJobsLock.Unlock()
}

func (jm *JobManager) deleteLoadedJob(jobName string) {
	jm.loadedJobsLock.Lock()
	delete(jm.loadedJobs, jobName)
	jm.loadedJobsLock.Unlock()
}

func (jm *JobManager) getRunningTask(taskName string) (task Task) {
	jm.runningTasksLock.Lock()
	task = jm.runningTasks[taskName]
	jm.runningTasksLock.Unlock()
	return
}

func (jm *JobManager) setRunningTask(taskName string, t Task) {
	jm.runningTasksLock.Lock()
	jm.runningTasks[taskName] = t
	jm.runningTasksLock.Unlock()
}

func (jm *JobManager) deleteRunningTask(taskName string) {
	jm.runningTasksLock.Lock()
	delete(jm.runningTasks, taskName)
	jm.runningTasksLock.Unlock()
}

func (jm *JobManager) getRunningTaskStartTime(taskName string) (startTime time.Time) {
	jm.runningTaskStartTimesLock.Lock()

	startTime = jm.runningTaskStartTimes[taskName]
	jm.runningTaskStartTimesLock.Unlock()
	return
}

func (jm *JobManager) setRunningTaskStartTime(taskName string, t time.Time) {
	jm.runningTaskStartTimesLock.Lock()
	jm.runningTaskStartTimes[taskName] = t
	jm.runningTaskStartTimesLock.Unlock()
}

func (jm *JobManager) deleteRunningTaskStartTime(taskName string) {
	jm.runningTaskStartTimesLock.Lock()
	delete(jm.runningTaskStartTimes, taskName)
	jm.runningTaskStartTimesLock.Unlock()
}

func (jm *JobManager) getSchedule(jobName string) (schedule Schedule) {
	jm.schedulesLock.Lock()
	schedule = jm.schedules[jobName]
	jm.schedulesLock.Unlock()
	return
}

func (jm *JobManager) setSchedule(jobName string, schedule Schedule) {
	jm.schedulesLock.Lock()
	jm.schedules[jobName] = schedule
	jm.schedulesLock.Unlock()
}

func (jm *JobManager) deleteSchedule(jobName string) {
	jm.schedulesLock.Lock()
	defer jm.schedulesLock.Unlock()

	delete(jm.schedules, jobName)
}
