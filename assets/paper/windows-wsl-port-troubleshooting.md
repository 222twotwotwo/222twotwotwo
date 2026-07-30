---
title: Windows + WSL 开发里的端口不可达：一次监控 Pull 模式排障
date: 2026-07-29
category: 网络排障
tags: Windows, WSL, Docker, 端口, Prometheus
readTime: 5 分钟阅读
summary: 记录一次在 Windows + WSL + Docker 开发环境中，业务调用正常但监控 Pull 模式抓取失败的端口排查过程。
cover: ../images/logos/git-icon.svg
---

这次测试的核心问题并不是业务服务没有跑起来，而是监控系统在 Pull 模式下抓不到应用暴露出来的 metrics endpoint。

为了方便复用，这里隐去具体项目、仓库和业务模块，只保留排障过程中真正有迁移价值的部分：Windows 宿主机、WSL 开发环境、Docker 容器、应用进程和端口之间的可达性。

## 测试背景

本地环境大致是这样的：

- 应用进程运行在 WSL 里。
- 监控组件运行在 Docker 容器里。
- 容器通过 `host.docker.internal:<port>` 回连宿主机或 WSL 暴露的端口。
- 应用需要暴露 `/prometheus` 之类的 metrics endpoint，供监控系统定时抓取。

业务链路本身是通的。客户端持续请求服务端，日志里可以看到正常响应。这说明问题不在注册、调用或业务端口上。

真正异常的是监控链路：监控系统的 target 一直是 `DOWN`，提示连接被拒绝。

## 第一层判断：业务正常不等于监控正常

一开始最容易混在一起的是两类端口：

- 业务端口：用于应用之间正常请求。
- 监控端口：用于暴露 metrics endpoint。

这次业务端口一直可用，所以客户端调用能成功。但监控端口没有成功监听，导致 Pull 模式无法抓取指标。

因此排障时不能只看“业务是否能调用成功”，还要单独验证 metrics endpoint：

```bash
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:<metrics-port>/prometheus
```

如果这里返回 `000`、连接拒绝，或者超时，就说明应用侧没有在这个端口提供可访问的指标接口。

## 第二层判断：容器里的地址要重新看

在 Windows + WSL + Docker Desktop 场景下，端口路径比纯 Linux 复杂一些：

```text
Docker 容器
  -> host.docker.internal
  -> Windows / Docker Desktop 网络层
  -> WSL 虚拟网络
  -> 应用进程监听端口
```

所以需要分别验证两件事：

- WSL 内部访问 `127.0.0.1:<port>` 是否成功。
- Docker 容器或监控系统访问 `host.docker.internal:<port>` 是否成功。

如果 WSL 内部都访问不到，就不要先怀疑监控配置，应该先回到应用进程本身：端口有没有监听、监听的是不是预期地址、启动日志有没有 bind 失败。

常用检查命令：

```bash
ss -ltnp
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:<port>/prometheus
```

如果监控系统提供 API，也可以直接看 target 状态：

```bash
curl -s http://127.0.0.1:<monitor-port>/api/v1/targets
```

## 第三层判断：端口看似没人占用，也可能不可用

这次比较容易误判的一点是：应用日志里出现过类似下面的错误：

```text
listen tcp :<port>: bind: address already in use
```

但随后在 WSL 里用 `ss -ltnp` 看，并没有发现这个端口正在监听。

在普通 Linux 环境里，这通常会让人怀疑旧进程已经退出、端口状态短暂残留，或者检查时机不对。但在 Windows + WSL 场景中，还要多考虑一层：端口可能被 Windows、Docker Desktop、端口转发、保留端口区间或其他宿主机侧组件影响，而这些状态不一定能完整体现在 WSL 内部的 `ss` 输出里。

也就是说，`ss` 没看到监听进程，只能说明“WSL 当前命名空间里没有进程监听它”，不能直接证明这个端口对整个 Windows + WSL 链路一定可用。

在 Windows PowerShell 里可以进一步检查：

```powershell
netstat -ano | findstr :<port>
netsh interface ipv4 show excludedportrange protocol=tcp
```

这次在 Windows 上执行 `netsh int ipv4 show excludedportrange protocol=tcp` 后，可以看到这样的排除范围：

