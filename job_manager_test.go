package chronometer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"strings"

	"github.com/blendlabs/go-assert"
	logger "github.com/blendlabs/go-logger"
)

const (
	runAtJobName = "runAt"
)

type runAtJob struct {
	RunAt       time.Time
	RunDelegate func(ctx context.Context) error
}

type runAt time.Time

func (ra runAt) GetNextRunTime(after *time.Time) *time.Time {
	typed := time.Time(ra)
	return &typed
}

func (raj *runAtJob) Name() string {
	return "runAt"
}

func (raj *runAtJob) Schedule() Schedule {
	return runAt(raj.RunAt)
}

func (raj *runAtJob) Execute(ctx context.Context) error {
	return raj.RunDelegate(ctx)
}

type testJobWithTimeout struct {
	RunAt                time.Time
	TimeoutDuration      time.Duration
	RunDelegate          func(ctx context.Context) error
	CancellationDelegate func()
}

func (tj *testJobWithTimeout) Name() string {
	return "testJobWithTimeout"
}

func (tj *testJobWithTimeout) Timeout() time.Duration {
	return tj.TimeoutDuration
}

func (tj *testJobWithTimeout) Schedule() Schedule {
	return Immediately()
}

func (tj *testJobWithTimeout) Execute(ctx context.Context) error {
	return tj.RunDelegate(ctx)
}

func (tj *testJobWithTimeout) OnCancellation() {
	tj.CancellationDelegate()
}

type testJobInterval struct {
	RunEvery    time.Duration
	RunDelegate func(ctx context.Context) error
}

func (tj *testJobInterval) Name() string {
	return "testJobInterval"
}

func (tj *testJobInterval) Schedule() Schedule {
	return Every(tj.RunEvery)
}

func (tj *testJobInterval) Execute(ctx context.Context) error {
	return tj.RunDelegate(ctx)
}

func TestRunTask(t *testing.T) {
	a := assert.New(t)
	a.StartTimeout(2000 * time.Millisecond)
	defer a.EndTimeout()

	jm := New().WithHighPrecisionHeartbeat()

	didRun := new(AtomicFlag)
	var runCount int32
	jm.RunTask(NewTask(func(ctx context.Context) error {
		atomic.AddInt32(&runCount, 1)
		didRun.Set(true)
		return nil
	}))

	elapsed := time.Duration(0)
	for elapsed < 1*time.Second {
		if didRun.Get() {
			break
		}

		func() {
			jm.runningTasksLock.Lock()
			defer jm.runningTasksLock.Unlock()

			jm.runningTaskStartTimesLock.Lock()
			defer jm.runningTaskStartTimesLock.Unlock()

			jm.cancelsLock.Lock()
			defer jm.cancelsLock.Unlock()

			a.Len(jm.runningTasks, 1)
			a.Len(jm.runningTaskStartTimes, 1)
			a.Len(jm.cancels, 1)
		}()

		elapsed = elapsed + 10*time.Millisecond
		time.Sleep(10 * time.Millisecond)
	}
	a.Equal(1, runCount)
	a.True(didRun.Get())
}

func TestRunTaskAndCancel(t *testing.T) {
	a := assert.New(t)
	a.StartTimeout(2000 * time.Millisecond)
	defer a.EndTimeout()

	jm := New()

	didRun := new(AtomicFlag)
	didFinish := new(AtomicFlag)
	jm.RunTask(NewTaskWithName("taskToCancel", func(ctx context.Context) error {
		defer func() {
			didFinish.Set(true)
		}()
		didRun.Set(true)
		taskElapsed := time.Duration(0)
		for taskElapsed < 1*time.Second {
			select {
			case <-ctx.Done():
				return nil
			default:
				taskElapsed = taskElapsed + 10*time.Millisecond
				time.Sleep(10 * time.Millisecond)
			}
		}

		return nil
	}))

	elapsed := time.Duration(0)
	for elapsed < 1*time.Second {
		if didRun.Get() {
			break
		}

		elapsed = elapsed + 10*time.Millisecond
		time.Sleep(10 * time.Millisecond)
	}

	jm.CancelTask("taskToCancel")
	elapsed = time.Duration(0)
	for elapsed < 1*time.Second {
		if didFinish.Get() {
			break
		}

		elapsed = elapsed + 10*time.Millisecond
		time.Sleep(10 * time.Millisecond)
	}
	a.True(didFinish.Get())
	a.True(didRun.Get())
}

