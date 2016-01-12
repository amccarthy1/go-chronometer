Chronometer
===========

Chronometer is a basic job scheduling, task handling library that wraps goroutines with a little metadata.

###What is a `CancellationToken`

It is the mechanism by which we signal that a task should abort. We don't have a reference to a thread or similar, so we use a simple object and signal with a boolean. We could use chanels, but this is simpler. 

###Schedules

Schedules are very basic right now, either the job runs on a fixed interval (every minute, every 2 hours etc) or on given days weekly (every day at a time, or once a week at a time).

You're free to implement your own schedules outside the basic ones; a schedule is just an interface for `GetNextRunTime(after time.Time)`.

###Tasks vs. Jobs

Jobs are tasks with schedules, thats about it. The interfaces are very similar otherwise. 