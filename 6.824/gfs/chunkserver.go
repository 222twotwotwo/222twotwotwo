package gfs

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

var chunkID = regexp.MustCompile(`^[0-9a-f]{32}-[0-9]+$`)

// NewChunkServer stores each chunk as a file in dir.
func NewChunkServer(dir string) (http.Handler, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/chunks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !chunkID.MatchString(id) {
			http.Error(w, "invalid chunk ID", http.StatusBadRequest)
			return
		}
		path := filepath.Join(dir, id)
		switch r.Method {
		case http.MethodGet:
			http.ServeFile(w, r, path)
		case http.MethodPut:
			if err := storeChunk(w, r, dir, path); err != nil {
				status := http.StatusInternalServerError
				var tooLarge *http.MaxBytesError
				if errors.As(err, &tooLarge) {
					status = http.StatusRequestEntityTooLarge
				}
				http.Error(w, err.Error(), status)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Allow", "GET, PUT")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	return mux, nil
}

func storeChunk(w http.ResponseWriter, r *http.Request, dir, path string) error {
	f, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	if _, err = io.Copy(f, http.MaxBytesReader(w, r.Body, MaxChunkSize)); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
