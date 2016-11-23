package chronometer

import (
	"time"

	"github.com/blendlabs/go-logger"
)

const (
	// EventTaskComplete is a logger diagnostics event for task completions.
	EventTaskComplete logger.EventFlag = "chronometer.task"
)

// EventTaskCompleteListener is a listener for task complete events.
type EventTaskCompleteListener func(w logger.Logger, ts logger.TimeSource, taskName string, elapsed time.Duration, err error)

// NewTaskCompleteListener returns a new event listener for task events.
func NewTaskCompleteListener(listener EventTaskCompleteListener) logger.EventListener {
	return func(writer logger.Logger, ts logger.TimeSource, eventFlag logger.EventFlag, state ...interface{}) {
		if state[2] == nil {
			listener(writer, ts, state[0].(string), state[1].(time.Duration), nil)
		} else {
			listener(writer, ts, state[0].(string), state[1].(time.Duration), state[2].(error))
		}
	}
}