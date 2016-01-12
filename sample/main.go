package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/blendlabs/go-chronometer"
)

type printJob struct {
}

func (pj *printJob) Timeout() time.Duration {
	return 2 * time.Second
}

func (pj *printJob) Name() string {
	return "printJob"
}

func (pj *printJob) OnStart() {
	fmt.Printf("(printJob) starting at %v\n", time.Now().UTC())
}

func (pj *printJob) Execute(ct *chronometer.CancellationToken) error {
	fmt.Printf("(printJob) run at %v\n", time.Now().UTC())
	if rand.Int()%2 == 1 {
		time.Sleep(2000 * time.Millisecond)
	} else {
		time.Sleep(8000 * time.Millisecond)
	}
	return nil
}

func (pj *printJob) OnCancellation() {
	fmt.Println("(printJob) CANCELLED!")
}

func (pj *printJob) OnComplete(err error) {
	fmt.Printf("(printJob) finished at %v\n", time.Now().UTC())
	if err != nil {
		fmt.Printf("(printJob) onComplete error: %v\n", err)
	}
}

func (pj *printJob) Schedule() chronometer.Schedule {
	return chronometer.Every(10 * time.Second)
}

func main() {
	chronometer.Default().LoadJob(&printJob{})
	chronometer.Default().Start()

	chronometer.Default().RunTask(chronometer.NewTask(func(ct *chronometer.CancellationToken) error {
		if ct.ShouldCancel {
			return ct.Cancel()
		}
		fmt.Print("Running quick task ...")
		time.Sleep(2000 * time.Millisecond)
		fmt.Println("complete")
		return nil
	}))

	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
}
