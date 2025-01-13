//go:build !solution

package artifact

import (
	"distributed_build/pkg/build"
	"distributed_build/pkg/tarstream"
	"net/http"

	"go.uber.org/zap"
)

type Handler struct {
	logger *zap.Logger
	cache  *Cache
}

func NewHandler(l *zap.Logger, c *Cache) *Handler {
	return &Handler{logger: l, cache: c}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/artifact", func(w http.ResponseWriter, r *http.Request) {
		artifactIDData := r.URL.Query().Get("id")
		var artifactID build.ID
		err := artifactID.UnmarshalText([]byte(artifactIDData))
		if err != nil {
			errorMessage := "unable to read get by artifactID: " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusBadRequest)
			return
		}

		path, unlock, err := h.cache.Get(artifactID)
		if err != nil {
			errorMessage := "unable to get by artifactID: " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			return
		}
		defer unlock()

		err = tarstream.Send(path, w)
		if err != nil {
			errorMessage := "unable to send artifact: " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			return
		}
	})
}
