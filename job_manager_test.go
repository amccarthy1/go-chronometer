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
		elapsed = elapsed + 10*time.Millisecond
		time.Sleep(10 * time.Millisecond)
	}

	a.True(didRun)
}
