package scheduler_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"

	"distributed_build/pkg/api"
	"distributed_build/pkg/build"
	"distributed_build/pkg/scheduler"
)

const (
	workerID0 api.WorkerID = "w0"
)

var (
	config = scheduler.Config{
		CacheTimeout: time.Second,
		DepsTimeout:  time.Second * 5,
	}
)

type testScheduler struct {
	*scheduler.Scheduler
	clockwork.FakeClock
	reset chan struct{}
}

func newTestScheduler(t *testing.T) *testScheduler {
	log := zaptest.NewLogger(t)

	fakeClock := clockwork.NewFakeClock()

	s := &testScheduler{
		FakeClock: *fakeClock,
		Scheduler: scheduler.NewScheduler(log, config, fakeClock.After),
		reset:     make(chan struct{}),
	}

	go func() {
		select {
		case <-time.After(time.Minute * 5):
			panic("test hang")
		case <-s.reset:
			return
		}
	}()

	return s
}

func (s *testScheduler) stop(t *testing.T) {
	close(s.reset)
	s.Scheduler.Stop()
	goleak.VerifyNone(t)
}

func TestScheduler_SingleJob(t *testing.T) {
	s := newTestScheduler(t)
	defer s.stop(t)

	job0 := &api.JobSpec{Job: build.Job{ID: build.NewID()}}
	pendingJob0 := s.ScheduleJob(job0)

	//s.BlockUntil(1)
	s.Advance(config.DepsTimeout) // At this point job must be in global queue.
	time.Sleep(10 * time.Second)
	s.RegisterWorker(workerID0)
	pickerJob := s.PickJob(context.Background(), workerID0)
	fmt.Println("PickJob")
	require.Equal(t, pendingJob0, pickerJob)

	result := &api.JobResult{ID: job0.ID, ExitCode: 0}
	s.OnJobComplete(workerID0, job0.ID, result)
	fmt.Println("OnJobComplete")
	select {
	case <-pendingJob0.Finished:
		require.Equal(t, pendingJob0.Result, result)

	default:
		t.Fatalf("job0 is not finished")
	}
}

func TestScheduler_PickJobCancelation(t *testing.T) {
	s := newTestScheduler(t)
	defer s.stop(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.RegisterWorker(workerID0)
	require.Nil(t, s.PickJob(ctx, workerID0))
}

func TestScheduler_CacheLocalScheduling(t *testing.T) {
	s := newTestScheduler(t)
	defer s.stop(t)

	cachedJob := &api.JobSpec{Job: build.Job{ID: build.NewID()}}
	uncachedJob := &api.JobSpec{Job: build.Job{ID: build.NewID()}}

	s.RegisterWorker(workerID0)
	s.OnJobComplete(workerID0, cachedJob.ID, &api.JobResult{})

	pendingUncachedJob := s.ScheduleJob(uncachedJob)
	pendingCachedJob := s.ScheduleJob(cachedJob)

	s.BlockUntil(2) // both jobs should be blocked

	firstPickedJob := s.PickJob(context.Background(), workerID0)
	assert.Equal(t, pendingCachedJob, firstPickedJob)

	s.Advance(config.DepsTimeout) // At this point uncachedJob is put into global queue.

	secondPickedJob := s.PickJob(context.Background(), workerID0)
	assert.Equal(t, pendingUncachedJob, secondPickedJob)
}

func TestScheduler_DependencyLocalScheduling(t *testing.T) {
	s := newTestScheduler(t)
	defer s.stop(t)

	job0 := &api.JobSpec{Job: build.Job{ID: build.NewID()}}
	s.RegisterWorker(workerID0)
	s.OnJobComplete(workerID0, job0.ID, &api.JobResult{})

	job1 := &api.JobSpec{Job: build.Job{ID: build.NewID(), Deps: []build.ID{job0.ID}}}
	job2 := &api.JobSpec{Job: build.Job{ID: build.NewID()}}

	pendingJob2 := s.ScheduleJob(job2)
	pendingJob1 := s.ScheduleJob(job1)

	s.BlockUntil(2) // both jobs should be blocked on DepsTimeout

	firstPickedJob := s.PickJob(context.Background(), workerID0)
	require.Equal(t, pendingJob1, firstPickedJob)

	s.Advance(config.DepsTimeout) // At this point job2 is put into global queue.

	secondPickedJob := s.PickJob(context.Background(), workerID0)
	require.Equal(t, pendingJob2, secondPickedJob)
}
