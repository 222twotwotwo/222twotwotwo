# Mini Raft (Go)

![Go logo](../../assets/images/logos/go-logo.svg)

用 Go 标准库实现的 Raft 教学项目：三到五个节点选主、复制日志、持久化状态、压缩日志做快照，
上面再搭一个线性一致的键值服务。代码只依赖标准库，不接入 MIT 官方实验框架，
也不是可以直接上生产的一致性库。

## 架构

```text
           客户端（Clerk，TCP RPC: KV.Get / KV.PutAppend）
                        |
        +---------------+---------------+
        v               v               v
   +---------+     +---------+     +---------+
   | 节点 0  |<--->| 节点 1  |<--->| 节点 2  |      AppendEntries / RequestVote
   |---------|     |---------|     |---------|      InstallSnapshot
   | KV 状态机|<----| KV 状态机|<----| KV 状态机|      <- ApplyCh（已提交日志）
   | Raft    |     | Raft    |     | Raft    |
   | 磁盘日志 |     | 磁盘日志 |     | 磁盘日志 |
   +---------+     +---------+     +---------+
```

- **Raft**：唯一决定日志顺序的组件。任期、投票、日志、快照全部先落盘再回复 RPC。
- **KV 状态机**：消费 `ApplyCh`，按提交顺序执行命令；所有读也走日志，所以天然线性一致。
- **传输层**：生产用 TCP 上的 `net/rpc`；测试和 `demo` 用进程内的假网络，可以随意断链、分区。

## 运行

需要 Go 1.22 或更新版本。以下命令均在 `6.824/raft` 目录执行。

```powershell
go build -o raft.exe ./cmd/raft
```

先看进程内的三节点演示，它会选主、读写、杀掉 leader 再看重新选主：

```powershell
.\raft.exe demo
```

真实多进程集群需要三个终端，每个节点一个数据目录。`-peers` 必须包含自己，
节点编号从 0 开始连续：

```powershell
# 终端 1
.\raft.exe server -id 0 -addr 127.0.0.1:9001 -peers 0=127.0.0.1:9001,1=127.0.0.1:9002,2=127.0.0.1:9003 -data data/n0
```

```powershell
# 终端 2
.\raft.exe server -id 1 -addr 127.0.0.1:9002 -peers 0=127.0.0.1:9001,1=127.0.0.1:9002,2=127.0.0.1:9003 -data data/n1
```

```powershell
# 终端 3
.\raft.exe server -id 2 -addr 127.0.0.1:9003 -peers 0=127.0.0.1:9001,1=127.0.0.1:9002,2=127.0.0.1:9003 -data data/n2
```

第四个终端读写和观察状态：

```powershell
.\raft.exe put    -servers 127.0.0.1:9001,127.0.0.1:9002,127.0.0.1:9003 -key greeting -value hello
.\raft.exe append -servers 127.0.0.1:9001,127.0.0.1:9002,127.0.0.1:9003 -key greeting -value "-raft"
.\raft.exe get    -servers 127.0.0.1:9001,127.0.0.1:9002,127.0.0.1:9003 -key greeting
.\raft.exe status -servers 127.0.0.1:9001,127.0.0.1:9002,127.0.0.1:9003
```

`get` 输出 `hello-raft`；`status` 会打印每个节点的角色、任期、已知 leader、
最后日志下标、提交下标、已应用下标和快照下标。任意关掉一个节点，读写仍然可用；
关掉两个就只剩少数派，`put` 会一直重试，直到节点重新上线。

Linux/macOS 把 `.\raft.exe` 换成 `./raft`、路径分隔符换成 `/` 即可。

## 协议要点

**选主。** 每个节点在 `[ElectionMin, ElectionMax]` 内随机一个选举超时，超时后任期加一、
投自己一票，并发向其他节点拉票。候选人必须拿到严格多数票（`votes*2 > N`）才成为 leader。
投票方只在“候选人日志不比自己旧”且“本任期没投给别人”时同意，这条规则保证新 leader
一定包含所有已提交日志。

