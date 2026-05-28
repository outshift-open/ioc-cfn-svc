package task

import (
	"time"

	"github.com/robfig/cron/v3"
)

// parser is a standard 5-field cron parser (minute, hour, day-of-month, month, day-of-week).
// Used only for computing the next run time — not for running a cron daemon.
var parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// NextRunTime computes the next fire time for the given cron expression after the specified time.
// The expression must be a valid 5-field cron string (e.g., "0 */1 * * *").
func NextRunTime(cronExpr string, after time.Time) (time.Time, error) {
	sched, err := parser.Parse(cronExpr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(after), nil
}
