package raft

import (
	"sort"
	"time"
)

// replicate 是 leader 发给某个 follower 的长连接式循环：
// 有变化就立刻发，没变化就按心跳间隔发一次。epoch 用来让换届后旧循环自行退出。
func (rf *Raft) replicate(peer, epoch, term int, done chan struct{}) {
	heartbeat := rf.cfg.Heartbeat
	for {
		if rf.killed() {
			return
		}
		select {
		case <-done:
			return
		case <-rf.done:
			return
		default:
		}

		rf.mu.Lock()
		if rf.state != Leader || rf.currentTerm != term || rf.epoch != epoch {
			rf.mu.Unlock()
			return
		}
		// follower 需要的日志已经被压缩进快照，只能整份发快照。
		if rf.nextIndex[peer] <= rf.snapshotIndex {
			args := InstallSnapshotArgs{
				Term:              term,
				LeaderID:          rf.me,
				LastIncludedIndex: rf.snapshotIndex,
				LastIncludedTerm:  rf.snapshotTerm,
				Data:              rf.snapshotData,
			}
			rf.mu.Unlock()

			reply := InstallSnapshotReply{}
			ok := rf.transport.Call(peer, "Raft.InstallSnapshot", &args, &reply)
			rf.mu.Lock()
			if rf.state != Leader || rf.currentTerm != term || rf.epoch != epoch {
				rf.mu.Unlock()
				return
			}
			switch {
			case !ok:
				rf.mu.Unlock()
			case reply.Term > rf.currentTerm:
				rf.becomeFollower(reply.Term, -1)
				rf.mu.Unlock()
				return
			default:
				if args.LastIncludedIndex > rf.matchIndex[peer] {
					rf.matchIndex[peer] = args.LastIncludedIndex
				}
				if rf.nextIndex[peer] < args.LastIncludedIndex+1 {
					rf.nextIndex[peer] = args.LastIncludedIndex + 1
				}
				rf.mu.Unlock()
			}
			if !rf.waitNext(peer, done, heartbeat) {
				return
			}
			continue
		}

		prev := rf.nextIndex[peer] - 1
		tail := rf.log[prev-rf.snapshotIndex+1:]
		entries := make([]LogEntry, len(tail))
		copy(entries, tail)
		args := AppendEntriesArgs{
			Term:         term,
			LeaderID:     rf.me,
			PrevLogIndex: prev,
			PrevLogTerm:  rf.logTerm(prev),
			Entries:      entries,
			LeaderCommit: rf.commitIndex,
		}
		rf.mu.Unlock()

		reply := AppendEntriesReply{}
		ok := rf.transport.Call(peer, "Raft.AppendEntries", &args, &reply)
		rf.mu.Lock()
		if rf.state != Leader || rf.currentTerm != term || rf.epoch != epoch {
			rf.mu.Unlock()
			return
		}
		switch {
		case !ok:
			rf.mu.Unlock()
		case reply.Term > rf.currentTerm:
			rf.becomeFollower(reply.Term, -1)
			rf.mu.Unlock()
			return
		case reply.Success:
			if match := args.PrevLogIndex + len(args.Entries); match > rf.matchIndex[peer] {
				rf.matchIndex[peer] = match
			}
			if rf.nextIndex[peer] < rf.matchIndex[peer]+1 {
				rf.nextIndex[peer] = rf.matchIndex[peer] + 1
			}
			rf.advanceCommit()
			rf.mu.Unlock()
		default:
			// 日志冲突：借助 follower 给出的冲突任期，把 nextIndex 一次退到位。
			next := reply.ConflictIndex
			if reply.ConflictTerm >= 0 {
				if i := rf.lastIndexWithTerm(reply.ConflictTerm); i >= 0 {
					next = i + 1
				}
			}
			if next < 1 {
				next = 1
			}
			if limit := rf.lastLogIndex(); next > limit {
				next = limit
			}
			if next < 1 {
				next = 1
			}
			rf.nextIndex[peer] = next
			rf.mu.Unlock()
			continue // 立刻重试，不必等心跳
		}
		if !rf.waitNext(peer, done, heartbeat) {
			return
		}
	}
}

