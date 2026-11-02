package mapreduce

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type MapFunc func(filename, contents string) []KeyValue
type ReduceFunc func(key string, values []string) string

// Worker processes tasks until the job finishes, its context is canceled, or an
// operation fails. A failed worker can be restarted without restarting the job.
func Worker(ctx context.Context, addr string, mapf MapFunc, reducef ReduceFunc) error {
	if mapf == nil || reducef == nil {
		return fmt.Errorf("map and reduce functions are required")
	}
	for {
		var task Task
		if err := call(ctx, addr, "Coordinator.RequestTask", &TaskRequest{}, &task); err != nil {
			return err
		}
		switch task.Type {
		case ExitTask:
			return nil
		case WaitTask:
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
			continue
		case MapTask, ReduceTask:
		default:
			return fmt.Errorf("unknown task type %q", task.Type)
		}
		var taskErr error
		if task.Type == MapTask {
			taskErr = runMap(task, mapf)
		} else {
			taskErr = runReduce(task, reducef)
		}
		if taskErr != nil {
			removeOutputs(task)
		}
		report := TaskReport{Type: task.Type, ID: task.ID, Attempt: task.Attempt, Failed: taskErr != nil}
		var reply ReportReply
		reportErr := call(ctx, addr, "Coordinator.ReportTask", &report, &reply)
		if taskErr != nil {
			return fmt.Errorf("%s task %d: %w", task.Type, task.ID, errors.Join(taskErr, reportErr))
		}
		if reportErr != nil {
			// The coordinator might already have committed the task. Keep its files
			// when the acknowledgement is uncertain.
			return reportErr
		}
		if !reply.Accepted {
			removeOutputs(task)
		}
	}
}

func call(ctx context.Context, addr, method string, args, reply any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return err
	}
	client := rpc.NewClient(conn)
	defer client.Close()
	pending := client.Go(method, args, reply, make(chan *rpc.Call, 1))
	select {
	case <-ctx.Done():
		return ctx.Err()
	case result := <-pending.Done:
		return result.Error
	}
}

func partition(key string, nReduce int) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	return int(hash.Sum32()&0x7fffffff) % nReduce
}

func runMap(task Task, mapf MapFunc) error {
	data, err := os.ReadFile(task.Inputs[0])
	if err != nil {
		return err
	}
	buckets := make([][]KeyValue, task.NReduce)
	for _, kv := range mapf(task.Inputs[0], string(data)) {
		id := partition(kv.Key, task.NReduce)
		buckets[id] = append(buckets[id], kv)
	}
	for id, records := range buckets {
		name := mapOutput(task.Dir, task.ID, task.Attempt, id)
		if err := writeOutput(name, func(w io.Writer) error {
			encoder := json.NewEncoder(w)
			for _, record := range records {
				if err := encoder.Encode(record); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func readRecords(name string) ([]KeyValue, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var records []KeyValue
	decoder := json.NewDecoder(file)
	for {
		var record KeyValue
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				return records, nil
			}
			return nil, fmt.Errorf("decode %s: %w", name, err)
		}
		records = append(records, record)
	}
}

func runReduce(task Task, reducef ReduceFunc) error {
	var records []KeyValue
	for _, name := range task.Inputs {
		batch, err := readRecords(name)
		if err != nil {
			return err
		}
		records = append(records, batch...)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Key < records[j].Key })
	return writeOutput(reduceOutput(task.Dir, task.ID, task.Attempt), func(w io.Writer) error {
		for start := 0; start < len(records); {
			end := start + 1
			values := []string{records[start].Value}
			for end < len(records) && records[end].Key == records[start].Key {
				values = append(values, records[end].Value)
				end++
			}
			key := records[start].Key
			if _, err := fmt.Fprintf(w, "%s %s\n", key, reducef(key, values)); err != nil {
				return err
			}
			start = end
		}
		return nil
	})
}

// Each attempt writes privately, closes the file, and only then renames it.
// Reduce outputs are subsequently published by the coordinator after validation.
func writeOutput(name string, write func(io.Writer) error) error {
	file, err := os.CreateTemp(filepath.Dir(name), ".mr-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	buffer := bufio.NewWriter(file)
	if err := write(buffer); err != nil {
		file.Close()
		return err
	}
	if err := buffer.Flush(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), name)
}

func removeOutputs(task Task) {
	for _, name := range outputPaths(task) {
		_ = os.Remove(name)
	}
}
