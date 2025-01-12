//go:build !solution

package api

import (
	"distributed_build/pkg/build"
	"encoding/json"
	"io"
	"net/http"

	"go.uber.org/zap"
)

type BuildHandler struct {
	logger  *zap.Logger
	service Service
}

func NewBuildService(l *zap.Logger, s Service) *BuildHandler {
	return &BuildHandler{logger: l, service: s}
}

func (h *BuildHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/build", func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			errorMessage := "unable to read request body " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var request BuildRequest
		err = json.Unmarshal(data, &request)
		if err != nil {
			errorMessage := "invalid request format " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusBadRequest)
			return
		}

		ctrl := http.NewResponseController(w)
		writer := NewResponseWriter(w, ctrl)
		err = h.service.StartBuild(r.Context(), &request, writer)
		if err != nil {
			errorMessage := "service error: unable to start build " + err.Error()
			h.logger.Error(errorMessage)
			if !writer.started {
				http.Error(w, errorMessage, http.StatusInternalServerError)
				return
			}
			err = writer.Updated(&StatusUpdate{BuildFailed: &BuildFailed{Error: errorMessage}})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
	})
	mux.HandleFunc("/signal", func(w http.ResponseWriter, r *http.Request) {
		buildIDData := r.URL.Query().Get("build_id")
		var buildID build.ID
		err := buildID.UnmarshalText([]byte(buildIDData))
		if err != nil {
			errorMessage := "unable to read signal buildID: " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusBadRequest)
			return
		}

		data, err := io.ReadAll(r.Body)
		if err != nil {
			errorMessage := "unable to read signal body: " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var signal SignalRequest
		if err := json.Unmarshal(data, &signal); err != nil {
			errorMessage := "invalid signal format: " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusBadRequest)
			return
		}
		resp, err := h.service.SignalBuild(r.Context(), buildID, &signal)
		if err != nil {
			errorMessage := "error signal build " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			return
		}
		respData, err := json.Marshal(resp)
		if err != nil {
			errorMessage := "error generating response " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, err = w.Write(respData)
		if err != nil {
			h.logger.Error("unable to write HeartbeatResponse", zap.Error(err))
			http.Error(w, "error writing response", http.StatusInternalServerError)
		}
	})
}

type ResponseWriter struct {
	w       http.ResponseWriter
	ctrl    *http.ResponseController
	started bool
}

func NewResponseWriter(w http.ResponseWriter, ctrl *http.ResponseController) *ResponseWriter {
	return &ResponseWriter{w: w, ctrl: ctrl, started: false}
}
func (rw *ResponseWriter) Started(rsp *BuildStarted) error {
	rw.started = true

	data, err := json.Marshal(*rsp)
	if err != nil {
		return err
	}

	if _, err := rw.w.Write(data); err != nil {
		return err
	}

	return rw.ctrl.Flush()
}

func (rw *ResponseWriter) Updated(update *StatusUpdate) error {
	data, err := json.Marshal(*update)
	if err != nil {
		return err
	}

	if _, err := rw.w.Write(data); err != nil {
		return err
	}

	return rw.ctrl.Flush()
}
