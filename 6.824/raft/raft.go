package raft

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Config 描述一个 Raft 节点的启动参数。
type Config struct {
	ID        int      // 本节点编号，等于 Peers 中的下标
	Peers     []string // 集群节点地址列表，下标即节点编号
	Storage   Storage  // 持久化实现
	Transport Transport
	ApplyCh   chan ApplyMsg // 已提交日志交给上层应用的通道，不可关闭

	Heartbeat   time.Duration // 心跳间隔
	ElectionMin time.Duration // 选举超时下限
	ElectionMax time.Duration // 选举超时上限
}

func (c *Config) setDefaults() error {
	if len(c.Peers) == 0 {
		return fmt.Errorf("raft: 至少需要一个节点")
	}
	if c.ID < 0 || c.ID >= len(c.Peers) {
		return fmt.Errorf("raft: 节点编号 %d 超出范围 [0,%d)", c.ID, len(c.Peers))
	}
	if c.Storage == nil {
		return fmt.Errorf("raft: 必须提供 Storage")
	}
	if c.Transport == nil {
		return fmt.Errorf("raft: 必须提供 Transport")
	}
	if c.ApplyCh == nil {
		return fmt.Errorf("raft: 必须提供 ApplyCh")
	}
	if c.Heartbeat <= 0 {
		c.Heartbeat = 100 * time.Millisecond
	}
	if c.ElectionMin <= 0 {
		c.ElectionMin = 300 * time.Millisecond
	}
	if c.ElectionMax <= 0 {
		c.ElectionMax = 600 * time.Millisecond
	}
	if c.ElectionMax < c.ElectionMin {
		return fmt.Errorf("raft: ElectionMax 必须不小于 ElectionMin")
	}
	if c.Heartbeat >= c.ElectionMin {
		return fmt.Errorf("raft: 心跳间隔必须明显小于选举超时")
	}
	return nil
}

// Raft 是一个节点。除 done 与 applyNotify 外，所有字段都由 mu 保护。
// 约定：凡是会改变持久状态的路径（任期、投票、日志、快照），都必须先 persist 再回复 RPC。
type Raft struct {
	cfg       Config
	me        int
	peers     []string
	storage   Storage
	transport Transport
	applyCh   chan ApplyMsg

	mu   sync.Mutex
	rand *rand.Rand

	state       State
	currentTerm int
	votedFor    int
	log         []LogEntry // log[0] 是快照边界，index 为 snapshotIndex

	snapshotIndex int
	snapshotTerm  int
	snapshotData  []byte

	commitIndex int
	lastApplied int
	leaderID    int

	electionDeadline time.Time
	electionVotes    int

	nextIndex  []int
	matchIndex []int
	epoch      int           // 任期内的领袖轮次，用于让旧的复制协程自动退出
	epochDone  chan struct{} // 本轮领袖结束时关闭
	trigger    []chan struct{}

	pendingSnapshot *ApplyMsg

	applyNotify chan struct{}
	done        chan struct{}
	killOnce    sync.Once
}

