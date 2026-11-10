---
title: 实现最简 Raft 共识算法
date: 2026-11-10
category: Go 后端
tags: Go, 分布式共识, Raft, 复制状态机, 快照
readTime: 7 分钟阅读
summary: 用 Go 标准库实现最简的 Raft：选举、日志复制、快照和一个线性一致的 KV 状态机
cover: ../images/logos/go-logo.svg
---

用 Go 标准库实现最简的 Raft 共识算法

## 架构

![架构图：客户端轮询三个节点；节点之间互发 AppendEntries 与 RequestVote，金色边框是当前任期的 leader](../images/logos/raft-architecture-inline.svg)

系统里只有两类角色：客户端 Clerk 和 Raft 节点。三个节点地位完全对等，跑同一套代码、同一份落盘状态；谁当 leader 由选举决定，任期一变就可能换人，所以图里的 leader / follower 只是某个任期下的瞬时快照。

每个节点内部三部分：Raft 负责共识，KV 状态机负责应用语义，状态文件负责持久化。客户端不认 leader，三条连线都会试：写请求打到 follower 会被拒绝，换一个节点重试即可。

Raft 只做一件事——让多个节点对"同一串命令、同样的顺序"达成一致。命令具体是什么它不关心，所以换个状态机这套实现照样能用。

## 环境

- **编译器**：Go 1.22
- **操作系统**：Windows 11

只依赖标准库：`net/rpc` 做节点间通信，`encoding/json` 做序列化与状态机快照。

## 节点设计

```go
type Raft struct {
	//...
	state         State
	currentTerm   int
	votedFor      int
	log           []LogEntry // log[0] 是快照边界，index 为 snapshotIndex
	snapshotIndex int
	commitIndex   int
	lastApplied   int
	leaderID      int
	//...
}
```

三个下标构成一条不变量：`lastApplied <= commitIndex <= lastLogIndex`。

- `lastLogIndex`：本地写到最后一条日志的位置
- `commitIndex`：多数派确认过、可以提交的位置
- `lastApplied`：已经交给状态机执行的位置

`commitIndex` 和 `lastApplied` 故意不落盘，重启后从快照位置重新推进；状态机重放日志是幂等的，重放不会改变结果。

节点启动时从 `Storage` 读回任期、投票、日志和快照。`NewRaft` 只负责恢复和拉起两个协程：`ticker` 管选举超时，`applier` 管提交投递。

## 选举

每个节点每个任期只能投一票，拿到过半票才能当 leader。为了避免多个节点同时竞选把票打散，等待时间是随机的：

```go
timeout := rf.cfg.ElectionMin
if gap := rf.cfg.ElectionMax - rf.cfg.ElectionMin; gap > 0 {
	timeout += time.Duration(rf.rand.Int63n(int64(gap)))
}
rf.electionDeadline = time.Now().Add(timeout)
```

默认心跳 100ms、选举 300–600ms。心跳显著小于选举超时，正常运行时 follower 不会误判 leader 失联；真失联了，最先超时的那个先拉票，随机区间让它大概率独占这一轮。

拉票时带上自己最后一条日志的任期和下标，投票方据此判断"日志是否至少和我一样新"：

```go
upToDate := args.LastLogTerm > lastTerm ||
	(args.LastLogTerm == lastTerm && args.LastLogIndex >= lastIndex)

if upToDate && (rf.votedFor == -1 || rf.votedFor == args.CandidateID) {
	rf.votedFor = args.CandidateID
	rf.resetElectionDeadline()
	rf.persist()
	reply.VoteGranted = true
}
```

先 `persist` 再回复是关键：如果先回包再落盘，进程在两步之间崩溃重启，同一个任期就可能投出两票，直接违反选举安全。

成为 leader 后不会单独发通知，而是靠心跳里的 `LeaderID` 让 follower 知道谁是 leader。`leaderID` 只用于观察和客户端寻址，不参与任何正确性判断。

## 日志

每条日志只有任期和命令两部分：

```go
type LogEntry struct {
	Term    int
	Command []byte
}
```

命令是 `[]byte`，Raft 不解析内容——KV 服务自己把 `Op` 编码成 JSON 塞进去。这层解耦是 Raft 能复用到别的状态机的前提。

`log[0]` 不是一条真实命令，而是快照边界，所以真实下标要加偏移：

