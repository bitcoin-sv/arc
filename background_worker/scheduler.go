package background_worker

import (
	"fmt"
	"github.com/bitcoin-sv/arc/background_worker/jobs"
	"github.com/go-co-op/gocron"
)

type ARCScheduler struct {
	Scheduler       *gocron.Scheduler
	IntervalInHours int
	Params          jobs.ClearRecrodsParams
}

func (sched *ARCScheduler) RunJob(table string, job func(params jobs.ClearRecrodsParams) error) {
	_, err := sched.Scheduler.Every(sched.IntervalInHours).Hours().Do(func() {
		jobs.Log(jobs.INFO, fmt.Sprintf("Clearing expired %s...", table))
		err := job(sched.Params)
		if err != nil {
			jobs.Log(jobs.ERROR, fmt.Sprintf("unable to clear expired %s %s", table, err))
			return
		}
		jobs.Log(jobs.INFO, fmt.Sprintf("%s cleanup complete", table))
	})

	if err != nil {
		jobs.Log(jobs.ERROR, fmt.Sprintf("unable to run %s job", table))
		return
	}
}

func (sched *ARCScheduler) Start() {
	sched.Scheduler.StartBlocking()
}

func (sched *ARCScheduler) Shutdown() {
	sched.Scheduler.Stop()
}
