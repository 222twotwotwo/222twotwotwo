# Mini GFS (Go)

用 Go 标准库实现的最简 GFS 教学项目，演示 Master、ChunkServer 和客户端如何协作。
这不是 MIT 6.824 官方实验的实现，也不是完整的 Google File System。

## 架构

```text
              allocate / commit / lookup
Client ------------------------------------> Master
  |
  | PUT / GET chunks (data does not pass through Master)
  +----------------------------------------> ChunkServer 1 -> disk
  +----------------------------------------> ChunkServer 2 -> disk
```

- Master：维护文件名、大小、块 ID 和副本位置，使用互斥锁保护元数据。
- ChunkServer：每个块保存为一个磁盘文件，临时文件写入成功后再重命名。
- Client：文件分块、写入所有副本、顺序拼接读取；副本不可用或长度不匹配时尝试下一个副本。
- 默认块大小为 1 MiB，每块两个副本，按轮转方式分配服务器。

## 运行

需要 Go 1.22 或更新版本，无第三方依赖。以下为 Windows PowerShell 命令。
在 `6.824` 目录编译：

```powershell
go build -o gfs.exe ./cmd/gfs
```

打开三个终端，各自在 `6.824` 目录运行一条命令：

```powershell
# Terminal 1
.\gfs.exe chunkserver -addr 127.0.0.1:8001 -dir data/chunk1
```

```powershell
# Terminal 2
.\gfs.exe chunkserver -addr 127.0.0.1:8002 -dir data/chunk2
```

```powershell
# Terminal 3
.\gfs.exe master -addr 127.0.0.1:8000 -servers http://127.0.0.1:8001,http://127.0.0.1:8002 -replicas 2 -chunk-size 1048576
```

第四个终端上传、列出并下载文件：

```powershell
.\gfs.exe put .\README.md /docs/readme.md
.\gfs.exe ls
.\gfs.exe get /docs/readme.md .\data\download.md
Get-FileHash .\README.md, .\data\download.md
```

两个 SHA256 应一致。停止任意一个 ChunkServer 后，再执行 `get` 仍应成功。
两个副本都在线时才能写入非空文件。各服务按 `Ctrl+C` 停止。

指定其他 Master 时，将参数放在文件名之前：

```powershell
.\gfs.exe ls -master http://127.0.0.1:9000
.\gfs.exe put -master http://127.0.0.1:9000 .\README.md /readme.md
```

Linux/macOS 使用 `go build -o gfs ./cmd/gfs`，将命令中的 `.\gfs.exe` 换成 `./gfs`，使用对应的路径分隔符即可。

## 读写流程

1. 客户端向 Master 申请写入，Master 根据文件大小生成新的块 ID 和副本位置。
2. 客户端直接向每个 ChunkServer 写入对应块。
3. 所有块的全部副本写入成功后，客户端请求 Master 提交元数据，文件才对读者可见。
4. 读取时，客户端先查询 Master，再直接读取各块并按顺序拼接。

覆盖文件会使用全新的块 ID，失败时旧文件仍然可读。并发覆盖以最后一次成功提交为准，
已获取旧元数据的读者可以继续读取旧块。提交请求网络中断时，客户端可能收到错误但提交实际已成功，
需要重新读取确认结果。

文件名是完整的逻辑名称；`/docs/readme.md` 不会在 Master 上创建真实目录。

## 测试

```powershell
go test ./...
go vet ./...
# Requires a working C compiler on Windows:
go test -race ./...
```

测试启动真实的本地 HTTP 服务，覆盖空文件、二进制数据、跨块读写、副本落盘、覆盖写入、
副本故障、失败写入不可见、截断副本回退、并发操作、非法请求和中断上传清理。

## 文件

| 文件 | 职责 |
| --- | --- |
| `protocol.go` | 文件与块的元数据结构 |
| `master.go` | 分配块、提交元数据、查询和列表 |
| `chunkserver.go` | 块的磁盘读写 |
| `client.go` | 客户端分块、复制和读取 |
| `cmd/gfs/main.go` | 命令行入口 |
| `gfs_test.go` | 集成测试 |

## 简化边界

- Master 元数据仅存在内存中，重启后文件索引丢失；ChunkServer 的块保留在磁盘上，但不能自动重建索引。
- 节点地址静态配置，没有心跳、自动补副本、Master 高可用或元数据持久化。
- 只支持整文件写入、读取和列表，没有追加、随机写、删除、租约或 primary 写入流水线。
- 文件内容整体放入客户端内存；每块最大 64 MiB，每个文件最多 10000 块。
- 旧版本块、失败上传的块和未提交记录不会自动回收。
- 没有校验和，仅检查读取长度，无法发现等长内容损坏。
- 只面向可信本地客户端，没有认证；Master 信任客户端在副本写完后提交。

这些限制使代码保持可读，重点展示 GFS 的元数据与数据分离、分块和副本机制。
