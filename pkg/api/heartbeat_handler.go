//go:build !solution

package api

import (
	"encoding/json"
	"io"
	"net/http"

	"go.uber.org/zap"
)

type HeartbeatHandler struct {
	logger  *zap.Logger
	service HeartbeatService
}

func NewHeartbeatHandler(l *zap.Logger, s HeartbeatService) *HeartbeatHandler {
	return &HeartbeatHandler{logger: l, service: s}
}

func (h *HeartbeatHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		// Read the request body
		data, err := io.ReadAll(r.Body)
		if err != nil {
			errorMessage := "unable to read request body " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var request HeartbeatRequest
		err = json.Unmarshal(data, &request)
		if err != nil {
			errorMessage := "invalid request format " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusBadRequest)
			return
		}

		// Call the service's Heartbeat method
		resp, err := h.service.Heartbeat(r.Context(), &request)
		if err != nil {
			errorMessage := "service error " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			return
		}

		// Marshal response data
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
