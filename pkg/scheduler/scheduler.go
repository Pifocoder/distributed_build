//go:build !solution

package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"

	"distributed_build/pkg/api"
	"distributed_build/pkg/build"
)

type PendingJob struct {
	Job      *api.JobSpec
	Finished chan struct{}
	Result   *api.JobResult
}

type Config struct {
	CacheTimeout time.Duration
	DepsTimeout  time.Duration
}

type Scheduler struct {
	logger    *zap.Logger
	timeAfter func(d time.Duration) <-chan time.Time
	queue     chan *api.JobSpec

	config      Config
	jobCache    map[string][]string
	workingJobs map[build.ID]api.WorkerID
}

func NewScheduler(l *zap.Logger, config Config, timeAfter func(d time.Duration) <-chan time.Time) *Scheduler {
	return &Scheduler{logger: l, config: config, timeAfter: timeAfter, queue: make(chan *api.JobSpec), jobCache: map[string][]string{}}
}

func (c *Scheduler) LocateArtifact(id build.ID) (api.WorkerID, bool) {
	res, ok := (c.workingJobs[id])
	return res, ok
}

func (c *Scheduler) OnJobComplete(workerID api.WorkerID, jobID build.ID, res *api.JobResult) bool {
	// arr, ok := c.jobCache[workerID.String()]
	// if !ok {
	// 	c.jobCache[workerID.String()] = []string{jobID.String()}
	// 	return true
	// }
	return false
}

func (c *Scheduler) ScheduleJob(job *api.JobSpec) *PendingJob {
	c.queue <- job
	return &PendingJob{
		Job:      job,
		Finished: make(chan struct{}),
		Result:   nil,
	}
}

func (c *Scheduler) PickJob(ctx context.Context, workerID api.WorkerID) *PendingJob {
	select {
	case job, ok := <-c.queue:
		if !ok {
			return nil
		}
		c.workingJobs[job.ID] = workerID
		return &PendingJob{
			Job:      job,
			Finished: make(chan struct{}),
			Result:   nil,
		}
	case <-ctx.Done():
		return nil
	}
}

func (c *Scheduler) Stop() {
	close(c.queue)
}
