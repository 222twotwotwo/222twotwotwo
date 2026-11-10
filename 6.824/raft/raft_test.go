package raft

import (
	"bytes"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testHeartbeat   = 30 * time.Millisecond
	testElectionMin = 150 * time.Millisecond
	testElectionMax = 300 * time.Millisecond
)

type serverHolder struct{ v atomic.Value }

func (h *serverHolder) set(srv *rpc.Server) { h.v.Store(srv) }

func (h *serverHolder) get() *rpc.Server {
	srv, _ := h.v.Load().(*rpc.Server)
	return srv
}

// cluster 是同进程内的测试集群：Raft 之间走 LocalNetwork，客户端走真实 TCP。
type cluster struct {
	t            *testing.T
	n            int
	net          *LocalNetwork
	storages     []Storage
	rafts        []*Raft
	kvs          []*KVServer
	applyChs     []chan ApplyMsg
	addrs        []string
	listeners    []net.Listener
	holders      []*serverHolder
	kvMode       bool
	maxRaftState int

	mu      sync.Mutex
	applied []map[int][]byte
	snaps   []int
}

func newCluster(t *testing.T, n int, kvMode bool) *cluster {
	return newClusterWith(t, n, kvMode, 0)
}

func newClusterWith(t *testing.T, n int, kvMode bool, maxRaftState int) *cluster {
	t.Helper()
	c := &cluster{
		t:            t,
		n:            n,
		net:          NewLocalNetwork(),
		kvMode:       kvMode,
		maxRaftState: maxRaftState,
		storages:     make([]Storage, n),
		rafts:        make([]*Raft, n),
		kvs:          make([]*KVServer, n),
		applyChs:     make([]chan ApplyMsg, n),
		addrs:        make([]string, n),
		listeners:    make([]net.Listener, n),
		holders:      make([]*serverHolder, n),
		applied:      make([]map[int][]byte, n),
		snaps:        make([]int, n),
	}
	c.net.SetDelay(time.Millisecond)

	for i := 0; i < n; i++ {
		c.storages[i] = NewMemoryStorage()
		c.applied[i] = make(map[int][]byte)
		c.addrs[i] = fmt.Sprintf("mem-%d", i)
		if kvMode {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("监听端口: %v", err)
			}
			c.listeners[i] = listener
			c.addrs[i] = listener.Addr().String()
			c.holders[i] = &serverHolder{}
			c.holders[i].set(nil)
			go c.acceptLoop(i)
		}
	}
	for i := 0; i < n; i++ {
		c.start(i)
	}
	return c
}

func (c *cluster) acceptLoop(i int) {
	for {
		conn, err := c.listeners[i].Accept()
		if err != nil {
			return
		}
		go func() {
			srv := c.holders[i].get()
			if srv == nil {
				conn.Close()
				return
			}
			srv.ServeConn(conn)
		}()
	}
}

func (c *cluster) start(i int) {
	c.t.Helper()
	applyCh := make(chan ApplyMsg, 512)
	rf, err := NewRaft(Config{
		ID:          i,
		Peers:       c.addrs,
		Storage:     c.storages[i],
		Transport:   c.net.Transport(i),
		ApplyCh:     applyCh,
		Heartbeat:   testHeartbeat,
		ElectionMin: testElectionMin,
		ElectionMax: testElectionMax,
	})
	if err != nil {
		c.t.Fatalf("启动节点 %d: %v", i, err)
	}
	c.rafts[i] = rf
	c.applyChs[i] = applyCh
	c.net.Add(i, rf)

	c.mu.Lock()
	c.applied[i] = make(map[int][]byte)
	c.snaps[i] = 0
	c.mu.Unlock()

	if c.kvMode {
		kv, err := NewKVServer(rf, applyCh, c.maxRaftState)
		if err != nil {
			c.t.Fatalf("启动节点 %d 的键值服务: %v", i, err)
		}
		c.kvs[i] = kv
		srv := rpc.NewServer()
		if err := srv.RegisterName("Raft", rf); err != nil {
			c.t.Fatalf("注册 Raft: %v", err)
		}
		if err := srv.RegisterName("KV", kv); err != nil {
			c.t.Fatalf("注册 KV: %v", err)
		}
		c.holders[i].set(srv)
		return
	}
	go c.consume(i, applyCh)
}