func TestRunJobBySchedule(t *testing.T) {
	a := assert.New(t)
	a.StartTimeout(2000 * time.Millisecond)
	defer a.EndTimeout()

	didRun := make(chan bool)
	jm := New()
	runAt := Now().Add(jm.HeartbeatInterval())
	err := jm.LoadJob(&runAtJob{
		RunAt: runAt,
		RunDelegate: func(ctx context.Context) error {
			didRun <- true
			return nil
		},
	})
	a.Nil(err)

	jm.Start()
	defer jm.Stop()

	before := Now()
	<-didRun

	a.True(Since(before) < 2*jm.HeartbeatInterval())
}

func TestDisableJob(t *testing.T) {
	a := assert.New(t)
	a.StartTimeout(2000 * time.Millisecond)
	defer a.EndTimeout()

	didRun := new(AtomicFlag)
	runCount := new(AtomicCounter)
	jm := New()
	err := jm.LoadJob(&runAtJob{RunAt: time.Now().UTC().Add(100 * time.Millisecond), RunDelegate: func(ctx context.Context) error {
		runCount.Increment()
		didRun.Set(true)
		return nil
	}})
	a.Nil(err)

	err = jm.DisableJob(runAtJobName)
	a.Nil(err)
	a.True(jm.disabledJobs.Contains(runAtJobName))
}

func TestRunTaskAndCancelWithTimeout(t *testing.T) {
	a := assert.New(t)

	jm := New()

	start := Now()
	didRun := new(AtomicFlag)
	didCancel := new(AtomicFlag)
	cancelCount := new(AtomicCounter)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	jm.LoadJob(&testJobWithTimeout{
		RunAt:           start,
		TimeoutDuration: 250 * time.Millisecond,
		RunDelegate: func(ctx context.Context) error {
			defer wg.Done()
			didRun.Set(true)
			for !didCancel.Get() {
				select {
				case <-ctx.Done():
					didCancel.Set(true)
					return nil
				default:
					time.Sleep(10 * time.Millisecond)
					continue
				}
			}

			return nil
		},
		CancellationDelegate: func() {
			cancelCount.Increment()
			didCancel.Set(true)
		},
	})
	jm.Start()
	defer jm.Stop()

	wg.Wait()
	elapsed := time.Now().UTC().Sub(start)

	a.True(didRun.Get())
	a.True(didCancel.Get())

	// elapsed should be less than the timeout + (2 heartbeat intervals)
	a.True(elapsed < (100+(DefaultHeartbeatInterval*2))*time.Millisecond, fmt.Sprintf("%v", elapsed))
}

func TestRunJobSimultaneously(t *testing.T) {
	a := assert.New(t)
	a.StartTimeout(2000 * time.Millisecond)
	defer a.EndTimeout()

	jm := New().WithHighPrecisionHeartbeat()

	wg := sync.WaitGroup{}
	wg.Add(2)

	jm.LoadJob(&runAtJob{
		RunAt: time.Now().UTC(),
		RunDelegate: func(ctx context.Context) error {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond)
			return nil
		},
	})

	go func() {
		err := jm.RunJob(runAtJobName)
		a.Nil(err)
	}()
	go func() {
		err := jm.RunJob(runAtJobName)
		a.Nil(err)
	}()

	wg.Wait()
}

func TestRunJobByScheduleRapid(t *testing.T) {
	a := assert.New(t)

	runEvery := DefaultHeartbeatInterval
	runFor := 1000 * time.Millisecond

	runCount := new(AtomicCounter)

	// high precision heartbeat is somewhere around 5ms
	jm := New().WithHighPrecisionHeartbeat()
	err := jm.LoadJob(&testJobInterval{RunEvery: runEvery, RunDelegate: func(ctx context.Context) error {
		runCount.Increment()
		return nil
	}})
	a.Nil(err)

	jm.Start()
	defer jm.Stop()

	elapsed := time.Duration(0)
	waitFor := 10 * time.Millisecond
	for elapsed < runFor {
		elapsed = elapsed + waitFor
		time.Sleep(waitFor)
	}

	expected := int(int64(runFor) / int64(DefaultHeartbeatInterval))
	a.True(runCount.Get()-expected < 2, fmt.Sprintf("%d vs. %d\n", runCount.Get(), expected))
}

