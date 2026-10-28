---
title: 实现最简分布式存储系统
date: 2026-10-28
category: Go 后端
tags: Go, 分布式存储, GFS, 副本, Master, ChunkServer
readTime: 7 分钟阅读
summary: 用 Go 标准库写一个最简 GFS 风格的教学项目：Master 只存元数据，ChunkServer 存块，Client 自己切块、写满全部副本再提交。围绕可用性、可扩展性和一致性这三个取舍，记录四个 handler 的分工、覆盖写的逻辑删除，以及还没有实现的 Master 高可用。
cover: ../images/logos/go-logo.svg
---

读完 GFS 论文之后，我最想搞清楚的不是它的吞吐有多高，而是一个更朴素的问题：把元数据和数据分开，到底改变了什么。

所以我照着这个思路，用 Go 标准库实现了一个最简的 GFS 风格存储系统：一个 Master、若干 ChunkServer、一个命令行客户端。它不是 MIT 6.824 官方实验的答案，也不是完整的 Google File System，只是把最关键的三个机制——元数据与数据分离、分块、副本——摆在一份能读完的代码里。

## 架构

![Mini GFS 架构：Client 先向 Master 申请块与副本位置，再直接与 ChunkServer 读写整块数据](../images/logos/gfs-architecture-inline.svg)

系统里只有三类角色，其中 ChunkServer 可以部署任意多个：

- **Client**：先向 Master 请求元数据，拿到 chunk 位置后，直接和 ChunkServer 读写数据。
- **Master**：控制平面，只保存文件名、大小、chunk ID 和副本位置。
- **ChunkServer**：数据平面，真正存放 chunk 数据，每个节点只负责一部分 chunk。

关键在于数据不经过 Master：控制流和数据流分开，Master 才不会成为带宽瓶颈。同一个 chunk 会按副本数写到不同的 ChunkServer 上，位置按 `(i+j) % len(servers)` 轮转取，相邻 chunk 的副本不会挤在同一台机器上。

## 环境

- **编译器**：Go 1.22
- **操作系统**：Windows 11

本次用多进程模拟多主机环境，通信协议使用 HTTP，不依赖任何第三方库。

## 三个取舍

接下来是本系统在分布式上的几个重要取舍：

- 可用性（容错）
- 可扩展性（可以通过增加节点来提升性能或存储容量）
- 一致性（数据的一致性）

这三个目标互相拉扯，任何实现都只能做选择，这个项目也一样。

### 可用性

ChunkServer 采用多节点部署，每个节点负责存储部分 chunk，同一个 chunk 按副本数存多份。当一个节点故障时，读请求可以退到其他副本继续服务，整体仍然可用。

不过这里的"可用"有明确边界，值得说清楚：读路径有副本回退，一个节点挂了还能从另一个副本读到数据；但写路径要求全部副本成功才 commit，所以只要有任意一个副本不在线，写入就会直接失败。这是本实现主动接受的代价——它换来的是副本间的强一致，而不是写入侧的高可用。

**以下是可扩展内容（本次没有实现）**

- **Master 节点**：采用 active-standby 模式，只有一个节点（Active）对外提供读写服务；另一个节点（Standby）处于待命状态，不承接业务流量，只同步数据。

流程：

1. 正常运行：所有业务请求打到 Active；Standby 只持续同步数据，不处理业务读写，资源闲置。
2. 故障检测：当 Active 宕机或失联，调度器（VIP、keepalived、etcd、控制器）感知故障。
3. 故障切换（Failover）：把流量切到 Standby，Standby 升级为新的 Active，对外提供服务。
4. 旧主恢复后：一般变成新的 Standby，也有少数方案会切回去，称为回切（Failback）。
5. 风险：脑裂，可能同时存在多个 Active 在活跃。

### 可扩展性

ChunkServer 采用水平扩展，增加 ChunkServer 节点即可提升存储容量。性能方面本项目没有提升：写入是逐块、逐副本串行发起的，加机器主要加的是容量，不是吞吐。

### 一致性

- **副本间强一致**：Put 要求全部副本写成功才 commit，缺任何一个就报错，因此读任意副本拿到的数据都相同，不存在"主写成功、从没写成功"的中间态。
- **提交后才可见**：commit 把文件从 pending 移入 files 之后才对外可查，两张表都由 `sync.RWMutex` 保护，lookup 不会读到半状态。

## 实现细节

### Master

```go
type Master struct {
	mu        sync.RWMutex
	servers   []string
	chunkSize int
	replicas  int
	files     map[string]File
	pending   map[string]File
}
```

`NewMaster` 接收服务器列表、块大小、副本数三个参数，其中块大小和副本数由命令行 `flag` 传入：`chunkSize` 可配置分片大小，默认 1024 KB（1 MiB）；`replicas` 可配置副本数，默认 2。构造时会校验块大小范围、副本数不超过服务器数量，以及地址合法且不重复。

Master 对外提供四个 handler：

| 方法 | 路径 | 作用 |
| --- | --- | --- |
| POST | `/allocate` | 分配 chunk 位置，把文件写入 pending |
| POST | `/commit` | 提交文件，把文件从 pending 移入 files |
| GET | `/file` | 查找单个文件的元数据 |
| GET | `/files` | 列出所有已提交的文件 |

`allocate` 是唯一有点讲究的地方：它生成一个 32 位十六进制的 token，chunk ID 就是 `token-序号`；副本位置按 `(i+j) % len(servers)` 轮转取，保证相邻 chunk 的副本不会落在同一台机器上。元数据先进 pending，等 Client 把所有副本写完再 commit，中间任何一步失败都不会污染 files。另外每个文件最多 10000 个 chunk，这个上限是独立于客户端声明的大小做的保护。

### ChunkServer

ChunkServer 对 Client 提供块的读写，路径是 `/chunks/{id}`：GET 直接返回文件，PUT 则是先写临时文件，`Sync`、`Close` 之后再 `Rename` 成正式名字。这个顺序保证了磁盘上要么是完整的块，要么是旧块，不会出现半截文件。chunk ID 还会用正则校验一遍，防止路径穿越。

覆盖写走的也是这条路，但语义是逻辑删除而不是物理删除：新的 put 会拿到全新的 token 和 chunk ID，旧块不会被改写，也不会被删除。好处是 put 失败时旧文件依然可读，代价是旧块需要另外回收——本项目没有做这部分。

### Client

Client 是用户读写文件的入口，逻辑最短，也最能说明数据流：先向 Master 请求元数据，再直接向 ChunkServer 读写数据。

- `put`：先 allocate，再逐块、逐副本 PUT 到 ChunkServer，全部成功后调用 commit。
- `get`：先 lookup 拿元数据，再逐块读取；每块的长度必须等于期望值，长度不符就换下一个副本，所有副本都不可用才报错。
- `ls`：直接取 Master 的文件列表。

## 还没做的部分

除了 Master 高可用，这个实现还省掉了很多东西：元数据只在内存里，重启就丢；节点地址静态配置，没有心跳和自动补副本；只支持整文件读写，没有追加、随机写和删除；没有校验和，只能靠长度发现截断；旧版本块和失败上传的块不会回收。

这些限制换来的是可读性：整套系统四百多行，刚好够把"元数据与数据分离、分块、副本"这三件事讲明白。对一个用来理解分布式存储的骨架来说，这个交换是值得的。
