PowerContext 为跨会话协作保存上下文，Memory 是其中承载长期信息的一部分。我的贡献集中在两个边界：记忆持续增长时如何控制容量，以及 Windows 下的中文等非 ASCII 输入如何可靠地进入服务。

以下根据 PR 说明、最终代码变更和其中记录的验证结果整理。

## PR #1746：为长期记忆建立可观测的容量边界

已合并 · [原始 PR](https://github.com/oceanbase/powercontext/pull/1746) · [代码变更](https://github.com/oceanbase/powercontext/pull/1746/files)

### 背景

Memory 会随使用不断追加、遗忘和整理条目。此前的优化减少了投影写放大，但每个 Revision 仍然保存完整 manifest，也就是当前版本的条目清单。被遗忘的条目以 tombstone 的形式留在清单里，仍然占用空间。

### 问题

写入没有明确的条目数和内容大小上限，调用方也无法提前查询剩余容量。即使活跃条目不多，长期积累的 tombstone 仍会使 manifest 增长。另一个相关问题是历史查询可能一次展开过多 Revision，扩大读取成本。

### 解决方式

- 在 Memory 写入和通用 Artifact 管理入口统一检查三类预算：活跃条目数、manifest 条目数、规范化 Revision 内容字节数。默认值分别为 5,000、10,000 和 4 MiB。
- 对继续扩大超限维度的写入返回 `409 memory_capacity_exceeded`，说明维度、限制和实际值；拒绝操作时不推进 head，也不留下部分写入。
- 增加 `POST /v1/memory/capacity` 以及运行时、Python 客户端和生成接口契约，让调用方可以查看容量。
- 提供显式调用的 `compact()`：支持预演、批量上限、恢复窗口和标签保护，清除当前 manifest 中符合条件的 tombstone，同时保留条目正文、历史版本和引用。
- 限制历史展开规模，默认最多展开 100 个 Revision，并将范围限制传递到后端查询。

例如，manifest 只允许一个条目时，第二次追加会明确失败；调用方可以查看容量并选择整理，而不是面对持续膨胀的版本内容。超限时，`forget()` 等不扩大受限维度的操作仍可用于恢复。

### 成果

长期 Memory 获得了可查询、可拒绝、可恢复的容量管理行为。回归测试覆盖拒绝写入不改变状态、压缩后的历史引用、标签保护以及历史读取边界。

PR 中记录的集中测试为 80 项通过、13 项因需要 OceanBase 连接而跳过；另一次本地 OceanBase 验证中，这 13 项全部通过。SQLite 与 OceanBase 的规模验证覆盖 200、1,000、5,000 条目，压缩阶段移除了 4,000 个 tombstone，写入零条投影记录，并保留了用于核对的搜索结果标识。

这里回收的是当前 manifest 的容量，保留的历史数据仍然占用存储；它并不等同于物理删除。压缩默认关闭，需要显式启用和调用。

## PR #1326：修复 Windows Hook 的 Unicode 输入与错误响应

已合并 · [原始 PR](https://github.com/oceanbase/powercontext/pull/1326) · [代码变更](https://github.com/oceanbase/powercontext/pull/1326/files)

### 背景

Claude Code 和 Codex 的 Hook 通过标准输入接收 JSON，再将用户提示词交给 PowerContext。对于包含中文的提示词，从字节流到 JSON 的编码必须一致。

### 问题

Windows 上的脚本可能按本地控制台编码读取 UTF-8 输入，导致非 ASCII 文本损坏，甚至产生孤立代理字符。服务端验证失败后又把原始非法输入放进错误详情，构造本应返回的 422 响应时可能再次触发编码错误。

### 解决方式

在两个 Hook 中改为从 `sys.stdin.buffer` 读取原始字节，显式以 UTF-8 解码后再解析 JSON。服务端的 `RequestValidationError` 处理器移除错误详情中的原始 `input` 字段，保留错误类型和位置等信息。

同时新增两端的回归测试：Hook 测试检查 Windows 输入处理，服务端测试覆盖含代理字符的非法请求与不回显原始输入的 422 响应。

### 成果

修复同时覆盖了输入链路和错误返回链路：非 ASCII 提示词按统一编码传递，非法请求仍返回 422，错误响应不再因为回显损坏文本而失败。PR 记录了 Hook 与服务端相关测试命令，变更随后合并。
