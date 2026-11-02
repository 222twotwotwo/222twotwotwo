# Mini MapReduce (Go)

![Go logo](../../../assets/images/logos/go-logo.svg)

用 Go 标准库实现的 MapReduce 教学项目：一个 Coordinator 调度多个独立 Worker 进程，
把输入文件转换为中间键值对，再按 key 分区、排序、归并。内置 Word Count，可替换 Map/Reduce 函数。
这是独立实现，没有接入 MIT 官方实验框架或测试脚本。

## 架构

```text
                    TCP RPC: request / report
         +-------------------------------------------+
         |                                           v
      Worker 1 <------------------------------- Coordinator
      Worker 2 <--------------------------------------|
      Worker N <--------------------------------------+
         |
         +-- Map: input -> hash(key) % NReduce -> JSON partitions
         +-- Reduce: read committed partitions -> sort/group -> mr-out-N
```

- **Coordinator**：记录任务状态、分配任务、重派超时任务、验收输出，全部 Map 完成后才发 Reduce。
- **Worker**：通过 RPC 领取任务，执行用户函数，将中间数据与结果写入共享目录。
- **分区**：同一个 key 通过 FNV-1a 哈希进入同一个 Reduce，保证同 key 的所有值被一起归并。
- **重试**：每次分配增加 `Attempt`。不同尝试使用不同文件名，迟到的 Worker 无法覆盖新尝试。
- **结果发布**：Worker 先写临时文件，关闭后重命名；最终 `mr-out-N` 由 Coordinator 验收当前尝试后发布。

## 运行

需要 Go 1.22 或更新版本，无第三方依赖。以下命令均在 `6.824/mapreduce` 目录执行。

```powershell
go build -o mapreduce.exe ./cmd/mapreduce
```

终端 1 启动协调器，`-out` 必须是尚不存在的目录。程序会自行展开输入文件通配符：

```powershell
.\mapreduce.exe coordinator -addr 127.0.0.1:9000 -reduce 3 -out data/run1 -timeout 10s examples/*.txt
```

终端 2、3 各启动一个 Worker，数量可自行增加：

```powershell
.\mapreduce.exe worker -coordinator 127.0.0.1:9000
```

所有任务完成后，Worker 自动退出；Coordinator 打印 `job complete` 并继续接收请求，
让仍在运行或稍晚启动的 Worker 能领取退出指令。确认 Worker 退出后，在终端 1 按 `Ctrl+C` 停止。

```powershell
Get-Content data/run1/mr-out-* | Sort-Object
```

预期结果：

```text
hello 3
map 2
reduce 2
world 3
```

每个分区内部按 key 排序；合并展示时再做全局排序。重复运行请换一个输出目录，例如 `data/run2`。

Linux/macOS 使用 `go build -o mapreduce ./cmd/mapreduce`，将 `.\mapreduce.exe` 换成 `./mapreduce`，
用 `cat data/run1/mr-out-* | sort` 查看结果。TCP RPC 不依赖 Unix socket 或 Go plugin。

## 容错与一致性

任务状态为 `idle -> running -> done`：

1. Worker 领取任务时，Coordinator 记录开始时间和新的尝试编号。
2. 默认超过 10 秒仍未完成的任务，会在下次领取请求时重新分配。
3. Worker 发现读写错误，会报告失败并退出；任务立即回到可分配状态。
4. Coordinator 只接收当前尝试的结果。重复成功报告具有幂等性，过期报告被拒绝。
5. Reduce 只读取已提交的 Map 尝试，全部 Reduce 提交成功才算整个作业完成。

Worker 进程退出后，只要还有 Worker 继续领取任务，作业就能继续推进。
慢任务也可能触发重试，因此 Map/Reduce 函数应是确定性的，并避免写外部系统等副作用；
超时设置应大于正常任务的运行时间。RPC 报告失败时，Worker 保留输出，以免误删已提交数据。

## 自定义任务

库函数 `Worker(ctx, address, mapf, reducef)` 接收：

```go
type MapFunc func(filename, contents string) []KeyValue
type ReduceFunc func(key string, values []string) string
```

命令行入口把 `WordCountMap` 和 `WordCountReduce` 传入 Worker。实现其他任务时，替换这两个参数，
并让同一作业的所有 Worker 使用一致的函数。Word Count 以连续 Unicode 字母为词，区分大小写。

## 文件

| 文件 | 职责 |
| --- | --- |
| `rpc.go` | 键值对、任务协议与尝试文件名 |
| `coordinator.go` | 任务调度、阶段屏障、超时重试、提交验收、RPC 服务 |
| `worker.go` | RPC 调用、Map 分区、Reduce 排序归并、临时文件写入 |
| `wordcount.go` | Word Count 示例函数 |
| `cmd/mapreduce/main.go` | Coordinator / Worker 命令行入口 |
| `examples/` | 两个可直接运行的输入文件 |
| `mapreduce_test.go` | 并发、重试、迟到提交与错误处理测试 |

## 测试

```powershell
go test ./...
go vet ./...
go test -race ./...
```

Windows 上 race 检查需要可用的 C 编译器。测试使用真实 TCP RPC，覆盖并行 Worker、词频正确性、
空输入、Map/Reduce 阶段屏障、Worker 消失后重派、迟到结果隔离、重复提交、
读文件失败后恢复、非法参数、中间文件损坏、失败写入清理和等待取消。

## 简化边界

- 通过同机多进程模拟分布式执行；Coordinator 与 Worker 必须共享同一文件系统和绝对路径。
  未接入相邻的 Mini GFS，也没有远程 shuffle 服务。
- Coordinator 元数据仅在内存中，不支持 Coordinator 故障恢复。
- 输入文件在任务运行期间必须保持不变；已提交的中间文件需要保留至作业结束。
- 单个 Map 的输入与单个 Reduce 的全部记录放入内存，没有外部排序、Combiner 或数据本地性调度。
- 任务为至少一次执行；尝试编号保护框架输出，但不能保证用户函数的外部副作用只发生一次。
- Worker 遇到 RPC 或执行错误会退出并报错，需要其他 Worker 继续处理或手动重启。
  永久性输入损坏需要先修复；没有自动重试上限、作业失败状态或心跳续租。
- 中间文件与崩溃留下的临时文件不自动回收，作业结束后可自行删除整个输出目录。
- 结果格式为 `key value` 每行一条，适合词频示例；自定义函数应避免在输出 key/value 中嵌入换行。
- 仅面向可信本地节点，没有身份认证或加密。
