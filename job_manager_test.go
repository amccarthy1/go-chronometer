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

type testJob struct {
	RunAt       time.Time
	RunDelegate func(ctx context.Context) error
}

type testJobSchedule struct {
	RunAt time.Time
}

func (tjs testJobSchedule) GetNextRunTime(after *time.Time) *time.Time {
	return &tjs.RunAt
}

func (tj *testJob) Name() string {
	return "testJob"
}

func (tj *testJob) Schedule() Schedule {
	return testJobSchedule{RunAt: tj.RunAt}
}

func (tj *testJob) Execute(ctx context.Context) error {
	return tj.RunDelegate(ctx)
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
	return testJobSchedule{RunAt: tj.RunAt}
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

	jm := NewJobManager()

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
	jm := NewJobManager()

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

	didRun := new(AtomicFlag)
	runCount := new(AtomicCounter)
	jm := NewJobManager()
	err := jm.LoadJob(&testJob{RunAt: time.Now().UTC().Add(100 * time.Millisecond), RunDelegate: func(ctx context.Context) error {
		runCount.Increment()
		didRun.Set(true)
		return nil
	}})
	a.Nil(err)

	jm.Start()
	defer jm.Stop()

	elapsed := time.Duration(0)
	for elapsed < 2*time.Second {
		if didRun.Get() {
			break
		}

		elapsed = elapsed + 10*time.Millisecond
		time.Sleep(10 * time.Millisecond)
	}

	a.True(didRun.Get())
	a.Equal(1, runCount.Get())
}

func TestDisableJob(t *testing.T) {
	a := assert.New(t)

	didRun := new(AtomicFlag)
	runCount := new(AtomicCounter)
	jm := NewJobManager()
	err := jm.LoadJob(&testJob{RunAt: time.Now().UTC().Add(100 * time.Millisecond), RunDelegate: func(ctx context.Context) error {
		runCount.Increment()
		didRun.Set(true)
		return nil
	}})
	a.Nil(err)

	err = jm.DisableJob("testJob")
	a.Nil(err)
	a.True(jm.disabledJobs.Contains("testJob"))
}

func TestRunTaskAndCancelWithTimeout(t *testing.T) {
	a := assert.New(t)

	jm := NewJobManager()

	start := time.Now().UTC()
	didRun := new(AtomicFlag)
	didCancel := new(AtomicFlag)
	cancelCount := new(AtomicCounter)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	jm.LoadJob(&testJobWithTimeout{
		RunAt:           start,
		TimeoutDuration: 100 * time.Millisecond,
		RunDelegate: func(ctx context.Context) error {
			defer wg.Done()
			didRun.Set(true)
			for !didCancel.Get() {
				select {
				case <-ctx.Done():
					didCancel.Set(true)
					return nil
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
	a.True(elapsed < (100+(HangingHeartbeatInterval*2))*time.Millisecond, fmt.Sprintf("%v", elapsed))
}

func TestRunJobSimultaneously(t *testing.T) {
	a := assert.New(t)

	jm := NewJobManager()

	runCount := new(AtomicCounter)
	completeCount := new(AtomicCounter)
	jm.LoadJob(&testJob{
		RunAt: time.Now().UTC(),
		RunDelegate: func(ctx context.Context) error {
			runCount.Increment()
			time.Sleep(50 * time.Millisecond)
			completeCount.Increment()
			return nil
		},
	})

	go func() {
		err := jm.RunJob("testJob")
		a.Nil(err)
	}()
	go func() {
		err := jm.RunJob("testJob")
		a.Nil(err)
	}()

	for completeCount.Get() != 2 {
		time.Sleep(10 * time.Millisecond)
	}

	a.Equal(2, runCount.Get())
	a.Equal(2, completeCount.Get())
}

func TestRunJobByScheduleRapid(t *testing.T) {
	a := assert.New(t)

	runEvery := 50 * time.Millisecond
	runFor := 1000 * time.Millisecond

	runCount := new(AtomicCounter)
	jm := NewJobManager()
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

	expected := int64(runFor) / int64(HeartbeatInterval)

	a.True(int64(runCount.Get()) >= expected, fmt.Sprintf("%d vs. %d\n", runCount, expected))
}

func TestJobManagerTaskListener(t *testing.T) {
	assert := assert.New(t)

	jm := NewJobManager()

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

	jm := NewJobManager()

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
	assert := assert.New(t)
	manager := NewJobManager()
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(1)
	err := manager.RunTask(NewTask(func(ctx context.Context) error {
		defer waitGroup.Done()
		array := []int{}
		foo := array[1] //this should index out of bounds
		assert.NotZero(foo)
		return nil
	}))

	waitGroup.Wait()
	assert.Nil(err)
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
	assert := assert.New(t)

	var didRun bool
	manager := NewJobManager()
	job := &testWithEnabled{
		isEnabled: true,
		action: func() {
			didRun = true
		},
	}

	manager.LoadJob(job)
	assert.False(manager.IsDisabled("testWithEnabled"))
	manager.DisableJob("testWithEnabled")
	assert.True(manager.IsDisabled("testWithEnabled"))
	job.isEnabled = false
	assert.True(manager.IsDisabled("testWithEnabled"))
	manager.EnableJob("testWithEnabled")
	assert.True(manager.IsDisabled("testWithEnabled"))
}