func (c *cluster) consume(i int, applyCh chan ApplyMsg) {
	for msg := range applyCh {
		if msg.CommandValid {
			c.mu.Lock()
			c.applied[i][msg.CommandIndex] = append([]byte(nil), msg.Command...)
			c.mu.Unlock()
		}
		if msg.SnapshotValid {
			c.mu.Lock()
			if msg.SnapshotIndex > c.snaps[i] {
				c.snaps[i] = msg.SnapshotIndex
			}
			for index := range c.applied[i] {
				if index <= msg.SnapshotIndex {
					delete(c.applied[i], index)
				}
			}
			c.mu.Unlock()
		}
	}
}

// crash 模拟进程崩溃：停止协程、断开网络、不再应答 RPC。
func (c *cluster) crash(i int) {
	c.t.Helper()
	if c.rafts[i] == nil {
		return
	}
	if c.kvs[i] != nil {
		c.kvs[i].Stop()
		c.kvs[i] = nil
	}
	c.rafts[i].Kill()
	c.rafts[i] = nil
	c.net.Remove(i)
	if c.holders[i] != nil {
		c.holders[i].set(nil)
	}
	c.mu.Lock()
	c.applied[i] = make(map[int][]byte)
	c.snaps[i] = 0
	c.mu.Unlock()
}

func (c *cluster) shutdown() {
	for i := 0; i < c.n; i++ {
		c.crash(i)
	}
	for _, listener := range c.listeners {
		if listener != nil {
			listener.Close()
		}
	}
}

func (c *cluster) quorum() int { return c.n/2 + 1 }

// leader 返回任期最高的自认为 leader 的节点；被隔离的旧 leader 任期更低，会被忽略。
func (c *cluster) leader() int {
	best, bestTerm := -1, -1
	for i, rf := range c.rafts {
		if rf == nil {
			continue
		}
		term, isLeader := rf.GetState()
		if isLeader && term > bestTerm {
			best, bestTerm = i, term
		}
	}
	return best
}

func (c *cluster) checkOneLeader() int {
	c.t.Helper()
	for attempt := 0; attempt < 60; attempt++ {
		time.Sleep(50 * time.Millisecond)
		if leader := c.leader(); leader >= 0 {
			// 同一任期不允许有两个 leader
			duplicated := false
			term, _ := c.rafts[leader].GetState()
			for i, rf := range c.rafts {
				if rf == nil || i == leader {
					continue
				}
				if other, isLeader := rf.GetState(); isLeader && other == term {
					duplicated = true
				}
			}
			if !duplicated {
				return leader
			}
		}
	}
	c.t.Fatalf("等待稳定的 leader 超时")
	return -1
}

func (c *cluster) checkNoLeader() {
	c.t.Helper()
	for attempt := 0; attempt < 20; attempt++ {
		time.Sleep(50 * time.Millisecond)
		if c.leader() < 0 {
			return
		}
	}
	c.t.Fatalf("少数派中不应该有 leader")
}

func (c *cluster) status(i int) StatusReply {
	c.t.Helper()
	reply := StatusReply{}
	if err := c.rafts[i].Status(&StatusArgs{}, &reply); err != nil {
		c.t.Fatalf("读取节点 %d 状态: %v", i, err)
	}
	return reply
}

func (c *cluster) nCommitted(index int) (int, []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	var cmd []byte
	for i := 0; i < c.n; i++ {
		value, ok := c.applied[i][index]
		if !ok {
			continue
		}
		if count == 0 {
			cmd = value
			count = 1
		} else if bytes.Equal(value, cmd) {
			count++
		}
	}
	return count, cmd
}

