package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/blendlabs/go-chronometer"
)

type printJob struct {
}

func (pj *printJob) Name() string {
	return "printJob"
}

func (pj *printJob) OnStart() {
	fmt.Printf("(printJob) starting at %v\n", time.Now().UTC())
}

func (pj *printJob) Execute(ct *chronometer.CancellationToken) error {
	fmt.Printf("(printJob) run at %v\n", time.Now().UTC())
	time.Sleep(250 * time.Millisecond)
	return nil
}

func (pj *printJob) OnComplete(err error) {
	fmt.Printf("(printJob) finished at %v\n", time.Now().UTC())
}

func (pj *printJob) Schedule() chronometer.Schedule {
	return chronometer.EverySecond()
}

func main() {
	chronometer.Default().LoadJob(&printJob{})
	chronometer.Default().Start()

	//handle os signals ...
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		<-sigs
		chronometer.Default().CancelTask("printJob")
		wg.Done()
	}()

	chronometer.Default().RunTask(chronometer.NewTask(func(ct *chronometer.CancellationToken) error {
		if ct.ShouldCancel {
			return ct.Cancel()
		}
		fmt.Print("Running quick task ...")
		time.Sleep(2000 * time.Millisecond)
		fmt.Println("complete")
		return nil
	}))

	wg.Wait()
}