**日志复制。** leader 为每个 follower 维护 `nextIndex` 和 `matchIndex`，心跳也走
`AppendEntries`（`Entries` 为空）。follower 先校验 `PrevLogIndex/PrevLogTerm`，匹配则
追加并截断冲突的旧日志，不匹配则返回冲突任期，让 leader 一次把 `nextIndex` 退到正确位置，
不用逐条试探。

**提交规则。** leader 只提交**当前任期**的日志：把各节点 `matchIndex` 排序，
第 `N/2+1` 大的值就是多数派都持有的最高下标，再检查它的任期是否等于当前任期。
旧任期的日志通过“随新任期日志一起提交”的方式补上，避免 Raft 论文 5.4.2 描述的
“提交了后来会被覆盖的日志”问题。follower 只按 leader 给出的 `LeaderCommit` 推进。

**持久化。** 每次任期、投票、日志变化都整体重写状态文件（临时文件 + fsync + 原子重命名）。
Raft 的安全性依赖于“回复 RPC 前状态已经落盘”，所以这里的顺序不能调换；
写盘失败直接 panic，因为带着不确定的磁盘状态继续运行会破坏一致性。

**快照。** 上层状态机在日志超过 `-maxraftstate` 时调用 `Snapshot(index, data)`，
Raft 丢掉 `index` 之前的日志，只保留边界项。落后太多的 follower 收到
`nextIndex <= snapshotIndex` 时改发 `InstallSnapshot`，一次把整份快照传过去。

**线性一致读与去重。** `Get` 也写一条日志，虽然多一次往返，但不用实现 ReadIndex 或租约读。
客户端每次操作带 `(Client, Seq)`，状态机记录每个客户端的最后一个 `Seq` 和回复；
重试导致同一条命令被提交两次时，第二次直接返回缓存结果，不会重复执行。

## 文件

| 文件 | 职责 |
| --- | --- |
| `types.go` | 日志项、应用消息、RPC 参数与状态快照结构 |
| `storage.go` | 持久化接口，内存实现与文件实现 |
| `transport.go` | TCP 传输，以及可断链、可分区的进程内假网络 |
| `raft.go` | 节点结构、启动恢复、`Start`、选举计时器、应用协程 |
| `election.go` | 发起选举、拉票、投票规则 |
| `replicate.go` | 日志复制循环、冲突回退、提交推进、`AppendEntries` |
| `snapshot.go` | 打快照、读取快照、`InstallSnapshot` |
| `kv.go` | 键值状态机、客户端会话去重、按日志长度触发快照 |
| `client.go` | 客户端重试与 leader 缓存 |
| `cmd/raft/main.go` | `server` / `get` / `put` / `append` / `status` / `demo` 入口 |
| `*_test.go` | 选举、复制、分区、持久化、快照、客户端并发测试 |

## 测试

```powershell
go test ./...
go vet ./...
go test -race ./...
```

Windows 上 race 检查需要可用的 C 编译器。测试用进程内假网络模拟节点崩溃、断链和网络分区，
客户端一侧走真实 TCP，覆盖：初次选主与任期稳定、leader 崩溃后重选、少数派无法提交、
并发提交拿到互不相同的下标、集群整体重启后状态不丢、五节点分区恢复、
落后 follower 通过 `InstallSnapshot` 追上、快照持久化、真实 TCP 下的复制，
以及键值服务的并发追加、leader 故障切换和快照重启。

## 简化边界

- 节点地址与编号静态配置，不支持成员变更（joint consensus 或单步变更都没做）。
- 日志整体重写成 JSON 文件，重启加载全量日志；没有分段、没有增量 fsync。
- 快照由上层状态机主动触发，传输是一次性全量拷贝，没有分块和断点续传。
- 客户端 `Get` 走日志，读吞吐受限于复制往返；也没做批量提交和流水线复制。
- 状态机把全部数据放在内存，只适合教学规模的数据量。
- 去重只缓存每个客户端的最后一条命令；同一个客户端乱序重放更早的请求会拿到最后一次的结果。
- 没有认证与加密，只面向可信本地网络。
- 与 MIT 6.824 实验的接口不同：这里用 `[]byte` 命令、显式 `Storage`/`Transport` 接口，
  不提供 `labrpc`、`config.go` 那套测试脚手架。
