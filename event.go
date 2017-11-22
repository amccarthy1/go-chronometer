package chronometer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	logger "github.com/blendlabs/go-logger"
)

const (
	// FlagStarted is a logger flag for task start.
	FlagStarted logger.Flag = "chronometer.task"
	// FlagComplete is a logger flag for task completions.
	FlagComplete logger.Flag = "chronometer.task.complete"
)

// NewEventStartedListener returns a new event started listener.
func NewEventStartedListener(listener func(wr logger.Writer, e EventStarted)) logger.Listener {
	return func(wr logger.Writer, e logger.Event) {
		if typed, isTyped := e.(EventStarted); isTyped {
			listener(wr, typed)
		}
	}
}

// EventStarted is a started event.
type EventStarted struct {
	ts       time.Time
	taskName string
}

// Flag returns the event flag.
func (e EventStarted) Flag() logger.Flag {
	return FlagStarted
}

// Timestamp returns an event timestamp.
func (e EventStarted) Timestamp() time.Time {
	return e.ts
}

// TaskName returns the event task name.
func (e EventStarted) TaskName() string {
	return e.taskName
}

// WriteText writes the event to a text output.
func (e EventStarted) WriteText(tf logger.TextFormatter, buf *bytes.Buffer) error {
	buf.WriteString(fmt.Sprintf("`%s` starting", e.taskName))
	return nil
}

// MarshalJSON marshals the event as json.
func (e EventStarted) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		logger.JSONFieldFlag:      e.Flag(),
		logger.JSONFieldTimestamp: e.Timestamp(),
		"taskName":                e.taskName,
	})
}

// NewEventCompleteListener returns a new event complete listener.
func NewEventCompleteListener(listener func(wr logger.Writer, e EventComplete)) logger.Listener {
	return func(wr logger.Writer, e logger.Event) {
		if typed, isTyped := e.(EventComplete); isTyped {
			listener(wr, typed)
		}
	}
}

// EventComplete is an event emitted to the logger.
type EventComplete struct {
	ts       time.Time
	taskName string
	err      error
	elapsed  time.Duration
}

// Flag returns the event flag.
func (e EventComplete) Flag() logger.Flag {
	return FlagComplete
}

// Timestamp returns an event timestamp.
func (e EventComplete) Timestamp() time.Time {
	return e.ts
}

// TaskName returns the event task name.
func (e EventComplete) TaskName() string {
	return e.taskName
}

// Elapsed returns the elapsed time for the task.
func (e EventComplete) Elapsed() time.Duration {
	return e.elapsed
}

// Err returns the event err (if any).
func (e EventComplete) Err() error {
	return e.err
}

// WriteText writes the event to a text output.
func (e EventComplete) WriteText(tf logger.TextFormatter, buf *bytes.Buffer) error {
	if e.err != nil {
		buf.WriteString(fmt.Sprintf("`%s` failed (%v)", e.taskName, e.elapsed))
	} else {
		buf.WriteString(fmt.Sprintf("`%s` completed (%v)", e.taskName, e.elapsed))
	}
	return nil
}

// MarshalJSON marshals the event as json.
func (e EventComplete) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		logger.JSONFieldFlag:      e.Flag(),
		logger.JSONFieldTimestamp: e.Timestamp(),
		"taskName":                e.taskName,
		logger.JSONFieldElapsed:   logger.Milliseconds(e.elapsed),
		logger.JSONFieldErr:       e.err,
	})
}
