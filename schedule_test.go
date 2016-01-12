package chronometer

import (
	"testing"
	"time"

	"github.com/blendlabs/go-assert"
)

func TestIntervalSchedule(t *testing.T) {
	a := assert.New(t)

	schedule := EveryHour()

	now := time.Now().UTC()

	firstRun := schedule.GetNextRunTime(nil)
	firstRunDiff := firstRun.Sub(now)
	a.InDelta(float64(firstRunDiff), float64(1*time.Hour), float64(1*time.Second))

	next := schedule.GetNextRunTime(&now)
	a.True(next.After(now))
}

func TestDailyScheduleEveryDay(t *testing.T) {
	a := assert.New(t)
	schedule := DailyAt(12, 0, 0) //noon
	now := time.Now().UTC()
	beforenoon := time.Date(now.Year(), now.Month(), now.Day(), 11, 0, 0, 0, time.UTC)
	afternoon := time.Date(now.Year(), now.Month(), now.Day(), 13, 0, 0, 0, time.UTC)
	todayAtNoon := schedule.GetNextRunTime(&beforenoon)
	tomorrowAtNoon := schedule.GetNextRunTime(&afternoon)

	a.True(todayAtNoon.Before(afternoon))
	a.True(tomorrowAtNoon.After(afternoon))
}

func TestDailyScheduleSingleDay(t *testing.T) {
	a := assert.New(t)
	schedule := WeeklyAt(12, 0, 0, time.Monday)                  //every monday at noon
	beforenoon := time.Date(2016, 01, 11, 11, 0, 0, 0, time.UTC) //these are both a monday
	afternoon := time.Date(2016, 01, 11, 13, 0, 0, 0, time.UTC)  //these are both a monday

	sundayBeforeNoon := time.Date(2016, 01, 17, 11, 0, 0, 0, time.UTC) //to gut check that it's monday

	todayAtNoon := schedule.GetNextRunTime(&beforenoon)
	nextWeekAtNoon := schedule.GetNextRunTime(&afternoon)

	a.NonFatal().True(todayAtNoon.Before(afternoon))
	a.NonFatal().True(nextWeekAtNoon.After(afternoon))
	a.NonFatal().True(nextWeekAtNoon.After(sundayBeforeNoon))
	a.NonFatal().Equal(time.Monday, nextWeekAtNoon.Weekday())
}
