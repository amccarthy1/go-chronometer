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

func (pj *printJob) Execute(ct *chronometer.CancellationToken) error {
	fmt.Printf("(printJob) run at %v\n", time.Now().UTC())
	return nil
}

func (pj *printJob) Schedule() chronometer.Schedule {
	return chronometer.EverySecond()
}

func main() {
	jm := chronometer.NewJobManager()
	jm.LoadJob(&printJob{})
	jm.Start()

	//handle os signals ...
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		<-sigs
		jm.CancelTask("printJob")
		wg.Done()
	}()

	wg.Wait()
}
