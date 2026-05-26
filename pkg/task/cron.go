package task

import (
	"time"

	"github.com/robfig/cron/v3"
)

var parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func NextRunTime(cronExpr string, after time.Time) (time.Time, error) {
	sched, err := parser.Parse(cronExpr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(after), nil
}
