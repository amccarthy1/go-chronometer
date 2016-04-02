package chronometer

import (
	"testing"
	"time"

	"github.com/blendlabs/go-assert"
)

func TestRunTask(t *testing.T) {
	a := assert.New(t)

	jm := NewJobManager()

	runCount := 0
	didRun := false
	jm.RunTask(NewTask(func(ct *CancellationToken) error {
		runCount++
		didRun = true
		return nil
	}))

	elapsed := time.Duration(0)
	for elapsed < 1*time.Second {
		if didRun {
			break
		}

		a.Len(jm.runningTasks, 1)
		a.Len(jm.runningTaskStartTimes, 1)
		a.Len(jm.cancellationTokens, 1)

		elapsed = elapsed + 10*time.Millisecond
		time.Sleep(10 * time.Millisecond)
	}
	a.Equal(1, runCount)
	a.True(didRun)
}

func TestRunTaskAndCancel(t *testing.T) {
	a := assert.New(t)

	jm := NewJobManager()

	didRun := false
	didCancel := false
	jm.RunTask(NewTask(func(ct *CancellationToken) error {
		didRun = true
		taskElapsed := time.Duration(0)
		for taskElapsed < 1*time.Second {
			if ct.ShouldCancel() {
				didCancel = true
				return nil
			}
			taskElapsed = taskElapsed + 10*time.Millisecond
			time.Sleep(10 * time.Millisecond)
		}

		return nil
	}))

	elapsed := time.Duration(0)
	for elapsed < 1*time.Second {
		if didRun {
			break
		}

		elapsed = elapsed + 10*time.Millisecond
		time.Sleep(10 * time.Millisecond)
	}

	for _, ct := range jm.cancellationTokens {
		ct.signalCancellation()
	}

	elapsed = time.Duration(0)
	for elapsed < 1*time.Second {
		if didCancel {
			break
		}

		elapsed = elapsed + 10*time.Millisecond
		time.Sleep(10 * time.Millisecond)
	}
	a.True(didCancel)
	a.True(didRun)
}

func TestRunTaskAndCancelWithPanic(t *testing.T) {
	a := assert.New(t)

	jm := NewJobManager()

	start := time.Now().UTC()
	didRun := false
	didRunToCompletion := false
	jm.RunTask(NewTask(func(ct *CancellationToken) error {
		didRun = true
		time.Sleep(1 * time.Second)
		didRunToCompletion = true
		return nil
	}))

	for !didRun {
		time.Sleep(1 * time.Millisecond)
	}

	for _, ct := range jm.cancellationTokens {
		ct.signalCancellation()
	}
	elapsed := time.Now().UTC().Sub(start)

	a.True(didRun)
	a.False(didRunToCompletion)
	a.True(elapsed < (CancellationGracePeriod + 10*time.Millisecond))
}

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

func TestRunJobBySchedule(t *testing.T) {
	a := assert.New(t)

	didRun := false
	runCount := 0
	jm := NewJobManager()
	err := jm.LoadJob(&testJob{RunAt: time.Now().UTC().Add(100 * time.Millisecond), RunDelegate: func(ct *CancellationToken) error {
		runCount++
		didRun = true
		return nil
	}})
	a.Nil(err)

	jm.Start()
	defer jm.Stop()

	elapsed := time.Duration(0)
	for elapsed < 2*time.Second {
		if didRun {
			break
		}

		elapsed = elapsed + 10*time.Millisecond
		time.Sleep(10 * time.Millisecond)
	}

	a.True(didRun)
	a.Equal(1, runCount)
}

func TestDisableJob(t *testing.T) {
	a := assert.New(t)

	didRun := false
	runCount := 0
	jm := NewJobManager()
	err := jm.LoadJob(&testJob{RunAt: time.Now().UTC().Add(100 * time.Millisecond), RunDelegate: func(ct *CancellationToken) error {
		runCount++
		didRun = true
		return nil
	}})
	a.Nil(err)

	err = jm.DisableJob("testJob")
	a.Nil(err)

	a.True(jm.disabledJobs.Contains("testJob"))
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
