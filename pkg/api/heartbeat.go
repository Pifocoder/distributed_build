package api

import (
	"context"

	"distributed_build/pkg/build"
)

// JobResult описывает результат работы джоба.
type JobResult struct {
	ID build.ID `json:"job_id"`

	Stdout   []byte `json:"stdout"`
	Stderr   []byte `json:"stderr"`
	ExitCode int    `json:"exit_code"`

	// Error описывает сообщение об ошибке, из-за которого джоб не удалось выполнить.
	//
	// Если Error == nil, значит джоб завершился успешно.
	Error *string `json:"error"`
}

type WorkerID string

func (w WorkerID) String() string {
	return string(w)
}

type HeartbeatRequest struct {
	// WorkerID задаёт персистентный идентификатор данного воркера.
	//
	// WorkerID также выступает в качестве endpoint-а, к которому можно подключиться по HTTP.
	//
	// В наших тестах идентификатор будет иметь вид "localhost:%d".
	WorkerID WorkerID `json:"worker_id"`

	// RunningJobs перечисляет список джобов, которые выполняются на этом воркере
	// в данный момент.
	RunningJobs []build.ID `json:"running_jobs"`

	// FreeSlots сообщает, сколько еще процессов можно запустить на этом воркере.
	FreeSlots int `json:"free_slots"`

	// JobResult сообщает координатору, какие джобы завершили исполнение на этом воркере
	// на этой итерации цикла.
	FinishedJob []JobResult `json:"finished_jobs"`

	// AddedArtifacts говорит, какие артефакты появились в кеше на этой итерации цикла.
	AddedArtifacts []build.ID `json:"added_artifacts"`
}

// JobSpec описывает джоб, который нужно запустить.
type JobSpec struct {
	// SourceFiles задаёт список файлов, который должны присутствовать в директории с исходным кодом при запуске этого джоба.
	SourceFiles map[build.ID]string

	// Artifacts задаёт воркеров, с которых можно скачать артефакты необходимые этому джобу.
	Artifacts map[build.ID]WorkerID

	build.Job
}

type HeartbeatResponse struct {
	JobsToRun map[build.ID]JobSpec `json:"jobs_to_run"`
}

type HeartbeatService interface {
	Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error)
}
