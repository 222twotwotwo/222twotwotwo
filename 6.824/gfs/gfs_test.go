package gfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type cluster struct {
	client *Client
	master *httptest.Server
	nodes  []*httptest.Server
	dirs   []string
}

func newCluster(t *testing.T) *cluster {
	t.Helper()
	c := &cluster{}
	var servers []string
	for i := 0; i < 2; i++ {
		dir := t.TempDir()
		handler, err := NewChunkServer(dir)
		if err != nil {
			t.Fatal(err)
		}
		node := httptest.NewServer(handler)
		t.Cleanup(node.Close)
		c.nodes = append(c.nodes, node)
		c.dirs = append(c.dirs, dir)
		servers = append(servers, node.URL)
	}
	m, err := NewMaster(servers, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	c.master = httptest.NewServer(m.Handler())
	t.Cleanup(c.master.Close)
	c.client = NewClient(c.master.URL)
	return c
}

func TestRoundTrip(t *testing.T) {
	c := newCluster(t)
	ctx := context.Background()
	for _, data := range [][]byte{{}, []byte("abcd"), {0, 255, 1, 2, 3, 4, 5, 6, 7, 8}} {
		name := fmt.Sprintf("/space and ?#/%d", len(data))
		if err := c.client.Put(ctx, name, data); err != nil {
			t.Fatal(err)
		}
		got, err := c.client.Get(ctx, name)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("get %q = %v, %v; want %v", name, got, err, data)
		}
	}
	files, err := c.client.List(ctx)
	if err != nil || len(files) != 3 {
		t.Fatalf("list = %v, %v", files, err)
	}
	for _, file := range files {
		wantChunks := (file.Size + 3) / 4
		if len(file.Chunks) != wantChunks {
			t.Fatalf("got %d chunks, want %d", len(file.Chunks), wantChunks)
		}
		for _, chunk := range file.Chunks {
			if len(chunk.Servers) != 2 || chunk.Servers[0] == chunk.Servers[1] {
				t.Fatalf("invalid replicas: %v", chunk.Servers)
			}
			for _, dir := range c.dirs {
				if _, err := os.Stat(filepath.Join(dir, chunk.ID)); err != nil {
					t.Fatalf("replica missing: %v", err)
				}
			}
		}
	}
	if _, err := c.client.Get(ctx, "/missing"); err == nil {
		t.Fatal("missing file should fail")
	}
}

func TestReplicaFailureAndFailedOverwrite(t *testing.T) {
	c := newCluster(t)
	ctx := context.Background()
	old := []byte("original data across several chunks")
	if err := c.client.Put(ctx, "/file", old); err != nil {
		t.Fatal(err)
	}
	c.nodes[0].Close()
	if got, err := c.client.Get(ctx, "/file"); err != nil || !bytes.Equal(got, old) {
		t.Fatalf("fallback = %q, %v", got, err)
	}
	if err := c.client.Put(ctx, "/file", []byte("replacement")); err == nil {
		t.Fatal("write should fail with a replica offline")
	}
	if got, err := c.client.Get(ctx, "/file"); err != nil || !bytes.Equal(got, old) {
		t.Fatalf("failed overwrite changed file: %q, %v", got, err)
	}
	if err := c.client.Put(ctx, "/unfinished", []byte("new file")); err == nil {
		t.Fatal("write should fail")
	}
	if _, err := c.client.Get(ctx, "/unfinished"); err == nil {
		t.Fatal("unfinished upload should not be visible")
	}
	c.nodes[1].Close()
	if _, err := c.client.Get(ctx, "/file"); err == nil {
		t.Fatal("read should fail when all replicas are offline")
	}
}

func TestOverwrite(t *testing.T) {
	c := newCluster(t)
	ctx := context.Background()
	for _, data := range [][]byte{[]byte("long original"), []byte("short"), {}} {
		if err := c.client.Put(ctx, "/file", data); err != nil {
			t.Fatal(err)
		}
		got, err := c.client.Get(ctx, "/file")
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("overwrite = %q, %v; want %q", got, err, data)
		}
	}
}

func TestTruncatedReplicaFallbackAndDiskStorage(t *testing.T) {
	c := newCluster(t)
	ctx := context.Background()
	data := []byte("abcdefghij")
	if err := c.client.Put(ctx, "/file", data); err != nil {
		t.Fatal(err)
	}
	files, err := c.client.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id := files[0].Chunks[0].ID
	if err := os.WriteFile(filepath.Join(c.dirs[0], id), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, err := c.client.Get(ctx, "/file"); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("truncated replica fallback = %q, %v", got, err)
	}
	// A fresh handler using the same directory must see the saved chunks.
	handler, err := NewChunkServer(c.dirs[1])
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/chunks/"+id, nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "abcd" {
		t.Fatalf("disk read = %d, %q", recorder.Code, recorder.Body.String())
	}
}

func TestConcurrentWrites(t *testing.T) {
	c := newCluster(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("/file-%d", i)
			data := bytes.Repeat([]byte{byte(i)}, 13)
			if err := c.client.Put(ctx, name, data); err != nil {
				t.Error(err)
				return
			}
			if got, err := c.client.Get(ctx, name); err != nil || !bytes.Equal(got, data) {
				t.Errorf("concurrent get = %v, %v", got, err)
			}
			if err := c.client.Put(ctx, "/shared", data); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	got, err := c.client.Get(ctx, "/shared")
	if err != nil || len(got) != 13 {
		t.Fatalf("shared file = %v, %v", got, err)
	}
	if !bytes.Equal(got, bytes.Repeat(got[:1], 13)) {
		t.Fatalf("concurrent writes mixed chunks: %v", got)
	}
}

func TestInvalidRequests(t *testing.T) {
	c := newCluster(t)
	ctx := context.Background()
	if err := c.client.Put(ctx, "", []byte("data")); err == nil {
		t.Fatal("empty name should fail")
	}
	for _, body := range []string{`{"name":"x","size":-1}`, `{"name":"x","size":40001}`, `{`} {
		resp, err := http.Post(c.master.URL+"/allocate", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode < 400 || resp.StatusCode >= 500 {
			t.Fatalf("invalid allocation returned %d", resp.StatusCode)
		}
	}
	if _, err := c.client.request(ctx, http.MethodPost, c.master.URL+"/commit?token=unknown", nil); err == nil {
		t.Fatal("unknown token should fail")
	}
	if _, err := c.client.request(ctx, http.MethodPut, c.nodes[0].URL+"/chunks/not-a-chunk", []byte("bad")); err == nil {
		t.Fatal("invalid chunk ID should fail")
	}
	for _, tc := range []struct {
		servers  []string
		size     int
		replicas int
	}{
		{nil, 4, 1},
		{[]string{c.nodes[0].URL}, 0, 1},
		{[]string{c.nodes[0].URL}, MaxChunkSize + 1, 1},
		{[]string{c.nodes[0].URL}, 4, 2},
		{[]string{c.nodes[0].URL, c.nodes[0].URL}, 4, 2},
		{[]string{"bad-url"}, 4, 1},
	} {
		if _, err := NewMaster(tc.servers, tc.size, tc.replicas); err == nil {
			t.Errorf("accepted invalid configuration: %+v", tc)
		}
	}
}

func TestInterruptedChunkWrite(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewChunkServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 32) + "-0"
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/chunks/"+id, io.MultiReader(strings.NewReader("partial"), brokenReader{}))
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("interrupted upload returned %d", recorder.Code)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("interrupted upload left files: %v, %v", entries, err)
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
