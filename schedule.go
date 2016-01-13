package chronometer

import "time"

var DAYS_OF_WEEK = []time.Weekday{
	time.Sunday,
	time.Monday,
	time.Tuesday,
	time.Wednesday,
	time.Thursday,
	time.Friday,
	time.Saturday,
}

// NOTE: time.Zero()? what's that?
var (
	EPOCH = time.Unix(0, 0)
)

// NOTE: we have to use shifts here because in their infinite wisdom google didn't make these values powers of two for masking

const (
	ALL_DAYS     = 1<<uint(time.Sunday) | 1<<uint(time.Monday) | 1<<uint(time.Tuesday) | 1<<uint(time.Wednesday) | 1<<uint(time.Thursday) | 1<<uint(time.Friday) | 1<<uint(time.Saturday)
	WEEK_DAYS    = 1<<uint(time.Monday) | 1<<uint(time.Tuesday) | 1<<uint(time.Wednesday) | 1<<uint(time.Thursday) | 1<<uint(time.Friday)
	WEEKEND_DAYS = 1<<uint(time.Sunday) | 1<<uint(time.Saturday)
)

// The Schedule interface defines the form a schedule should take. All schedules are resposible for is giving a next run time after a last run time.
type Schedule interface {
	// Returns the next start time after a given "last run time". Note: after will be `nil` if the job is running for the first time.
	GetNextRunTime(after *time.Time) time.Time
}

type IntervalSchedule struct {
	Every      time.Duration
	StartDelay *time.Duration
}

func (i IntervalSchedule) GetNextRunTime(after *time.Time) time.Time {
	if after == nil {
		if i.StartDelay == nil {
			return time.Now().UTC().Add(i.Every)
		} else {
			return time.Now().UTC().Add(*i.StartDelay).Add(i.Every)
		}
	} else {
		last := *after
		return last.Add(i.Every)
	}
}

func EverySecond() Schedule {
	return IntervalSchedule{Every: 1 * time.Second}
}

func EveryMinute() Schedule {
	return IntervalSchedule{Every: 1 * time.Minute}
}

func EveryHour() Schedule {
	return IntervalSchedule{Every: 1 * time.Hour}
}

func Every(interval time.Duration) Schedule {
	return IntervalSchedule{Every: interval}
}

type DailySchedule struct {
	DayOfWeekMask uint
	TimeOfDayUTC  time.Time
}

func (ds DailySchedule) checkDayOfWeekMask(day time.Weekday) bool {
	trialDayMask := uint(1 << uint(day))
	bitwiseResult := (ds.DayOfWeekMask & trialDayMask)
	return bitwiseResult > uint(0)
}

func (ds DailySchedule) GetNextRunTime(after *time.Time) time.Time {
	if after == nil {
		now := time.Now().UTC()
		after = &now
	}

	todayInstance := time.Date(after.Year(), after.Month(), after.Day(), ds.TimeOfDayUTC.Hour(), ds.TimeOfDayUTC.Minute(), ds.TimeOfDayUTC.Second(), 0, time.UTC)
	for day := 0; day < 8; day++ {
		next := todayInstance.AddDate(0, 0, day) //the first run here it should be adding nothing, i.e. returning todayInstance ...

		if ds.checkDayOfWeekMask(next.Weekday()) && next.After(*after) { //we're on a day ...
			return next
		}
	}

	return EPOCH
}

func WeeklyAt(hour, minute, second int, days ...time.Weekday) Schedule {
	dayOfWeekMask := uint(0)
	for _, day := range days {
		dayOfWeekMask = dayOfWeekMask | 1<<uint(day)
	}

	return &DailySchedule{DayOfWeekMask: dayOfWeekMask, TimeOfDayUTC: time.Date(0, 0, 0, hour, minute, second, 0, time.UTC)}
}

func DailyAt(hour, minute, second int) Schedule {
	return &DailySchedule{DayOfWeekMask: ALL_DAYS, TimeOfDayUTC: time.Date(0, 0, 0, hour, minute, second, 0, time.UTC)}
}

func WeekdaysAt(hour, minute, second int) Schedule {
	return &DailySchedule{DayOfWeekMask: WEEK_DAYS, TimeOfDayUTC: time.Date(0, 0, 0, hour, minute, second, 0, time.UTC)}
}

func WeekendsAt(hour, minute, second int) Schedule {
	return &DailySchedule{DayOfWeekMask: WEEKEND_DAYS, TimeOfDayUTC: time.Date(0, 0, 0, hour, minute, second, 0, time.UTC)}
}