```go
func (rf *Raft) lastLogIndex() int {
	return rf.snapshotIndex + len(rf.log) - 1
}

func (rf *Raft) logTerm(index int) int {
	if index < rf.snapshotIndex || index > rf.lastLogIndex() {
		return -1
	}
	return rf.log[index-rf.snapshotIndex].Term
}
```

### 追加与复制

leader 收到命令先追加本地日志，再叫醒复制协程，不是每条命令起一个 goroutine：

```go
rf.log = append(rf.log, LogEntry{Term: rf.currentTerm, Command: command})
rf.persist()
rf.signalReplication()
```

每个 follower 对应一个长期运行的复制协程，维护两个数：`nextIndex` 是下次要发的起始下标，`matchIndex` 是已确认它拥有的最高下标。有新日志立刻发，没有就按心跳间隔发一个空的 `AppendEntries`——心跳和日志复制复用同一个 RPC，少一套消息类型。

### 冲突回退

follower 先校验 `PrevLogIndex` / `PrevLogTerm`，对不上就返回冲突任期和该任期第一次出现的下标：

```go
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
```

leader 拿到 `ConflictTerm` 后在自己日志里找该任期的最后一条，把 `nextIndex` 一次退到它后面；自己也没有这个任期，就直接用 `ConflictIndex`。逐条回退的话，差几百条就要几百个 RPC，这样一次就够。

截断前还要判断 `idx <= commitIndex`：已提交的日志不能被覆盖。真出现说明算法已经错了，直接拒绝这次追加。

### 提交规则

提交位置不能简单取"多数派都有的最小下标"，得从各节点的 `matchIndex` 里取多数派那一档：

```go
matches = append(matches, rf.matchIndex[i])
sort.Sort(sort.Reverse(sort.IntSlice(matches)))

quorum := len(matches)/2 + 1
candidate := matches[quorum-1]
if candidate > rf.commitIndex && rf.logTerm(candidate) == rf.currentTerm {
	rf.commitIndex = candidate
	rf.signalApply()
}
```

排序后第 quorum 大的值，就是"过半数节点都持有"的最高下标。后面那个 `logTerm(candidate) == rf.currentTerm` 是 Raft 5.4.2 的要求：只能直接提交当前任期的日志，否则可能把已经被新 leader 覆盖的旧日志误提交。旧任期的日志不单独提交，会跟着当前任期的日志一起被带上去。

follower 这边用 `min(LeaderCommit, lastNew)` 限制提交位置——leader 说的提交位置可能超过自己刚追加到的位置，不能盲信。

## 快照

日志不能无限增长。节点重启要重放全部日志，落后的 follower 也要从第一条开始追，两者都会被日志长度拖死。快照的作用是把"某个下标之前的日志"压成一份状态机数据。

这里要澄清一个常见误解：快照存的是**状态机数据加边界下标**，不是日志本身。生成快照时真正发生的是"日志被裁掉"：

```go
rf.log = append([]LogEntry{{Term: term}}, tail...)
rf.snapshotIndex = index
rf.snapshotTerm = term
rf.snapshotData = append([]byte(nil), data...)
```

`log[0]` 保留为边界哨兵，后面只留 `index` 之后的日志。释放的是日志空间，保留的是状态。

快照模块对外三个方法：

```go
func (rf *Raft) ReadSnapshot() (int, []byte)
func (rf *Raft) Snapshot(index int, data []byte) bool
func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) error
```

`Snapshot` 由上层状态机调用：它保证 `data` 恰好等于执行到 `index` 时的状态，Raft 只做裁剪和落盘。`ReadSnapshot` 在节点启动时把快照交给状态机恢复。`InstallSnapshot` 处理 leader 推来的快照。

触发时机也由上层决定：KV 每次应用后看一眼日志字节数，超过 `maxRaftState` 就打快照。Raft 本身不知道"多大算大"，这个策略留给应用。

`InstallSnapshot` 真正的用武之地是 follower 落后太多：leader 发现 `nextIndex` 已经退到 `snapshotIndex` 之前，要发的日志自己都裁掉了，只能整体发快照。follower 收到后丢弃本地日志，把 `commitIndex` 推到 `LastIncludedIndex`，再通过 `ApplyCh` 投一条 `SnapshotValid` 的消息，让状态机整体替换。

## 键值状态机

