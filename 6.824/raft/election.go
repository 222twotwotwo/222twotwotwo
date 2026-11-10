package raft

// startElection 必须持有锁调用。每个任期只投自己一票，然后并发拉票。
func (rf *Raft) startElection() {
	rf.state = Candidate
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.leaderID = -1
	rf.electionVotes = 1
	rf.resetElectionDeadline()
	rf.persist()

	lastIndex := rf.lastLogIndex()
	args := RequestVoteArgs{
		Term:         rf.currentTerm,
		CandidateID:  rf.me,
		LastLogIndex: lastIndex,
		LastLogTerm:  rf.logTerm(lastIndex),
	}
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		go rf.requestVote(i, args)
	}
}

func (rf *Raft) requestVote(peer int, args RequestVoteArgs) {
	reply := RequestVoteReply{}
	if !rf.transport.Call(peer, "Raft.RequestVote", &args, &reply) {
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if reply.Term > rf.currentTerm {
		rf.becomeFollower(reply.Term, -1)
		return
	}
	// 只统计本轮竞选、且任期没变的票，迟到的回包直接丢弃。
	if rf.state != Candidate || rf.currentTerm != args.Term || !reply.VoteGranted {
		return
	}
	rf.electionVotes++
	if rf.electionVotes*2 > len(rf.peers) {
		rf.becomeLeader()
	}
}

// RequestVote 处理拉票。投票规则：
//  1. 任期比自己小直接拒绝；
//  2. 本任期已经投给别人就拒绝（重复请求同一候选人视为重传，可以再投一次）；
//  3. 候选人日志必须至少和自己一样新，否则即使对方当选也可能丢数据。
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.Term = rf.currentTerm
	reply.VoteGranted = false

	if args.Term < rf.currentTerm {
		return nil
	}
	if args.Term > rf.currentTerm {
		rf.becomeFollower(args.Term, -1)
	}

	lastIndex := rf.lastLogIndex()
	lastTerm := rf.logTerm(lastIndex)
	upToDate := args.LastLogTerm > lastTerm ||
		(args.LastLogTerm == lastTerm && args.LastLogIndex >= lastIndex)

	if upToDate && (rf.votedFor == -1 || rf.votedFor == args.CandidateID) {
		rf.votedFor = args.CandidateID
		rf.resetElectionDeadline()
		rf.persist()
		reply.VoteGranted = true
	}
	reply.Term = rf.currentTerm
	return nil
}
