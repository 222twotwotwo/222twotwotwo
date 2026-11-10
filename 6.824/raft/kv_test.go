package raft

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func (c *cluster) clerk() *Clerk {
	return NewClerk(c.addrs)
}

func TestKVBasic(t *testing.T) {
	c := newCluster(t, 3, true)
	defer c.shutdown()
	clerk := c.clerk()

	if value := clerk.Get("missing"); value != "" {
		t.Fatalf("不存在的 key 应返回空串，拿到 %q", value)
	}
	clerk.Put("x", "a")
	if value := clerk.Get("x"); value != "a" {
		t.Fatalf("put 后读取到 %q", value)
	}
	clerk.Append("x", "b")
	clerk.Append("x", "c")
	if value := clerk.Get("x"); value != "abc" {
		t.Fatalf("append 后读取到 %q", value)
	}
	clerk.Put("x", "reset")
	if value := clerk.Get("x"); value != "reset" {
		t.Fatalf("覆盖写后读取到 %q", value)
	}
}

func TestKVConcurrentClients(t *testing.T) {
	c := newCluster(t, 3, true)
	defer c.shutdown()

	const (
		clients = 5
		appends = 10
	)
	var wg sync.WaitGroup
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			clerk := c.clerk()
			for j := 0; j < appends; j++ {
				clerk.Append("counter", "x")
			}
		}()
	}
	wg.Wait()

	if value := c.clerk().Get("counter"); len(value) != clients*appends {
		t.Fatalf("并发 append 之后长度为 %d，期望 %d", len(value), clients*appends)
	}
}

func TestKVFailover(t *testing.T) {
	c := newCluster(t, 3, true)
	defer c.shutdown()
	clerk := c.clerk()

	for i := 0; i < 5; i++ {
		clerk.Append("counter", "x")
	}
	leader := c.checkOneLeader()
	c.crash(leader)

	// 客户端还不知道新 leader 是谁，靠重试自己找过去。
	clerk.Append("counter", "y")
	if value := clerk.Get("counter"); value != "xxxxxy" {
		t.Fatalf("leader 崩溃后读到 %q", value)
	}

	c.start(leader)
	c.checkOneLeader()
	if value := clerk.Get("counter"); value != "xxxxxy" {
		t.Fatalf("节点重启后读到 %q", value)
	}
}

func TestKVSnapshotRestart(t *testing.T) {
	c := newClusterWith(t, 3, true, 512)
	defer c.shutdown()

	clerk := c.clerk()
	for i := 0; i < 40; i++ {
		clerk.Put(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
	}
	// 快照由日志长度触发，等三个节点都压过一次。
	deadline := time.Now().Add(10 * time.Second)
	for {
		done := true
		for i := 0; i < c.n; i++ {
			if c.status(i).SnapshotIndex == 0 {
				done = false
			}
		}
		if done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("日志没有触发快照")
		}
		time.Sleep(20 * time.Millisecond)
	}

	for i := 0; i < c.n; i++ {
		c.crash(i)
	}
	for i := 0; i < c.n; i++ {
		c.start(i)
	}
	leader := c.checkOneLeader()
	status := c.status(leader)
	if status.LastLogIndex-status.SnapshotIndex > 40 {
		t.Fatalf("重启后日志仍然很长: last=%d snapshot=%d",
			status.LastLogIndex, status.SnapshotIndex)
	}

	clerk = c.clerk()
	for i := 0; i < 40; i++ {
		key := fmt.Sprintf("key-%d", i)
		want := fmt.Sprintf("value-%d", i)
		if value := clerk.Get(key); value != want {
			t.Fatalf("重启后 %s = %q，期望 %q", key, value, want)
		}
	}
}

func TestConfigValidation(t *testing.T) {
	network := NewLocalNetwork()
	valid := func() Config {
		return Config{
			ID:          0,
			Peers:       []string{"a", "b", "c"},
			Storage:     NewMemoryStorage(),
			Transport:   network.Transport(0),
			ApplyCh:     make(chan ApplyMsg, 1),
			Heartbeat:   10 * time.Millisecond,
			ElectionMin: 50 * time.Millisecond,
			ElectionMax: 100 * time.Millisecond,
		}
	}

	tests := []struct {
		name   string
		damage func(*Config)
	}{
		{"空集群", func(c *Config) { c.Peers = nil }},
		{"编号越界", func(c *Config) { c.ID = 3 }},
		{"缺少存储", func(c *Config) { c.Storage = nil }},
		{"缺少传输", func(c *Config) { c.Transport = nil }},
		{"缺少应用通道", func(c *Config) { c.ApplyCh = nil }},
		{"心跳过快", func(c *Config) { c.Heartbeat = 200 * time.Millisecond }},
		{"选举区间颠倒", func(c *Config) { c.ElectionMax = 10 * time.Millisecond }},
	}
	for _, test := range tests {
		cfg := valid()
		test.damage(&cfg)
		if _, err := NewRaft(cfg); err == nil {
			t.Fatalf("%s: 期望返回错误", test.name)
		}
	}

	cfg := valid()
	rf, err := NewRaft(cfg)
	if err != nil {
		t.Fatalf("合法配置启动失败: %v", err)
	}
	rf.Kill()
}
