package raft

import (
	"bytes"
	"testing"
)

func TestMemoryStorageRoundTrip(t *testing.T) {
	storage := NewMemoryStorage()
	state := PersistentState{
		CurrentTerm:   3,
		VotedFor:      1,
		SnapshotIndex: 5,
		SnapshotTerm:  2,
		Snapshot:      []byte("snap"),
		Log: []LogEntry{
			{Term: 2},
			{Term: 3, Command: []byte("cmd")},
		},
	}
	if err := storage.Save(state); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	got, err := storage.Load()
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if got.CurrentTerm != 3 || got.VotedFor != 1 || got.SnapshotIndex != 5 || got.SnapshotTerm != 2 {
		t.Fatalf("读取到的状态不对: %+v", got)
	}
	if !bytes.Equal(got.Snapshot, []byte("snap")) || len(got.Log) != 2 {
		t.Fatalf("快照或日志没有恢复: %+v", got)
	}
}

func TestFileStorageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	storage, err := NewFileStorage(dir)
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}
	empty, err := storage.Load()
	if err != nil {
		t.Fatalf("空目录读取失败: %v", err)
	}
	if empty.VotedFor != -1 || len(empty.Log) != 0 {
		t.Fatalf("空目录应返回未投票的空状态: %+v", empty)
	}

	state := PersistentState{
		CurrentTerm: 4,
		VotedFor:    2,
		Log: []LogEntry{
			{Term: 4},
			{Term: 4, Command: []byte("hello")},
		},
	}
	if err := storage.Save(state); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	// 再存一次，验证覆盖写入不会留下半截文件。
	state.CurrentTerm = 5
	if err := storage.Save(state); err != nil {
		t.Fatalf("覆盖保存失败: %v", err)
	}

	reopened, err := NewFileStorage(dir)
	if err != nil {
		t.Fatalf("重新打开失败: %v", err)
	}
	got, err := reopened.Load()
	if err != nil {
		t.Fatalf("重新读取失败: %v", err)
	}
	if got.CurrentTerm != 5 || got.VotedFor != 2 {
		t.Fatalf("重新读到的状态不对: %+v", got)
	}
	if len(got.Log) != 2 || string(got.Log[1].Command) != "hello" {
		t.Fatalf("日志没有正确恢复: %+v", got.Log)
	}
}
