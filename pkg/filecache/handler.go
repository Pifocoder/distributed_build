//go:build !solution

package filecache

import (
	"distributed_build/pkg/build"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/sync/singleflight"

	"go.uber.org/zap"
)

type Handler struct {
	group  *singleflight.Group
	logger *zap.Logger
	cache  *Cache
}

func NewHandler(l *zap.Logger, cache *Cache) *Handler {
	return &Handler{logger: l, cache: cache, group: new(singleflight.Group)}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		fileIDData := r.URL.Query().Get("id")
		var fileID build.ID
		err := fileID.UnmarshalText([]byte(fileIDData))
		if err != nil {
			errorMessage := "unable to read get by fileID: " + err.Error()
			h.logger.Error(errorMessage)
			http.Error(w, errorMessage, http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			h.getFile(w, r, fileID)
		case http.MethodPut:
			h.putFile(w, r, fileID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func (h *Handler) getFile(w http.ResponseWriter, r *http.Request, id build.ID) {
	path, unlock, err := h.cache.Get(id)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer unlock()

	http.ServeFile(w, r, path)
}

type WriterAbort struct {
	Writer io.WriteCloser
	Abort  func() error
}

func (h *Handler) putFile(w http.ResponseWriter, r *http.Request, id build.ID) {
	defer r.Body.Close()
	_, err, _ := h.group.Do(id.String(), func() (any, error) {
		err := h.cache.Remove(id)
		if err != nil {
			return nil, fmt.Errorf("unable to remove exisring file: %s", err.Error())
		}
		writer, abort, err := h.cache.Write(id)
		if err != nil {
			return nil, fmt.Errorf("invalid cache entry: %s", err.Error())
		}
		if _, err := io.Copy(writer, r.Body); err != nil {
			abort() // Call abort if there's an error while writing
			return nil, fmt.Errorf("could not write to cache: %s", err.Error())
		}
		defer writer.Close()
		return nil, nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
