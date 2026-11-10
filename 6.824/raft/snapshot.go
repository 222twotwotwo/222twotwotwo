package raft

// Snapshot 把 index 及之前的日志替换成一份快照，释放日志空间。
// index 必须是已经应用过的位置；调用方（上层状态机）负责保证快照内容
// 恰好等于它执行到 index 时的状态。
func (rf *Raft) Snapshot(index int, data []byte) bool {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if index <= rf.snapshotIndex || index > rf.lastApplied || index > rf.lastLogIndex() {
		return false
	}
	term := rf.logTerm(index)

	var tail []LogEntry
	if index < rf.lastLogIndex() {
		tail = make([]LogEntry, rf.lastLogIndex()-index)
		copy(tail, rf.log[index-rf.snapshotIndex+1:])
	}
	rf.log = append([]LogEntry{{Term: term}}, tail...)
	rf.snapshotIndex = index
	rf.snapshotTerm = term
	rf.snapshotData = append([]byte(nil), data...)
	rf.persist()
	return true
}

// ReadSnapshot 返回当前快照，供上层状态机在启动时恢复。
func (rf *Raft) ReadSnapshot() (int, []byte) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.snapshotIndex, append([]byte(nil), rf.snapshotData...)
}

// InstallSnapshot 处理 leader 发来的快照。快照可能比本地已提交位置还新，
// 所以直接丢弃本地日志、把应用位置推到 LastIncludedIndex。
func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm {
		return nil
	}
	if args.Term > rf.currentTerm {
		rf.becomeFollower(args.Term, args.LeaderID)
	} else if rf.state != Follower {
		rf.state = Follower
		rf.stopLeading()
	}
	rf.leaderID = args.LeaderID
	rf.resetElectionDeadline()
	reply.Term = rf.currentTerm

	if args.LastIncludedIndex <= rf.commitIndex || args.LastIncludedIndex <= rf.snapshotIndex {
		return nil
	}

	rf.snapshotIndex = args.LastIncludedIndex
	rf.snapshotTerm = args.LastIncludedTerm
	rf.snapshotData = append([]byte(nil), args.Data...)
	rf.log = []LogEntry{{Term: args.LastIncludedTerm}}
	rf.commitIndex = args.LastIncludedIndex
	if rf.lastApplied < args.LastIncludedIndex {
		rf.lastApplied = args.LastIncludedIndex
	}
	rf.pendingSnapshot = &ApplyMsg{
		SnapshotValid: true,
		Snapshot:      rf.snapshotData,
		SnapshotIndex: rf.snapshotIndex,
		SnapshotTerm:  rf.snapshotTerm,
	}
	rf.persist()
	rf.signalApply()
	return nil
}
