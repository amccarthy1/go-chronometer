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

		a.Len(jm.RunningTasks, 1)
		a.Len(jm.RunningTaskStartTimes, 1)
		a.Len(jm.CancellationTokens, 1)

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
			if ct.ShouldCancel {
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

	for _, ct := range jm.CancellationTokens {
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

type testJob struct {
	RunAt       time.Time
	RunDelegate func(ct *CancellationToken) error
}

type testJobSchedule struct {
	RunAt time.Time
}

func (tjs testJobSchedule) GetNextRunTime(after *time.Time) time.Time {
	return tjs.RunAt
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
	jm.LoadJob(&testJob{RunAt: time.Now().UTC().Add(100 * time.Millisecond), RunDelegate: func(ct *CancellationToken) error {
		runCount++
		didRun = true
		return nil
	}})
	jm.Start()
	defer jm.Stop()

	elapsed := time.Duration(0)
	for elapsed < 1*time.Second {
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
	jm.LoadJob(&testJob{RunAt: time.Now().UTC().Add(100 * time.Millisecond), RunDelegate: func(ct *CancellationToken) error {
		runCount++
		didRun = true
		return nil
	}})

	disableErr := jm.DisableJob("testJob")
	a.Nil(disableErr)
	a.True(jm.DisabledJobs.Contains("testJob"))
}
