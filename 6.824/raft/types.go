package raft

// State 是节点在 Raft 中扮演的角色。
type State int

const (
	Follower State = iota
	Candidate
	Leader
)

func (s State) String() string {
	switch s {
	case Follower:
		return "follower"
	case Candidate:
		return "candidate"
	case Leader:
		return "leader"
	default:
		return "unknown"
	}
}

// LogEntry 是一条日志。命令对 Raft 不透明，由上层应用自行编码，
// 这样持久化实现不必认识业务类型。
type LogEntry struct {
	Term    int
	Command []byte
}

// ApplyMsg 是 Raft 交给上层应用的消息。两种消息按顺序在同一个通道上投递，
// 上层应用只要顺序处理即可重建状态机。
type ApplyMsg struct {
	CommandValid bool
	Command      []byte
	CommandIndex int
	CommandTerm  int

	SnapshotValid bool
	Snapshot      []byte
	SnapshotIndex int
	SnapshotTerm  int
}

// RequestVoteArgs 拉票请求。LastLogIndex/LastLogTerm 用于比较日志新旧。
type RequestVoteArgs struct {
	Term         int
	CandidateID  int
	LastLogIndex int
	LastLogTerm  int
}

type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

// AppendEntriesArgs 既是日志复制，也是心跳（Entries 为空时）。
type AppendEntriesArgs struct {
	Term         int
	LeaderID     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

// AppendEntriesReply 的 ConflictIndex/ConflictTerm 让 leader 一次回退到位，
// 不用逐条试探。ConflictTerm == -1 表示 follower 的日志比 PrevLogIndex 短，
// 此时 ConflictIndex 是它第一个缺失的下标。
type AppendEntriesReply struct {
	Term          int
	Success       bool
	ConflictIndex int
	ConflictTerm  int
}

// InstallSnapshotArgs 把 leader 的快照整体发给落后太多的 follower。
type InstallSnapshotArgs struct {
	Term              int
	LeaderID          int
	LastIncludedIndex int
	LastIncludedTerm  int
	Data              []byte
}

type InstallSnapshotReply struct {
	Term int
}

// StatusArgs/StatusReply 只用于命令行和测试观察，不参与 Raft 协议。
type StatusArgs struct{}

type StatusReply struct {
	ID            int
	Term          int
	State         string
	Leader        int
	LastLogIndex  int
	CommitIndex   int
	LastApplied   int
	SnapshotIndex int
	LogBytes      int
}