func (c *cluster) countApplied(index int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for i := 0; i < c.n; i++ {
		if _, ok := c.applied[i][index]; ok {
			count++
		}
	}
	return count
}

func (c *cluster) waitApplied(index int, need int) {
	c.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c.countApplied(index) >= need {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatalf("等待下标 %d 被 %d 个节点应用超时", index, need)
}

// one 提交一条命令并等到多数派应用，返回它最终的下标。
func (c *cluster) one(cmd []byte) int {
	c.t.Helper()
	for attempt := 0; attempt < 50; attempt++ {
		leader := c.leader()
		if leader < 0 {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		index, _, ok := c.rafts[leader].Start(cmd)
		if !ok {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		deadline := time.Now().Add(500 * time.Millisecond)
		for time.Now().Before(deadline) {
			if count, got := c.nCommitted(index); count >= c.quorum() && bytes.Equal(got, cmd) {
				return index
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	c.t.Fatalf("命令 %q 未能达成一致", cmd)
	return -1
}

// checkConsistency 验证任何两个节点在同一日志下标上都不会出现不同命令。
func (c *cluster) checkConsistency() {
	c.t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := 0; i < c.n; i++ {
		for j := i + 1; j < c.n; j++ {
			for index, cmd := range c.applied[i] {
				if other, ok := c.applied[j][index]; ok && !bytes.Equal(cmd, other) {
					c.t.Fatalf("节点 %d 与 %d 在下标 %d 上应用了不同命令: %q vs %q",
						i, j, index, cmd, other)
				}
			}
		}
	}
}

func (c *cluster) hasApplied(cmd []byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := 0; i < c.n; i++ {
		for _, value := range c.applied[i] {
			if bytes.Equal(value, cmd) {
				return true
			}
		}
	}
	return false
}

func TestInitialElection(t *testing.T) {
	c := newCluster(t, 3, false)
	defer c.shutdown()

	leader := c.checkOneLeader()
	term1, _ := c.rafts[leader].GetState()
	time.Sleep(2 * testElectionMax)
	leader2 := c.checkOneLeader()
	term2, _ := c.rafts[leader2].GetState()
	if term1 != term2 {
		t.Fatalf("没有故障却发生了换任期: %d -> %d", term1, term2)
	}
}

func TestReElection(t *testing.T) {
	c := newCluster(t, 3, false)
	defer c.shutdown()

	leader1 := c.checkOneLeader()
	c.crash(leader1)
	leader2 := c.checkOneLeader()
	if leader2 == leader1 {
		t.Fatalf("崩溃的 leader 仍然当选")
	}

	c.crash(leader2)
	time.Sleep(2 * testElectionMax)
	c.checkNoLeader()

	c.start(leader1)
	c.start(leader2)
	c.checkOneLeader()
}

func TestBasicAgree(t *testing.T) {
	c := newCluster(t, 3, false)
	defer c.shutdown()

	for i := 1; i <= 5; i++ {
		c.one([]byte(fmt.Sprintf("cmd-%d", i)))
	}
	c.checkConsistency()
}

func TestNoAgreeWithoutMajority(t *testing.T) {
	c := newCluster(t, 5, false)
	defer c.shutdown()

	leader := c.checkOneLeader()
	for i := 0; i < c.n; i++ {
		if i != leader {
			c.crash(i)
		}
	}
	index, _, ok := c.rafts[leader].Start([]byte("orphan"))
	if !ok {
		t.Fatalf("孤立节点应仍自认为 leader")
	}
	time.Sleep(500 * time.Millisecond)
	if count, _ := c.nCommitted(index); count != 0 {
		t.Fatalf("少数派提交了日志: %d 个节点应用", count)
	}

	for i := 0; i < c.n; i++ {
		if i != leader {
			c.start(i)
		}
	}
	c.one([]byte("after-heal"))
	c.checkConsistency()
}

func TestConcurrentStart(t *testing.T) {
	c := newCluster(t, 3, false)
	defer c.shutdown()
	c.checkOneLeader()

	const total = 20
	indices := make([]int, total)
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := []byte(fmt.Sprintf("concurrent-%d", i))
			for attempt := 0; attempt < 100; attempt++ {
				leader := c.leader()
				if leader >= 0 {
					if index, _, ok := c.rafts[leader].Start(cmd); ok {
						indices[i] = index
						return
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}
	wg.Wait()

	seen := make(map[int]bool, total)
	for i, index := range indices {
		if index <= 0 {
			t.Fatalf("第 %d 条命令没有拿到下标", i)
		}
		if seen[index] {
			t.Fatalf("下标 %d 被重复分配", index)
		}
		seen[index] = true
	}

	last := c.one([]byte("final"))
	c.waitApplied(last, c.quorum())
	c.checkConsistency()
}

func TestRestartPersistence(t *testing.T) {
	c := newCluster(t, 3, false)
	defer c.shutdown()
	c.checkOneLeader()

	var commands [][]byte
	for i := 1; i <= 5; i++ {
		cmd := []byte(fmt.Sprintf("persist-%d", i))
		c.one(cmd)
		commands = append(commands, cmd)
	}
	beforeLog := c.status(c.checkOneLeader()).LastLogIndex
	beforeTerms := make([]int, c.n)
	for i := 0; i < c.n; i++ {
		beforeTerms[i] = c.status(i).Term
	}

	for i := 0; i < c.n; i++ {
		c.crash(i)
	}
	for i := 0; i < c.n; i++ {
		c.start(i)
	}
	c.checkOneLeader()

	for i := 0; i < c.n; i++ {
		if term := c.status(i).Term; term < beforeTerms[i] {
			t.Fatalf("节点 %d 重启后任期倒退: %d < %d", i, term, beforeTerms[i])
		}
		if got := c.status(i).LastLogIndex; got < beforeLog {
			t.Fatalf("节点 %d 重启后丢失日志: %d < %d", i, got, beforeLog)
		}
	}

	// 新任期提交一条命令后，之前已提交的日志会一起回放。
	last := c.one([]byte("after-restart"))
	c.waitApplied(last, c.quorum())
	for _, cmd := range commands {
		if !c.hasApplied(cmd) {
			t.Fatalf("重启后丢失已提交命令 %q", cmd)
		}
	}
	c.checkConsistency()
}

func TestPartition(t *testing.T) {
	c := newCluster(t, 5, false)
	defer c.shutdown()

	leader := c.checkOneLeader()
	var others []int
	for i := 0; i < c.n; i++ {
		if i != leader {
			others = append(others, i)
		}
	}
	minority := []int{leader, others[0]}
	majority := others[1:]
	c.net.Partition(minority, majority)

	// 少数派里的旧 leader 收不到足够的确认，命令无法提交。
	index, _, ok := c.rafts[leader].Start([]byte("minority"))
	if ok {
		time.Sleep(400 * time.Millisecond)
		if count, _ := c.nCommitted(index); count >= c.quorum() {
			t.Fatalf("少数派提交了日志")
		}
	}

	index = c.one([]byte("majority"))
	c.waitApplied(index, c.quorum())

	c.net.Heal()
	last := c.one([]byte("healed"))
	c.waitApplied(last, c.quorum())
	c.checkConsistency()
}

func TestInstallSnapshot(t *testing.T) {
	c := newCluster(t, 3, false)
	defer c.shutdown()

	leader := c.checkOneLeader()
	follower := -1
	for i := 0; i < c.n; i++ {
		if i != leader {
			follower = i
			break
		}
	}
	c.net.Connect(leader, follower, false)

	for i := 1; i <= 12; i++ {
		c.one([]byte(fmt.Sprintf("snap-%d", i)))
	}
	status := c.status(leader)
	if !c.rafts[leader].Snapshot(status.LastApplied, []byte("snapshot-payload")) {
		t.Fatalf("leader 打快照失败")
	}
	c.one([]byte("after-snapshot"))

	c.net.Connect(leader, follower, true)

	deadline := time.Now().Add(5 * time.Second)
	for {
		c.mu.Lock()
		snap := c.snaps[follower]
		c.mu.Unlock()
		if snap >= status.LastApplied {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("节点 %d 没有装上新快照", follower)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// 装上快照后还要能继续追日志。
	last := c.one([]byte("after-install"))
	c.waitApplied(last, c.quorum())
	c.checkConsistency()
}

func TestSnapshotPersistence(t *testing.T) {
	c := newCluster(t, 3, false)
	defer c.shutdown()

	for i := 1; i <= 10; i++ {
		c.one([]byte(fmt.Sprintf("compact-%d", i)))
	}
	for i := 0; i < c.n; i++ {
		status := c.status(i)
		if !c.rafts[i].Snapshot(status.LastApplied, []byte("compact-snapshot")) {
			t.Fatalf("节点 %d 打快照失败", i)
		}
	}

	for i := 0; i < c.n; i++ {
		c.crash(i)
	}
	for i := 0; i < c.n; i++ {
		c.start(i)
	}
	leader := c.checkOneLeader()
	status := c.status(leader)
	if status.SnapshotIndex == 0 {
		t.Fatalf("快照没有持久化")
	}
	if status.LastLogIndex-status.SnapshotIndex > 3 {
		t.Fatalf("重启后日志没有被压缩: last=%d snapshot=%d",
			status.LastLogIndex, status.SnapshotIndex)
	}

	last := c.one([]byte("post-snapshot"))
	c.waitApplied(last, c.quorum())
	c.checkConsistency()
}

// TestNetTransportCluster 用真实 TCP 跑一遍选主和复制，覆盖 server 模式的代码路径。
func TestNetTransportCluster(t *testing.T) {
	const n = 3
	addrs := make([]string, n)
	listeners := make([]net.Listener, n)
	for i := 0; i < n; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("监听端口: %v", err)
		}
		listeners[i] = listener
		addrs[i] = listener.Addr().String()
	}
	defer func() {
		for _, listener := range listeners {
			listener.Close()
		}
	}()

	rafts := make([]*Raft, n)
	applied := make([]map[int][]byte, n)
	var mu sync.Mutex
	for i := 0; i < n; i++ {
		applyCh := make(chan ApplyMsg, 256)
		rf, err := NewRaft(Config{
			ID:          i,
			Peers:       addrs,
			Storage:     NewMemoryStorage(),
			Transport:   NewNetTransport(addrs, 300*time.Millisecond),
			ApplyCh:     applyCh,
			Heartbeat:   40 * time.Millisecond,
			ElectionMin: 200 * time.Millisecond,
			ElectionMax: 400 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("启动节点 %d: %v", i, err)
		}
		rafts[i] = rf
		applied[i] = make(map[int][]byte)
		node, idx := rf, i
		go func() {
			for msg := range applyCh {
				if msg.CommandValid {
					mu.Lock()
					applied[idx][msg.CommandIndex] = append([]byte(nil), msg.Command...)
					mu.Unlock()
				}
			}
		}()

		srv := rpc.NewServer()
		if err := srv.RegisterName("Raft", node); err != nil {
			t.Fatalf("注册 Raft: %v", err)
		}
		go func(listener net.Listener, srv *rpc.Server) {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				go srv.ServeConn(conn)
			}
		}(listeners[i], srv)
	}
	defer func() {
		for _, rf := range rafts {
			rf.Kill()
		}
	}()

	leader := -1
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && leader < 0 {
		for i, rf := range rafts {
			if _, isLeader := rf.GetState(); isLeader {
				leader = i
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if leader < 0 {
		t.Fatalf("TCP 集群没有选出 leader")
	}

	cmd := []byte("over-tcp")
	index, _, ok := rafts[leader].Start(cmd)
	if !ok {
		t.Fatalf("leader 拒绝了命令")
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := 0
		for i := 0; i < n; i++ {
			if bytes.Equal(applied[i][index], cmd) {
				count++
			}
		}
		mu.Unlock()
		if count >= 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("TCP 集群没有在多数派上提交命令")
}
