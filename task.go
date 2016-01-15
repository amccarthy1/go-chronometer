package chronometer

import (
	"fmt"
	"time"

	"github.com/blendlabs/go-util"
)

// --------------------------------------------------------------------------------
// interfaces
// --------------------------------------------------------------------------------

type CancellationSignalReciever func()
type TaskAction func(ct *CancellationToken) error

type ResumeProvider interface {
	State() interface{}
	Resume(state interface{}) error
}

type TimeoutProvider interface {
	Timeout() time.Duration
}

type StatusProvider interface {
	Status() string
}

type OnStartReceiver interface {
	OnStart()
}

type OnCancellationReceiver interface {
	OnCancellation()
}

type OnCompleteReceiver interface {
	OnComplete(err error)
}

type Task interface {
	Name() string
	Execute(ct *CancellationToken) error
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
func (bt basicTask) Execute(ct *CancellationToken) error {
	return bt.action(ct)
}
func (bt basicTask) OnStart()             {}
func (bt basicTask) OnCancellation()      {}
func (bt basicTask) OnComplete(err error) {}

func NewTask(action TaskAction) Task {
	name := fmt.Sprintf("task_%s", util.UUID_v4().ToShortString())
	return &basicTask{name: name, action: action}
}

func NewTaskWithName(name string, action TaskAction) Task {
	return &basicTask{name: name, action: action}
}

// --------------------------------------------------------------------------------
// task status
// --------------------------------------------------------------------------------

type TaskStatus struct {
	Name       string `json:"name"`
	State      string `json:"state"`
	Status     string `json:"status"`
	RunningFor string `json:"running_for,omitempty"`
}
