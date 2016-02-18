package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/blendlabs/go-chronometer"
)

type loadTestJob struct {
	id      int
	running bool
}

func (j *loadTestJob) Timeout() time.Duration {
	return 2 * time.Second
}

func (j *loadTestJob) Name() string {
	return fmt.Sprintf("loadTestJob_%d", j.id)
}

func (j *loadTestJob) Execute(ct *chronometer.CancellationToken) error {
	j.running = true
	if rand.Int()%2 == 1 {
		time.Sleep(2000 * time.Millisecond)
	} else {
		time.Sleep(8000 * time.Millisecond)
	}
	j.running = false
	fmt.Printf("%s ran\n", j.Name())
	return nil
}

func (j *loadTestJob) OnCancellation() {
	j.running = false
}

func (j *loadTestJob) Status() string {
	if j.running {
		return "Request in progress"
	} else {
		return "Request idle."
	}
}

func (j *loadTestJob) Schedule() chronometer.Schedule {
	return chronometer.Every(10 * time.Second)
}

func main() {
	for x := 1; x < 1000; x++ {
		chronometer.Default().LoadJob(&loadTestJob{id: x})
		fmt.Printf("loaded job %d\n", x)
	}
	chronometer.Default().Start()

	time.Sleep(30 * time.Second)
}
