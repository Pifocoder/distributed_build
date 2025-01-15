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

	config        Config
	jobCache      map[build.ID][]api.WorkerID
	jobWorker     map[build.ID]api.WorkerID
	wokerIDWorker map[build.ID]api.WorkerID
	workers       []Worker
}
type Worker struct {
	WorkerID         api.WorkerID
	firstLocalQueue  chan *api.JobSpec
	secondLocalQueue chan *api.JobSpec
}

func NewScheduler(l *zap.Logger, config Config, timeAfter func(d time.Duration) <-chan time.Time) *Scheduler {
	return &Scheduler{
		logger:    l,
		config:    config,
		timeAfter: timeAfter,
		queue:     make(chan *api.JobSpec),
		jobCache:  make(map[build.ID][]api.WorkerID),
		jobWorker: make(map[build.ID]api.WorkerID),
	}
}

func (c *Scheduler) RegisterWorker(workerID api.WorkerID) {
	c.workers = append(c.workers, Worker{
		WorkerID:         workerID,
		firstLocalQueue:  make(chan *api.JobSpec),
		secondLocalQueue: make(chan *api.JobSpec),
	})
}
func (c *Scheduler) LocateArtifact(id build.ID) (api.WorkerID, bool) {
	res, ok := c.jobWorker[id]
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
	cacheTimeout := c.timeAfter(c.config.CacheTimeout)
	depsTimeout := c.timeAfter(c.config.DepsTimeout)
	jobFinished := make(chan struct{})
	go func(jobFinished chan struct{}) {
		select {
		case _, ok := <-cacheTimeout:
			if !ok {
				return
			}
		case _, ok := <-depsTimeout:
			if !ok {
				return
			}
		case <-jobFinished:
			return
		}
	}(jobFinished)

	suitableWorkers, ok := c.jobCache[job.ID]
	if !ok {
		for _, jobID := range job.Deps {
			workerID, ok := job.Artifacts[jobID]
			if !ok {
				continue
			}

			c.secondLocalWorkerQueue[workerID] <- job
		}
	} else {
		for _, workerID := range suitableWorkers {
			c.firstLocalWorkerQueue[workerID] <- job
		}
	}

	return &PendingJob{
		Job:      job,
		Finished: jobFinished,
		Result:   nil,
	}
}

func (c *Scheduler) PickJob(ctx context.Context, workerID api.WorkerID) *PendingJob {
	select {
	case job, ok := <-c.firstLocalWorkerQueue[workerID]:
		if !ok {
			return nil
		}
		c.jobWorker[job.ID] = workerID
		return &PendingJob{
			Job:      job,
			Finished: make(chan struct{}),
			Result:   nil,
		}
	case job, ok := <-c.secondLocalWorkerQueue[workerID]:
		if !ok {
			return nil
		}
		c.jobWorker[job.ID] = workerID
		return &PendingJob{
			Job:      job,
			Finished: make(chan struct{}),
			Result:   nil,
		}
	case job, ok := <-c.queue:
		if !ok {
			return nil
		}
		c.jobWorker[job.ID] = workerID
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