// waitNext 等下一次触发（新命令）或者心跳超时。返回 false 表示循环应该退出。
func (rf *Raft) waitNext(peer int, done chan struct{}, heartbeat time.Duration) bool {
	timer := time.NewTimer(heartbeat)
	defer timer.Stop()
	select {
	case <-done:
		return false
	case <-rf.done:
		return false
	case <-rf.trigger[peer]:
		return true
	case <-timer.C:
		return true
	}
}

func (rf *Raft) lastIndexWithTerm(term int) int {
	for i := rf.lastLogIndex(); i > rf.snapshotIndex; i-- {
		if rf.logTerm(i) == term {
			return i
		}
	}
	return -1
}

// advanceCommit 找出“多数派都已有”的最高下标。
//
// 把各节点的 matchIndex 从大到小排序，第 quorum 大的值就是多数派都持有的最高下标。
// 再加一条限制：只能提交当前任期的日志（Raft 5.4.2），否则可能把已被新 leader
// 覆盖的旧日志误提交。旧任期的日志会随着当前任期的日志一起提交。
func (rf *Raft) advanceCommit() {
	matches := make([]int, 0, len(rf.peers))
	for i := range rf.peers {
		if i == rf.me {
			matches = append(matches, rf.lastLogIndex())
			continue
		}
		matches = append(matches, rf.matchIndex[i])
	}
	sort.Sort(sort.Reverse(sort.IntSlice(matches)))

	quorum := len(matches)/2 + 1
	candidate := matches[quorum-1]
	if candidate > rf.commitIndex && candidate <= rf.lastLogIndex() &&
		rf.logTerm(candidate) == rf.currentTerm {
		rf.commitIndex = candidate
		rf.signalApply()
	}
}

// AppendEntries 处理日志复制和心跳。失败时返回冲突任期，让 leader 快速回头。
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.Term = rf.currentTerm
	reply.Success = false
	reply.ConflictIndex = -1
	reply.ConflictTerm = -1

	if args.Term < rf.currentTerm {
		return nil
	}
	if args.Term > rf.currentTerm {
		rf.becomeFollower(args.Term, args.LeaderID)
	} else if rf.state != Follower {
		// 同任期内收到心跳说明别人已经当选，自己退回 follower。
		rf.state = Follower
		rf.stopLeading()
	}
	rf.leaderID = args.LeaderID
	rf.resetElectionDeadline()
	reply.Term = rf.currentTerm

	if args.PrevLogIndex < rf.snapshotIndex {
		// 这条前序日志已经被本节点压缩进快照，说明双方进度差太远。
		reply.ConflictIndex = rf.snapshotIndex + 1
		return nil
	}
	if args.PrevLogIndex > rf.lastLogIndex() {
		reply.ConflictIndex = rf.lastLogIndex() + 1
		return nil
	}
	if rf.logTerm(args.PrevLogIndex) != args.PrevLogTerm {
		term := rf.logTerm(args.PrevLogIndex)
		reply.ConflictTerm = term
		idx := args.PrevLogIndex
		for idx > rf.snapshotIndex+1 && rf.logTerm(idx-1) == term {
			idx--
		}
		reply.ConflictIndex = idx
		return nil
	}

	changed := false
	for i, entry := range args.Entries {
		idx := args.PrevLogIndex + 1 + i
		if idx <= rf.lastLogIndex() {
			if rf.logTerm(idx) == entry.Term {
				continue
			}
			if idx <= rf.commitIndex {
				// 已提交日志不可能被覆盖；真出现说明算法被破坏，拒绝而不破坏状态。
				reply.ConflictIndex = idx
				return nil
			}
			rf.log = rf.log[:idx-rf.snapshotIndex]
			changed = true
		}
		rf.log = append(rf.log, args.Entries[i:]...)
		changed = true
		break
	}
	if changed {
		rf.persist()
	}
	reply.Success = true

	lastNew := args.PrevLogIndex + len(args.Entries)
	newCommit := args.LeaderCommit
	if newCommit > lastNew {
		newCommit = lastNew
	}
	if newCommit > rf.commitIndex {
		rf.commitIndex = newCommit
		rf.signalApply()
	}
	return nil
}
