package gfs

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
)

// Master keeps metadata in memory; restarting it forgets all files.
type Master struct {
	mu        sync.RWMutex
	servers   []string
	chunkSize int
	replicas  int
	files     map[string]File
	pending   map[string]File
}

func NewMaster(servers []string, chunkSize, replicas int) (*Master, error) {
	if chunkSize < 1 || chunkSize > MaxChunkSize {
		return nil, fmt.Errorf("chunk size must be between 1 and %d", MaxChunkSize)
	}
	if replicas < 1 || replicas > len(servers) {
		return nil, fmt.Errorf("replicas must be between 1 and the number of servers")
	}
	normalized := make([]string, len(servers))
	seen := make(map[string]bool)
	for i, server := range servers {
		server = strings.TrimRight(strings.TrimSpace(server), "/")
		u, err := url.Parse(server)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return nil, fmt.Errorf("invalid chunkserver URL: %q", server)
		}
		if seen[server] {
			return nil, fmt.Errorf("duplicate chunkserver: %s", server)
		}
		seen[server] = true
		normalized[i] = server
	}
	return &Master{
		servers: normalized, chunkSize: chunkSize, replicas: replicas,
		files: make(map[string]File), pending: make(map[string]File),
	}, nil
}

func (m *Master) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /allocate", m.allocate)
	mux.HandleFunc("POST /commit", m.commit)
	mux.HandleFunc("GET /file", m.lookup)
	mux.HandleFunc("GET /files", m.list)
	return mux
}

func (m *Master) allocate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Size int    `json:"size"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" || req.Size < 0 {
		http.Error(w, "name and nonnegative size are required", http.StatusBadRequest)
		return
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(id[:])
	file := File{Name: req.Name, Size: req.Size, ChunkSize: m.chunkSize, Chunks: []Chunk{}}
	count := req.Size / m.chunkSize
	if req.Size%m.chunkSize != 0 {
		count++
	}
	// Bound metadata allocation independently of the client's declared file size.
	if count > 10000 {
		http.Error(w, "file exceeds 10000 chunks", http.StatusRequestEntityTooLarge)
		return
	}
	for i := 0; i < count; i++ {
		chunk := Chunk{ID: fmt.Sprintf("%s-%d", token, i)}
		for j := 0; j < m.replicas; j++ {
			chunk.Servers = append(chunk.Servers, m.servers[(i+j)%len(m.servers)])
		}
		file.Chunks = append(file.Chunks, chunk)
	}
	m.mu.Lock()
	m.pending[token] = file
	m.mu.Unlock()
	writeJSON(w, allocation{Token: token, File: file})
}

func (m *Master) commit(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	token := r.URL.Query().Get("token")
	file, ok := m.pending[token]
	if !ok {
		http.Error(w, "unknown upload token", http.StatusNotFound)
		return
	}
	// Clients commit only after every replica has acknowledged the new chunks.
	m.files[file.Name] = file
	delete(m.pending, token)
	w.WriteHeader(http.StatusNoContent)
}

func (m *Master) lookup(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	file, ok := m.files[r.URL.Query().Get("name")]
	m.mu.RUnlock()
	if !ok {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	writeJSON(w, file)
}

func (m *Master) list(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	files := make([]File, 0, len(m.files))
	for _, file := range m.files {
		files = append(files, file)
	}
	m.mu.RUnlock()
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	writeJSON(w, files)
}
