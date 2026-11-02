package gfs

import (
	"encoding/json"
	"net/http"
)

const MaxChunkSize = 64 << 20

// File is the metadata returned by the master. Data lives on chunkservers.
type File struct {
	Name      string  `json:"name"`
	Size      int     `json:"size"`
	ChunkSize int     `json:"chunk_size"`
	Chunks    []Chunk `json:"chunks"`
}

type Chunk struct {
	ID      string   `json:"id"`
	Servers []string `json:"servers"`
}

type allocation struct {
	Token string `json:"token"`
	File  File   `json:"file"`
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
