package api

import (
	"context"

	"distributed_build/pkg/build"
)

type BuildRequest struct {
	Graph build.Graph `json:"graph"`
}

type BuildStarted struct {
	ID           build.ID   `json:"id"`
	MissingFiles []build.ID `json:"missing_files"`
}

type StatusUpdate struct {
	JobFinished   *JobResult     `json:"job_finished"`
	BuildFailed   *BuildFailed   `json:"build_failed"`
	BuildFinished *BuildFinished `json:"build_finished"`
}

type BuildFailed struct {
	Error string `json:"error"`
}

type BuildFinished struct {
}

type UploadDone struct{}

type SignalRequest struct {
	UploadDone *UploadDone `json:"upload_done"`
}

type SignalResponse struct {
}

type StatusWriter interface {
	Started(rsp *BuildStarted) error
	Updated(update *StatusUpdate) error
}

type Service interface {
	StartBuild(ctx context.Context, request *BuildRequest, w StatusWriter) error
	SignalBuild(ctx context.Context, buildID build.ID, signal *SignalRequest) (*SignalResponse, error)
}

type StatusReader interface {
	Close() error
	Next() (*StatusUpdate, error)
}
