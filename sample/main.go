package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/blendlabs/go-chronometer"
)

type emptyJob struct {
	running bool
}

func (j *emptyJob) Timeout() time.Duration {
	return 2 * time.Second
}

func (j *emptyJob) Name() string {
	return "printJob"
}

func (j *emptyJob) Execute(ct *chronometer.CancellationToken) error {
	j.running = true
	if rand.Int()%2 == 1 {
		time.Sleep(2000 * time.Millisecond)

		chronometer.Default().RunTask(chronometer.NewTask(func(ct *chronometer.CancellationToken) error {
			time.Sleep(2000 * time.Millisecond)
			return nil
		}))

	} else {
		time.Sleep(8000 * time.Millisecond)
	}
	j.running = false
	return nil
}

func (j *emptyJob) OnCancellation() {
	j.running = false
}

func (j *emptyJob) Status() string {
	if j.running {
		return "Request in progress"
	} else {
		return "Request idle."
	}
}

func (j *emptyJob) Schedule() chronometer.Schedule {
	return chronometer.Every(10 * time.Second)
}

func main() {
	chronometer.Default().LoadJob(&emptyJob{})
	chronometer.Default().Start()

	for {
		statuses := chronometer.Default().Status()
		for _, status := range statuses {
			if len(status.Status) != 0 {
				fmt.Printf("task: %s state: %s status: %s\n", status.Name, status.State, status.Status)
			} else {
				fmt.Printf("task: %s state: %s\n", status.Name, status.State)
			}
		}

		time.Sleep(1000 * time.Millisecond)
	}
}
