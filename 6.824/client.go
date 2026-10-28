package gfs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	master string
	http   *http.Client
}

func NewClient(master string) *Client {
	return &Client{
		master: strings.TrimRight(master, "/"),
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Put replaces a file after all replicas are written. Concurrent puts use
// last-commit-wins semantics, and readers continue using their old chunk IDs.
func (c *Client) Put(ctx context.Context, name string, data []byte) error {
	body, err := json.Marshal(struct {
		Name string `json:"name"`
		Size int    `json:"size"`
	}{name, len(data)})
	if err != nil {
		return err
	}
	var upload allocation
	if err := c.json(ctx, http.MethodPost, "/allocate", body, &upload); err != nil {
		return err
	}
	for i, chunk := range upload.File.Chunks {
		start := i * upload.File.ChunkSize
		end := min(start+upload.File.ChunkSize, len(data))
		for _, server := range chunk.Servers {
			if _, err := c.request(ctx, http.MethodPut, server+"/chunks/"+chunk.ID, data[start:end]); err != nil {
				return fmt.Errorf("write chunk %s: %w", chunk.ID, err)
			}
		}
	}
	return c.json(ctx, http.MethodPost, "/commit?token="+url.QueryEscape(upload.Token), nil, nil)
}

func (c *Client) Get(ctx context.Context, name string) ([]byte, error) {
	var file File
	if err := c.json(ctx, http.MethodGet, "/file?name="+url.QueryEscape(name), nil, &file); err != nil {
		return nil, err
	}
	data := make([]byte, 0)
	for i, chunk := range file.Chunks {
		want := min(file.ChunkSize, file.Size-i*file.ChunkSize)
		var part []byte
		var lastErr error
		found := false
		for _, server := range chunk.Servers {
			part, lastErr = c.request(ctx, http.MethodGet, server+"/chunks/"+chunk.ID, nil)
			if lastErr == nil && len(part) == want {
				found = true
				break
			}
			if lastErr == nil {
				lastErr = fmt.Errorf("size mismatch: got %d, want %d", len(part), want)
			}
		}
		if !found {
			return nil, fmt.Errorf("no readable replica for chunk %s: %v", chunk.ID, lastErr)
		}
		data = append(data, part...)
	}
	return data, nil
}

func (c *Client) List(ctx context.Context) ([]File, error) {
	var files []File
	err := c.json(ctx, http.MethodGet, "/files", nil, &files)
	return files, err
}

func (c *Client) json(ctx context.Context, method, path string, body []byte, out any) error {
	data, err := c.request(ctx, method, c.master+path, body)
	if err != nil || out == nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func (c *Client) request(ctx context.Context, method, endpoint string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%s %s: %s: %s", method, endpoint, resp.Status, strings.TrimSpace(string(message)))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxChunkSize+1))
	if len(data) > MaxChunkSize {
		return nil, fmt.Errorf("response exceeds %d bytes", MaxChunkSize)
	}
	return data, err
}
