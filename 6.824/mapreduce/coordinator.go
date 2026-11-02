package mapreduce

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const DefaultTaskTimeout = 10 * time.Second

type taskState struct {
	attempt int
	started time.Time
	done    bool
}

// Coordinator schedules one job. All workers must share its filesystem.
type Coordinator struct {
	mu      sync.Mutex
	inputs  []string
	maps    []taskState
	reduces []taskState
	dir     string
	timeout time.Duration
}

// NewCoordinator reserves a new output directory; existing directories are rejected
// so concurrent jobs and earlier results cannot overwrite one another.
func NewCoordinator(files []string, nReduce int, dir string, timeout time.Duration) (*Coordinator, error) {
	if nReduce < 1 || timeout <= 0 {
		return nil, fmt.Errorf("reduce count and task timeout must be positive")
	}
	inputs := make([]string, len(files))
	for i, name := range files {
		abs, err := filepath.Abs(name)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("input %q is not a regular file", name)
		}
		inputs[i] = abs
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absDir), 0755); err != nil {
		return nil, err
	}
	if err := os.Mkdir(absDir, 0755); err != nil {
		return nil, fmt.Errorf("create new output directory: %w", err)
	}
	return &Coordinator{
		inputs: inputs, maps: make([]taskState, len(files)),
		reduces: make([]taskState, nReduce), dir: absDir, timeout: timeout,
	}, nil
}

func allDone(tasks []taskState) bool {
	for _, task := range tasks {
		if !task.done {
			return false
		}
	}
	return true
}

func (c *Coordinator) RequestTask(_ *TaskRequest, reply *Task) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	*reply = Task{Type: WaitTask}
	kind, tasks := MapTask, c.maps
	if allDone(c.maps) {
		kind, tasks = ReduceTask, c.reduces
		if allDone(c.reduces) {
			reply.Type = ExitTask
			return nil
		}
	}
	now := time.Now()
	for id := range tasks {
		state := &tasks[id]
		if state.done || (!state.started.IsZero() && now.Sub(state.started) < c.timeout) {
			continue
		}
		state.attempt++
		state.started = now
		*reply = Task{Type: kind, ID: id, Attempt: state.attempt, NReduce: len(c.reduces), Dir: c.dir}
		if kind == MapTask {
			reply.Inputs = []string{c.inputs[id]}
		} else {
			reply.Inputs = make([]string, len(c.maps))
			for mapID, task := range c.maps {
				reply.Inputs[mapID] = mapOutput(c.dir, mapID, task.attempt, id)
			}
		}
		return nil
	}
	return nil
}

func (c *Coordinator) ReportTask(report *TaskReport, reply *ReportReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	*reply = ReportReply{}
	var tasks []taskState
	switch report.Type {
	case MapTask:
		tasks = c.maps
	case ReduceTask:
		tasks = c.reduces
	default:
		return fmt.Errorf("invalid task type %q", report.Type)
	}
	if report.ID < 0 || report.ID >= len(tasks) || report.Attempt < 1 {
		return fmt.Errorf("invalid task ID or attempt")
	}
	state := &tasks[report.ID]
	// A late worker must never publish over a newer attempt. Duplicate successful
	// reports remain accepted, including when the original RPC reply was lost.
	if state.attempt != report.Attempt {
		return nil
	}
	if state.done {
		reply.Accepted = true
		return nil
	}
	if state.started.IsZero() {
		return nil
	}
	if report.Failed {
		state.started = time.Time{}
		return nil
	}
	task := Task{Type: report.Type, ID: report.ID, Attempt: report.Attempt, Dir: c.dir, NReduce: len(c.reduces)}
	for _, name := range outputPaths(task) {
		info, err := os.Stat(name)
		if err != nil {
			return fmt.Errorf("missing task output: %w", err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("output %q is not a regular file", name)
		}
	}
	if report.Type == ReduceTask {
		source := reduceOutput(c.dir, report.ID, report.Attempt)
		destination := filepath.Join(c.dir, fmt.Sprintf("mr-out-%d", report.ID))
		if err := os.Rename(source, destination); err != nil {
			return err
		}
	}
	state.done = true
	reply.Accepted = true
	return nil
}

func (c *Coordinator) Done() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return allDone(c.reduces)
}

// Serve accepts TCP RPC requests until the caller closes the listener.
func (c *Coordinator) Serve(listener net.Listener) error {
	server := rpc.NewServer()
	if err := server.RegisterName("Coordinator", c); err != nil {
		return err
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go server.ServeConn(conn)
	}
}
