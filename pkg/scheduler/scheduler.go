//go:build !solution

package scheduler

import (
	"context"
	"fmt"
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

	config                 Config
	jobCache               map[build.ID][]api.WorkerID
	jobWorker              map[build.ID]api.WorkerID
	firstLocalWorkerQueue  map[api.WorkerID]chan *api.JobSpec
	secondLocalWorkerQueue map[api.WorkerID]chan *api.JobSpec
	workerPending          map[api.WorkerID]*PendingJob
}

func NewScheduler(l *zap.Logger, config Config, timeAfter func(d time.Duration) <-chan time.Time) *Scheduler {
	return &Scheduler{
		logger:    l,
		config:    config,
		timeAfter: timeAfter,
		queue:     make(chan *api.JobSpec),

		jobCache:               make(map[build.ID][]api.WorkerID),
		jobWorker:              make(map[build.ID]api.WorkerID),
		workerPending:          make(map[api.WorkerID]*PendingJob),
		firstLocalWorkerQueue:  make(map[api.WorkerID]chan *api.JobSpec),
		secondLocalWorkerQueue: make(map[api.WorkerID]chan *api.JobSpec),
	}
}

func (c *Scheduler) RegisterWorker(workerID api.WorkerID) {
	c.firstLocalWorkerQueue[workerID] = make(chan *api.JobSpec)
	c.secondLocalWorkerQueue[workerID] = make(chan *api.JobSpec)
}
func (c *Scheduler) LocateArtifact(id build.ID) (api.WorkerID, bool) {
	res, ok := c.jobWorker[id]
	return res, ok
}

func (c *Scheduler) OnJobComplete(workerID api.WorkerID, jobID build.ID, res *api.JobResult) bool {
	c.jobCache[jobID] = append(c.jobCache[jobID], workerID)
	_, ok := c.workerPending[workerID]
	if !ok {
		return false
	}
	c.workerPending[workerID].Result = res
	c.workerPending[workerID].Finished <- struct{}{}
	// arr, ok := c.jobCache[workerID.String()]
	// if !ok {
	// 	c.jobCache[workerID.String()] = []string{jobID.String()}
	// 	return true
	// }
	return true
}

func (c *Scheduler) ScheduleJob(job *api.JobSpec) *PendingJob {
	jobFinished := make(chan struct{})

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

	go func() {
		select {
		case <-c.timeAfter(c.config.CacheTimeout):
			fmt.Println("CacheTimeout")
			// Если задача ждёт дольше CacheTimeout, добавляем во все вторые локальные очереди
			for _, jobID := range job.Deps {
				workerID, ok := job.Artifacts[jobID]
				if !ok {
					continue
				}
				c.secondLocalWorkerQueue[workerID] <- job
			}
			return
		case <-jobFinished:
			fmt.Println("jobFinished")
			return // Задача завершена до истечения таймаута
		}
	}()

	go func() {
		select {
		case <-time.After(c.config.DepsTimeout):
			fmt.Println("DepsTimeout")
			c.queue <- job
			return
		case <-jobFinished:
			fmt.Println("jobFinished")
			return
		}
	}()

	return &PendingJob{
		Job:      job,
		Finished: jobFinished,
		Result:   nil,
	}
}

func (c *Scheduler) PickJob(ctx context.Context, workerID api.WorkerID) *PendingJob {
	fmt.Println(workerID, c.queue)
	select {
	case job, ok := <-c.firstLocalWorkerQueue[workerID]:
		if !ok {
			return nil
		}
		c.jobWorker[job.ID] = workerID
		c.workerPending[workerID] = &PendingJob{
			Job:      job,
			Finished: make(chan struct{}),
			Result:   nil,
		}
		return c.workerPending[workerID]
	case job, ok := <-c.secondLocalWorkerQueue[workerID]:
		if !ok {
			return nil
		}
		c.jobWorker[job.ID] = workerID
		c.workerPending[workerID] = &PendingJob{
			Job:      job,
			Finished: make(chan struct{}),
			Result:   nil,
		}
		return c.workerPending[workerID]
	case job, ok := <-c.queue:
		if !ok {
			return nil
		}
		c.workerPending[workerID] = &PendingJob{
			Job:      job,
			Finished: make(chan struct{}),
			Result:   nil,
		}
		return c.workerPending[workerID]
	case <-ctx.Done():
		return nil
	}
}

func (c *Scheduler) Stop() {
	close(c.queue)
}