// NewRaft 从 Storage 恢复状态并启动后台协程。
// 持久化失败会直接 panic：Raft 在不落盘的情况下继续运行会破坏安全性。
func NewRaft(cfg Config) (*Raft, error) {
	if err := cfg.setDefaults(); err != nil {
		return nil, err
	}
	stored, err := cfg.Storage.Load()
	if err != nil {
		return nil, fmt.Errorf("raft: 读取持久状态失败: %w", err)
	}

	rf := &Raft{
		cfg:           cfg,
		me:            cfg.ID,
		peers:         append([]string(nil), cfg.Peers...),
		storage:       cfg.Storage,
		transport:     cfg.Transport,
		applyCh:       cfg.ApplyCh,
		rand:          rand.New(rand.NewSource(time.Now().UnixNano() + int64(cfg.ID)*7919)),
		state:         Follower,
		currentTerm:   stored.CurrentTerm,
		votedFor:      stored.VotedFor,
		snapshotIndex: stored.SnapshotIndex,
		snapshotTerm:  stored.SnapshotTerm,
		snapshotData:  stored.Snapshot,
		log:           stored.Log,
		leaderID:      -1,
		nextIndex:     make([]int, len(cfg.Peers)),
		matchIndex:    make([]int, len(cfg.Peers)),
		trigger:       make([]chan struct{}, len(cfg.Peers)),
		applyNotify:   make(chan struct{}, 1),
		done:          make(chan struct{}),
	}
	if rf.votedFor < -1 {
		rf.votedFor = -1
	}
	if len(rf.log) == 0 {
		rf.log = []LogEntry{{Term: rf.snapshotTerm}}
	}
	if rf.log[0].Term != rf.snapshotTerm {
		return nil, fmt.Errorf("raft: 快照边界与日志不一致")
	}
	rf.commitIndex = rf.snapshotIndex
	rf.lastApplied = rf.snapshotIndex
	for i := range rf.trigger {
		rf.trigger[i] = make(chan struct{}, 1)
	}
	rf.resetElectionDeadline()

	go rf.ticker()
	go rf.applier()
	return rf, nil
}

// Kill 停止后台协程。已经进入处理函数的 RPC 仍会执行完，但不会再发起新的广播。
func (rf *Raft) Kill() {
	rf.killOnce.Do(func() { close(rf.done) })
}

func (rf *Raft) killed() bool {
	select {
	case <-rf.done:
		return true
	default:
		return false
	}
}

// GetState 返回当前任期以及自己是否是 leader。
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm, rf.state == Leader
}

// Leader 返回已知 leader 的编号，-1 表示未知。
func (rf *Raft) Leader() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.leaderID
}

// Status 供命令行与测试观察内部状态。
func (rf *Raft) Status(_ *StatusArgs, reply *StatusReply) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	*reply = StatusReply{
		ID:            rf.me,
		Term:          rf.currentTerm,
		State:         rf.state.String(),
		Leader:        rf.leaderID,
		LastLogIndex:  rf.lastLogIndex(),
		CommitIndex:   rf.commitIndex,
		LastApplied:   rf.lastApplied,
		SnapshotIndex: rf.snapshotIndex,
		LogBytes:      rf.logSizeLocked(),
	}
	return nil
}

// Start 把一条命令追加到 leader 日志。返回的 index 是这条命令将来的提交下标。
// 只有 leader 能接受命令；follower 和 candidate 返回 isLeader=false。
func (rf *Raft) Start(command []byte) (index int, term int, isLeader bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state != Leader {
		return -1, rf.currentTerm, false
	}
	rf.log = append(rf.log, LogEntry{Term: rf.currentTerm, Command: append([]byte(nil), command...)})
	rf.persist()
	index = rf.lastLogIndex()
	rf.matchIndex[rf.me] = index
	rf.signalReplication()
	return index, rf.currentTerm, true
}

func (rf *Raft) lastLogIndex() int {
	return rf.snapshotIndex + len(rf.log) - 1
}

// logTerm 返回真实下标 index 对应日志的任期，越界返回 -1。
func (rf *Raft) logTerm(index int) int {
	if index < rf.snapshotIndex || index > rf.lastLogIndex() {
		return -1
	}
	return rf.log[index-rf.snapshotIndex].Term
}

// entry 返回真实下标 index 的日志，调用者必须保证 index 在快照之后。
func (rf *Raft) entry(index int) LogEntry {
	return rf.log[index-rf.snapshotIndex]
}

func (rf *Raft) logSizeLocked() int {
	size := 0
	for _, entry := range rf.log {
		size += len(entry.Command) + 24
	}
	return size
}

// LogSize 返回日志占用的大致字节数，上层用它决定何时打快照。
func (rf *Raft) LogSize() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.logSizeLocked()
}

