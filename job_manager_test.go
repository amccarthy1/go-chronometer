package chronometer

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blendlabs/go-assert"
)

type testJob struct {
	RunAt       time.Time
	RunDelegate func(ct *CancellationToken) error
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

func (tj *testJob) Execute(ct *CancellationToken) error {
	return tj.RunDelegate(ct)
}

type testJobWithTimeout struct {
	RunAt           time.Time
	TimeoutDuration time.Duration
	RunDelegate     func(ct *CancellationToken) error
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

func (tj *testJobWithTimeout) Execute(ct *CancellationToken) error {
	return tj.RunDelegate(ct)
}

type testJobInterval struct {
	RunEvery    time.Duration
	RunDelegate func(ct *CancellationToken) error
}

func (tj *testJobInterval) Name() string {
	return "testJobInterval"
}

func (tj *testJobInterval) Schedule() Schedule {
	return Every(tj.RunEvery)
}

func (tj *testJobInterval) Execute(ct *CancellationToken) error {
	return tj.RunDelegate(ct)
}

func TestRunTask(t *testing.T) {
	a := assert.New(t)

	jm := NewJobManager()

	didRun := new(AtomicFlag)
	var runCount int32
	jm.RunTask(NewTask(func(ct *CancellationToken) error {
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
			jm.runningTasksLock.RLock()
			defer jm.runningTasksLock.RUnlock()

			jm.runningTaskStartTimesLock.RLock()
			defer jm.runningTaskStartTimesLock.RUnlock()

			jm.cancellationTokensLock.RLock()
			defer jm.cancellationTokensLock.RUnlock()

			a.Len(jm.runningTasks, 1)
			a.Len(jm.runningTaskStartTimes, 1)
			a.Len(jm.cancellationTokens, 1)
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
	didCancel := new(AtomicFlag)
	jm.RunTask(NewTask(func(ct *CancellationToken) error {
		didRun.Set(true)
		taskElapsed := time.Duration(0)
		for taskElapsed < 1*time.Second {
			if ct.ShouldCancel() {
				didCancel.Set(true)
				return ct.CanceledGracefully() // THIS MUST BE CALLED.
			}
			taskElapsed = taskElapsed + 10*time.Millisecond
			time.Sleep(10 * time.Millisecond)
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

	for _, ct := range jm.cancellationTokens {
		ct.Cancel()
	}

	elapsed = time.Duration(0)
	for elapsed < 1*time.Second {
		if didCancel.Get() {
			break
		}

		elapsed = elapsed + 10*time.Millisecond
		time.Sleep(10 * time.Millisecond)
	}
	a.True(didCancel.Get())
	a.True(didRun.Get())
}

func TestRunTaskAndCancelWithPanic(t *testing.T) {
	a := assert.New(t)

	jm := NewJobManager()

	start := time.Now().UTC()
	didRun := new(AtomicFlag)
	didRunToCompletion := new(AtomicFlag)
	jm.RunTask(NewTaskWithName("taskToCancel", func(ct *CancellationToken) error {
		didRun.Set(true)
		time.Sleep(1 * time.Second)
		didRunToCompletion.Set(true)
		return nil
	}))

	for !didRun.Get() {
		time.Sleep(1 * time.Millisecond)
	}

	jm.CancelTask("taskToCancel")
	elapsed := time.Now().UTC().Sub(start)

	a.True(didRun.Get())
	a.False(didRunToCompletion.Get())
	a.True(elapsed < (CancellationGracePeriod + 10*time.Millisecond))
}

func TestRunJobBySchedule(t *testing.T) {
	a := assert.New(t)

	didRun := new(AtomicFlag)
	var runCount int32
	jm := NewJobManager()
	err := jm.LoadJob(&testJob{RunAt: time.Now().UTC().Add(100 * time.Millisecond), RunDelegate: func(ct *CancellationToken) error {
		atomic.AddInt32(&runCount, 1)
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
	a.Equal(1, runCount)
}

func TestDisableJob(t *testing.T) {
	a := assert.New(t)

	didRun := new(AtomicFlag)
	var runCount int32
	jm := NewJobManager()
	err := jm.LoadJob(&testJob{RunAt: time.Now().UTC().Add(100 * time.Millisecond), RunDelegate: func(ct *CancellationToken) error {
		atomic.AddInt32(&runCount, 1)
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
	jm.LoadJob(&testJobWithTimeout{
		RunAt:           start,
		TimeoutDuration: 100 * time.Millisecond,
		RunDelegate: func(ct *CancellationToken) error {
			didRun.Set(true)
			for !didCancel.Get() {
				if ct.ShouldCancel() {
					didCancel.Set(true)
					return ct.CanceledGracefully()
				}
				time.Sleep(10 * time.Millisecond)
			}

			return nil
		},
	})
	jm.Start()
	defer jm.Stop()

	for !didCancel.Get() {
		time.Sleep(1 * time.Millisecond)
	}

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
		RunDelegate: func(ct *CancellationToken) error {
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
	err := jm.LoadJob(&testJobInterval{RunEvery: runEvery, RunDelegate: func(ct *CancellationToken) error {
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
