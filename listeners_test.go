package chronometer

import (
	"sync"
	"testing"
	"time"

	assert "github.com/blendlabs/go-assert"
	logger "github.com/blendlabs/go-logger"
)

func TestTaskListener(t *testing.T) {
	assert := assert.New(t)

	agent := logger.New(logger.NewEventFlagSetWithEvents(EventTask))
	wg := sync.WaitGroup{}
	wg.Add(1)
	var returnedTaskName string
	agent.AddEventListener(EventTask, NewTaskListener(func(w logger.Logger, ts logger.TimeSource, taskName string) {
		defer wg.Done()
		returnedTaskName = taskName
	}))

	testTaskName := "test_task"
	agent.OnEvent(EventTask, testTaskName)

	wg.Wait()
	assert.Equal(returnedTaskName, testTaskName)
}

func TestTaskCompleteListener(t *testing.T) {
	assert := assert.New(t)

	agent := logger.New(logger.NewEventFlagSetWithEvents(EventTaskComplete))
	wg := sync.WaitGroup{}
	wg.Add(1)
	var returnedTaskName string
	agent.AddEventListener(EventTaskComplete, NewTaskCompleteListener(func(w logger.Logger, ts logger.TimeSource, taskName string, elapsed time.Duration, err error) {
		defer wg.Done()
		returnedTaskName = taskName
	}))

	testTaskName := "test_task"
	agent.OnEvent(EventTaskComplete, testTaskName, time.Duration(0), nil)

	wg.Wait()
	assert.Equal(returnedTaskName, testTaskName)
}
