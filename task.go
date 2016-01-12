package chronometer

import (
	"fmt"

	"github.com/blendlabs/go-util"
)

type TaskAction func(ct *CancellationToken) error

type Task interface {
	Name() string
	Execute(ct *CancellationToken) error
	OnStart()
	OnCancellation()
	OnComplete(err error)
}

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