```text
协议 tcp 端口排除范围

开始端口    结束端口
----------    --------
      2126        2225
      4217        4316
      5354        5453
      8915        9014
      9081        9180
      9498        9597
     11751       11850
     50000       50059     *

* - 管理的端口排除。
```

这些区间里的端口可以直接视为当前 Windows 环境下不适合作为应用监听端口。比如目标 metrics 端口如果落在 `8915-9014`、`9081-9180` 或其他排除范围内，即使 `ss`、`netstat` 没看到明确的用户进程占用，也可能在绑定时失败。

所以排查时不只要问“这个端口有没有进程占用”，还要问“这个端口是不是被 Windows 排除或保留了”。如果端口处在系统保留范围、被端口代理占用，或者被其他 Windows 进程短暂占用过，应用在 WSL 里绑定时仍然可能失败。

## 验证方式：换一组高位端口

为了验证是不是端口本身的问题，这次没有急着改业务逻辑，而是把 metrics endpoint 换到一组更高、更少冲突的端口。

调整点包括：

- 应用服务端 metrics 端口：从默认端口改为高位端口。
- 应用客户端 metrics 端口：从默认端口改为高位端口。
- 监控系统 Pull 配置：同步把 target 改成新的 `host.docker.internal:<port>`。

然后重启监控组件和应用进程，重新验证：

```bash
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:<server-metrics-port>/prometheus
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:<client-metrics-port>/prometheus
```

结果两个 endpoint 都返回 `200`。等待一个抓取周期后，监控系统里的两个 target 也都变成 `UP`。

这说明原端口在当前 Windows + WSL 环境中不可用或不稳定；换端口后，应用指标暴露和容器侧抓取都恢复正常。

## 这次排障的结论

这次失败不是因为旧业务进程还在占用端口，也不是因为业务服务没有启动。更准确的结论是：

- 业务调用正常，只能证明业务链路正常。
- Pull 模式依赖独立的 metrics endpoint，必须单独验证。
- 原 metrics 端口在当前 Windows + WSL 环境下绑定失败。
- WSL 内部 `ss` 没看到端口监听，并不能完全排除宿主机侧端口影响。
- 更换到高位端口后，endpoint 返回 `200`，监控 target 恢复 `UP`，说明端口问题是主要原因。

如果同一套示例在 Linux 服务器上正常，但在 Windows + WSL 上失败，优先不要把问题归因到业务代码。先把端口路径拆开验证，通常会更快。

## 通用排查清单

以后遇到类似问题，可以按这个顺序检查：

1. 确认业务调用是否正常，避免把业务失败和监控失败混在一起。
2. 在 WSL 内部直接访问 metrics endpoint，看是否返回 `200`。
3. 用 `ss -ltnp` 确认应用是否真的监听了目标端口。
4. 查看应用启动日志里是否有 `bind: address already in use`。
5. 在监控系统里查看 target 的 `lastError` 和 `health`。
6. 确认容器里的 target 地址是否应该使用 `host.docker.internal`。
7. 在 Windows PowerShell 里检查端口占用和保留端口范围。
8. 换一组高位端口做最小验证，确认是否为端口冲突或端口保留问题。

这套顺序的好处是，它先判断“端口是否真的起来”，再判断“容器是否能访问”，最后才进入代码或框架配置层面。对于 Windows + WSL 开发环境，这通常比直接改业务代码更稳。

## 小结

Windows + WSL + Docker Desktop 的本地开发环境里，端口问题经常横跨多个网络边界。一个端口在应用配置里写了，不代表它已经监听；一个端口在 WSL 里看不到占用，也不代表它没有受到宿主机侧影响。

这次测试最后用换端口的方式确认了问题：业务链路没有坏，监控 Pull 模式失败的关键在于 metrics endpoint 没有成功暴露。把默认端口换成高位端口，并同步更新监控 target 后，endpoint 和 target 都恢复正常。

遇到类似问题时，最有效的思路是把链路拆成三段：应用有没有监听、WSL 内部能不能访问、容器能不能通过 `host.docker.internal` 抓到。三段都通了，再去看指标内容；其中任何一段不通，先处理端口和网络边界。
