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
	queue     chan *PendingJob

	config        Config
	jobCache      map[build.ID][]api.WorkerID
	workerPending map[api.WorkerID]*PendingJob
}

func NewScheduler(l *zap.Logger, config Config, timeAfter func(d time.Duration) <-chan time.Time) *Scheduler {
	return &Scheduler{
		logger:        l,
		config:        config,
		timeAfter:     timeAfter,
		queue:         make(chan *PendingJob),
		workerPending: make(map[api.WorkerID]*PendingJob),
		jobCache:      make(map[build.ID][]api.WorkerID),
	}
}

func (c *Scheduler) LocateArtifact(id build.ID) (api.WorkerID, bool) {
	worker, ok := c.jobCache[id]
	if !ok || len(worker) == 0 {
		return "", false
	}
	return worker[0], true
}

func (c *Scheduler) OnJobComplete(workerID api.WorkerID, jobID build.ID, res *api.JobResult) bool {
	c.jobCache[jobID] = append(c.jobCache[jobID], workerID)
	_, ok := c.workerPending[workerID]
	if !ok {
		return false
	}
	c.workerPending[workerID].Result = res
	go func() {
		c.workerPending[workerID].Finished <- struct{}{}
	}()

	return true
}

func (c *Scheduler) ScheduleJob(job *api.JobSpec) *PendingJob {

	pendingJob := &PendingJob{
		Job:      job,
		Finished: make(chan struct{}),
		Result:   nil,
	}
	go func() {
		c.queue <- pendingJob
	}()
	return pendingJob
}

func (c *Scheduler) PickJob(ctx context.Context, workerID api.WorkerID) *PendingJob {
	select {
	case job, ok := <-c.queue:
		if !ok {
			return nil
		}
		c.workerPending[workerID] = job
		return job
	case <-ctx.Done():
		return nil
	}
}

func (c *Scheduler) Stop() {
	close(c.queue)
}
