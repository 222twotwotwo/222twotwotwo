package mapreduce

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newJob(t *testing.T, contents []string, nReduce int) *Coordinator {
	t.Helper()
	dir := t.TempDir()
	files := make([]string, len(contents))
	for i, content := range contents {
		files[i] = filepath.Join(dir, fmt.Sprintf("input-%d.txt", i))
		if err := os.WriteFile(files[i], []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := NewCoordinator(files, nReduce, filepath.Join(dir, "result"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func serveJob(t *testing.T, c *Coordinator) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- c.Serve(listener) }()
	t.Cleanup(func() {
		listener.Close()
		if err := <-done; !errors.Is(err, net.ErrClosed) {
			t.Errorf("serve: %v", err)
		}
	})
	return listener.Addr().String()
}

func requestTask(t *testing.T, c *Coordinator) Task {
	t.Helper()
	var task Task
	if err := c.RequestTask(&TaskRequest{}, &task); err != nil {
		t.Fatal(err)
	}
	return task
}

func reportTask(t *testing.T, c *Coordinator, task Task, failed bool) bool {
	t.Helper()
	var reply ReportReply
	err := c.ReportTask(&TaskReport{Type: task.Type, ID: task.ID, Attempt: task.Attempt, Failed: failed}, &reply)
	if err != nil {
		t.Fatal(err)
	}
	return reply.Accepted
}

func expireTask(c *Coordinator, task Task) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tasks := c.maps
	if task.Type == ReduceTask {
		tasks = c.reduces
	}
	tasks[task.ID].started = time.Now().Add(-2 * c.timeout)
}

func checkOutput(t *testing.T, c *Coordinator, want map[string]int) {
	t.Helper()
	got := make(map[string]int)
	for id := range c.reduces {
		data, err := os.ReadFile(filepath.Join(c.dir, fmt.Sprintf("mr-out-%d", id)))
		if err != nil {
			t.Fatal(err)
		}
		var keys []string
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) != 2 {
				t.Fatalf("invalid output %q", line)
			}
			count, err := strconv.Atoi(fields[1])
			if err != nil {
				t.Fatal(err)
			}
			if _, exists := got[fields[0]]; exists {
				t.Fatalf("duplicate output key %q", fields[0])
			}
			got[fields[0]] = count
			keys = append(keys, fields[0])
		}
		if !sort.StringsAreSorted(keys) {
			t.Fatalf("partition %d is not sorted: %v", id, keys)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("output = %v, want %v", got, want)
	}
}

func TestParallelWordCount(t *testing.T) {
	c := newJob(t, []string{"hello world hello\nmap reduce", "hello map\nreduce world world", "Go go 世界 世界", ""}, 4)
	addr := serveJob(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Hold the first two maps at a barrier to prove that different workers can
	// execute tasks simultaneously, rather than only producing a correct total.
	var started atomic.Int32
	parallel := make(chan struct{})
	mapf := func(name, data string) []KeyValue {
		if started.Add(1) == 2 {
			close(parallel)
		}
		select {
		case <-parallel:
		case <-ctx.Done():
		}
		return WordCountMap(name, data)
	}
	results := make(chan error, 6)
	for i := 0; i < cap(results); i++ {
		go func() { results <- Worker(ctx, addr, mapf, WordCountReduce) }()
	}
	for i := 0; i < cap(results); i++ {
		if err := <-results; err != nil {
			t.Errorf("worker: %v", err)
		}
	}
	if !c.Done() {
		t.Fatal("job did not finish")
	}
	checkOutput(t, c, map[string]int{"hello": 3, "world": 3, "map": 2, "reduce": 2, "Go": 1, "go": 1, "世界": 2})
}

func TestEmptyJob(t *testing.T) {
	for _, contents := range [][]string{nil, {""}} {
		c := newJob(t, contents, 3)
		addr := serveJob(t, c)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := Worker(ctx, addr, WordCountMap, WordCountReduce)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		checkOutput(t, c, map[string]int{})
	}
}

func TestTimeoutReassignsAbandonedWorker(t *testing.T) {
	c := newJob(t, []string{"retry retry success"}, 2)
	c.timeout = 100 * time.Millisecond
	addr := serveJob(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var abandoned Task
	if err := call(ctx, addr, "Coordinator.RequestTask", &TaskRequest{}, &abandoned); err != nil {
		t.Fatal(err)
	}
	// This worker disappears after assignment and never reports completion.
	if err := Worker(ctx, addr, WordCountMap, WordCountReduce); err != nil {
		t.Fatal(err)
	}
	if c.maps[0].attempt <= abandoned.Attempt {
		t.Fatal("abandoned task was not reassigned")
	}
	checkOutput(t, c, map[string]int{"retry": 2, "success": 1})
}

func TestStageBarrierAndStaleResults(t *testing.T) {
	c := newJob(t, []string{"fresh fresh", "fresh"}, 1)
	oldMap := requestTask(t, c)
	otherMap := requestTask(t, c)
	if task := requestTask(t, c); task.Type != WaitTask {
		t.Fatalf("expected wait while maps run, got %s", task.Type)
	}
	expireTask(c, oldMap)
	newMap := requestTask(t, c)
	if newMap.ID != oldMap.ID || newMap.Attempt != oldMap.Attempt+1 {
		t.Fatalf("unexpected retry: %+v", newMap)
	}
	if err := runMap(newMap, WordCountMap); err != nil {
		t.Fatal(err)
	}
	if !reportTask(t, c, newMap, false) || !reportTask(t, c, newMap, false) {
		t.Fatal("current and duplicate reports should be accepted")
	}
	if task := requestTask(t, c); task.Type != WaitTask {
		t.Fatal("reduce started before every map completed")
	}
	if err := runMap(otherMap, WordCountMap); err != nil {
		t.Fatal(err)
	}
	reportTask(t, c, otherMap, false)
	oldReduce := requestTask(t, c)
	if oldReduce.Type != ReduceTask || oldReduce.Inputs[0] != mapOutput(c.dir, newMap.ID, newMap.Attempt, 0) {
		t.Fatalf("reduce did not select committed map attempt: %+v", oldReduce)
	}
	if err := runMap(oldMap, func(_, _ string) []KeyValue { return []KeyValue{{Key: "stale", Value: "1"}} }); err != nil {
		t.Fatal(err)
	}
	if reportTask(t, c, oldMap, false) {
		t.Fatal("stale map was accepted")
	}
	expireTask(c, oldReduce)
	newReduce := requestTask(t, c)
	if newReduce.Attempt != oldReduce.Attempt+1 {
		t.Fatal("reduce task was not retried")
	}
	if err := runReduce(newReduce, WordCountReduce); err != nil {
		t.Fatal(err)
	}
	final := filepath.Join(c.dir, "mr-out-0")
	if _, err := os.Stat(final); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("reduce output became visible before coordinator commit")
	}
	if !reportTask(t, c, newReduce, false) || !reportTask(t, c, newReduce, false) {
		t.Fatal("reduce commit must be idempotent")
	}
	if err := runReduce(oldReduce, func(_ string, _ []string) string { return "999" }); err != nil {
		t.Fatal(err)
	}
	if reportTask(t, c, oldReduce, false) {
		t.Fatal("stale reduce was accepted")
	}
	checkOutput(t, c, map[string]int{"fresh": 3})
	if !c.Done() || requestTask(t, c).Type != ExitTask {
		t.Fatal("completed job should ask workers to exit")
	}
}

func TestFailedWorkerReleasesTask(t *testing.T) {
	c := newJob(t, []string{"recovered"}, 1)
	addr := serveJob(t, c)
	if err := os.Remove(c.inputs[0]); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := Worker(ctx, addr, WordCountMap, WordCountReduce); err == nil {
		t.Fatal("missing input should fail the worker")
	}
	if !c.maps[0].started.IsZero() || c.maps[0].done {
		t.Fatal("failed task should become available immediately")
	}
	if err := os.WriteFile(c.inputs[0], []byte("recovered"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Worker(ctx, addr, WordCountMap, WordCountReduce); err != nil {
		t.Fatal(err)
	}
	checkOutput(t, c, map[string]int{"recovered": 1})
}

func TestRejectsInvalidInputsAndReports(t *testing.T) {
	dir := t.TempDir()
	for _, config := range []struct {
		files []string
		n     int
		ttl   time.Duration
	}{
		{nil, 0, time.Second},
		{nil, -1, time.Second},
		{nil, 1, 0},
		{[]string{filepath.Join(dir, "missing")}, 1, time.Second},
		{[]string{dir}, 1, time.Second},
	} {
		if _, err := NewCoordinator(config.files, config.n, filepath.Join(dir, "out"), config.ttl); err == nil {
			t.Fatalf("accepted invalid configuration: %+v", config)
		}
	}
	c := newJob(t, []string{"x"}, 1)
	if _, err := NewCoordinator(nil, 1, c.dir, time.Second); err == nil {
		t.Fatal("existing job directory should be rejected")
	}
	task := requestTask(t, c)
	for _, report := range []TaskReport{
		{Type: ExitTask, ID: 0, Attempt: 1},
		{Type: MapTask, ID: -1, Attempt: 1},
		{Type: MapTask, ID: 10, Attempt: 1},
		{Type: MapTask, ID: 0, Attempt: 0},
		{Type: task.Type, ID: task.ID, Attempt: task.Attempt},
	} {
		var reply ReportReply
		if err := c.ReportTask(&report, &reply); err == nil || reply.Accepted {
			t.Fatalf("invalid or incomplete report accepted: %+v", report)
		}
	}
}

func TestCorruptIntermediateAndWriteFailure(t *testing.T) {
	c := newJob(t, []string{"hello"}, 1)
	mapTask := requestTask(t, c)
	if err := runMap(mapTask, WordCountMap); err != nil {
		t.Fatal(err)
	}
	reportTask(t, c, mapTask, false)
	task := requestTask(t, c)
	if err := os.WriteFile(task.Inputs[0], []byte("{broken json"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runReduce(task, WordCountReduce); err == nil {
		t.Fatal("corrupt intermediate records must not be silently skipped")
	}
	name := filepath.Join(c.dir, "failed-output")
	injected := errors.New("injected write failure")
	err := writeOutput(name, func(w io.Writer) error {
		_, _ = io.WriteString(w, "partial")
		return injected
	})
	if !errors.Is(err, injected) {
		t.Fatalf("write error = %v", err)
	}
	if _, err := os.Stat(name); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed output was published")
	}
	leftovers, err := filepath.Glob(filepath.Join(c.dir, ".mr-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary files leaked: %v, %v", leftovers, err)
	}
}

func TestWaitingWorkerCanBeCanceled(t *testing.T) {
	c := newJob(t, []string{"held"}, 1)
	requestTask(t, c)
	addr := serveJob(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := Worker(ctx, addr, WordCountMap, WordCountReduce); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("canceled worker returned %v", err)
	}
}
