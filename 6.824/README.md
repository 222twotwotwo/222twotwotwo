# 6.824 分布式系统学习项目

各项目使用独立 Go 模块，均只依赖标准库，需要 Go 1.22 或更新版本。

| 目录 | 内容 | 运行说明 |
| --- | --- | --- |
| `gfs/` | Mini GFS：元数据与数据分离、分块存储、多副本读写 | [GFS README](gfs/README.md) |
| `mapreduce/` | Mini MapReduce：任务调度、并行 Map/Reduce、分区归并、超时重试 | [MapReduce README](mapreduce/README.md) |

原先直接放在本目录的 GFS 源码、笔记、运行数据和本地可执行文件已整体移入 `gfs/`，模块名保持不变。
这些项目用于学习分布式系统，不是 MIT 官方实验仓库或完整生产实现。

在 `6.824` 目录统一检查：

```powershell
go test ./gfs/... ./mapreduce/...
go vet ./gfs/... ./mapreduce/...
go test -race ./gfs/... ./mapreduce/...
```

Windows 的 race 检查需要可用的 C 编译器。也可以进入各项目目录运行 `go test ./...`。
