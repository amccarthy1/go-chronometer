package chronometer

import (
	"time"

	"github.com/blendlabs/go-logger"
)

// NewTaskListener returns a new event listener for task events.
func NewTaskListener(listener func(logger.Logger, logger.TimeSource, string, time.Duration, error)) logger.EventListener {
	return func(writer logger.Logger, ts logger.TimeSource, eventFlag logger.EventFlag, state ...interface{}) {
		if state[2] == nil {
			listener(writer, ts, state[0].(string), state[1].(time.Duration), nil)
		} else {
			listener(writer, ts, state[0].(string), state[1].(time.Duration), state[2].(error))
		}
	}
}