func (rf *Raft) persist() {
	state := PersistentState{
		CurrentTerm:   rf.currentTerm,
		VotedFor:      rf.votedFor,
		SnapshotIndex: rf.snapshotIndex,
		SnapshotTerm:  rf.snapshotTerm,
		Snapshot:      rf.snapshotData,
		Log:           rf.log,
	}
	if err := rf.storage.Save(state); err != nil {
		panic(fmt.Sprintf("raft: 持久化失败，不能继续运行: %v", err))
	}
}

func (rf *Raft) resetElectionDeadline() {
	timeout := rf.cfg.ElectionMin
	if gap := rf.cfg.ElectionMax - rf.cfg.ElectionMin; gap > 0 {
		timeout += time.Duration(rf.rand.Int63n(int64(gap)))
	}
	rf.electionDeadline = time.Now().Add(timeout)
}

func (rf *Raft) becomeFollower(term int, leader int) {
	if term > rf.currentTerm {
		rf.currentTerm = term
		rf.votedFor = -1
	}
	if rf.state == Leader {
		rf.stopLeading()
	}
	rf.state = Follower
	rf.leaderID = leader
	rf.persist()
}

func (rf *Raft) stopLeading() {
	if rf.epochDone != nil {
		close(rf.epochDone)
		rf.epochDone = nil
	}
}

func (rf *Raft) becomeLeader() {
	rf.state = Leader
	rf.leaderID = rf.me
	rf.epoch++
	done := make(chan struct{})
	rf.epochDone = done
	epoch := rf.epoch
	term := rf.currentTerm

	last := rf.lastLogIndex()
	for i := range rf.peers {
		rf.nextIndex[i] = last + 1
		rf.matchIndex[i] = 0
		if i == rf.me {
			rf.matchIndex[i] = last
		}
	}
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		go rf.replicate(i, epoch, term, done)
	}
}

func (rf *Raft) signalApply() {
	select {
	case rf.applyNotify <- struct{}{}:
	default:
	}
}

func (rf *Raft) signalReplication() {
	for i := range rf.trigger {
		select {
		case rf.trigger[i] <- struct{}{}:
		default:
		}
	}
}

// ticker 只做一件事：选举超时后发起新一轮投票。
func (rf *Raft) ticker() {
	interval := rf.cfg.Heartbeat / 4
	if interval < 5*time.Millisecond {
		interval = 5 * time.Millisecond
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-rf.done:
			return
		case <-timer.C:
		}
		rf.mu.Lock()
		if rf.state != Leader && !time.Now().Before(rf.electionDeadline) {
			rf.startElection()
		}
		rf.mu.Unlock()
		timer.Reset(interval)
	}
}

// applier 按顺序把已提交日志投递给上层应用，也负责投递快照。
// 只有它一个协程往 applyCh 写，所以顺序天然是递增的。
func (rf *Raft) applier() {
	for {
		select {
		case <-rf.done:
			return
		case <-rf.applyNotify:
		}
		for {
			rf.mu.Lock()
			if rf.pendingSnapshot != nil {
				msg := *rf.pendingSnapshot
				rf.pendingSnapshot = nil
				rf.mu.Unlock()
				select {
				case rf.applyCh <- msg:
				case <-rf.done:
					return
				}
				continue
			}
			if rf.lastApplied >= rf.commitIndex {
				rf.mu.Unlock()
				break
			}
			start := rf.lastApplied + 1
			end := rf.commitIndex
			msgs := make([]ApplyMsg, 0, end-start+1)
			for i := start; i <= end; i++ {
				entry := rf.entry(i)
				msgs = append(msgs, ApplyMsg{
					CommandValid: true,
					Command:      entry.Command,
					CommandIndex: i,
					CommandTerm:  entry.Term,
				})
			}
			rf.lastApplied = end
			rf.mu.Unlock()
			for _, msg := range msgs {
				select {
				case rf.applyCh <- msg:
				case <-rf.done:
					return
				}
			}
		}
	}
}