func TestJobManagerTaskListener(t *testing.T) {
	assert := assert.New(t)

	jm := New()

	wg := sync.WaitGroup{}
	wg.Add(2)
	output := bytes.NewBuffer(nil)
	jm.SetLogger(logger.None(logger.NewWriter(output)))
	jm.Logger().EnableEvent(EventTaskComplete)
	jm.Logger().AddEventListener(EventTaskComplete, NewTaskCompleteListener(func(_ *logger.Writer, _ logger.TimeSource, taskName string, elapsed time.Duration, err error) {
		defer wg.Done()
		assert.Equal("test_task", taskName)
		assert.NotZero(elapsed)
		assert.Nil(err)
	}))

	var didRun bool
	jm.RunTask(NewTaskWithName("test_task", func(ctx context.Context) error {
		defer wg.Done()
		didRun = true
		return nil
	}))
	wg.Wait()

	assert.True(didRun)
}

func TestJobManagerTaskListenerWithError(t *testing.T) {
	assert := assert.New(t)

	jm := New()

	wg := sync.WaitGroup{}
	wg.Add(2)

	output := bytes.NewBuffer(nil)
	agent := logger.None(logger.NewWriter(output))
	agent.Writer().SetUseAnsiColors(false)
	agent.Writer().SetShowTimestamp(false)

	jm.SetLogger(agent)
	jm.Logger().EnableEvent(EventTaskComplete)
	jm.Logger().AddEventListener(EventTaskComplete, NewTaskCompleteListener(func(_ *logger.Writer, _ logger.TimeSource, taskName string, elapsed time.Duration, err error) {
		defer wg.Done()
		assert.Equal("test_task", taskName)
		assert.NotZero(elapsed)
		assert.NotNil(err)
	}))

	var didRun bool
	jm.RunTask(NewTaskWithName("test_task", func(ctx context.Context) error {
		defer wg.Done()
		didRun = true
		return errors.New("testError")
	}))
	wg.Wait()

	assert.True(didRun)
	assert.True(strings.HasPrefix(output.String(), "[chronometer.task.complete] `test_task`"), output.String())
}

// The goal with this test is to see if panics take down the test process or not.
func TestJobManagerTaskPanicHandling(t *testing.T) {
	a := assert.New(t)
	a.StartTimeout(2000 * time.Millisecond)
	defer a.EndTimeout()

	manager := New()
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(1)
	err := manager.RunTask(NewTask(func(ctx context.Context) error {
		defer waitGroup.Done()
		array := []int{}
		foo := array[1] //this should index out of bounds
		a.NotZero(foo)
		return nil
	}))

	waitGroup.Wait()
	a.Nil(err)
}

type testWithEnabled struct {
	isEnabled bool
	action    func()
}

func (twe testWithEnabled) Name() string {
	return "testWithEnabled"
}

func (twe testWithEnabled) Schedule() Schedule {
	return OnDemand()
}

func (twe testWithEnabled) Enabled() bool {
	return twe.isEnabled
}

func (twe testWithEnabled) Execute(ctx context.Context) error {
	twe.action()
	return nil
}

func TestEnabledProvider(t *testing.T) {
	a := assert.New(t)
	a.StartTimeout(2000 * time.Millisecond)
	defer a.EndTimeout()

	var didRun bool
	manager := New()
	job := &testWithEnabled{
		isEnabled: true,
		action: func() {
			didRun = true
		},
	}

	manager.LoadJob(job)
	a.False(manager.IsDisabled("testWithEnabled"))
	manager.DisableJob("testWithEnabled")
	a.True(manager.IsDisabled("testWithEnabled"))
	job.isEnabled = false
	a.True(manager.IsDisabled("testWithEnabled"))
	manager.EnableJob("testWithEnabled")
	a.True(manager.IsDisabled("testWithEnabled"))
}

func TestFiresErrorOnTaskError(t *testing.T) {
	a := assert.New(t)
	a.StartTimeout(2000 * time.Millisecond)
	defer a.EndTimeout()

	agent := logger.New(logger.EventError)
	manager := New()
	manager.SetLogger(agent)

	var errorDidFire bool
	var errorMatched bool
	wg := sync.WaitGroup{}
	wg.Add(2)
	agent.AddEventListener(logger.EventError, logger.NewErrorListener(func(_ *logger.Writer, _ logger.TimeSource, err error) {
		defer wg.Done()
		errorDidFire = true
		if err != nil {
			errorMatched = err.Error() == "this is only a test"
		}
	}))
	manager.LoadJob(NewJob().WithAction(func(ctx context.Context) error {
		defer wg.Done()
		return fmt.Errorf("this is only a test")
	}).WithName("error_test"))
	manager.RunJob("error_test")
	wg.Wait()

	a.True(errorDidFire)
	a.True(errorMatched)
}
