package chronometer

import (
	"testing"
	"time"

	"github.com/blendlabs/go-assert"
)

func TestRunTask(t *testing.T) {
	a := assert.New(t)

	jm := NewJobManager()

	didRun := false
	jm.RunTask(NewTask(func(ct *CancellationToken) error {
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
