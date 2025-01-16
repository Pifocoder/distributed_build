package scheduler_test

import (
	"context"
	"testing"
	"time"

	"distributed_build/pkg/api"
	"distributed_build/pkg/build"
	"distributed_build/pkg/scheduler"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupScheduler() (*scheduler.Scheduler, func()) {
	logger, _ := zap.NewDevelopment()
	config := scheduler.Config{
		CacheTimeout: 5 * time.Second,
		DepsTimeout:  10 * time.Second,
	}
	s := scheduler.NewScheduler(logger, config, time.After)
	return s, func() {
		logger.Sync()
	}
}

func TestScheduleJob(t *testing.T) {
	s, teardown := setupScheduler()
	defer teardown()

	job := &api.JobSpec{Job: build.Job{ID: build.NewID()}}
	pendingJob := s.ScheduleJob(job)

	if pendingJob.Job.ID != job.ID {
		t.Errorf("expected job ID %s, got %s", job.ID, pendingJob.Job.ID)
	}
}

func TestOnJobComplete(t *testing.T) {
	s, teardown := setupScheduler()
	defer teardown()

	job := &api.JobSpec{Job: build.Job{ID: build.NewID()}}
	pendingJob := s.ScheduleJob(job)
	s.PickJob(context.Background(), "worker1")

	result := &api.JobResult{ID: job.ID, ExitCode: 0}
	if ok := s.OnJobComplete("worker1", job.ID, result); !ok {
		t.Error("expected OnJobComplete to return true")
	}
	time.Sleep(time.Second)
	select {
	case <-pendingJob.Finished:
		require.Equal(t, pendingJob.Result, result)

	default:
		t.Fatalf("job is not finished")
	}

}
