package raft

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// PersistentState 是 Raft 必须落盘的全部状态：任期、投票、日志和快照。
// 日志第 0 项是快照边界，真实下标从 SnapshotIndex 开始隐式编号。
type PersistentState struct {
	CurrentTerm   int
	VotedFor      int // -1 表示本任期还没有投票
	SnapshotIndex int
	SnapshotTerm  int
	Snapshot      []byte
	Log           []LogEntry
}

// Storage 保存 Raft 的持久状态。Save 必须在返回前把数据写入稳定存储，
// 因为 Raft 在回复 RPC 之前必须确认状态已经落盘。
type Storage interface {
	Save(PersistentState) error
	Load() (PersistentState, error)
}

// MemoryStorage 只用于测试和进程内的演示集群。
type MemoryStorage struct {
	mu     sync.Mutex
	state  PersistentState
	exists bool
}

func NewMemoryStorage() *MemoryStorage { return &MemoryStorage{} }

func (m *MemoryStorage) Save(state PersistentState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = cloneState(state)
	m.exists = true
	return nil
}

func (m *MemoryStorage) Load() (PersistentState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.exists {
		return PersistentState{VotedFor: -1}, nil
	}
	return cloneState(m.state), nil
}

func cloneState(state PersistentState) PersistentState {
	out := PersistentState{
		CurrentTerm:   state.CurrentTerm,
		VotedFor:      state.VotedFor,
		SnapshotIndex: state.SnapshotIndex,
		SnapshotTerm:  state.SnapshotTerm,
		Snapshot:      append([]byte(nil), state.Snapshot...),
		Log:           make([]LogEntry, len(state.Log)),
	}
	for i, entry := range state.Log {
		out.Log[i] = LogEntry{Term: entry.Term, Command: append([]byte(nil), entry.Command...)}
	}
	return out
}

// FileStorage 把状态写成一个 JSON 文件：先写临时文件并 fsync，再原子重命名。
// 整体重写足够教学使用，真实系统会改成追加写日志加分段压缩。
type FileStorage struct {
	mu   sync.Mutex
	path string
}

func NewFileStorage(dir string) (*FileStorage, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &FileStorage{path: filepath.Join(dir, "raft-state.json")}, nil
}

func (f *FileStorage) Save(state PersistentState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	tmp := f.path + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

func (f *FileStorage) Load() (PersistentState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return PersistentState{VotedFor: -1}, nil
	}
	if err != nil {
		return PersistentState{}, err
	}
	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return PersistentState{}, err
	}
	return state, nil
}
