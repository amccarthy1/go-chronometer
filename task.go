package chronometer

import (
	"context"
	"fmt"
	"time"

	uuid "github.com/blendlabs/go-util/uuid"
)

// --------------------------------------------------------------------------------
// interfaces
// --------------------------------------------------------------------------------

// CancellationSignalReciever is a function that can be used as a receiver for cancellation signals.
type CancellationSignalReciever func()

// TaskAction is an function that can be run as a task
type TaskAction func(ctx context.Context) error

// ResumeProvider is an interface that allows a task to be resumed.
type ResumeProvider interface {
	State() interface{}
	Resume(state interface{}) error
}

// TimeoutProvider is an interface that allows a task to be timed out.
type TimeoutProvider interface {
	Timeout() time.Duration
}

// StatusProvider is an interface that allows a task to report its status.
type StatusProvider interface {
	Status() string
}

// OnStartReceiver is an interface that allows a task to be signaled when it has started.
type OnStartReceiver interface {
	OnStart()
}

// OnCancellationReceiver is an interface that allows a task to be signaled when it has been canceled.
type OnCancellationReceiver interface {
	OnCancellation()
}

// OnCompleteReceiver is an interface that allows a task to be signaled when it has been completed.
type OnCompleteReceiver interface {
	OnComplete(err error)
}

// Task is an interface that structs can satisfy to allow them to be run as tasks.
type Task interface {
	Name() string
	Execute(ctx context.Context) error
}

// --------------------------------------------------------------------------------
// quick task creation
// --------------------------------------------------------------------------------

type basicTask struct {
	name   string
	action TaskAction
}

func (bt basicTask) Name() string {
	return bt.name
}
func (bt basicTask) Execute(ctx context.Context) error {
	return bt.action(ctx)
}
func (bt basicTask) OnStart()             {}
func (bt basicTask) OnCancellation()      {}
func (bt basicTask) OnComplete(err error) {}

// generateTaskName returns a unique identifier that can be used to name/tag
// tasks
func generateTaskName() string {
	return fmt.Sprintf("task_%s", uuid.V4().ToShortString())
}

// NewTask returns a new task wrapper for a given TaskAction.
func NewTask(action TaskAction) Task {
	name := generateTaskName()
	return &basicTask{name: name, action: action}
}

// NewTaskWithName returns a new task wrapper with a given name for a given
// TaskAction.
func NewTaskWithName(name string, action TaskAction) Task {
	return &basicTask{name: name, action: action}
}

// --------------------------------------------------------------------------------
// task status
// --------------------------------------------------------------------------------

// TaskStatus is the basic format of a status of a task.
type TaskStatus struct {
	Name        string `json:"name"`
	State       State  `json:"state"`
	Status      string `json:"status,omitempty"`
	LastRunTime string `json:"last_run_time,omitempty"`
	NextRunTime string `json:"next_run_time,omitempy"`
	RunningFor  string `json:"running_for,omitempty"`
}
