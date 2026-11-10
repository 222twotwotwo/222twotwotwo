package raft

import (
	"math/rand"
	"net"
	"net/rpc"
	"sync"
	"time"
)

// Transport 负责把一次 Raft RPC 送到指定节点。
// 返回 false 表示这次调用失败（节点不可达或者超时），调用方按丢包处理：
// Raft 的算法本身就有重试，不需要传输层保证送达。
type Transport interface {
	Call(peer int, method string, args, reply interface{}) bool
}

// NetTransport 用 TCP 上的 net/rpc 调用远端节点，每次调用新建一条连接。
// 心跳频率不高时这样最省事；真实系统会复用连接或直接用 gRPC。
type NetTransport struct {
	addrs   []string
	timeout time.Duration
}

func NewNetTransport(addrs []string, timeout time.Duration) *NetTransport {
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	return &NetTransport{addrs: addrs, timeout: timeout}
}

func (t *NetTransport) Call(peer int, method string, args, reply interface{}) bool {
	if peer < 0 || peer >= len(t.addrs) {
		return false
	}
	conn, err := net.DialTimeout("tcp", t.addrs[peer], t.timeout)
	if err != nil {
		return false
	}
	client := rpc.NewClient(conn)
	defer client.Close()

	call := client.Go(method, args, reply, make(chan *rpc.Call, 1))
	select {
	case <-call.Done:
		return call.Error == nil
	case <-time.After(t.timeout):
		return false
	}
}

// LocalNetwork 是进程内的假网络：测试和 demo 用它模拟丢包、断链和分区，
// 不必真的占用端口，也不用等 TCP 超时。生产代码走 NetTransport。
type LocalNetwork struct {
	mu      sync.Mutex
	nodes   map[int]*Raft
	blocked map[connKey]bool
	crashed map[int]bool
	delay   time.Duration
}

type connKey struct{ from, to int }

func NewLocalNetwork() *LocalNetwork {
	return &LocalNetwork{
		nodes:   make(map[int]*Raft),
		blocked: make(map[connKey]bool),
		crashed: make(map[int]bool),
	}
}

func (n *LocalNetwork) SetDelay(d time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.delay = d
}

func (n *LocalNetwork) Add(id int, rf *Raft) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.nodes[id] = rf
	delete(n.crashed, id)
}

// Remove 让节点从网络上消失，等同于进程崩溃。
func (n *LocalNetwork) Remove(id int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.nodes, id)
	n.crashed[id] = true
}

func (n *LocalNetwork) Transport(id int) Transport {
	return &localTransport{net: n, from: id}
}

// Connect 控制两个节点之间是否能通信（双向）。
func (n *LocalNetwork) Connect(a, b int, ok bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.blocked[connKey{a, b}] = !ok
	n.blocked[connKey{b, a}] = !ok
}

// Partition 按分组切断网络：不同分组之间互相不可达。分组之外（比如已崩溃）的节点不受影响。
func (n *LocalNetwork) Partition(groups ...[]int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.blocked = make(map[connKey]bool)
	group := make(map[int]int)
	for gi, g := range groups {
		for _, id := range g {
			group[id] = gi
		}
	}
	for a := range n.nodes {
		for b := range n.nodes {
			if a != b && group[a] != group[b] {
				n.blocked[connKey{a, b}] = true
			}
		}
	}
}

func (n *LocalNetwork) Heal() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.blocked = make(map[connKey]bool)
}

type localTransport struct {
	net  *LocalNetwork
	from int
}

func (t *localTransport) Call(peer int, method string, args, reply interface{}) bool {
	return t.net.call(t.from, peer, method, args, reply)
}

func (n *LocalNetwork) call(from, to int, method string, args, reply interface{}) bool {
	n.mu.Lock()
	target := n.nodes[to]
	blocked := n.crashed[from] || n.crashed[to] ||
		n.blocked[connKey{from, to}] || n.blocked[connKey{to, from}]
	delay := n.delay
	n.mu.Unlock()

	if target == nil || blocked {
		return false
	}
	if delay > 0 {
		time.Sleep(time.Duration(rand.Int63n(int64(delay) + 1)))
	}

	switch method {
	case "Raft.RequestVote":
		return target.RequestVote(args.(*RequestVoteArgs), reply.(*RequestVoteReply)) == nil
	case "Raft.AppendEntries":
		return target.AppendEntries(args.(*AppendEntriesArgs), reply.(*AppendEntriesReply)) == nil
	case "Raft.InstallSnapshot":
		return target.InstallSnapshot(args.(*InstallSnapshotArgs), reply.(*InstallSnapshotReply)) == nil
	default:
		return false
	}
}
