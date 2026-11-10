package raft

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// 支持的操作类型。所有读写都走 Raft 日志，因此 Get 也是线性一致的：
// 代价是读也要一次复制往返，好处是不用实现 ReadIndex/租约读。
const (
	OpGet    = "get"
	OpPut    = "put"
	OpAppend = "append"
)

const (
	ErrWrongLeader = "wrong leader"
	ErrTimeout     = "timeout"
)

// Op 是写进 Raft 日志的命令。Client/Seq 用于去重：
// 客户端重试同一条命令时，状态机只执行一次。
type Op struct {
	Kind   string
	Key    string
	Value  string
	Client int64
	Seq    int
}

type GetArgs struct {
	Key    string
	Client int64
	Seq    int
}

type GetReply struct {
	Err   string
	Value string
}

type PutAppendArgs struct {
	Kind   string
	Key    string
	Value  string
	Client int64
	Seq    int
}

type PutAppendReply struct {
	Err string
}

type opResult struct {
	client int64
	seq    int
	value  string
	err    string
}

type kvState struct {
	Store     map[string]string `json:"store"`
	LastSeq   map[int64]int     `json:"last_seq"`
	LastReply map[int64]string  `json:"last_reply"`
}

// KVServer 是基于 Raft 的键值状态机。它消费 Raft 的 ApplyCh，
// 把提交后的命令应用到内存 map，并应答客户端 RPC。
type KVServer struct {
	rf           *Raft
	applyCh      chan ApplyMsg
	maxRaftState int // 日志超过这个字节数就打快照，<= 0 表示关闭

	mu            sync.Mutex
	store         map[string]string
	lastSeq       map[int64]int
	lastReply     map[int64]string
	applied       int
	snapshotIndex int
	results       map[int]opResult

	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewKVServer 从 Raft 快照恢复状态，然后开始消费 ApplyCh。
func NewKVServer(rf *Raft, applyCh chan ApplyMsg, maxRaftState int) (*KVServer, error) {
	s := &KVServer{
		rf:           rf,
		applyCh:      applyCh,
		maxRaftState: maxRaftState,
		store:        make(map[string]string),
		lastSeq:      make(map[int64]int),
		lastReply:    make(map[int64]string),
		results:      make(map[int]opResult),
		stopCh:       make(chan struct{}),
	}
	if index, data := rf.ReadSnapshot(); index > 0 && len(data) > 0 {
		if err := s.restore(data); err != nil {
			return nil, fmt.Errorf("kv: 快照损坏: %w", err)
		}
		s.applied = index
		s.snapshotIndex = index
	}
	go s.run()
	return s, nil
}

// Stop 停止状态机协程，用于模拟节点崩溃或测试收尾。
func (s *KVServer) Stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
}

func (s *KVServer) run() {
	for {
		select {
		case <-s.stopCh:
			return
		case msg := <-s.applyCh:
			s.apply(msg)
			s.maybeSnapshot()
		}
	}
}

func (s *KVServer) apply(msg ApplyMsg) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msg.SnapshotValid {
		if msg.SnapshotIndex <= s.applied {
			return
		}
		if err := s.restore(msg.Snapshot); err != nil {
			return
		}
		s.applied = msg.SnapshotIndex
		s.snapshotIndex = msg.SnapshotIndex
		for index := range s.results {
			if index <= s.applied {
				delete(s.results, index)
			}
		}
		return
	}

	if !msg.CommandValid || msg.CommandIndex <= s.applied {
		return
	}
	var op Op
	if err := json.Unmarshal(msg.Command, &op); err != nil {
		// 坏命令不会由本包产生；跳过它以免卡住整条日志。
		s.applied = msg.CommandIndex
		return
	}

	res := opResult{client: op.Client, seq: op.Seq}
	if op.Seq <= s.lastSeq[op.Client] {
		// 重复请求：直接返回上次的结果。
		res.value = s.lastReply[op.Client]
	} else {
		switch op.Kind {
		case OpGet:
			res.value = s.store[op.Key]
		case OpPut:
			s.store[op.Key] = op.Value
		case OpAppend:
			s.store[op.Key] += op.Value
		default:
			res.err = "unknown op"
		}
		s.lastSeq[op.Client] = op.Seq
		s.lastReply[op.Client] = res.value
	}
	s.results[msg.CommandIndex] = res
	s.applied = msg.CommandIndex

	if msg.CommandIndex%32 == 0 {
		for index := range s.results {
			if index < s.applied-64 {
				delete(s.results, index)
			}
		}
	}
}

func (s *KVServer) restore(data []byte) error {
	var state kvState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	if state.Store == nil {
		state.Store = make(map[string]string)
	}
	if state.LastSeq == nil {
		state.LastSeq = make(map[int64]int)
	}
	if state.LastReply == nil {
		state.LastReply = make(map[int64]string)
	}
	s.store = state.Store
	s.lastSeq = state.LastSeq
	s.lastReply = state.LastReply
	return nil
}

func (s *KVServer) encode() ([]byte, error) {
	return json.Marshal(kvState{Store: s.store, LastSeq: s.lastSeq, LastReply: s.lastReply})
}

func (s *KVServer) maybeSnapshot() {
	if s.maxRaftState <= 0 || s.rf.LogSize() <= s.maxRaftState {
		return
	}
	s.mu.Lock()
	index := s.applied
	if index <= s.snapshotIndex {
		s.mu.Unlock()
		return
	}
	data, err := s.encode()
	s.mu.Unlock()
	if err != nil {
		return
	}
	if s.rf.Snapshot(index, data) {
		s.mu.Lock()
		s.snapshotIndex = index
		s.mu.Unlock()
	}
}

// Get 是线性一致的读：命令同样进日志，提交后返回状态机的值。
func (s *KVServer) Get(args *GetArgs, reply *GetReply) error {
	op := Op{Kind: OpGet, Key: args.Key, Client: args.Client, Seq: args.Seq}
	res, ok := s.propose(op)
	if !ok {
		reply.Err = ErrWrongLeader
		return nil
	}
	reply.Value = res.value
	reply.Err = res.err
	return nil
}

func (s *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) error {
	op := Op{Kind: args.Kind, Key: args.Key, Value: args.Value, Client: args.Client, Seq: args.Seq}
	res, ok := s.propose(op)
	if !ok {
		reply.Err = ErrWrongLeader
		return nil
	}
	reply.Err = res.err
	return nil
}

// propose 提交命令并等它被应用。返回 ok=false 表示本节点不是 leader 或者任期已变，
// 客户端应该换一个节点重试。轮询间隔很小，因为一条命令通常在一次心跳内就提交。
func (s *KVServer) propose(op Op) (opResult, bool) {
	data, err := json.Marshal(op)
	if err != nil {
		return opResult{err: err.Error()}, true
	}
	index, term, isLeader := s.rf.Start(data)
	if !isLeader {
		return opResult{}, false
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		res, ok := s.results[index]
		s.mu.Unlock()
		if ok && res.client == op.Client && res.seq == op.Seq {
			return res, true
		}
		if current, leader := s.rf.GetState(); !leader || current != term {
			return opResult{}, false
		}
		time.Sleep(time.Millisecond)
	}
	return opResult{err: ErrTimeout}, true
}
