package mapreduce

import (
	"fmt"
	"path/filepath"
)

// KeyValue is the intermediate record produced by Map and grouped for Reduce.
type KeyValue struct {
	Key   string
	Value string
}

type TaskType string

const (
	MapTask    TaskType = "map"
	ReduceTask TaskType = "reduce"
	WaitTask   TaskType = "wait"
	ExitTask   TaskType = "exit"
)

type Task struct {
	Type    TaskType
	ID      int
	Attempt int
	Inputs  []string
	NReduce int
	Dir     string
}

type TaskRequest struct{}

type TaskReport struct {
	Type    TaskType
	ID      int
	Attempt int
	Failed  bool
}

type ReportReply struct {
	Accepted bool
}

func mapOutput(dir string, id, attempt, partition int) string {
	return filepath.Join(dir, fmt.Sprintf("mr-map-%d-%d-%d.json", id, attempt, partition))
}

func reduceOutput(dir string, id, attempt int) string {
	return filepath.Join(dir, fmt.Sprintf("mr-reduce-%d-%d.tmp", id, attempt))
}

func outputPaths(task Task) []string {
	if task.Type == ReduceTask {
		return []string{reduceOutput(task.Dir, task.ID, task.Attempt)}
	}
	paths := make([]string, task.NReduce)
	for partition := range paths {
		paths[partition] = mapOutput(task.Dir, task.ID, task.Attempt, partition)
	}
	return paths
}