Raft 在这里像中间件：leader 收到请求，等它在过半节点落盘后提交，follower 的 Raft 再把已提交的命令交给 KV 状态机执行，执行完才应答客户端。

状态机只做三件事——`Put`、`Append`、`Get`。它通过 `ApplyCh` 接收两类消息：

- `CommandValid`：一条已提交的命令，按 `CommandIndex` 顺序执行
- `SnapshotValid`：一份快照，整体替换当前状态

写请求要防重。客户端超时重试可能让同一条命令进两次日志，所以命令里带 `(Client, Seq)`：

```go
if op.Seq <= s.lastSeq[op.Client] {
	// 重复请求：直接返回上次的结果。
	res.value = s.lastReply[op.Client]
}
```

状态机记住每个客户端最后执行的序号和返回值，重复的命令不执行，只把上次的结果回给 `propose` 的等待方。这样"至少一次"的 RPC 在语义上变成"恰好一次"。

读也走日志。这看起来浪费——一次 `Get` 也要写磁盘、也要多数派确认——但换来的是线性一致：读请求排在写请求后面提交，读到的一定是最新已提交的值。想优化可以上 ReadIndex 或 lease read，本项目没做。

`propose` 提交后轮询 `results[index]` 等结果，同时盯着 `GetState()`：如果任期变了或自己不再是 leader，立刻返回失败让客户端换节点，而不是傻等到超时。

## 持久化

需要落盘的只有四项：`currentTerm`、`votedFor`、日志、快照。任期和投票决定"会不会投两票"，日志和快照决定"提交过的会不会丢"，少一样都不安全。

文件实现走的是临时文件 + fsync + 原子重命名：

```go
tmp := f.path + ".tmp"
file, _ := os.Create(tmp)
file.Write(data)
file.Sync()
file.Close()
os.Rename(tmp, f.path)
```

`Rename` 在同一个文件系统内是原子的，读到的要么是旧文件、要么是新文件，不会是写了一半的 JSON。

约定是"先落盘再回复 RPC"：凡是改变任期、投票、日志、快照的路径，都先 `persist()` 再构造响应。`persist` 失败直接 panic——落不了盘的 Raft 继续跑下去只会破坏安全性，不如当场停下。

## 客户端

Clerk 记住上一次成功的节点，失败就按 `(leader + i) % len(servers)` 依次换下一个，成功后把这个节点记成新的起点。因为 leader 会换人，硬绑一个节点没有意义。

每次重试都用同一个 `Seq`，服务端据此去重。这个组合让客户端可以无脑重试，不必区分"请求没到"和"回复丢了"。

## 测试

17 个用例覆盖协议的关键路径：

- 选举：初次选主、任期稳定、leader 崩溃后重选、配置校验
- 提交：基本达成一致、少数派不能提交、并发提交下标唯一、集群整体重启后任期与日志不丢
- 故障：五节点分区恢复、落后 follower 通过 `InstallSnapshot` 追上、快照持久化与日志压缩
- 存储：内存与文件实现的往返、真实 TCP 集群复制
- 应用：KV 并发追加、leader 切换、快照重启

```bash
go test ./... -count=2
go test -race ./...
```

`-race` 是必须跑的：Raft 的坑大多不在逻辑，而在"读字段忘了加锁""协程退出后还在写"。除模拟断链和分区，测试里还会真起三个进程做冒烟：`put`、`append`、杀掉 leader、重启节点追平日志。

## 简化边界

为了保持"最简"，下面这些都没做：

- 成员变更（joint consensus）：节点列表启动时固定
- PreVote：分区恢复的节点会先涨任期，可能打断现有 leader
- ReadIndex / lease read：读也走日志，代价是延迟
- 快照分块传输：整个快照一条 RPC，快照大了会卡
- 日志批量落盘：每条命令一次 fsync，吞吐有限

清楚边界在哪、为什么不做，比"实现了但说不清"更有价值。

## 参考文献

- Diego Ongaro, John Ousterhout. [In Search of an Understandable Consensus Algorithm](https://raft.github.io/raft.pdf). USENIX ATC 2014.
- [6.824: Distributed Systems](https://pdos.csail.mit.edu/6.824/)

## 项目源码

- [Raft 项目源码](https://github.com/222twotwotwo/222twotwotwo/tree/main/6.824/raft)
